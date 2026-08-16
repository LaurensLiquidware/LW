// Package reportmail automatically emails the MSP-wide portfolio PDF
// report once a month, on an in-process ticker (the same "no external
// cron dependency" approach internal/scheduler uses for collection).
// Whether the feature runs at all is a single on/off switch — an empty
// SMTP host — checked on every tick via SetConfig's current value, so
// enabling/disabling it from the Settings screen needs no restart.
package reportmail

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"profileunity-msp-console/internal/dashboard"
	"profileunity-msp-console/internal/mailer"
	"profileunity-msp-console/internal/reportemail"
	"profileunity-msp-console/internal/reportpdf"
)

// checkInterval is how often Run wakes up to ask "is today the send day".
// A monthly job doesn't need finer granularity than this; unlike
// collection's ticker, this isn't exposed as its own config knob since
// there's nothing an operator would tune it for.
const checkInterval = time.Hour

// Status is a point-in-time view of the report-mail scheduler.
type Status struct {
	LastCheckAt time.Time
	LastSendAt  time.Time
	LastOutcome string // "sent", "already_sent", "not_due", "no_recipients", "disabled", or "error"
	LastError   string
	LastYear    int
	LastMonth   int
}

// liveConfig is everything about a Scheduler's send behavior an operator
// can change at runtime via the Settings screen. Held behind an atomic
// pointer (see Scheduler.cfg) rather than as plain fields, since a
// settings-update HTTP handler and the running scheduler goroutine touch
// these concurrently.
type liveConfig struct {
	smtp       mailer.Config
	recipients []string
	day        int
	location   *time.Location
}

// enabled mirrors config.Config.ReportEmailEnabled/settings.Settings.
// ReportEmailEnabled: an empty SMTP host means the feature is off,
// regardless of what else is configured.
func (c *liveConfig) enabled() bool {
	return c.smtp.Host != ""
}

// Scheduler checks once per checkInterval tick whether it's on or after
// the configured day of the month (in its current config's location) to
// email the previous month's portfolio report, and sends it at most
// once per month.
type Scheduler struct {
	repos    dashboard.Repos
	emails   *reportemail.Repo
	demoMode bool

	cfg atomic.Pointer[liveConfig]

	mu     sync.Mutex
	status Status
}

// New creates a Scheduler. It always runs once started via Run,
// regardless of whether SMTP is configured yet — checkAndSendAt no-ops
// with outcome "disabled" whenever the current config's SMTP host is
// empty, so enabling the feature later via SetConfig takes effect on the
// very next check without needing to start a new goroutine. demoMode
// watermarks any PDF this scheduler ever renders/emails (see send) —
// running against a demo.db sidecar database doesn't itself disable
// report-email (SMTP just stays unconfigured by convention), but if an
// operator enables it anyway against demo data, the watermark still
// applies.
func New(repos dashboard.Repos, emails *reportemail.Repo, smtp mailer.Config, recipients []string, day int, location *time.Location, demoMode bool) *Scheduler {
	s := &Scheduler{repos: repos, emails: emails, demoMode: demoMode}
	s.SetConfig(smtp, recipients, day, location)
	return s
}

// SetConfig changes the SMTP settings, recipients, send day, and
// timezone a running Scheduler uses, effective from its very next check
// — no restart needed. Safe to call from any goroutine.
func (s *Scheduler) SetConfig(smtp mailer.Config, recipients []string, day int, location *time.Location) {
	s.cfg.Store(&liveConfig{smtp: smtp, recipients: recipients, day: day, location: location})
}

func (s *Scheduler) current() *liveConfig {
	return s.cfg.Load()
}

// Run blocks, checking immediately and then on every tick, until ctx is
// canceled. Call it in its own goroutine.
func (s *Scheduler) Run(ctx context.Context) {
	s.checkAndSend(ctx)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAndSend(ctx)
		}
	}
}

func (s *Scheduler) checkAndSend(ctx context.Context) {
	s.checkAndSendAt(ctx, time.Now().In(s.current().location))
}

// checkAndSendAt sends the previous month's portfolio report if, and
// only if, the feature is enabled, at is on or after the configured send
// day of its calendar month, and that month's report hasn't already
// been sent successfully. "On or after", not "exactly on", so a server
// that's down on the configured day still sends as soon as it's back up
// rather than silently skipping that month — AlreadySent is what
// prevents a duplicate once it has.
func (s *Scheduler) checkAndSendAt(ctx context.Context, now time.Time) {
	cur := s.current()
	s.recordCheck(now)

	if !cur.enabled() {
		slog.Debug("reportmail: check skipped, SMTP not configured")
		s.recordOutcome(now, 0, 0, "disabled", nil)
		return
	}
	if now.Day() < cur.day {
		slog.Debug(fmt.Sprintf("reportmail: not due yet, today is day %d, send day is %d", now.Day(), cur.day))
		s.recordOutcome(now, 0, 0, "not_due", nil)
		return
	}
	if len(cur.recipients) == 0 {
		slog.Debug("reportmail: check skipped, no recipients configured")
		s.recordOutcome(now, 0, 0, "no_recipients", nil)
		return
	}

	year, month := previousMonth(now)

	alreadySent, err := s.emails.AlreadySent(ctx, year, month)
	if err != nil {
		slog.Error(fmt.Sprintf("reportmail: check already-sent for %04d-%02d: %v", year, month, err))
		s.recordOutcome(now, year, month, "error", err)
		return
	}
	if alreadySent {
		slog.Debug(fmt.Sprintf("reportmail: %04d-%02d already sent, skipping", year, month))
		s.recordOutcome(now, year, month, "already_sent", nil)
		return
	}

	if err := s.send(ctx, year, month, cur); err != nil {
		slog.Error(fmt.Sprintf("reportmail: send portfolio report for %04d-%02d: %v", year, month, err))
		s.recordOutcome(now, year, month, "error", err)
		return
	}

	if err := s.emails.MarkSent(ctx, year, month, cur.recipients, time.Now()); err != nil {
		// The email itself was already sent -- failing to record that
		// isn't something to retry by resending, only to log loudly, or
		// next month's run risks resending this month's report too.
		slog.Error(fmt.Sprintf("reportmail: sent %04d-%02d but failed to record it: %v", year, month, err))
	}
	slog.Info(fmt.Sprintf("reportmail: emailed the %04d-%02d portfolio report to %v", year, month, cur.recipients))
	s.recordOutcome(now, year, month, "sent", nil)
}

// SendNow builds and emails last calendar month's portfolio report
// immediately, bypassing the day-of-month gate -- for an
// operator-triggered "Send Now" button on the Settings screen. It marks
// the month as sent (reportemail.Repo.MarkSent) on success, exactly like
// the automatic scheduled send, so the scheduler won't duplicate it
// later in the month once the configured send day arrives.
func (s *Scheduler) SendNow(ctx context.Context) (year, month int, err error) {
	cur := s.current()
	if !cur.enabled() {
		return 0, 0, fmt.Errorf("SMTP is not configured")
	}
	if len(cur.recipients) == 0 {
		return 0, 0, fmt.Errorf("no report recipients configured")
	}

	now := time.Now().In(cur.location)
	year, month = previousMonth(now)
	if err := s.send(ctx, year, month, cur); err != nil {
		return 0, 0, fmt.Errorf("send %04d-%02d report: %w", year, month, err)
	}
	if err := s.emails.MarkSent(ctx, year, month, cur.recipients, time.Now()); err != nil {
		slog.Error(fmt.Sprintf("reportmail: sent %04d-%02d via Send Now but failed to record it: %v", year, month, err))
	}
	slog.Info(fmt.Sprintf("reportmail: Send Now emailed the %04d-%02d portfolio report to %v", year, month, cur.recipients))
	s.recordOutcome(now, year, month, "sent", nil)
	return year, month, nil
}

// send builds and emails the portfolio PDF for one month.
func (s *Scheduler) send(ctx context.Context, year, month int, cur *liveConfig) error {
	days, from, to := monthRange(year, month)

	report, err := dashboard.LoadPortfolioMonthlyReport(ctx, s.repos, year, month, days, from, to)
	if err != nil {
		return fmt.Errorf("build report: %w", err)
	}

	pdf := reportpdf.RenderPortfolioReportPDF(report, s.demoMode)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return fmt.Errorf("render PDF: %w", err)
	}

	subject := fmt.Sprintf("ProfileUnity MSP Licensing Console — Portfolio report %04d-%02d", year, month)
	body := fmt.Sprintf("The ProfileUnity MSP Licensing Console monthly portfolio license report for %04d-%02d is attached.\n", year, month)
	attachment := mailer.Attachment{
		Filename: fmt.Sprintf("portfolio-%04d-%02d.pdf", year, month),
		MimeType: "application/pdf",
		Data:     buf.Bytes(),
	}

	return mailer.Send(cur.smtp, cur.recipients, subject, body, []mailer.Attachment{attachment})
}

// previousMonth returns the year/month immediately before now's calendar
// month — the "just-completed" month a send on the 1st should report on.
func previousMonth(now time.Time) (year, month int) {
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	prev := firstOfThisMonth.AddDate(0, -1, 0)
	return prev.Year(), int(prev.Month())
}

// monthRange returns the day count and "YYYY-MM-DD" first/last day of
// (year, month) — the inputs dashboard.LoadPortfolioMonthlyReport needs.
func monthRange(year, month int) (days int, from, to string) {
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)
	return last.Day(), first.Format("2006-01-02"), last.Format("2006-01-02")
}

// Status returns the current report-mail scheduler state.
func (s *Scheduler) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Scheduler) recordCheck(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastCheckAt = at
}

func (s *Scheduler) recordOutcome(at time.Time, year, month int, outcome string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastOutcome = outcome
	s.status.LastYear = year
	s.status.LastMonth = month
	if outcome == "sent" {
		s.status.LastSendAt = at
	}
	if err != nil {
		s.status.LastError = err.Error()
	} else {
		s.status.LastError = ""
	}
}
