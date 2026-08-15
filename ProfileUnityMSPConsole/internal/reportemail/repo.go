// Package reportemail tracks which monthly portfolio reports have
// already been emailed, so the report-mail scheduler can tell "already
// sent" apart from "not sent yet" across restarts.
package reportemail

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)

const isoUTC = "2006-01-02T15:04:05Z"

// Repo stores report_emails rows.
type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// AlreadySent reports whether the portfolio report for (year, month) has
// already been emailed successfully.
func (r *Repo) AlreadySent(ctx context.Context, year, month int) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM report_emails WHERE year = ? AND month = ?`, year, month).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MarkSent records that the portfolio report for (year, month) was
// emailed to recipients just now. Call this only after Send succeeds —
// a failed attempt should leave the month unrecorded so the next
// scheduler tick retries it.
func (r *Repo) MarkSent(ctx context.Context, year, month int, recipients []string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO report_emails (id, year, month, sent_at_utc, recipients)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.NewString(), year, month, now.UTC().Format(isoUTC), strings.Join(recipients, ","))
	return err
}
