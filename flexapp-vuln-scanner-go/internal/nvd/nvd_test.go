package nvd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is a deterministic, manually-advanced clock + no-op sleep
// that advances the clock by the requested amount -- lets rate-limit
// tests run instantly instead of waiting real wall-clock seconds. Mirrors
// ../../../flexapp-vuln-scanner/stage2-resolve/tests/test_nvd_client.py's
// FakeClock.
type fakeClock struct {
	base time.Time
	t    time.Duration
}

func newFakeClock() *fakeClock             { return &fakeClock{base: time.Unix(0, 0)} }
func (c *fakeClock) now() time.Time        { return c.base.Add(c.t) }
func (c *fakeClock) sleep(d time.Duration) { c.t += d }

func newTestClient(t *testing.T, url string, apiKey string) (*Client, *fakeClock) {
	t.Helper()
	c, err := New(t.TempDir(), apiKey)
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = url
	c.HTTPClient = http.DefaultClient
	clock := newFakeClock()
	c.Sleep = clock.sleep
	c.Now = clock.now
	return c, clock
}

func TestQueryCPE_Caches(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		json.NewEncoder(w).Encode(map[string]any{"vulnerabilities": []any{}})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv.URL, "")
	first, err := c.QueryCPE(context.Background(), "cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*")
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.QueryCPE(context.Background(), "cpe:2.3:a:openssl:openssl:1.1.1w:*:*:*:*:*:*:*")
	if err != nil {
		t.Fatal(err)
	}
	if len(first["vulnerabilities"].([]any)) != 0 || len(second["vulnerabilities"].([]any)) != 0 {
		t.Errorf("unexpected result: %v / %v", first, second)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestQueryCPE_404ReturnsEmptyResultNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv.URL, "")
	result, err := c.QueryCPE(context.Background(), "cpe:2.3:a:vendor:nonexistent-product:1.0:*:*:*:*:*:*:*")
	if err != nil {
		t.Fatal(err)
	}
	if len(result["vulnerabilities"].([]any)) != 0 {
		t.Errorf("result = %v, want empty vulnerabilities", result)
	}
}

func TestQueryCPE_404IsCached(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv.URL, "")
	c.QueryCPE(context.Background(), "cpe:2.3:a:vendor:nonexistent-product:1.0:*:*:*:*:*:*:*")
	c.QueryCPE(context.Background(), "cpe:2.3:a:vendor:nonexistent-product:1.0:*:*:*:*:*:*:*")
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestQueryCPE_429RetriesAndSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"vulnerabilities": []any{}})
	}))
	defer srv.Close()

	c, clock := newTestClient(t, srv.URL, "")
	result, err := c.QueryCPE(context.Background(), "cpe:2.3:a:vendor:product:1.0:*:*:*:*:*:*:*")
	if err != nil {
		t.Fatal(err)
	}
	if len(result["vulnerabilities"].([]any)) != 0 {
		t.Errorf("result = %v", result)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
	if clock.t <= 0 {
		t.Error("expected backoff before retrying")
	}
}

func TestQueryCPE_429HonorsRetryAfterHeader(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"vulnerabilities": []any{}})
	}))
	defer srv.Close()

	c, clock := newTestClient(t, srv.URL, "")
	if _, err := c.QueryCPE(context.Background(), "cpe:2.3:a:vendor:product:1.0:*:*:*:*:*:*:*"); err != nil {
		t.Fatal(err)
	}
	if clock.t != 5*time.Second {
		t.Errorf("clock.t = %v, want 5s", clock.t)
	}
}

func TestQueryCPE_429GivesUpAfterMaxRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv.URL, "")
	if _, err := c.QueryCPE(context.Background(), "cpe:2.3:a:vendor:product:1.0:*:*:*:*:*:*:*"); err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
}

func TestNoAPIKeySendsNoHeader(t *testing.T) {
	var gotHeader string
	var sawHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("apiKey")
		sawHeader = r.Header.Get("apiKey") != ""
		json.NewEncoder(w).Encode(map[string]any{"vulnerabilities": []any{}})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv.URL, "")
	c.QueryCPE(context.Background(), "cpe:2.3:a:zlib:zlib:1.3:*:*:*:*:*:*:*")
	if sawHeader {
		t.Errorf("apiKey header = %q, want none", gotHeader)
	}
}

func TestAPIKeySentAsHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("apiKey")
		json.NewEncoder(w).Encode(map[string]any{"vulnerabilities": []any{}})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv.URL, "secret-key")
	c.QueryCPE(context.Background(), "cpe:2.3:a:zlib:zlib:1.3:*:*:*:*:*:*:*")
	if gotHeader != "secret-key" {
		t.Errorf("apiKey header = %q, want secret-key", gotHeader)
	}
}

func TestRateLimitWithoutKeyThrottlesAfterFiveRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"vulnerabilities": []any{}})
	}))
	defer srv.Close()

	c, clock := newTestClient(t, srv.URL, "")
	for i := 0; i < 5; i++ {
		c.QueryCPE(context.Background(), cpeFor(i))
	}
	if clock.t != 0 {
		t.Errorf("clock.t = %v, want 0 (first 5 requests free)", clock.t)
	}

	c.QueryCPE(context.Background(), cpeFor(5))
	if clock.t <= 0 {
		t.Error("expected the 6th request to wait out the window")
	}
}

func TestRateLimitWithKeyAllowsFifty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"vulnerabilities": []any{}})
	}))
	defer srv.Close()

	c, clock := newTestClient(t, srv.URL, "secret-key")
	for i := 0; i < 50; i++ {
		c.QueryCPE(context.Background(), cpeFor(i))
	}
	if clock.t != 0 {
		t.Errorf("clock.t = %v, want 0", clock.t)
	}

	c.QueryCPE(context.Background(), cpeFor(50))
	if clock.t <= 0 {
		t.Error("expected the 51st request to wait out the window")
	}
}

func TestCachedHitsNeverThrottle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"vulnerabilities": []any{}})
	}))
	defer srv.Close()

	c, clock := newTestClient(t, srv.URL, "")
	for i := 0; i < 5; i++ {
		c.QueryCPE(context.Background(), cpeFor(i))
	}

	for round := 0; round < 20; round++ {
		for i := 0; i < 5; i++ {
			c.QueryCPE(context.Background(), cpeFor(i))
		}
	}
	if clock.t != 0 {
		t.Errorf("clock.t = %v, want 0 (cache hits bypass throttle)", clock.t)
	}
}

func cpeFor(i int) string {
	return "cpe:2.3:a:vendor:product" + itoa(i) + ":1.0:*:*:*:*:*:*:*"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}

func TestExtractCVEs_FlattensResponse(t *testing.T) {
	response := map[string]any{
		"vulnerabilities": []any{
			map[string]any{
				"cve": map[string]any{
					"id": "CVE-2023-0001",
					"descriptions": []any{
						map[string]any{"lang": "en", "value": "A bad thing"},
					},
					"metrics": map[string]any{
						"cvssMetricV31": []any{
							map[string]any{"cvssData": map[string]any{"baseScore": 9.8, "baseSeverity": "CRITICAL"}},
						},
					},
				},
			},
		},
	}
	cves := ExtractCVEs(response)
	if len(cves) != 1 {
		t.Fatalf("len(cves) = %d, want 1", len(cves))
	}
	if cves[0].ID != "CVE-2023-0001" {
		t.Errorf("ID = %q", cves[0].ID)
	}
	if cves[0].Summary == nil || *cves[0].Summary != "A bad thing" {
		t.Errorf("Summary = %v", cves[0].Summary)
	}
	if cves[0].SeverityLevel == nil || *cves[0].SeverityLevel != "CRITICAL" {
		t.Errorf("SeverityLevel = %v", cves[0].SeverityLevel)
	}
}
