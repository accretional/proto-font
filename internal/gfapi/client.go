// Package gfapi is a tiny client for the Google Fonts developer API.
//
// API spec: https://developers.google.com/fonts/docs/developer_api
// Base URL:  https://www.googleapis.com/webfonts/v1/webfonts
//
// Auth: an API key (free, project-scoped) is required and passed as the
// `key` query parameter. No OAuth.
//
// Rate limit: as of writing, the docs don't advertise a hard limit; the
// service returns HTTP 429 when exceeded. Callers should back off.
package gfapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://www.googleapis.com/webfonts/v1/webfonts"

// Client talks to the Google Fonts developer API. The zero value is NOT
// usable — always construct via New().
type Client struct {
	apiKey  string
	base    string
	httpC   *http.Client
}

// New returns a Client bound to the supplied API key. httpClient may be nil
// to use http.DefaultClient with a 30s timeout.
func New(apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{apiKey: apiKey, base: defaultBaseURL, httpC: httpClient}
}

// ListOptions are the query parameters supported by /v1/webfonts.
//
// All fields optional. `Sort` accepts "alpha", "date", "popularity", "style",
// "trending" per the API spec. `Subset`, `Category` and `Family` correspond
// to the eponymous query parameters; if `Family` lists multiple names they
// are sent as repeated `family=` params (which the API supports to narrow
// down to a specific set).
type ListOptions struct {
	Sort     string   // alpha|date|popularity|style|trending
	Subset   string   // e.g. "latin"
	Category []string // e.g. ["sans-serif","serif"]
	Family   []string // specific family names
	Capability []string // e.g. ["VF"] (variable), ["WOFF2"]
}

// ListResponse mirrors the JSON body returned by the webfonts endpoint.
type ListResponse struct {
	Kind  string       `json:"kind"`
	Items []FontFamily `json:"items"`
}

// FontFamily models a single element in the `items` array.
// See https://developers.google.com/fonts/docs/developer_api#api_response.
type FontFamily struct {
	Family       string            `json:"family"`
	Variants     []string          `json:"variants"`
	Subsets      []string          `json:"subsets"`
	Version      string            `json:"version"`
	LastModified string            `json:"lastModified"`
	Files        map[string]string `json:"files"`
	Category     string            `json:"category"`
	Kind         string            `json:"kind"`
	Menu         string            `json:"menu,omitempty"`
	// Variable-font axis ranges (only present when capability=VF is
	// requested).
	Axes []AxisRange `json:"axes,omitempty"`
	// Colour capability returned by the API when the font includes a COLR
	// table.
	ColorCapabilities []string `json:"colorCapabilities,omitempty"`
}

type AxisRange struct {
	Tag   string  `json:"tag"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// ListWebfonts calls GET /v1/webfonts and returns the decoded response.
func (c *Client) ListWebfonts(ctx context.Context, opts ListOptions) (*ListResponse, error) {
	if c == nil || c.apiKey == "" {
		return nil, errors.New("gfapi: missing API key")
	}
	u, err := url.Parse(c.base)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("key", c.apiKey)
	if opts.Sort != "" {
		q.Set("sort", opts.Sort)
	}
	if opts.Subset != "" {
		q.Set("subset", opts.Subset)
	}
	for _, cat := range opts.Category {
		q.Add("category", cat)
	}
	for _, fam := range opts.Family {
		q.Add("family", fam)
	}
	if len(opts.Capability) > 0 {
		q.Set("capability", strings.Join(opts.Capability, ","))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("gfapi: HTTP %d: %s", resp.StatusCode, string(body))
	}
	var out ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gfapi: decoding response: %w", err)
	}
	return &out, nil
}

// Download fetches a font file URL (typically one of the values in
// FontFamily.Files) and returns its raw bytes. This is a convenience for
// callers feeding results straight into fontcodec.Decode.
func (c *Client) Download(ctx context.Context, fileURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("gfapi: download HTTP %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}
