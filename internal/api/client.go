package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	baseURL    = "https://graph.facebook.com/v23.0"
	adLibPath  = "/ads_archive"
)

// Client is an authenticated Meta Graph API client.
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient creates a new Client.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// baseParams returns common query parameters added to every request.
func (c *Client) baseParams() url.Values {
	params := url.Values{}
	params.Set("access_token", c.token)
	return params
}

// checkRateLimit reads X-App-Usage and warns to stderr if high.
func checkRateLimit(headers http.Header) {
	usage := headers.Get("X-App-Usage")
	if usage == "" {
		return
	}
	var parsed struct {
		CallCount      int `json:"call_count"`
		TotalCPUTime   int `json:"total_cputime"`
		TotalTime      int `json:"total_time"`
	}
	if err := json.Unmarshal([]byte(usage), &parsed); err != nil {
		return
	}
	pct := parsed.CallCount
	if parsed.TotalTime > pct {
		pct = parsed.TotalTime
	}
	if pct > 75 {
		fmt.Fprintf(os.Stderr, "warning: rate limit %d%% used — slow down to avoid HTTP 613\n", pct)
	}
}

// doRequest executes an HTTP request and returns the body bytes.
func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	checkRateLimit(resp.Header)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var errResp struct {
		Error *MetaError `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
		return nil, errResp.Error
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// Get makes an authenticated GET request.
func (c *Client) Get(path string, params url.Values) ([]byte, error) {
	reqURL, err := buildURL(path, c.baseParams(), params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return c.doRequest(req)
}

// SearchAds queries the /ads_archive endpoint with the given params.
// It follows paging.next cursors and returns all results up to limit (0 = all).
func (c *Client) SearchAds(params url.Values, limit int) ([]json.RawMessage, error) {
	var all []json.RawMessage

	// Clone to avoid mutating caller's map
	p := url.Values{}
	for k, v := range params {
		p[k] = v
	}

	// API max per page is 2000; use 100 as default batch size
	if p.Get("limit") == "" {
		p.Set("limit", "100")
	}

	currentPath := adLibPath

	for {
		body, err := c.Get(currentPath, p)
		if err != nil {
			return nil, err
		}

		var page struct {
			Data   []json.RawMessage `json:"data"`
			Paging *Paging           `json:"paging"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("parsing page: %w", err)
		}

		all = append(all, page.Data...)

		// Enforce caller's limit
		if limit > 0 && len(all) >= limit {
			all = all[:limit]
			break
		}

		if page.Paging == nil || page.Paging.Next == "" {
			break
		}

		// Next page URL already contains all params
		currentPath = page.Paging.Next
		p = url.Values{}
	}

	return all, nil
}

// buildURL constructs a full URL from path, base params, and extra params.
// If path starts with "http", it's used as-is (for paging.next).
func buildURL(path string, base, extra url.Values) (string, error) {
	var u *url.URL
	var err error

	if strings.HasPrefix(path, "http") {
		u, err = url.Parse(path)
	} else {
		u, err = url.Parse(baseURL + path)
	}
	if err != nil {
		return "", err
	}

	q := u.Query()
	for k, vs := range base {
		q.Set(k, vs[0])
	}
	for k, vs := range extra {
		q.Set(k, vs[0])
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
