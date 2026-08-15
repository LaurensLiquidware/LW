package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"profileunity-msp-console/internal/auth"
)

// withUser returns ctx carrying userID the same way RequireSession does,
// so a handler under test sees UserIDFromContext exactly as it would in
// a real authenticated request -- without going through a real session
// cookie/token.
func withUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userContextKey{}, userID)
}

func usersPostRequest(ctx context.Context, body any) *http.Request {
	b, _ := json.Marshal(body)
	return httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(b)).WithContext(ctx)
}

func TestListUsersHandler_ReturnsAllOrderedByUsername(t *testing.T) {
	deps := newTestDeps(t)
	ctx := context.Background()
	if _, err := deps.auth.Users.CreateUser(ctx, "zack", "correct-horse-battery-staple", auth.RoleOperator); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.auth.Users.CreateUser(ctx, "amy", "correct-horse-battery-staple", auth.RoleOperator); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	ListUsersHandler(deps.auth)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var dtos []userDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dtos); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(dtos) != 2 {
		t.Fatalf("len(dtos) = %d, want 2", len(dtos))
	}
	if dtos[0].Username != "amy" || dtos[1].Username != "zack" {
		t.Errorf("order = [%q, %q], want [amy, zack]", dtos[0].Username, dtos[1].Username)
	}
	if dtos[0].Role != "operator" {
		t.Errorf("Role = %q, want operator", dtos[0].Role)
	}
}

func TestCreateUserHandler_CreatesAsOperator(t *testing.T) {
	deps := newTestDeps(t)

	req := usersPostRequest(context.Background(), userWriteRequest{Username: "jane", Password: "correct-horse-battery-staple"})
	rec := httptest.NewRecorder()
	CreateUserHandler(deps.auth)(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}
	var dto userDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if dto.Username != "jane" || dto.Role != "operator" {
		t.Errorf("got username=%q role=%q, want jane/operator", dto.Username, dto.Role)
	}

	if _, err := deps.auth.Users.Authenticate(context.Background(), "jane", "correct-horse-battery-staple"); err != nil {
		t.Errorf("Authenticate as the newly created user: %v", err)
	}
}

func TestCreateUserHandler_RejectsDuplicateUsername(t *testing.T) {
	deps := newTestDeps(t)
	ctx := context.Background()
	if _, err := deps.auth.Users.CreateUser(ctx, "jane", "correct-horse-battery-staple", auth.RoleOperator); err != nil {
		t.Fatal(err)
	}

	req := usersPostRequest(ctx, userWriteRequest{Username: "jane", Password: "another-long-password"})
	rec := httptest.NewRecorder()
	CreateUserHandler(deps.auth)(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 for a duplicate username", rec.Code)
	}
}

func TestCreateUserHandler_RejectsShortPassword(t *testing.T) {
	deps := newTestDeps(t)

	req := usersPostRequest(context.Background(), userWriteRequest{Username: "jane", Password: "short"})
	rec := httptest.NewRecorder()
	CreateUserHandler(deps.auth)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a too-short password", rec.Code)
	}
}

func TestDeleteUserHandler_DeletesAndRevokesSessions(t *testing.T) {
	deps := newTestDeps(t)
	ctx := context.Background()

	self, err := deps.auth.Users.CreateUser(ctx, "admin", "correct-horse-battery-staple", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	target, err := deps.auth.Users.CreateUser(ctx, "jane", "correct-horse-battery-staple", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	targetToken, err := deps.auth.Sessions.Create(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+target.ID, nil).WithContext(withUser(ctx, self.ID))
	req.SetPathValue("id", target.ID)
	rec := httptest.NewRecorder()
	DeleteUserHandler(deps.auth)(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body: %s", rec.Code, rec.Body.String())
	}
	if _, err := deps.auth.Users.GetByID(ctx, target.ID); err != auth.ErrUserNotFound {
		t.Errorf("GetByID after delete: err = %v, want ErrUserNotFound", err)
	}
	if _, err := deps.auth.Sessions.Validate(ctx, targetToken); err != auth.ErrSessionInvalid {
		t.Errorf("the deleted user's session should be invalidated, Validate err = %v, want ErrSessionInvalid", err)
	}
}

func TestDeleteUserHandler_RejectsDeletingYourself(t *testing.T) {
	deps := newTestDeps(t)
	ctx := context.Background()

	self, err := deps.auth.Users.CreateUser(ctx, "admin", "correct-horse-battery-staple", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	// A second account so this isn't also rejected as "the last account".
	if _, err := deps.auth.Users.CreateUser(ctx, "jane", "correct-horse-battery-staple", auth.RoleOperator); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+self.ID, nil).WithContext(withUser(ctx, self.ID))
	req.SetPathValue("id", self.ID)
	rec := httptest.NewRecorder()
	DeleteUserHandler(deps.auth)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when deleting your own account", rec.Code)
	}
	if _, err := deps.auth.Users.GetByID(ctx, self.ID); err != nil {
		t.Errorf("self account should still exist, GetByID err = %v", err)
	}
}

func TestDeleteUserHandler_RejectsDeletingTheLastAccount(t *testing.T) {
	deps := newTestDeps(t)
	ctx := context.Background()

	only, err := deps.auth.Users.CreateUser(ctx, "admin", "correct-horse-battery-staple", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	otherSessionOwner := "some-other-caller-not-the-target" // simulates a different logged-in operator

	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+only.ID, nil).WithContext(withUser(ctx, otherSessionOwner))
	req.SetPathValue("id", only.ID)
	rec := httptest.NewRecorder()
	DeleteUserHandler(deps.auth)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when deleting the last remaining account", rec.Code)
	}
	if _, err := deps.auth.Users.GetByID(ctx, only.ID); err != nil {
		t.Errorf("the last account should still exist, GetByID err = %v", err)
	}
}
