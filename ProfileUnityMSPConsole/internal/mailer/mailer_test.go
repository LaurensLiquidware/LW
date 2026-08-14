package mailer

import (
	"bufio"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBuildMessage_HeadersBodyAndAttachment(t *testing.T) {
	msg := string(buildMessage(
		"reports@example.com",
		[]string{"msp@liquidware.eu", "ops@example.com"},
		"Monthly Portfolio Report",
		"See the attached PDF.",
		[]Attachment{{Filename: "portfolio-2026-08.pdf", MimeType: "application/pdf", Data: []byte("%PDF-1.4 fake content")}},
	))

	for _, want := range []string{
		"From: reports@example.com\r\n",
		"To: msp@liquidware.eu, ops@example.com\r\n",
		"Subject: Monthly Portfolio Report\r\n",
		"Content-Type: multipart/mixed; boundary=\"" + mimeBoundary + "\"",
		"See the attached PDF.",
		"Content-Type: application/pdf; name=\"portfolio-2026-08.pdf\"",
		"Content-Transfer-Encoding: base64",
		"Content-Disposition: attachment; filename=\"portfolio-2026-08.pdf\"",
		"--" + mimeBoundary + "--\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing expected substring %q\nfull message:\n%s", want, msg)
		}
	}
}

func TestBuildMessage_EncodesNonASCIISubject(t *testing.T) {
	msg := string(buildMessage("a@x.com", []string{"b@x.com"}, "Rapport — Août", "body", nil))
	if strings.Contains(msg, "Août") {
		t.Error("raw non-ASCII subject text leaked into the header unencoded")
	}
	if !strings.Contains(msg, "Subject: =?UTF-8?") {
		t.Errorf("expected an RFC 2047 encoded-word subject, got message:\n%s", msg)
	}
}

// fakeSMTPServer runs a minimal single-connection SMTP server good enough
// to exercise Send's real wire protocol (EHLO/MAIL/RCPT/DATA/QUIT) without
// a real mail relay. It records every command line and the full DATA
// payload it received, and signals done once the client disconnects.
type fakeSMTPServer struct {
	addr     string
	commands []string
	data     string
	done     chan struct{}
}

func startFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	srv := &fakeSMTPServer{addr: ln.Addr().String(), done: make(chan struct{})}

	go func() {
		defer close(srv.done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
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
			srv.commands = append(srv.commands, line)
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
				srv.data = body.String()
				writeLine("250 OK")
			case line == "QUIT":
				writeLine("221 Bye")
				return
			default:
				writeLine("500 unrecognized")
			}
		}
	}()

	return srv
}

func (s *fakeSMTPServer) waitDone(t *testing.T) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
		t.Fatal("fake SMTP server never saw the client disconnect")
	}
}

func TestSend_DeliversOverPlainConnection(t *testing.T) {
	srv := startFakeSMTPServer(t)
	host, portStr, err := net.SplitHostPort(srv.addr)
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	cfg := Config{Host: host, Port: port, From: "reports@example.com", Security: "none"}
	err = Send(cfg, []string{"msp@liquidware.eu"}, "Monthly Portfolio Report", "See attached.",
		[]Attachment{{Filename: "portfolio.pdf", MimeType: "application/pdf", Data: []byte("%PDF-1.4 fake")}})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	srv.waitDone(t)

	joined := strings.Join(srv.commands, "\n")
	for _, want := range []string{"MAIL FROM", "RCPT TO", "DATA", "QUIT"} {
		if !strings.Contains(joined, want) {
			t.Errorf("server never saw a %s command; commands were: %v", want, srv.commands)
		}
	}
	if !strings.Contains(srv.data, "Content-Type: application/pdf") {
		t.Errorf("DATA payload missing the PDF attachment part:\n%s", srv.data)
	}
}

func TestSend_RejectsEmptyRecipientList(t *testing.T) {
	if err := Send(Config{}, nil, "subject", "body", nil); err == nil {
		t.Fatal("expected an error for an empty recipient list")
	}
}
