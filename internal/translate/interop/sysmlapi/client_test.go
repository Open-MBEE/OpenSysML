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
	"sync/atomic"
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
	for _, elsewhere := range []string{"https://other.example/projects", "https://evil.api.example/projects", "https://api.example:8443/projects"} {
		if err := withToken.checkRedirect(request(elsewhere), []*http.Request{from}); err == nil {
			t.Errorf("a redirect to %s carried the token", elsewhere)
		} else if strings.Contains(err.Error(), "secret") {
			t.Errorf("the refusal names the token: %v", err)
		}
	}
	for _, canonical := range []string{"https://API.example/projects", "https://api.example:443/projects"} {
		if err := withToken.checkRedirect(request(canonical), []*http.Request{from}); err != nil {
			t.Errorf("a redirect to %s, the same server, was refused: %v", canonical, err)
		}
	}
	explicit := New(Config{BaseURL: "https://api.example:443", Token: "secret"})
	if err := explicit.checkRedirect(request("https://api.example/projects"), []*http.Request{from}); err != nil {
		t.Errorf("a redirect dropping the default port was refused: %v", err)
	}
	loopback := New(Config{BaseURL: "https://localhost:8443", Token: "secret"})
	if err := loopback.checkRedirect(request("http://localhost:8083/projects"), []*http.Request{from}); err == nil {
		t.Error("a redirect to another port of loopback carried the token")
	}
	if err := loopback.checkRedirect(request("http://localhost:8443/projects"), []*http.Request{from}); err == nil {
		t.Error("a downgrade to plaintext on loopback carried the token")
	}
	t.Setenv(EnvPlainHTTP, "1")
	plain, _ := http.NewRequest(http.MethodGet, "http://localhost:8083/projects", nil)
	plainBase := New(Config{BaseURL: "http://localhost:8083", Token: "secret"})
	if err := plainBase.checkRedirect(request("http://localhost:8083/projects/"), []*http.Request{plain}); err != nil {
		t.Errorf("a plain redirect of a plain loopback request was refused: %v", err)
	}
	if err := plainBase.checkRedirect(request("https://localhost:8083/projects/"), []*http.Request{plain}); err != nil {
		t.Errorf("an upgrade to https was refused: %v", err)
	}
	t.Setenv(EnvPlainHTTP, "")
	via := make([]*http.Request, maxRedirects)
	for i := range via {
		via[i] = from
	}
	if err := withToken.checkRedirect(request("https://api.example/projects"), via); err == nil {
		t.Error("an endless redirect chain was followed")
	}
	tokenless := New(Config{BaseURL: "https://api.example"})
	if err := tokenless.checkRedirect(request("http://other.example/projects/"), []*http.Request{from}); err != nil {
		t.Errorf("without a token, a redirect elsewhere was refused: %v", err)
	}
}

// TestLinkedPagesKeepTheTokenOnTheServer: a next-page link is held to the rule
// a redirect is, so a server cannot have the token sent on to another host.
func TestLinkedPagesKeepTheTokenOnTheServer(t *testing.T) {
	t.Setenv(EnvPlainHTTP, "1")
	var elsewhere int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&elsewhere, 1)
		_ = json.NewEncoder(w).Encode([]map[string]any{{"@id": "p2", "@type": "Project", "name": "Two"}})
	}))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", fmt.Sprintf(`<%s%s>; rel="next"`, other.URL, r.URL.RequestURI()))
		_ = json.NewEncoder(w).Encode([]map[string]any{{"@id": "p1", "@type": "Project", "name": "One"}})
	}))
	defer server.Close()
	otherHost, serverHost := strings.TrimPrefix(other.URL, "http://"), strings.TrimPrefix(server.URL, "http://")

	c := New(Config{BaseURL: server.URL, Token: "secret"})
	for name, list := range map[string]func() error{
		"projects": func() error { _, err := c.Projects(context.Background()); return err },
		"elements": func() error { _, err := c.Elements(context.Background(), "p", "c", 1); return err },
	} {
		err := list()
		if err == nil {
			t.Fatalf("%s: a page on another server was read with the token", name)
		}
		if !strings.Contains(err.Error(), otherHost) || !strings.Contains(err.Error(), serverHost) {
			t.Errorf("%s: the refusal names neither %s nor %s: %v", name, otherHost, serverHost, err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: the refusal names the token: %v", name, err)
		}
	}
	if n := atomic.LoadInt32(&elsewhere); n != 0 {
		t.Errorf("the other server was asked %d time(s)", n)
	}

	tokenless := New(Config{BaseURL: server.URL})
	if projects, err := tokenless.Projects(context.Background()); err != nil || len(projects) != 2 {
		t.Errorf("without a token, the linked page was not followed: %d projects, %v", len(projects), err)
	}
}
