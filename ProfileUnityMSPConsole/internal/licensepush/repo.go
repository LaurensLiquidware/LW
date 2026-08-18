package licensepush

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const isoUTC = "2006-01-02T15:04:05Z"

// Repo stores license push history. Rows are never updated or deleted —
// an append-only audit trail (project brief for this feature: "what
// replaced what, when, by whom", which the License Server itself does
// not retain).
type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Create records one push attempt. If r.ID is empty, one is generated.
func (r *Repo) Create(ctx context.Context, rec Record) (Record, error) {
	if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	if rec.PushedAtUTC.IsZero() {
		rec.PushedAtUTC = time.Now().UTC()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO license_pushes (
			id, tenant_id, pushed_at_utc, operator_username, outcome, error_message, license_code_base64,
			organization, contact_name, contact_email, valid_until, license_type, max_users, is_machine, is_concurrent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.TenantID, rec.PushedAtUTC.Format(isoUTC), rec.OperatorUsername, string(rec.Outcome), rec.ErrorMessage, rec.LicenseCodeBase64,
		rec.Organization, rec.ContactName, rec.ContactEmail, rec.ValidUntil, rec.LicenseType, nullableInt(rec.MaxUsers), boolToInt(rec.IsMachine), boolToInt(rec.IsConcurrent))
	if err != nil {
		return Record{}, fmt.Errorf("licensepush: insert: %w", err)
	}
	return rec, nil
}

const selectColumns = `SELECT id, tenant_id, pushed_at_utc, operator_username, outcome, error_message, license_code_base64,
	organization, contact_name, contact_email, valid_until, license_type, max_users, is_machine, is_concurrent`

// ListForTenant returns tenantID's push history, newest first.
func (r *Repo) ListForTenant(ctx context.Context, tenantID string) ([]Record, error) {
	rows, err := r.db.QueryContext(ctx, selectColumns+` FROM license_pushes WHERE tenant_id = ? ORDER BY pushed_at_utc DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("licensepush: list: %w", err)
	}
	defer rows.Close()

	var result []Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("licensepush: scan: %w", err)
		}
		result = append(result, rec)
	}
	return result, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(row rowScanner) (Record, error) {
	var rec Record
	var outcome string
	var pushedAt string
	var maxUsers sql.NullInt64
	var isMachine, isConcurrent int

	if err := row.Scan(&rec.ID, &rec.TenantID, &pushedAt, &rec.OperatorUsername, &outcome, &rec.ErrorMessage, &rec.LicenseCodeBase64,
		&rec.Organization, &rec.ContactName, &rec.ContactEmail, &rec.ValidUntil, &rec.LicenseType, &maxUsers, &isMachine, &isConcurrent); err != nil {
		return Record{}, err
	}

	rec.Outcome = Outcome(outcome)
	rec.IsMachine = isMachine != 0
	rec.IsConcurrent = isConcurrent != 0
	if maxUsers.Valid {
		v := int(maxUsers.Int64)
		rec.MaxUsers = &v
	}

	var err error
	rec.PushedAtUTC, err = time.Parse(isoUTC, pushedAt)
	if err != nil {
		return Record{}, fmt.Errorf("parse pushed_at_utc: %w", err)
	}
	return rec, nil
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
