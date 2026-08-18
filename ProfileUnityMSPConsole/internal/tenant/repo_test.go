package tenant

import (
	"context"
	"path/filepath"
	"testing"

	"profileunity-msp-console/internal/db"
)

func newTestRepo(t *testing.T, encryptionKey []byte) *Repo {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewRepo(sqlDB, encryptionKey)
}

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestRepo_CreateGetList(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	created, err := repo.Create(ctx, CreateInput{
		DisplayName: "Acme Corp",
		Hostname:    "acme.example.com",
		Port:        8000,
		Username:    "admin",
		Password:    "s3cr3t",
		Enabled:     true,
		Tags:        []string{"east-region", "tier-1"},
		Notes:       "primary contact: jane",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected a generated ID")
	}
	if !created.HasPassword {
		t.Error("HasPassword should be true")
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DisplayName != "Acme Corp" || got.Hostname != "acme.example.com" || got.Port != 8000 {
		t.Errorf("got %+v", got)
	}
	if len(got.Tags) != 2 {
		t.Errorf("Tags = %v", got.Tags)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d tenants, want 1", len(list))
	}
}

func TestRepo_Create_WithoutCredentials(t *testing.T) {
	repo := newTestRepo(t, nil)
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{DisplayName: "No Creds", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tn.HasPassword {
		t.Error("HasPassword should be false")
	}

	creds, err := repo.GetCredentials(ctx, tn.ID)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if creds != nil {
		t.Errorf("expected nil credentials, got %+v", creds)
	}
}

func TestRepo_Create_RejectsMismatchedCredentials(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	_, err := repo.Create(ctx, CreateInput{DisplayName: "x", Hostname: "h", Port: 8000, Username: "admin"})
	if err != ErrCredentialsMismatch {
		t.Errorf("err = %v, want ErrCredentialsMismatch", err)
	}
}

func TestRepo_Create_RequiresEncryptionKeyForPassword(t *testing.T) {
	repo := newTestRepo(t, nil)
	ctx := context.Background()

	_, err := repo.Create(ctx, CreateInput{DisplayName: "x", Hostname: "h", Port: 8000, Username: "admin", Password: "p"})
	if err != ErrEncryptionKeyRequired {
		t.Errorf("err = %v, want ErrEncryptionKeyRequired", err)
	}
}

func TestRepo_GetCredentials_RoundTrip(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{
		DisplayName: "Acme", Hostname: "h", Port: 8000,
		Username: "admin", Password: "correct-password&special=chars",
	})
	if err != nil {
		t.Fatal(err)
	}

	creds, err := repo.GetCredentials(ctx, tn.ID)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if creds == nil || creds.Username != "admin" || creds.Password != "correct-password&special=chars" {
		t.Errorf("got %+v", creds)
	}
}

func TestRepo_PasswordNeverStoredInPlaintext(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{
		DisplayName: "Acme", Hostname: "h", Port: 8000,
		Username: "admin", Password: "very-secret-value",
	})
	if err != nil {
		t.Fatal(err)
	}

	var blob []byte
	row := repo.db.QueryRowContext(ctx, `SELECT encrypted_password FROM tenants WHERE id = ?`, tn.ID)
	if err := row.Scan(&blob); err != nil {
		t.Fatal(err)
	}
	if string(blob) == "very-secret-value" {
		t.Fatal("password was stored in plaintext")
	}
	for i := 0; i+len("very-secret-value") <= len(blob); i++ {
		if string(blob[i:i+len("very-secret-value")]) == "very-secret-value" {
			t.Fatal("plaintext password bytes found inside stored blob")
		}
	}
}

func TestRepo_Update_ClearCredentialsByEmptyingUsername(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{DisplayName: "x", Hostname: "h", Port: 8000, Username: "admin", Password: "p", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := repo.Update(ctx, tn.ID, UpdateInput{DisplayName: "x", Hostname: "h", Port: 8000, Username: "", Enabled: true})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.HasPassword {
		t.Error("HasPassword should be false after clearing the username")
	}

	creds, err := repo.GetCredentials(ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if creds != nil {
		t.Errorf("expected nil credentials after clearing, got %+v", creds)
	}
}

func TestRepo_Update_KeepsPasswordWhenNotProvided(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{DisplayName: "x", Hostname: "h", Port: 8000, Username: "admin", Password: "original", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := repo.Update(ctx, tn.ID, UpdateInput{DisplayName: "renamed", Hostname: "h", Port: 8000, Username: "admin", Enabled: true})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.HasPassword || updated.DisplayName != "renamed" {
		t.Errorf("got %+v", updated)
	}

	creds, err := repo.GetCredentials(ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil || creds.Password != "original" {
		t.Errorf("password should be unchanged, got %+v", creds)
	}
}

func TestRepo_Update_ReplacesPassword(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{DisplayName: "x", Hostname: "h", Port: 8000, Username: "admin", Password: "original", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	newPassword := "replacement"
	_, err = repo.Update(ctx, tn.ID, UpdateInput{DisplayName: "x", Hostname: "h", Port: 8000, Username: "admin", Password: &newPassword, Enabled: true})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	creds, err := repo.GetCredentials(ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil || creds.Password != "replacement" {
		t.Errorf("got %+v", creds)
	}
}

func TestRepo_Update_RejectsUsernameWithoutExistingOrNewPassword(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{DisplayName: "x", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.Update(ctx, tn.ID, UpdateInput{DisplayName: "x", Hostname: "h", Port: 8000, Username: "admin", Enabled: true})
	if err != ErrCredentialsMismatch {
		t.Errorf("err = %v, want ErrCredentialsMismatch", err)
	}
}

func TestRepo_Delete(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{DisplayName: "x", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, tn.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, tn.ID); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRepo_Get_NotFound(t *testing.T) {
	repo := newTestRepo(t, testKey())
	if _, err := repo.Get(context.Background(), "nonexistent"); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRepo_UpdateLicenseServer_RoundTrip(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if tn.LicenseServerHostname != "" || tn.LicenseServerHasPassword {
		t.Fatalf("expected no License Server configured on a fresh tenant, got %+v", tn)
	}

	password := "ls-secret"
	if err := repo.UpdateLicenseServer(ctx, tn.ID, "ld-lw01.example.com", 443, "prou_services", &password, true); err != nil {
		t.Fatalf("UpdateLicenseServer: %v", err)
	}

	got, err := repo.Get(ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LicenseServerHostname != "ld-lw01.example.com" || got.LicenseServerPort != 443 ||
		got.LicenseServerUsername != "prou_services" || !got.LicenseServerHasPassword || !got.LicenseServerTLSSkipVerify {
		t.Errorf("got %+v", got)
	}

	creds, err := repo.GetLicenseServerCredentials(ctx, tn.ID)
	if err != nil {
		t.Fatalf("GetLicenseServerCredentials: %v", err)
	}
	if creds == nil || creds.Username != "prou_services" || creds.Password != "ls-secret" {
		t.Errorf("got %+v", creds)
	}

	// The tenant's own console credentials must be unaffected.
	if got.Username != "" || got.HasPassword {
		t.Errorf("UpdateLicenseServer touched the tenant's own credentials: %+v", got)
	}
}

func TestRepo_UpdateLicenseServer_KeepsPasswordWhenNotProvided(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	password := "ls-secret"
	if err := repo.UpdateLicenseServer(ctx, tn.ID, "ld-lw01.example.com", 443, "prou_services", &password, false); err != nil {
		t.Fatal(err)
	}

	// nil password + same username -> keep the stored password.
	if err := repo.UpdateLicenseServer(ctx, tn.ID, "ld-lw02.example.com", 443, "prou_services", nil, true); err != nil {
		t.Fatalf("UpdateLicenseServer (keep password): %v", err)
	}

	got, err := repo.Get(ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LicenseServerHostname != "ld-lw02.example.com" || !got.LicenseServerTLSSkipVerify {
		t.Errorf("got %+v", got)
	}
	creds, err := repo.GetLicenseServerCredentials(ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil || creds.Password != "ls-secret" {
		t.Errorf("password not kept: %+v", creds)
	}
}

func TestRepo_UpdateLicenseServer_ClearsCredentialsByEmptyingUsername(t *testing.T) {
	repo := newTestRepo(t, testKey())
	ctx := context.Background()

	tn, err := repo.Create(ctx, CreateInput{DisplayName: "Acme", Hostname: "h", Port: 8000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	password := "ls-secret"
	if err := repo.UpdateLicenseServer(ctx, tn.ID, "ld-lw01.example.com", 443, "prou_services", &password, false); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateLicenseServer(ctx, tn.ID, "ld-lw01.example.com", 443, "", nil, false); err != nil {
		t.Fatalf("UpdateLicenseServer (clear): %v", err)
	}

	got, err := repo.Get(ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LicenseServerUsername != "" || got.LicenseServerHasPassword {
		t.Errorf("credentials not cleared: %+v", got)
	}
	creds, err := repo.GetLicenseServerCredentials(ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if creds != nil {
		t.Errorf("expected nil credentials after clearing, got %+v", creds)
	}
}
