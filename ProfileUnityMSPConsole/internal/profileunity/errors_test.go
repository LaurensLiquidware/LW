package profileunity

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMalformedPayloadError_TruncatesByRuneNotByte(t *testing.T) {
	// A multi-byte rune repeated past the 200-rune truncation boundary --
	// a byte-index slice here would cut a rune in half and produce
	// invalid UTF-8.
	body := strings.Repeat("日", 250)
	err := &MalformedPayloadError{Body: []byte(body)}

	msg := err.Error()
	if !utf8.ValidString(msg) {
		t.Fatalf("Error() produced invalid UTF-8: %q", msg)
	}
	if !strings.Contains(msg, "…") {
		t.Errorf("expected truncation marker in %q", msg)
	}
}

func TestMalformedPayloadError_ShortBodyNotTruncated(t *testing.T) {
	err := &MalformedPayloadError{Body: []byte("short body")}
	msg := err.Error()
	if strings.Contains(msg, "…") {
		t.Errorf("did not expect truncation marker in %q", msg)
	}
	if !strings.Contains(msg, "short body") {
		t.Errorf("expected body content in %q", msg)
	}
}
