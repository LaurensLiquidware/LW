package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"profileunity-msp-console/internal/auth"
)

// userDTO is the account shape the Users screen sees -- never a password
// hash, same discipline as tenantDTO never carrying a tenant's password.
type userDTO struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	CreatedAtUTC string `json:"createdAtUtc"`
	UpdatedAtUTC string `json:"updatedAtUtc"`
}

func toUserDTO(u auth.User) userDTO {
	return userDTO{
		ID:           u.ID,
		Username:     u.Username,
		Role:         string(u.Role),
		CreatedAtUTC: u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAtUTC: u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// userWriteRequest is the POST /api/users request body. There is no
// role field -- every account created from this screen is a plain
// operator (auth.RoleOperator), matching how every existing account
// already behaves, since nothing in this app enforces a distinction
// between operator and viewer today.
type userWriteRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ListUsersHandler lists every console login account.
func ListUsersHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := deps.Users.List(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		dtos := make([]userDTO, 0, len(users))
		for _, u := range users {
			dtos = append(dtos, toUserDTO(u))
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

// CreateUserHandler adds a new operator login account.
func CreateUserHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req userWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		u, err := deps.Users.CreateUser(r.Context(), req.Username, req.Password, auth.RoleOperator)
		if err != nil {
			if errors.Is(err, auth.ErrUsernameTaken) {
				http.Error(w, "that username is already taken", http.StatusConflict)
				return
			}
			// The only other errors CreateUser returns are plain
			// validation failures (empty username, password too short)
			// -- both are the caller's mistake to fix, not a server fault.
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, toUserDTO(u))
	}
}

// DeleteUserHandler removes a login account. Two things this API must
// refuse regardless of who's asking: deleting the account making the
// request, and deleting the last remaining account -- either would
// leave the console impossible to sign into, and there is no separate
// admin-reset path to recover from that.
func DeleteUserHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if selfID, ok := UserIDFromContext(r.Context()); ok && selfID == id {
			http.Error(w, "you cannot delete your own account", http.StatusBadRequest)
			return
		}

		count, err := deps.Users.Count(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if count <= 1 {
			http.Error(w, "cannot delete the last remaining account", http.StatusBadRequest)
			return
		}

		if err := deps.Users.Delete(r.Context(), id); err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// The account is gone; make sure any session it's still holding
		// stops working immediately rather than lingering until it
		// naturally expires (see RevokeAllForUser's own comment for why
		// this can't be left to the database's ON DELETE CASCADE alone).
		// The user row is already deleted at this point, so there's
		// nothing to retry -- only log loudly if this somehow fails.
		if err := deps.Sessions.RevokeAllForUser(r.Context(), id); err != nil {
			slog.Error(fmt.Sprintf("httpapi: deleted user %s but failed to revoke its sessions: %v", id, err))
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
