package reportmail

import (
	"bufio"
	"context"
	"net"
	"net/textproto"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"profileunity-msp-console/internal/dashboard"
	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/mailer"
	"profileunity-msp-console/internal/reportemail"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

// fakeSMTPServer is a minimal single-connection SMTP server, good enough
// to prove Scheduler actually talks real SMTP end to end rather than just
// exercising its own decision logic.
type fakeSMTPServer struct {
	addr string
	done chan struct{}
	got  chan string // the DATA payload of every accepted message
}

func startFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	srv := &fakeSMTPServer{addr: ln.Addr().String(), done: make(chan struct{}), got: make(chan string, 4)}

	go func() {
		defer close(srv.done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.handle(conn)
		}
	}()

	return srv
}

func (srv *fakeSMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	w := bufio.NewWriter(conn)
	writeLine := func(s string) {
		w.WriteString(s + "\r\n")
		w.Flush()
	}
	reader := textproto.NewReader(bufio.NewReader(conn))

	writeLine("220 localhost ESMTP fake")
	for {
		line, err := reader.ReadLine()
		if err != nil {
			return
		}
		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			writeLine("250 localhost")
		case strings.HasPrefix(line, "MAIL FROM"):
			writeLine("250 OK")
		case strings.HasPrefix(line, "RCPT TO"):
			writeLine("250 OK")
		case line == "DATA":
			writeLine("354 End with <CRLF>.<CRLF>")
			var body strings.Builder
			for {
				dline, err := reader.ReadLine()
				if err != nil {
					return
				}
				if dline == "." {
					break
				}
				body.WriteString(dline)
				body.WriteString("\n")
			}
			srv.got <- body.String()
			writeLine("250 OK")
		case line == "QUIT":
			writeLine("221 Bye")
			return
		default:
			writeLine("500 unrecognized")
		}
	}
}

func (srv *fakeSMTPServer) smtpConfig(t *testing.T) mailer.Config {
	t.Helper()
	host, portStr, err := net.SplitHostPort(srv.addr)
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return mailer.Config{Host: host, Port: port, From: "reports@example.com", Security: "none"}
}

func newTestScheduler(t *testing.T, smtp mailer.Config, day int, recipients []string) *Scheduler {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	tenantRepo := tenant.NewRepo(sqlDB, nil)
	if _, err := tenantRepo.Create(context.Background(), tenant.CreateInput{DisplayName: "Acme", Hostname: "acme.example", Port: 8000, Enabled: true}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	repos := dashboard.Repos{Tenants: tenantRepo, Snapshots: snapshot.NewRepo(sqlDB)}
	emails := reportemail.NewRepo(sqlDB)

	return New(repos, emails, smtp, recipients, day, time.UTC)
}

func TestCheckAndSendAt_SendsOnTheConfiguredDay(t *testing.T) {
	srv := startFakeSMTPServer(t)
	s := newTestScheduler(t, srv.smtpConfig(t), 1, []string{"msp@liquidware.eu"})

	s.checkAndSendAt(context.Background(), time.Date(2026, time.August, 1, 9, 0, 0, 0, time.UTC))

	select {
	case body := <-srv.got:
		if !strings.Contains(body, "Content-Type: application/pdf") {
			t.Errorf("sent message missing the PDF attachment:\n%s", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler never sent a message on the configured day")
	}

	status := s.Status()
	if status.LastOutcome != "sent" {
		t.Errorf("LastOutcome = %q, want sent", status.LastOutcome)
	}
	if status.LastYear != 2026 || status.LastMonth != 7 {
		t.Errorf("LastYear/LastMonth = %d-%02d, want 2026-07 (the month before the send date)", status.LastYear, status.LastMonth)
	}
}

func TestCheckAndSendAt_SkipsBeforeTheConfiguredDay(t *testing.T) {
	srv := startFakeSMTPServer(t)
	s := newTestScheduler(t, srv.smtpConfig(t), 5, []string{"msp@liquidware.eu"})

	s.checkAndSendAt(context.Background(), time.Date(2026, time.August, 3, 9, 0, 0, 0, time.UTC))

	select {
	case body := <-srv.got:
		t.Fatalf("scheduler sent a message before its configured day:\n%s", body)
	case <-time.After(200 * time.Millisecond):
	}

	if got := s.Status().LastOutcome; got != "not_due" {
		t.Errorf("LastOutcome = %q, want not_due", got)
	}
}

func TestCheckAndSendAt_StillSendsWhenLateAfterTheConfiguredDay(t *testing.T) {
	// A server that was down on the 1st (e.g. restarted, or the box was
	// offline) must still send once it notices the month is unsent --
	// "on or after", not "exactly on", the configured day.
	srv := startFakeSMTPServer(t)
	s := newTestScheduler(t, srv.smtpConfig(t), 1, []string{"msp@liquidware.eu"})

	s.checkAndSendAt(context.Background(), time.Date(2026, time.August, 4, 9, 0, 0, 0, time.UTC))

	select {
	case <-srv.got:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler never caught up on a late check")
	}
	if got := s.Status().LastOutcome; got != "sent" {
		t.Errorf("LastOutcome = %q, want sent", got)
	}
}

func TestCheckAndSendAt_DoesNotResendTheSameMonthTwice(t *testing.T) {
	srv := startFakeSMTPServer(t)
	s := newTestScheduler(t, srv.smtpConfig(t), 1, []string{"msp@liquidware.eu"})

	s.checkAndSendAt(context.Background(), time.Date(2026, time.August, 1, 9, 0, 0, 0, time.UTC))
	select {
	case <-srv.got:
	case <-time.After(5 * time.Second):
		t.Fatal("first checkAndSendAt never sent a message")
	}

	// A second tick the same day (or any later day still in August) for
	// the same target month must not send again -- report_emails'
	// UNIQUE(year, month) row is what makes this idempotent across
	// restarts, not just within one process.
	s.checkAndSendAt(context.Background(), time.Date(2026, time.August, 1, 10, 0, 0, 0, time.UTC))
	select {
	case body := <-srv.got:
		t.Fatalf("scheduler resent the same month's report:\n%s", body)
	case <-time.After(200 * time.Millisecond):
	}

	if got := s.Status().LastOutcome; got != "already_sent" {
		t.Errorf("LastOutcome = %q, want already_sent", got)
	}
}

func TestCheckAndSendAt_SkipsWhenThereAreNoRecipients(t *testing.T) {
	srv := startFakeSMTPServer(t)
	s := newTestScheduler(t, srv.smtpConfig(t), 1, nil)

	s.checkAndSendAt(context.Background(), time.Date(2026, time.August, 1, 9, 0, 0, 0, time.UTC))

	select {
	case body := <-srv.got:
		t.Fatalf("scheduler sent a message with no recipients configured:\n%s", body)
	case <-time.After(200 * time.Millisecond):
	}

	if got := s.Status().LastOutcome; got != "no_recipients" {
		t.Errorf("LastOutcome = %q, want no_recipients", got)
	}
}

func TestPreviousMonth_HandlesJanuaryRollover(t *testing.T) {
	year, month := previousMonth(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))
	if year != 2025 || month != 12 {
		t.Errorf("previousMonth(Jan 2026) = %d-%02d, want 2025-12", year, month)
	}
}
