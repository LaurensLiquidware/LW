package osv

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// The test cases here mirror
// ../../../flexapp-vuln-scanner/stage2-resolve/tests/test_osv_client.py,
// for output parity with the Python implementation.

func newTestClient(t *testing.T, url string, batchSize int) *Client {
	t.Helper()
	c, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = url
	if batchSize > 0 {
		c.BatchSize = batchSize
	}
	return c
}

func TestQueryBatch_SingleCall(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"vulns": []map[string]any{{"id": "GHSA-aaaa"}}},
				{"vulns": []map[string]any{}},
			},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, 0)
	result, err := c.QueryBatch([]string{"pkg:npm/lodash@4.17.15", "pkg:npm/left-pad@1.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result["pkg:npm/lodash@4.17.15"]) != 1 || result["pkg:npm/lodash@4.17.15"][0] != "GHSA-aaaa" {
		t.Errorf("result = %v", result)
	}
	if len(result["pkg:npm/left-pad@1.3.0"]) != 0 {
		t.Errorf("result = %v", result)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestQueryBatch_CachesAndSkipsNetworkOnSecondCall(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"vulns": []map[string]any{{"id": "GHSA-aaaa"}}}}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, 0)
	first, err := c.QueryBatch([]string{"pkg:npm/lodash@4.17.15"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.QueryBatch([]string{"pkg:npm/lodash@4.17.15"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first["pkg:npm/lodash@4.17.15"]) != 1 || len(second["pkg:npm/lodash@4.17.15"]) != 1 {
		t.Errorf("first = %v, second = %v", first, second)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (second call served from cache)", calls)
	}
}

func TestQueryBatch_SplitsByBatchSize(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"vulns": []map[string]any{}}, {"vulns": []map[string]any{}}}})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"vulns": []map[string]any{}}}})
		}
	}))
	defer srv.Close()

	// batch_size=2, 3 purls -> 2 HTTP calls (2 + 1)
	c := newTestClient(t, srv.URL, 2)
	result, err := c.QueryBatch([]string{"pkg:npm/a@1", "pkg:npm/b@1", "pkg:npm/c@1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Errorf("len(result) = %d, want 3", len(result))
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestQueryBatch_MixedCacheHitAndMiss(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"vulns": []map[string]any{{"id": "GHSA-aaaa"}}}}})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"vulns": []map[string]any{}}}})
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, 0)
	if _, err := c.QueryBatch([]string{"pkg:npm/lodash@4.17.15"}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}

	result, err := c.QueryBatch([]string{"pkg:npm/lodash@4.17.15", "pkg:npm/new-pkg@1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (only the uncached purl triggers a second call)", calls)
	}
	if len(result["pkg:npm/lodash@4.17.15"]) != 1 {
		t.Errorf("lodash = %v", result["pkg:npm/lodash@4.17.15"])
	}
	if len(result["pkg:npm/new-pkg@1.0.0"]) != 0 {
		t.Errorf("new-pkg = %v", result["pkg:npm/new-pkg@1.0.0"])
	}
}

func TestGetVulnerability_Caches(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		json.NewEncoder(w).Encode(map[string]any{"id": "GHSA-aaaa", "summary": "A bad thing"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, 0)
	first, err := c.GetVulnerability("GHSA-aaaa")
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.GetVulnerability("GHSA-aaaa")
	if err != nil {
		t.Fatal(err)
	}
	if first["summary"] != "A bad thing" || second["summary"] != "A bad thing" {
		t.Errorf("first = %v, second = %v", first, second)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestResolve_CombinesBatchAndDetailLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/querybatch":
			json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"vulns": []map[string]any{{"id": "GHSA-aaaa"}}}}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"id": "GHSA-aaaa", "summary": "A bad thing", "severity": []any{}})
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, 0)
	result, err := c.Resolve([]string{"pkg:npm/lodash@4.17.15"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	vulns := result["pkg:npm/lodash@4.17.15"]
	if len(vulns) != 1 || vulns[0]["id"] != "GHSA-aaaa" || vulns[0]["summary"] != "A bad thing" {
		t.Errorf("vulns = %v", vulns)
	}
}

func TestResolve_EmptyPurlList(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, 0)
	result, err := c.Resolve([]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("result = %v, want empty", result)
	}
	if calls != 0 {
		t.Errorf("calls = %d, want 0", calls)
	}
}

func TestResolve_ReportsProgressPerVulnID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/querybatch":
			json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"vulns": []map[string]any{{"id": "GHSA-aaaa"}, {"id": "GHSA-bbbb"}}}}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"id": "x", "summary": "s", "severity": []any{}})
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, 0)
	type progress struct{ done, total int }
	var calls []progress
	_, err := c.Resolve([]string{"pkg:npm/lodash@4.17.15"}, func(done, total int) {
		calls = append(calls, progress{done, total})
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []progress{{1, 2}, {2, 2}}
	if len(calls) != len(want) || calls[0] != want[0] || calls[1] != want[1] {
		t.Errorf("calls = %v, want %v", calls, want)
	}
}

func TestCachePersistsAcrossClientInstances(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"vulns": []map[string]any{{"id": "GHSA-aaaa"}}}}})
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	client1, err := New(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	client1.BaseURL = srv.URL
	if _, err := client1.QueryBatch([]string{"pkg:npm/lodash@4.17.15"}); err != nil {
		t.Fatal(err)
	}

	client2, err := New(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	client2.BaseURL = srv.URL
	result, err := client2.QueryBatch([]string{"pkg:npm/lodash@4.17.15"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result["pkg:npm/lodash@4.17.15"]) != 1 {
		t.Errorf("result = %v", result)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (second client should hit the on-disk cache)", calls)
	}
}
