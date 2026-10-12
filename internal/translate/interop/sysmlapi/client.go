// Package sysmlapi drives any server speaking the standard SysML v2 API: the
// projects, branches, commits and elements resources the OMG pilot's
// SysML-v2-API-Services and Flexo MMS alike serve. It knows nothing of a
// server's extras — Flexo's Layer 1, orgs and Turtle graph writes live in
// package flexo, which builds on this client.
package sysmlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment variables a client is configured from. The base URL and the token
// are the ones `sysml -sync` reads, so a REPL and the command line address one
// server the same way; the token is optional, as the pilot's server takes none.
const (
	EnvBaseURL   = "FLEXO_SYSMLV2_URL"
	EnvToken     = "FLEXO_INTEROP_TOKEN" // #nosec G101 -- a variable name, not a credential
	EnvPlainHTTP = "FLEXO_ALLOW_PLAIN_HTTP"
)

// DefaultBaseURL is the SysML v2 API port of flexo-mms-sysmlv2's compose file.
const DefaultBaseURL = "http://localhost:8083"

// DefaultTimeout bounds one request when the configuration names none.
const DefaultTimeout = 60 * time.Second

// pageSize is how many items one listing request asks for.
const pageSize = 200

// maxPages bounds a listing against a server that pages forever.
const maxPages = 1000

// maxRedirects is how many redirects one request follows, as net/http does.
const maxRedirects = 10

// Config addresses one SysML v2 API server.
type Config struct {
	BaseURL string        // the API's base path, without a trailing slash
	Token   string        // bearer token, sent only when set
	Timeout time.Duration // per-request bound; DefaultTimeout when zero
}

// ConfigFromEnv reads the base URL and the token from the environment; the URL
// defaults to the compose file's port, the token to none.
func ConfigFromEnv() Config {
	base := os.Getenv(EnvBaseURL)
	if base == "" {
		base = DefaultBaseURL
	}
	return Config{BaseURL: strings.TrimRight(base, "/"), Token: os.Getenv(EnvToken), Timeout: DefaultTimeout}
}

// PlaintextError is a bearer token about to cross the network unencrypted: an
// http:// URL whose host is not this machine.
type PlaintextError struct {
	URL string
}

func (e *PlaintextError) Error() string {
	return fmt.Sprintf("%s would send the bearer token in the clear; use https://, or set %s=1 for a server you trust the network to", e.URL, EnvPlainHTTP)
}

// CheckURL refuses a plaintext URL to anything but a loopback host, unless the
// environment opts in with FLEXO_ALLOW_PLAIN_HTTP=1, and any URL that is not
// http(s).
func CheckURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("server url %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("server url %q is not an http(s) URL", raw)
	}
	if u.Scheme == "http" && !Loopback(u.Hostname()) && os.Getenv(EnvPlainHTTP) != "1" {
		return &PlaintextError{URL: raw}
	}
	return nil
}

// Loopback reports a host name that resolves to this machine without a lookup.
func Loopback(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Client talks to one server. It is safe for sequential use.
type Client struct {
	cfg  Config
	http *http.Client
}

// New returns a client for cfg.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	c := &Client{cfg: cfg}
	c.http = &http.Client{Timeout: timeout, CheckRedirect: c.checkRedirect}
	return c
}

// checkRedirect holds a redirect to the rule of the first request when there
// is a bearer token to carry: it stays on the server's host, off plaintext and
// on https once there, so a redirect cannot take the token elsewhere or into
// the clear — not even on loopback, where a first request may be plain.
func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	from := via[len(via)-1].URL
	if err := c.tokenStays(from, req.URL); err != nil {
		return fmt.Errorf("redirect from %s refused: %w", from, err)
	}
	return nil
}

// tokenStays is the rule a request's successor — a redirect's target, a linked
// next page — is held to when there is a bearer token to carry: no plaintext,
// no leaving https once there, and no other server than the configured one.
func (c *Client) tokenStays(from, to *url.URL) error {
	if c.cfg.Token == "" {
		return nil
	}
	if err := CheckURL(to.String()); err != nil {
		return err
	}
	if from.Scheme == "https" && to.Scheme != "https" {
		return fmt.Errorf("%s: the token stays on https", to.Scheme)
	}
	if base, err := url.Parse(c.cfg.BaseURL); err == nil && !sameServer(to, base) {
		return fmt.Errorf("%s: the token is for %s only", to.Host, base.Host)
	}
	return nil
}

// sameServer compares two URLs by host name and effective port, so that
// https://api.example and https://api.example:443 are one server.
func sameServer(a, b *url.URL) bool {
	return strings.EqualFold(a.Hostname(), b.Hostname()) && effectivePort(a) == effectivePort(b)
}

func effectivePort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// Config returns the configuration the client was built with.
func (c *Client) Config() Config { return c.cfg }

// BaseURL is the server's base path.
func (c *Client) BaseURL() string { return c.cfg.BaseURL }

// StatusError is a response outside 2xx. It carries the body, because servers
// answer with their diagnostic there.
type StatusError struct {
	Method string
	URL    string
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s %s: %s: %s", e.Method, e.URL, http.StatusText(e.Status), e.Message())
}

// Message is the server's diagnostic: the body's message field when it is a
// JSON object stating one, else the body itself, trimmed to a line's worth.
func (e *StatusError) Message() string {
	body := strings.TrimSpace(e.Body)
	var object map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &object) == nil {
		for _, key := range []string{"message", "error", "detail"} {
			var text string
			if raw, ok := object[key]; ok && json.Unmarshal(raw, &text) == nil && text != "" {
				body = text
				break
			}
		}
	}
	if len(body) > 400 {
		body = body[:400] + "..."
	}
	return body
}

// Status returns the HTTP status of a failed request, or 0 if err is not one.
func Status(err error) int {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Status
	}
	return 0
}

// UnreachableError is a server that answered nothing: the connection failed
// before any status came back.
type UnreachableError struct {
	URL string
	Err error
}

func (e *UnreachableError) Error() string {
	return fmt.Sprintf("the server at %s did not answer: %v", e.URL, e.Err)
}

func (e *UnreachableError) Unwrap() error { return e.Err }

// Do performs one request, authorized when the configuration holds a token,
// and returns the response body and headers, failing on any status outside 2xx.
func (c *Client) Do(ctx context.Context, method, target string, body []byte, contentType string, headers map[string]string) ([]byte, http.Header, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, nil, err
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, &UnreachableError{URL: c.cfg.BaseURL, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return content, resp.Header, &StatusError{Method: method, URL: target, Status: resp.StatusCode, Body: string(content)}
	}
	return content, resp.Header, nil
}

func (c *Client) getJSON(ctx context.Context, target string, into any, what string) error {
	content, _, err := c.Do(ctx, http.MethodGet, target, nil, "", nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, into); err != nil {
		return fmt.Errorf("decode %s: %w", what, err)
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, target string, request any, into any, what string) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	content, _, err := c.Do(ctx, http.MethodPost, target, body, "application/json", nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, into); err != nil {
		return fmt.Errorf("decode %s: %w", what, err)
	}
	return nil
}

// Reachable reports whether the server answers: the project list is the
// cheapest read every server has.
func (c *Client) Reachable(ctx context.Context) error {
	_, _, err := c.Do(ctx, http.MethodGet, c.cfg.BaseURL+"/projects", nil, "", nil)
	return err
}

// Ref is a reference to another resource, as the API spells it.
type Ref struct {
	ID string `json:"@id"`
}

// Project is one project as the API describes it.
type Project struct {
	ID            string `json:"@id"`
	Name          string `json:"name"`
	DefaultBranch Ref    `json:"defaultBranch"`
}

// Branch is one branch as the API describes it: its id, its name and the
// commit at its head.
type Branch struct {
	ID   string `json:"@id"`
	Name string `json:"name"`
	Head Commit `json:"head"`
}

// Commit identifies one commit of a project.
type Commit struct {
	ID   string `json:"@id"`
	Type string `json:"@type"`
}

// Element is one element as the API delivers it: property name to raw JSON
// value, kept raw so a value's shape — reference, literal, array — is
// observable rather than normalized away by decoding into a Go type.
type Element map[string]json.RawMessage

// ID returns an element's identity, preferring @id over elementId.
func (e Element) ID() string {
	for _, key := range []string{"@id", "elementId"} {
		if raw, ok := e[key]; ok {
			var id string
			if json.Unmarshal(raw, &id) == nil && id != "" {
				return id
			}
		}
	}
	return ""
}

// Type returns an element's @type, or "" when it carries none.
func (e Element) Type() string {
	var name string
	if raw, ok := e["@type"]; ok && json.Unmarshal(raw, &name) == nil {
		return name
	}
	return ""
}

func (c *Client) projectPath(project string) string {
	return c.cfg.BaseURL + "/projects/" + url.PathEscape(project)
}

func (c *Client) commitPath(project, commit string) string {
	return c.projectPath(project) + "/commits/" + url.PathEscape(commit)
}

// paged walks one collection to its end. The API spells the paging parameters
// `page[size]` and `page[after]` and links the next page in a Link header;
// Flexo spells them `pageSize` and `pageAfter` and links nothing, so both
// spellings are sent and the next page is the linked one, else the one after
// the last item. A short page ends the walk; so does a page repeating an item.
func (c *Client) paged(ctx context.Context, target string, what string, each func(json.RawMessage) (id string, fresh bool)) error {
	next := withPaging(target, "")
	for page := 1; page <= maxPages; page++ {
		content, header, err := c.Do(ctx, http.MethodGet, next, nil, "", nil)
		if err != nil {
			return err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(content, &items); err != nil {
			return fmt.Errorf("decode %s page %d: %w", what, page, err)
		}
		last, repeated := "", false
		for _, item := range items {
			id, fresh := each(item)
			if !fresh {
				repeated = true
			}
			last = id
		}
		if repeated || last == "" {
			return nil
		}
		// A server linking its pages is followed to the end whatever each
		// page holds; without a link, a short page is the last.
		linked, err := c.nextPage(header, next)
		if err != nil {
			return err
		}
		if linked != "" && linked != next {
			next = linked
			continue
		}
		if len(items) < pageSize {
			return nil
		}
		next = withPaging(target, last)
	}
	return fmt.Errorf("%s: the server paged on past %d pages", what, maxPages)
}

// withPaging adds both spellings of the page size and, when after is set, of
// the cursor to a collection URL.
func withPaging(target, after string) string {
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	size := strconv.Itoa(pageSize)
	out := target + sep + "page%5Bsize%5D=" + size + "&pageSize=" + size
	if after != "" {
		out += "&page%5Bafter%5D=" + url.QueryEscape(after) + "&pageAfter=" + url.QueryEscape(after)
	}
	return out
}

// nextPage is the page a response links as next, held to the rule a redirect
// is: a link to another server, or into the clear, is refused rather than
// followed with the token.
func (c *Client) nextPage(header http.Header, requested string) (string, error) {
	linked := nextLink(header, requested)
	if linked == "" {
		return "", nil
	}
	from, err := url.Parse(requested)
	if err != nil {
		return "", err
	}
	to, err := url.Parse(linked)
	if err != nil {
		return "", err
	}
	if err := c.tokenStays(from, to); err != nil {
		return "", fmt.Errorf("next page linked from %s refused: %w", from, err)
	}
	return linked, nil
}

// nextLink is the rel="next" target of a Link header, resolved against the
// request's URL; empty when the response links no next page.
func nextLink(header http.Header, requested string) string {
	base, err := url.Parse(requested)
	if err != nil {
		return ""
	}
	for _, field := range header.Values("Link") {
		for _, link := range strings.Split(field, ",") {
			target, params, _ := strings.Cut(strings.TrimSpace(link), ";")
			if !strings.Contains(params, `rel="next"`) && !strings.Contains(params, "rel=next") {
				continue
			}
			target = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(target), "<"), ">")
			u, err := url.Parse(target)
			if err != nil {
				return ""
			}
			return base.ResolveReference(u).String()
		}
	}
	return ""
}

// Projects lists every project, paged through to the end.
func (c *Client) Projects(ctx context.Context) ([]Project, error) {
	var projects []Project
	seen := make(map[string]bool)
	err := c.paged(ctx, c.cfg.BaseURL+"/projects", "projects", func(raw json.RawMessage) (string, bool) {
		var p Project
		if json.Unmarshal(raw, &p) != nil || p.ID == "" || seen[p.ID] {
			return p.ID, false
		}
		seen[p.ID] = true
		projects = append(projects, p)
		return p.ID, true
	})
	return projects, err
}

// Project reads one project by id.
func (c *Client) Project(ctx context.Context, id string) (Project, error) {
	var p Project
	err := c.getJSON(ctx, c.projectPath(id), &p, "project")
	return p, err
}

// CreateProject creates a project. An id names the project when the server
// honours client ids, as Flexo does; a default branch names the branch the
// server creates with it. Either empty leaves the choice to the server.
func (c *Client) CreateProject(ctx context.Context, id, name, branch string) (Project, error) {
	request := map[string]any{"@type": "Project", "name": name}
	if id != "" {
		request["@id"] = id
	}
	if branch != "" {
		request["defaultBranch"] = map[string]any{"@id": branch, "name": branch}
	}
	var created Project
	if err := c.postJSON(ctx, c.cfg.BaseURL+"/projects", request, &created, "created project"); err != nil {
		return Project{}, err
	}
	if created.ID == "" {
		created.ID = id
	}
	if created.Name == "" {
		created.Name = name
	}
	return created, nil
}

// Branches lists every branch of a project, paged through to the end.
func (c *Client) Branches(ctx context.Context, project string) ([]Branch, error) {
	var branches []Branch
	seen := make(map[string]bool)
	err := c.paged(ctx, c.projectPath(project)+"/branches", "branches", func(raw json.RawMessage) (string, bool) {
		var b Branch
		if json.Unmarshal(raw, &b) != nil || b.ID == "" || seen[b.ID] {
			return b.ID, false
		}
		seen[b.ID] = true
		branches = append(branches, b)
		return b.ID, true
	})
	return branches, err
}

// Branch reads one branch of a project, head commit included.
func (c *Client) Branch(ctx context.Context, project, branch string) (Branch, error) {
	var b Branch
	err := c.getJSON(ctx, c.projectPath(project)+"/branches/"+url.PathEscape(branch), &b, "branch")
	return b, err
}

// Commits lists a project's commits, in the order the server returns them.
func (c *Client) Commits(ctx context.Context, project string) ([]Commit, error) {
	var commits []Commit
	err := c.getJSON(ctx, c.projectPath(project)+"/commits", &commits, "commits")
	return commits, err
}

// PostChanges commits a CommitRequest body through the server's commit path
// and returns the commit it made. An empty branch takes the default one.
func (c *Client) PostChanges(ctx context.Context, project, branch string, changes []byte) (Commit, error) {
	target := c.projectPath(project) + "/commits"
	if branch != "" {
		target += "?branchId=" + url.QueryEscape(branch)
	}
	content, _, err := c.Do(ctx, http.MethodPost, target, changes, "application/json", nil)
	if err != nil {
		return Commit{}, err
	}
	var commit Commit
	if err := json.Unmarshal(content, &commit); err != nil {
		return Commit{}, fmt.Errorf("decode commit: %w", err)
	}
	return commit, nil
}

// Listing is one element listing: the elements it delivered, the number of
// responses it took, and whether the server ignored the paging parameters.
type Listing struct {
	Elements      []Element
	Responses     int
	IgnoredPaging bool
}

// Elements reads a commit's elements a page at a time. A page that overruns
// size or repeats an element is a server ignoring paging; the repeat is
// dropped rather than counted twice.
func (c *Client) Elements(ctx context.Context, project, commit string, size int) (Listing, error) {
	var listing Listing
	seen := make(map[string]bool)
	first := c.commitPath(project, commit) + "/elements?pageSize=" + strconv.Itoa(size) + "&page%5Bsize%5D=" + strconv.Itoa(size)
	target := first
	for {
		content, header, err := c.Do(ctx, http.MethodGet, target, nil, "", nil)
		if err != nil {
			return listing, err
		}
		var page []Element
		if err := json.Unmarshal(content, &page); err != nil {
			return listing, fmt.Errorf("decode elements page %d: %w", listing.Responses+1, err)
		}
		listing.Responses++

		fresh := 0
		for _, element := range page {
			if id := element.ID(); id != "" {
				if seen[id] {
					continue
				}
				seen[id] = true
			}
			listing.Elements = append(listing.Elements, element)
			fresh++
		}

		if len(page) > size || fresh < len(page) {
			listing.IgnoredPaging = true
			return listing, nil
		}
		if len(page) == 0 || listing.Responses > maxPages {
			return listing, nil
		}
		linked, err := c.nextPage(header, target)
		if err != nil {
			return listing, err
		}
		switch {
		case linked != "" && linked != target:
			// A server linking its pages names the continuation itself,
			// whatever its cursor is; the link is followed as given.
			target = linked
		case len(page) < size:
			// Without a link to a next page, a short page is the last.
			return listing, nil
		default:
			last := page[len(page)-1].ID()
			if last == "" {
				return listing, nil
			}
			target = first + "&pageAfter=" + url.QueryEscape(last) + "&page%5Bafter%5D=" + url.QueryEscape(last)
		}
	}
}

// ElementByID reads one element directly. A non-2xx answer is returned as the
// error, with Status(err) carrying the code.
func (c *Client) ElementByID(ctx context.Context, project, commit, id string) (Element, error) {
	var element Element
	err := c.getJSON(ctx, c.commitPath(project, commit)+"/elements/"+url.PathEscape(id), &element, "element "+id)
	return element, err
}

// Roots lists the elements the server considers roots of a commit: those with
// neither an owner nor an owning related element.
func (c *Client) Roots(ctx context.Context, project, commit string) ([]Element, error) {
	var roots []Element
	err := c.getJSON(ctx, c.commitPath(project, commit)+"/roots", &roots, "roots")
	return roots, err
}
