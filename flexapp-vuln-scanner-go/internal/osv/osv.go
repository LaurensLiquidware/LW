// Package osv is an OSV.dev client: batch purl -> vuln ID lookup, then
// per-ID detail fetch. OSV.dev needs no API key and matching against a
// purl is exact, not fuzzy. All responses are cached to disk and never
// re-queried once cached.
package osv

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
)

const (
	defaultBaseURL   = "https://api.osv.dev"
	defaultBatchSize = 100
)

// Client is an OSV.dev API client with an on-disk response cache.
type Client struct {
	CacheDir   string
	BaseURL    string
	BatchSize  int
	HTTPClient *http.Client

	purlCacheDir string
	vulnCacheDir string
}

// New creates a Client, ensuring its cache subdirectories exist.
func New(cacheDir string) (*Client, error) {
	c := &Client{
		CacheDir:   cacheDir,
		BaseURL:    defaultBaseURL,
		BatchSize:  defaultBatchSize,
		HTTPClient: http.DefaultClient,
	}
	c.purlCacheDir = filepath.Join(cacheDir, "osv-purl")
	c.vulnCacheDir = filepath.Join(cacheDir, "osv-vuln")
	if err := os.MkdirAll(c.purlCacheDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.vulnCacheDir, 0o755); err != nil {
		return nil, err
	}
	return c, nil
}

func cacheKey(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func readCache(dir, key string, out any) (bool, error) {
	path := filepath.Join(dir, cacheKey(key)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return false, err
	}
	return true, nil
}

func writeCache(dir, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, cacheKey(key)+".json")
	return os.WriteFile(path, data, 0o644)
}

type querybatchRequest struct {
	Queries []querybatchQuery `json:"queries"`
}

type querybatchQuery struct {
	Package querybatchPackage `json:"package"`
}

type querybatchPackage struct {
	Purl string `json:"purl"`
}

type querybatchResponse struct {
	Results []struct {
		Vulns []struct {
			ID string `json:"id"`
		} `json:"vulns"`
	} `json:"results"`
}

// QueryBatch returns {purl: [vulnID, ...]} for every purl given. Cached
// purls never hit the network again. Uncached purls are sent to
// /v1/querybatch in groups of c.BatchSize.
func (c *Client) QueryBatch(purls []string) (map[string][]string, error) {
	results := map[string][]string{}
	var uncached []string

	for _, purl := range purls {
		var cached []string
		ok, err := readCache(c.purlCacheDir, purl, &cached)
		if err != nil {
			return nil, err
		}
		if ok {
			results[purl] = cached
		} else {
			uncached = append(uncached, purl)
		}
	}

	batchSize := c.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	for i := 0; i < len(uncached); i += batchSize {
		end := i + batchSize
		if end > len(uncached) {
			end = len(uncached)
		}
		batch := uncached[i:end]
		batchResults, err := c.queryBatchUncached(batch)
		if err != nil {
			return nil, err
		}
		for purl, vulnIDs := range batchResults {
			if err := writeCache(c.purlCacheDir, purl, vulnIDs); err != nil {
				return nil, err
			}
			results[purl] = vulnIDs
		}
	}

	return results, nil
}

func (c *Client) queryBatchUncached(purls []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(purls) == 0 {
		return out, nil
	}

	body := querybatchRequest{Queries: make([]querybatchQuery, len(purls))}
	for i, p := range purls {
		body.Queries[i] = querybatchQuery{Package: querybatchPackage{Purl: p}}
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v1/querybatch", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("osv querybatch: unexpected status %d", resp.StatusCode)
	}

	var payload querybatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	for i, p := range purls {
		var ids []string
		if i < len(payload.Results) {
			for _, v := range payload.Results[i].Vulns {
				if v.ID != "" {
					ids = append(ids, v.ID)
				}
			}
		}
		out[p] = ids
	}
	return out, nil
}

// GetVulnerability fetches (or reads from cache) the full vulnerability
// record for a single OSV vuln ID.
func (c *Client) GetVulnerability(vulnID string) (map[string]any, error) {
	var cached map[string]any
	ok, err := readCache(c.vulnCacheDir, vulnID, &cached)
	if err != nil {
		return nil, err
	}
	if ok {
		return cached, nil
	}

	resp, err := c.HTTPClient.Get(c.BaseURL + "/v1/vulns/" + vulnID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("osv get vulnerability %s: unexpected status %d", vulnID, resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if err := writeCache(c.vulnCacheDir, vulnID, data); err != nil {
		return nil, err
	}
	return data, nil
}

// Resolve returns {purl: [full_vuln, ...]} for every purl given.
// onProgress(done, total), if non-nil, is called after each per-ID
// detail fetch (the sequential part) -- not the batch lookup, which is
// a single request regardless of purl count.
func (c *Client) Resolve(purls []string, onProgress func(done, total int)) (map[string][]map[string]any, error) {
	purlToIDs, err := c.QueryBatch(purls)
	if err != nil {
		return nil, err
	}

	idSet := map[string]struct{}{}
	for _, ids := range purlToIDs {
		for _, id := range ids {
			idSet[id] = struct{}{}
		}
	}
	uniqueIDs := make([]string, 0, len(idSet))
	for id := range idSet {
		uniqueIDs = append(uniqueIDs, id)
	}
	sort.Strings(uniqueIDs)

	vulnByID := map[string]map[string]any{}
	for i, id := range uniqueIDs {
		v, err := c.GetVulnerability(id)
		if err != nil {
			// Matches the Python client's behavior: log and skip rather
			// than aborting the whole resolve on one bad ID.
			v = nil
		}
		if v != nil {
			vulnByID[id] = v
		}
		if onProgress != nil {
			onProgress(i+1, len(uniqueIDs))
		}
	}

	out := map[string][]map[string]any{}
	for purl, ids := range purlToIDs {
		var vulns []map[string]any
		for _, id := range ids {
			if v, ok := vulnByID[id]; ok {
				vulns = append(vulns, v)
			}
		}
		out[purl] = vulns
	}
	return out, nil
}
