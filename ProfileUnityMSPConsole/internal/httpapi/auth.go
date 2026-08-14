package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"profileunity-msp-console/internal/auth"
)

// SessionCookieName is the HttpOnly cookie carrying the session token.
const SessionCookieName = "pumc_session"

type userContextKey struct{}

// AuthDeps bundles what the auth handlers and middleware need. Secure
// controls the cookies' Secure flag — true whenever the server is
// actually serving over TLS (project brief §9's self-signed-cert setup),
// false only for local HTTP development.
type AuthDeps struct {
	Users    *auth.UserRepo
	Sessions *auth.SessionRepo
	Secure   bool
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type userResponse struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// CSRFHandler issues a fresh CSRF cookie and returns its value, so the
// frontend can echo it back in a header on every mutating request
// (project brief §6's carried-over pattern: a cookie alone is not enough).
func CSRFHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.NewCSRFToken()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		auth.SetCSRFCookie(w, token, deps.Secure)
		writeJSON(w, http.StatusOK, map[string]string{"csrfToken": token})
	}
}

// LoginHandler authenticates a username/password and, on success, starts
// a session. Errors are deliberately generic ("invalid username or
// password") regardless of whether the username exists.
func LoginHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		user, err := deps.Users.Authenticate(r.Context(), req.Username, req.Password)
		if err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				http.Error(w, "invalid username or password", http.StatusUnauthorized)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		token, err := deps.Sessions.Create(r.Context(), user.ID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   deps.Secure,
			SameSite: http.SameSiteStrictMode,
		})

		writeJSON(w, http.StatusOK, userResponse{Username: user.Username, Role: string(user.Role)})
	}
}

// LogoutHandler revokes the current session and clears its cookie.
func LogoutHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(SessionCookieName); err == nil {
			_ = deps.Sessions.Revoke(r.Context(), cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   deps.Secure,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// MeHandler returns the current session's user. RequireSession must run
// first — it is what actually rejects an invalid/missing session.
func MeHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
		user, err := deps.Users.GetByID(r.Context(), userID)
		if err != nil {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, userResponse{Username: user.Username, Role: string(user.Role)})
	}
}

// ChangePasswordHandler lets the signed-in user replace their own
// password after proving they know the current one. RequireSession must
// run first. Unlike LoginHandler, the error here is specific (wrong
// current password vs. a too-short new one) since the caller is already
// authenticated -- there's no username-enumeration risk to hide behind a
// generic message.
func ChangePasswordHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		err := deps.Users.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword)
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, auth.ErrCurrentPasswordIncorrect):
			http.Error(w, "current password is incorrect", http.StatusForbidden)
		case errors.Is(err, auth.ErrUserNotFound):
			http.Error(w, "not authenticated", http.StatusUnauthorized)
		default:
			http.Error(w, "new password must be at least 12 characters", http.StatusBadRequest)
		}
	}
}

// RequireSession wraps handler, rejecting the request with 401 unless it
// carries a valid, non-expired, non-idled-out session cookie. On success
// it stores the session's user ID in the request context for downstream
// handlers (see UserIDFromContext) and touches last-seen (handled inside
// SessionRepo.Validate), which is what makes the idle timeout a rolling
// window rather than a fixed one.
func RequireSession(sessions *auth.SessionRepo, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
		session, err := sessions.Validate(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey{}, session.UserID)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext retrieves the user ID RequireSession stored.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userContextKey{}).(string)
	return id, ok
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
