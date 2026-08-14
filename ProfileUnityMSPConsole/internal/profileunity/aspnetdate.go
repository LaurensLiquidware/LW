package profileunity

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// aspNetDatePattern matches legacy ASP.NET JSON date serialization, §3.5:
// "/Date(1786690772847)/", optionally with a trailing timezone offset such
// as "/Date(1786690772847+0200)/".
var aspNetDatePattern = regexp.MustCompile(`^/Date\((-?\d+)([+-]\d{4})?\)/$`)

// aspNetDate holds a timestamp decoded from the §3.5 "/Date(ms)/" format.
// Valid is false for an empty string, so a missing field never gets
// silently treated as the Unix epoch.
type aspNetDate struct {
	Time  time.Time
	Valid bool
}

func (d *aspNetDate) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("aspnet date: %w", err)
	}
	if s == "" {
		*d = aspNetDate{}
		return nil
	}

	match := aspNetDatePattern.FindStringSubmatch(s)
	if match == nil {
		return fmt.Errorf("aspnet date: unrecognized format %q", s)
	}

	ms, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return fmt.Errorf("aspnet date: %w", err)
	}

	// The offset (if present) only tells us how the server's local clock
	// related to UTC when it serialized the value; the millisecond count
	// itself is already a Unix timestamp. The project brief prefers UTC
	// throughout, so it is discarded rather than applied as a shift.
	*d = aspNetDate{Time: time.UnixMilli(ms).UTC(), Valid: true}
	return nil
}
