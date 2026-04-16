package gfapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestListWebfontsQueryParams(t *testing.T) {
	var gotURL *url.URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL
		_ = json.NewEncoder(w).Encode(ListResponse{
			Kind: "webfonts#webfontList",
			Items: []FontFamily{{Family: "Noto Sans", Category: "sans-serif"}},
		})
	}))
	defer srv.Close()

	c := New("test-key", srv.Client())
	c.base = srv.URL
	resp, err := c.ListWebfonts(context.Background(), ListOptions{
		Sort:       "popularity",
		Subset:     "latin",
		Category:   []string{"sans-serif"},
		Family:     []string{"Noto Sans", "Roboto"},
		Capability: []string{"VF", "WOFF2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Family != "Noto Sans" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	q := gotURL.Query()
	if q.Get("key") != "test-key" {
		t.Errorf("key = %q", q.Get("key"))
	}
	if q.Get("sort") != "popularity" {
		t.Errorf("sort = %q", q.Get("sort"))
	}
	if got := q["family"]; len(got) != 2 {
		t.Errorf("family = %v", got)
	}
	if !strings.Contains(q.Get("capability"), "VF") {
		t.Errorf("capability = %q", q.Get("capability"))
	}
}

func TestListWebfontsRequiresKey(t *testing.T) {
	c := New("", nil)
	if _, err := c.ListWebfonts(context.Background(), ListOptions{}); err == nil {
		t.Fatal("expected error for empty API key")
	}
}
