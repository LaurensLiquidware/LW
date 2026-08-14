// Package mailer sends outgoing email over SMTP. It talks the wire
// protocol directly via the standard library (net/smtp, crypto/tls)
// rather than pulling in a third-party mail dependency, matching this
// project's minimal-dependency stance (see go.mod).
package mailer

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
)

// Config is the SMTP connection and sender identity needed to send mail.
// Kept as its own type (rather than importing internal/config) so this
// package has no dependency beyond the standard library.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string

	// Security is "starttls" (a plain connection upgraded via STARTTLS,
	// the common case for port 587), "tls" (implicit TLS from the first
	// byte, port 465), or "none" (unencrypted, for a local relay only).
	Security string
}

// Attachment is one file to attach to an outgoing message.
type Attachment struct {
	Filename string
	MimeType string
	Data     []byte
}

// mimeBoundary separates the text body from each attachment part. It's a
// fixed string, not a random one: this package sends one message at a
// time on its own connection, so there's no concurrent-message collision
// risk to guard against, and a fixed boundary keeps Send deterministic
// and easy to test.
const mimeBoundary = "pumc-report-boundary"

// Send delivers one email with zero or more attachments to every address
// in to, over a fresh SMTP connection.
func Send(cfg Config, to []string, subject, textBody string, attachments []Attachment) error {
	if len(to) == 0 {
		return fmt.Errorf("mailer: no recipients")
	}

	client, err := dial(cfg)
	if err != nil {
		return fmt.Errorf("mailer: connect: %w", err)
	}
	defer client.Close()

	if cfg.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return fmt.Errorf("mailer: authenticate: %w", err)
		}
	}

	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("mailer: MAIL FROM: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("mailer: RCPT TO %s: %w", rcpt, err)
		}
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA: %w", err)
	}
	if _, err := wc.Write(buildMessage(cfg.From, to, subject, textBody, attachments)); err != nil {
		wc.Close()
		return fmt.Errorf("mailer: write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("mailer: close body: %w", err)
	}

	return client.Quit()
}

func dial(cfg Config) (*smtp.Client, error) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	if cfg.Security == "tls" {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.Host})
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, cfg.Host)
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if cfg.Security == "starttls" {
		if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
			client.Close()
			return nil, err
		}
	}
	return client, nil
}

// buildMessage renders a full RFC 5322 message with a multipart/mixed
// body: a plain-text part followed by one base64-encoded part per
// attachment. Split out from Send so the message format can be tested
// without a real SMTP server.
func buildMessage(from string, to []string, subject, textBody string, attachments []Attachment) []byte {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mimeBoundary)

	fmt.Fprintf(&buf, "--%s\r\n", mimeBoundary)
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	buf.WriteString(textBody)
	buf.WriteString("\r\n")

	for _, a := range attachments {
		fmt.Fprintf(&buf, "--%s\r\n", mimeBoundary)
		fmt.Fprintf(&buf, "Content-Type: %s; name=%q\r\n", a.MimeType, a.Filename)
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(&buf, "Content-Disposition: attachment; filename=%q\r\n\r\n", a.Filename)

		encoded := base64.StdEncoding.EncodeToString(a.Data)
		for i := 0; i < len(encoded); i += 76 {
			end := min(i+76, len(encoded))
			buf.WriteString(encoded[i:end])
			buf.WriteString("\r\n")
		}
	}

	fmt.Fprintf(&buf, "--%s--\r\n", mimeBoundary)
	return buf.Bytes()
}
