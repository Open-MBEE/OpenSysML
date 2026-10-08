package sysmlapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// TestElementsFollowLinksWithAnyCursor pages a server whose next links name an
// offset, not an id: every page is read as the server links it.
func TestElementsFollowLinksWithAnyCursor(t *testing.T) {
	const total = 7
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		if r.URL.Query().Get("page[after]") != "" || r.URL.Query().Get("pageAfter") != "" {
			http.Error(w, "this server pages by offset", http.StatusBadRequest)
			return
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("page[offset]"))
		var page []map[string]any
		for i := offset; i < total && len(page) < 3; i++ {
			page = append(page, map[string]any{"@id": fmt.Sprintf("e%d", i), "@type": "Package"})
		}
		if offset+len(page) < total {
			next := *r.URL
			q := next.Query()
			q.Set("page[offset]", strconv.Itoa(offset+len(page)))
			next.RawQuery = q.Encode()
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, next.String()))
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()

	listing, err := New(Config{BaseURL: server.URL}).Elements(context.Background(), "p", "c", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Elements) != total || listing.Responses != 3 || listing.IgnoredPaging {
		t.Fatalf("%d elements in %d responses (ignored paging: %v), want %d in 3\n%s",
			len(listing.Elements), listing.Responses, listing.IgnoredPaging, total, strings.Join(requests, "\n"))
	}
	for i, element := range listing.Elements {
		if element.ID() != fmt.Sprintf("e%d", i) {
			t.Errorf("element %d is %s", i, element.ID())
		}
	}
}

// TestRedirectsKeepTheTokenOffPlaintext: a redirect is held to the same rule as
// the first request, so an https server cannot send the token on to http.
func TestRedirectsKeepTheTokenOffPlaintext(t *testing.T) {
	t.Setenv(EnvPlainHTTP, "")
	from, _ := http.NewRequest(http.MethodGet, "https://api.example/projects", nil)
	request := func(target string) *http.Request {
		u, err := url.Parse(target)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Request{URL: u}
	}
	withToken := New(Config{BaseURL: "https://api.example", Token: "secret"})
	if err := withToken.checkRedirect(request("http://api.example/projects/"), []*http.Request{from}); err == nil {
		t.Error("a downgrade to http carried the token")
	} else if strings.Contains(err.Error(), "secret") {
		t.Errorf("the refusal names the token: %v", err)
	}
	if err := withToken.checkRedirect(request("https://api.example/v2/projects"), []*http.Request{from}); err != nil {
		t.Errorf("a redirect within https was refused: %v", err)
	}
	if err := withToken.checkRedirect(request("http://localhost:8083/projects"), []*http.Request{from}); err != nil {
		t.Errorf("a redirect to loopback was refused: %v", err)
	}
	via := make([]*http.Request, maxRedirects)
	for i := range via {
		via[i] = from
	}
	if err := withToken.checkRedirect(request("https://api.example/projects"), via); err == nil {
		t.Error("an endless redirect chain was followed")
	}
	tokenless := New(Config{BaseURL: "https://api.example"})
	if err := tokenless.checkRedirect(request("http://api.example/projects/"), []*http.Request{from}); err != nil {
		t.Errorf("without a token, a plaintext redirect was refused: %v", err)
	}
}
