package tenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"profileunity-msp-console/internal/crypto"
)

// ErrNotFound is returned when a tenant ID does not exist.
var ErrNotFound = errors.New("tenant: not found")

// ErrCredentialsMismatch is returned when exactly one of Username/Password
// is set — a stored credential that can never authenticate is a mistake,
// not a valid configuration.
var ErrCredentialsMismatch = errors.New("tenant: username and password must be both set or both empty")

// ErrEncryptionKeyRequired is returned when a password is supplied but no
// credential encryption key was configured — storing it in plaintext is
// never an acceptable fallback.
var ErrEncryptionKeyRequired = errors.New("tenant: a credential encryption key must be configured to store a password")

const isoUTC = "2006-01-02T15:04:05Z"

// Repo stores tenants in the database, encrypting/decrypting credentials
// with encryptionKey (project brief §9). encryptionKey may be nil; in
// that case any attempt to store a password fails with
// ErrEncryptionKeyRequired rather than writing plaintext.
type Repo struct {
	db            *sql.DB
	encryptionKey []byte
}

// NewRepo creates a Repo. encryptionKey is typically config.CredentialEncryptionKey.
func NewRepo(db *sql.DB, encryptionKey []byte) *Repo {
	return &Repo{db: db, encryptionKey: encryptionKey}
}

func (r *Repo) Create(ctx context.Context, in CreateInput) (Tenant, error) {
	if (in.Username == "") != (in.Password == "") {
		return Tenant{}, ErrCredentialsMismatch
	}

	var encPassword []byte
	if in.Password != "" {
		if len(r.encryptionKey) == 0 {
			return Tenant{}, ErrEncryptionKeyRequired
		}
		blob, err := crypto.Encrypt(r.encryptionKey, in.Password)
		if err != nil {
			return Tenant{}, fmt.Errorf("tenant: encrypt password: %w", err)
		}
		encPassword = blob
	}

	now := time.Now().UTC()
	t := Tenant{
		ID:            uuid.NewString(),
		DisplayName:   in.DisplayName,
		Hostname:      in.Hostname,
		Port:          in.Port,
		Username:      in.Username,
		HasPassword:   in.Password != "",
		TLSSkipVerify: in.TLSSkipVerify,
		Enabled:       in.Enabled,
		Tags:          in.Tags,
		Notes:         in.Notes,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tenants (id, display_name, hostname, port, username, encrypted_password, tls_skip_verify, enabled, tags, notes, created_at_utc, updated_at_utc)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.DisplayName, t.Hostname, t.Port, t.Username, nullableBytes(encPassword),
		boolToInt(t.TLSSkipVerify), boolToInt(t.Enabled), joinTags(t.Tags), t.Notes,
		now.Format(isoUTC), now.Format(isoUTC))
	if err != nil {
		return Tenant{}, fmt.Errorf("tenant: insert: %w", err)
	}
	return t, nil
}

func (r *Repo) Update(ctx context.Context, id string, in UpdateInput) (Tenant, error) {
	existing, err := r.Get(ctx, id)
	if err != nil {
		return Tenant{}, err
	}

	// keepPassword: leave the encrypted_password column untouched.
	// clearPassword: set it to NULL.
	// Otherwise, encPassword holds a freshly-encrypted replacement.
	var encPassword []byte
	keepPassword := false
	clearPassword := false

	switch {
	case in.Username == "":
		clearPassword = true
	case in.Password == nil:
		if !existing.HasPassword {
			return Tenant{}, ErrCredentialsMismatch
		}
		keepPassword = true
	case *in.Password == "":
		return Tenant{}, ErrCredentialsMismatch
	default:
		if len(r.encryptionKey) == 0 {
			return Tenant{}, ErrEncryptionKeyRequired
		}
		blob, err := crypto.Encrypt(r.encryptionKey, *in.Password)
		if err != nil {
			return Tenant{}, fmt.Errorf("tenant: encrypt password: %w", err)
		}
		encPassword = blob
	}

	now := time.Now().UTC()

	if keepPassword {
		_, err = r.db.ExecContext(ctx, `
			UPDATE tenants SET display_name=?, hostname=?, port=?, username=?, tls_skip_verify=?, enabled=?, tags=?, notes=?, updated_at_utc=?
			WHERE id=?`,
			in.DisplayName, in.Hostname, in.Port, in.Username, boolToInt(in.TLSSkipVerify), boolToInt(in.Enabled), joinTags(in.Tags), in.Notes, now.Format(isoUTC), id)
	} else {
		var passwordArg any
		if !clearPassword {
			passwordArg = encPassword
		}
		_, err = r.db.ExecContext(ctx, `
			UPDATE tenants SET display_name=?, hostname=?, port=?, username=?, encrypted_password=?, tls_skip_verify=?, enabled=?, tags=?, notes=?, updated_at_utc=?
			WHERE id=?`,
			in.DisplayName, in.Hostname, in.Port, in.Username, passwordArg, boolToInt(in.TLSSkipVerify), boolToInt(in.Enabled), joinTags(in.Tags), in.Notes, now.Format(isoUTC), id)
	}
	if err != nil {
		return Tenant{}, fmt.Errorf("tenant: update: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *Repo) Get(ctx context.Context, id string) (Tenant, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, display_name, hostname, port, username, encrypted_password, tls_skip_verify, enabled, tags, notes, created_at_utc, updated_at_utc,
			license_server_hostname, license_server_port, license_server_username, license_server_encrypted_password, license_server_tls_skip_verify
		FROM tenants WHERE id = ?`, id)
	t, err := scanTenant(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	if err != nil {
		return Tenant{}, fmt.Errorf("tenant: get: %w", err)
	}
	return t, nil
}

func (r *Repo) List(ctx context.Context) ([]Tenant, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, display_name, hostname, port, username, encrypted_password, tls_skip_verify, enabled, tags, notes, created_at_utc, updated_at_utc,
			license_server_hostname, license_server_port, license_server_username, license_server_encrypted_password, license_server_tls_skip_verify
		FROM tenants ORDER BY display_name`)
	if err != nil {
		return nil, fmt.Errorf("tenant: list: %w", err)
	}
	defer rows.Close()

	var result []Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, fmt.Errorf("tenant: scan: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *Repo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("tenant: delete: %w", err)
	}
	return nil
}

// GetCredentials decrypts and returns the stored credential for id. It
// returns (nil, nil) when the tenant has no username/password configured
// — that is a valid state, not an error. This is the only path in the
// package that ever produces a plaintext password; callers must not log
// or persist the result.
func (r *Repo) GetCredentials(ctx context.Context, id string) (*Credentials, error) {
	var username string
	var encPassword []byte
	row := r.db.QueryRowContext(ctx, `SELECT username, encrypted_password FROM tenants WHERE id = ?`, id)
	if err := row.Scan(&username, &encPassword); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("tenant: get credentials: %w", err)
	}
	if username == "" || len(encPassword) == 0 {
		return nil, nil
	}
	if len(r.encryptionKey) == 0 {
		return nil, ErrEncryptionKeyRequired
	}
	password, err := crypto.Decrypt(r.encryptionKey, encPassword)
	if err != nil {
		return nil, fmt.Errorf("tenant: decrypt password: %w", err)
	}
	return &Credentials{Username: username, Password: password}, nil
}

// UpdateLicenseServer overwrites only the stored License Server
// connection for id, leaving every other tenant field untouched --
// mirrors internal/settings' UpdateTLSCert (a narrow, dedicated update
// path separate from the main tenant Create/Update). password uses the
// same three-way pointer semantics as CreateInput/UpdateInput's
// Password: nil leaves the stored password untouched, a pointer to ""
// clears it, and a pointer to a non-empty string replaces it.
func (r *Repo) UpdateLicenseServer(ctx context.Context, id, hostname string, port int, username string, password *string, tlsSkipVerify bool) error {
	existing, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	var encPassword []byte
	keepPassword := false
	clearPassword := false

	switch {
	case username == "":
		clearPassword = true
	case password == nil:
		if !existing.LicenseServerHasPassword {
			return ErrCredentialsMismatch
		}
		keepPassword = true
	case *password == "":
		return ErrCredentialsMismatch
	default:
		if len(r.encryptionKey) == 0 {
			return ErrEncryptionKeyRequired
		}
		blob, err := crypto.Encrypt(r.encryptionKey, *password)
		if err != nil {
			return fmt.Errorf("tenant: encrypt license server password: %w", err)
		}
		encPassword = blob
	}

	now := time.Now().UTC()
	if keepPassword {
		_, err = r.db.ExecContext(ctx, `
			UPDATE tenants SET license_server_hostname=?, license_server_port=?, license_server_username=?, license_server_tls_skip_verify=?, updated_at_utc=?
			WHERE id=?`,
			hostname, port, username, boolToInt(tlsSkipVerify), now.Format(isoUTC), id)
	} else {
		var passwordArg any
		if !clearPassword {
			passwordArg = encPassword
		}
		_, err = r.db.ExecContext(ctx, `
			UPDATE tenants SET license_server_hostname=?, license_server_port=?, license_server_username=?, license_server_encrypted_password=?, license_server_tls_skip_verify=?, updated_at_utc=?
			WHERE id=?`,
			hostname, port, username, passwordArg, boolToInt(tlsSkipVerify), now.Format(isoUTC), id)
	}
	if err != nil {
		return fmt.Errorf("tenant: update license server: %w", err)
	}
	return nil
}

// GetLicenseServerCredentials decrypts and returns the stored License
// Server credential for id. Returns (nil, nil) when none is configured
// -- a valid state, not an error. Mirrors GetCredentials.
func (r *Repo) GetLicenseServerCredentials(ctx context.Context, id string) (*Credentials, error) {
	var username string
	var encPassword []byte
	row := r.db.QueryRowContext(ctx, `SELECT license_server_username, license_server_encrypted_password FROM tenants WHERE id = ?`, id)
	if err := row.Scan(&username, &encPassword); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("tenant: get license server credentials: %w", err)
	}
	if username == "" || len(encPassword) == 0 {
		return nil, nil
	}
	if len(r.encryptionKey) == 0 {
		return nil, ErrEncryptionKeyRequired
	}
	password, err := crypto.Decrypt(r.encryptionKey, encPassword)
	if err != nil {
		return nil, fmt.Errorf("tenant: decrypt license server password: %w", err)
	}
	return &Credentials{Username: username, Password: password}, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTenant(row rowScanner) (Tenant, error) {
	var t Tenant
	var encPassword, licenseServerEncPassword []byte
	var tlsSkipVerify, enabled, licenseServerTLSSkipVerify int
	var tags string
	var createdAt, updatedAt string

	if err := row.Scan(&t.ID, &t.DisplayName, &t.Hostname, &t.Port, &t.Username, &encPassword,
		&tlsSkipVerify, &enabled, &tags, &t.Notes, &createdAt, &updatedAt,
		&t.LicenseServerHostname, &t.LicenseServerPort, &t.LicenseServerUsername, &licenseServerEncPassword, &licenseServerTLSSkipVerify); err != nil {
		return Tenant{}, err
	}

	t.HasPassword = len(encPassword) > 0
	t.TLSSkipVerify = tlsSkipVerify != 0
	t.Enabled = enabled != 0
	t.Tags = splitTags(tags)
	t.LicenseServerHasPassword = len(licenseServerEncPassword) > 0
	t.LicenseServerTLSSkipVerify = licenseServerTLSSkipVerify != 0

	var err error
	t.CreatedAt, err = time.Parse(isoUTC, createdAt)
	if err != nil {
		return Tenant{}, fmt.Errorf("parse created_at_utc: %w", err)
	}
	t.UpdatedAt, err = time.Parse(isoUTC, updatedAt)
	if err != nil {
		return Tenant{}, fmt.Errorf("parse updated_at_utc: %w", err)
	}
	return t, nil
}

func joinTags(tags []string) string {
	return strings.Join(tags, ",")
}

func splitTags(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
