package snapshot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when no snapshot exists for a given tenant/date.
var ErrNotFound = errors.New("snapshot: not found")

const isoUTC = "2006-01-02T15:04:05Z"

// Repo stores snapshots.
type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Upsert stores s, keyed on (TenantID, CollectionDate). Re-running
// collection for the same tenant on the same day replaces that day's row
// rather than creating a duplicate — this is what makes collection
// idempotent (project brief §7.2). The ON CONFLICT clause below never
// touches the id column, so an existing row keeps its original ID even
// though s.ID here is a freshly minted one for the INSERT branch.
func (r *Repo) Upsert(ctx context.Context, s Snapshot) (Snapshot, error) {
	if s.CollectedAtUTC.IsZero() {
		return Snapshot{}, fmt.Errorf("snapshot: CollectedAtUTC is required")
	}
	if s.ID == "" {
		s.ID = uuid.NewString()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO snapshots (
			id, tenant_id, collection_date, collected_at_utc, status, auth_path, error_message, raw_payload,
			registered_to, license_mode, license_product, total_licenses, used_licenses, evaluation,
			console_version, is_trial_expired, is_trial, is_pro_u_only, is_flex_only, support_ends_iso
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (tenant_id, collection_date) DO UPDATE SET
			collected_at_utc = excluded.collected_at_utc,
			status = excluded.status,
			auth_path = excluded.auth_path,
			error_message = excluded.error_message,
			raw_payload = excluded.raw_payload,
			registered_to = excluded.registered_to,
			license_mode = excluded.license_mode,
			license_product = excluded.license_product,
			total_licenses = excluded.total_licenses,
			used_licenses = excluded.used_licenses,
			evaluation = excluded.evaluation,
			console_version = excluded.console_version,
			is_trial_expired = excluded.is_trial_expired,
			is_trial = excluded.is_trial,
			is_pro_u_only = excluded.is_pro_u_only,
			is_flex_only = excluded.is_flex_only,
			support_ends_iso = excluded.support_ends_iso`,
		s.ID, s.TenantID, s.CollectionDate, s.CollectedAtUTC.UTC().Format(isoUTC), string(s.Status), s.AuthPath, s.ErrorMessage, s.RawPayload,
		s.RegisteredTo, s.LicenseMode, s.LicenseProduct, nullableInt(s.TotalLicenses), nullableInt(s.UsedLicenses), nullableBool(s.Evaluation),
		s.ConsoleVersion, nullableBool(s.IsTrialExpired), nullableBool(s.IsTrial), nullableBool(s.IsProUOnly), nullableBool(s.IsFlexOnly), s.SupportEndsISO)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: upsert: %w", err)
	}

	// Re-read rather than returning s directly: on an update, the row kept
	// its original ID (see the comment above), which may differ from the
	// s.ID this call minted for the INSERT branch.
	return r.GetByTenantAndDate(ctx, s.TenantID, s.CollectionDate)
}

// GetByTenantAndDate returns the snapshot for tenantID on collectionDate.
func (r *Repo) GetByTenantAndDate(ctx context.Context, tenantID, collectionDate string) (Snapshot, error) {
	row := r.db.QueryRowContext(ctx, selectColumns+` FROM snapshots WHERE tenant_id = ? AND collection_date = ?`, tenantID, collectionDate)
	s, err := scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, ErrNotFound
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: get: %w", err)
	}
	return s, nil
}

// GetLatest returns the most recent collection attempt for tenantID,
// regardless of outcome, or (nil, nil) if this tenant has never been
// collected.
func (r *Repo) GetLatest(ctx context.Context, tenantID string) (*Snapshot, error) {
	row := r.db.QueryRowContext(ctx, selectColumns+` FROM snapshots WHERE tenant_id = ? ORDER BY collection_date DESC LIMIT 1`, tenantID)
	return scanOptional(row)
}

// GetLatestSuccess returns the most recent successful collection for
// tenantID, which may be older than GetLatest's result, or (nil, nil) if
// there has never been a success.
func (r *Repo) GetLatestSuccess(ctx context.Context, tenantID string) (*Snapshot, error) {
	row := r.db.QueryRowContext(ctx, selectColumns+` FROM snapshots WHERE tenant_id = ? AND status = ? ORDER BY collection_date DESC LIMIT 1`, tenantID, string(StatusSuccess))
	return scanOptional(row)
}

// LatestForAllTenants returns each tenant's most recent collection
// attempt, keyed by tenant ID, for tenants that have at least one. This
// is one query rather than one-per-tenant, for the dashboard's sake.
func (r *Repo) LatestForAllTenants(ctx context.Context) (map[string]Snapshot, error) {
	return r.queryLatestMap(ctx, selectColumns+`
		FROM snapshots
		WHERE (tenant_id, collection_date) IN (
			SELECT tenant_id, MAX(collection_date) FROM snapshots GROUP BY tenant_id
		)`)
}

// LatestSuccessForAllTenants is LatestForAllTenants restricted to
// successful collections.
func (r *Repo) LatestSuccessForAllTenants(ctx context.Context) (map[string]Snapshot, error) {
	return r.queryLatestMap(ctx, selectColumns+`
		FROM snapshots
		WHERE status = ? AND (tenant_id, collection_date) IN (
			SELECT tenant_id, MAX(collection_date) FROM snapshots WHERE status = ? GROUP BY tenant_id
		)`, string(StatusSuccess), string(StatusSuccess))
}

func (r *Repo) queryLatestMap(ctx context.Context, query string, args ...any) (map[string]Snapshot, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("snapshot: query latest: %w", err)
	}
	defer rows.Close()

	result := make(map[string]Snapshot)
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("snapshot: scan: %w", err)
		}
		result[s.TenantID] = s
	}
	return result, rows.Err()
}

func scanOptional(row rowScanner) (*Snapshot, error) {
	s, err := scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("snapshot: get: %w", err)
	}
	return &s, nil
}

// ListByTenant returns every snapshot for tenantID, oldest first.
func (r *Repo) ListByTenant(ctx context.Context, tenantID string) ([]Snapshot, error) {
	rows, err := r.db.QueryContext(ctx, selectColumns+` FROM snapshots WHERE tenant_id = ? ORDER BY collection_date`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("snapshot: list: %w", err)
	}
	defer rows.Close()

	var result []Snapshot
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("snapshot: scan: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ListAllSuccess returns every successful snapshot across every tenant,
// ordered by collection date — the raw material for a portfolio-wide
// history view (project brief §7.4's "aggregate view across all tenants"),
// built in one query rather than one per tenant.
func (r *Repo) ListAllSuccess(ctx context.Context) ([]Snapshot, error) {
	rows, err := r.db.QueryContext(ctx, selectColumns+` FROM snapshots WHERE status = ? ORDER BY collection_date`, string(StatusSuccess))
	if err != nil {
		return nil, fmt.Errorf("snapshot: list all success: %w", err)
	}
	defer rows.Close()

	var result []Snapshot
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("snapshot: scan: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ListByTenantInRange returns every snapshot for tenantID with
// collection_date in [from, to] (inclusive, both "YYYY-MM-DD"), regardless
// of status — a monthly report needs the failed/missing days too, to
// report data-collection completeness (project brief §7.5), not just the
// successful ones.
func (r *Repo) ListByTenantInRange(ctx context.Context, tenantID, from, to string) ([]Snapshot, error) {
	rows, err := r.db.QueryContext(ctx, selectColumns+` FROM snapshots WHERE tenant_id = ? AND collection_date >= ? AND collection_date <= ? ORDER BY collection_date`, tenantID, from, to)
	if err != nil {
		return nil, fmt.Errorf("snapshot: list by tenant in range: %w", err)
	}
	defer rows.Close()

	var result []Snapshot
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("snapshot: scan: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ListAllInRange is ListByTenantInRange across every tenant, regardless of
// status — the raw material for a portfolio-wide monthly report.
func (r *Repo) ListAllInRange(ctx context.Context, from, to string) ([]Snapshot, error) {
	rows, err := r.db.QueryContext(ctx, selectColumns+` FROM snapshots WHERE collection_date >= ? AND collection_date <= ? ORDER BY collection_date`, from, to)
	if err != nil {
		return nil, fmt.Errorf("snapshot: list all in range: %w", err)
	}
	defer rows.Close()

	var result []Snapshot
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("snapshot: scan: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

const selectColumns = `
	SELECT id, tenant_id, collection_date, collected_at_utc, status, auth_path, error_message, raw_payload,
		registered_to, license_mode, license_product, total_licenses, used_licenses, evaluation,
		console_version, is_trial_expired, is_trial, is_pro_u_only, is_flex_only, support_ends_iso`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(row rowScanner) (Snapshot, error) {
	var s Snapshot
	var status, collectedAt string
	var totalLicenses, usedLicenses sql.NullInt64
	var evaluation, isTrialExpired, isTrial, isProUOnly, isFlexOnly sql.NullInt64

	if err := row.Scan(&s.ID, &s.TenantID, &s.CollectionDate, &collectedAt, &status, &s.AuthPath, &s.ErrorMessage, &s.RawPayload,
		&s.RegisteredTo, &s.LicenseMode, &s.LicenseProduct, &totalLicenses, &usedLicenses, &evaluation,
		&s.ConsoleVersion, &isTrialExpired, &isTrial, &isProUOnly, &isFlexOnly, &s.SupportEndsISO); err != nil {
		return Snapshot{}, err
	}

	s.Status = Status(status)
	var err error
	s.CollectedAtUTC, err = time.Parse(isoUTC, collectedAt)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse collected_at_utc: %w", err)
	}
	s.TotalLicenses = fromNullInt(totalLicenses)
	s.UsedLicenses = fromNullInt(usedLicenses)
	s.Evaluation = fromNullBool(evaluation)
	s.IsTrialExpired = fromNullBool(isTrialExpired)
	s.IsTrial = fromNullBool(isTrial)
	s.IsProUOnly = fromNullBool(isProUOnly)
	s.IsFlexOnly = fromNullBool(isFlexOnly)
	return s, nil
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableBool(v *bool) any {
	if v == nil {
		return nil
	}
	if *v {
		return 1
	}
	return 0
}

func fromNullInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func fromNullBool(n sql.NullInt64) *bool {
	if !n.Valid {
		return nil
	}
	v := n.Int64 != 0
	return &v
}
