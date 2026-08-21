// Package nvd is an NVD 2.0 API client: CPE-based CVE lookup,
// rate-limit aware, on-disk cache.
//
// 5 requests per 30 seconds without an API key, 50 with one. Every
// response is cached to disk and a cached CPE is never re-queried.
package nvd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"
	noKeyLimit     = 5
	withKeyLimit   = 50
	windowSeconds  = 30.0
	max429Retries  = 5
)

func cacheKey(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Client is an NVD 2.0 API client.
type Client struct {
	CacheDir   string
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration

	// Injectable for deterministic tests -- real callers never set these.
	Sleep func(time.Duration)
	Now   func() time.Time

	limit        int
	requestTimes []time.Time
}

// New creates a Client, ensuring its cache directory exists.
func New(cacheDir, apiKey string) (*Client, error) {
	dir := filepath.Join(cacheDir, "nvd-cpe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	limit := noKeyLimit
	if apiKey != "" {
		limit = withKeyLimit
	}
	return &Client{
		CacheDir:   dir,
		APIKey:     apiKey,
		BaseURL:    defaultBaseURL,
		HTTPClient: http.DefaultClient,
		Sleep:      time.Sleep,
		Now:        time.Now,
		limit:      limit,
	}, nil
}

func (c *Client) readCache(cpe23 string) (map[string]any, bool, error) {
	path := filepath.Join(c.CacheDir, cacheKey(cpe23)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func (c *Client) writeCache(cpe23 string, value map[string]any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	path := filepath.Join(c.CacheDir, cacheKey(cpe23)+".json")
	return os.WriteFile(path, data, 0o644)
}

// throttle blocks (via c.Sleep) until making another request stays
// within the sliding rate-limit window. Cached hits never call this.
func (c *Client) throttle() {
	now := c.Now()
	for len(c.requestTimes) > 0 && now.Sub(c.requestTimes[0]).Seconds() > windowSeconds {
		c.requestTimes = c.requestTimes[1:]
	}

	if len(c.requestTimes) >= c.limit {
		wait := windowSeconds - now.Sub(c.requestTimes[0]).Seconds()
		if wait > 0 {
			c.Sleep(time.Duration(wait * float64(time.Second)))
		}
	}

	c.requestTimes = append(c.requestTimes, c.Now())
	if len(c.requestTimes) > c.limit {
		c.requestTimes = c.requestTimes[len(c.requestTimes)-c.limit:]
	}
}

// QueryCPE returns the raw NVD 2.0 API response for a CPE 2.3 string.
// Cached forever once fetched -- a cache hit never touches the network
// or the rate limiter.
func (c *Client) QueryCPE(ctx context.Context, cpe23 string) (map[string]any, error) {
	if cached, ok, err := c.readCache(cpe23); err != nil {
		return nil, err
	} else if ok {
		return cached, nil
	}

	for attempt := 0; attempt <= max429Retries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.throttle()

		u, _ := url.Parse(c.BaseURL)
		q := u.Query()
		q.Set("cpeName", cpe23)
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		if c.APIKey != "" {
			req.Header.Set("apiKey", c.APIKey)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			// Documented NVD 2.0 API behavior: a cpeName with no
			// matching entry in NVD's CPE dictionary returns 404, not an
			// empty 200 result -- a real "no CVEs known for this CPE"
			// answer, not a connectivity failure.
			data := map[string]any{"vulnerabilities": []any{}}
			if err := c.writeCache(cpe23, data); err != nil {
				return nil, err
			}
			return data, nil
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := resp.Header.Get("Retry-After")
			resp.Body.Close()
			if attempt == max429Retries {
				return nil, fmt.Errorf("nvd query cpe %s: rate limited (429) after %d retries", cpe23, max429Retries)
			}
			wait := windowSeconds
			if retryAfter != "" {
				if f, err := strconv.ParseFloat(retryAfter, 64); err == nil {
					wait = f
				}
			}
			c.Sleep(time.Duration(wait * float64(time.Second)))
			continue
		}

		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("nvd query cpe %s: unexpected status %d", cpe23, resp.StatusCode)
		}

		var data map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, err
		}
		if err := c.writeCache(cpe23, data); err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, fmt.Errorf("nvd query cpe %s: exhausted retries", cpe23)
}

// Severity is one CVSS metric entry.
type Severity struct {
	Source       string   `json:"source"`
	BaseScore    *float64 `json:"baseScore"`
	BaseSeverity *string  `json:"baseSeverity"`
}

// CVE is a flattened NVD vulnerability record, matching the shape used
// for OSV results so downstream reporting can treat both sources
// uniformly.
type CVE struct {
	ID            string     `json:"id"`
	Summary       *string    `json:"summary"`
	Severity      []Severity `json:"severity"`
	SeverityLevel *string    `json:"severityLevel"`
}

var cvssMetricKeys = []string{"cvssMetricV31", "cvssMetricV30", "cvssMetricV2"}

// ExtractCVEs flattens an NVD 2.0 response into a simple list of CVEs.
func ExtractCVEs(nvdResponse map[string]any) []CVE {
	var out []CVE

	vulnerabilities, _ := nvdResponse["vulnerabilities"].([]any)
	for _, item := range vulnerabilities {
		itemMap, _ := item.(map[string]any)
		cve, _ := itemMap["cve"].(map[string]any)

		var summary *string
		if descriptions, ok := cve["descriptions"].([]any); ok {
			for _, d := range descriptions {
				dm, _ := d.(map[string]any)
				if lang, _ := dm["lang"].(string); lang == "en" {
					if v, ok := dm["value"].(string); ok {
						summary = &v
					}
					break
				}
			}
		}

		var severities []Severity
		metrics, _ := cve["metrics"].(map[string]any)
		for _, metricKey := range cvssMetricKeys {
			entries, _ := metrics[metricKey].([]any)
			for _, e := range entries {
				em, _ := e.(map[string]any)
				cvssData, _ := em["cvssData"].(map[string]any)

				var baseScore *float64
				if v, ok := cvssData["baseScore"].(float64); ok {
					baseScore = &v
				}
				var baseSeverity *string
				if v, ok := cvssData["baseSeverity"].(string); ok {
					baseSeverity = &v
				} else if v, ok := em["baseSeverity"].(string); ok {
					baseSeverity = &v
				}
				severities = append(severities, Severity{Source: metricKey, BaseScore: baseScore, BaseSeverity: baseSeverity})
			}
		}

		var severityLevel *string
		for _, s := range severities {
			if s.BaseSeverity != nil && *s.BaseSeverity != "" {
				upper := strings.ToUpper(*s.BaseSeverity)
				severityLevel = &upper
				break
			}
		}

		id, _ := cve["id"].(string)
		out = append(out, CVE{ID: id, Summary: summary, Severity: severities, SeverityLevel: severityLevel})
	}
	return out
}
