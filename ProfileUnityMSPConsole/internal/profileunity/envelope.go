package profileunity

import (
	"encoding/json"
	"fmt"
)

// envelope is the response wrapper every ProfileUnity endpoint uses (§3.1).
// The console returns HTTP 200 even on failure, including authentication
// failure — success is Type == "success", never the HTTP status. Tag's
// shape varies by endpoint, so it stays raw here and each caller decodes
// it into whatever shape that endpoint actually uses.
type envelope struct {
	WebMessageType int             `json:"WebMessageType"`
	Type           string          `json:"Type"`
	Message        string          `json:"Message"`
	MessageKey     *string         `json:"MessageKey"`
	Tag            json.RawMessage `json:"Tag"`
}

// APIError is returned when the console responded with a well-formed
// envelope but Type != "success". It carries the console's own message so
// callers/logs can show precisely what the console said, without ever
// having treated the HTTP 200 as success.
type APIError struct {
	Message    string
	MessageKey *string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("profileunity: %s", e.Message)
	}
	return "profileunity: request failed (Type != success, no message)"
}

// decodeEnvelope parses the response body as a §3.1 envelope and returns
// the raw Tag only if Type == "success". Any other Type is surfaced as an
// *APIError, distinct from a JSON decode failure (*MalformedPayloadError).
func decodeEnvelope(body []byte) (json.RawMessage, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, &MalformedPayloadError{Body: body, Cause: err}
	}
	if env.Type != "success" {
		return nil, &APIError{Message: env.Message, MessageKey: env.MessageKey}
	}
	return env.Tag, nil
}
