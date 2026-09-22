// Package flexo drives a running Flexo MMS stack — the Layer 1 service and the
// SysML v2 API in front of it — so the RDF this project writes can be measured
// against what that service can actually read back, and so a project branch
// can be read and written as a model's repository: `sysml -sync-diff` and
// `-sync-apply` sync element-wise, `sysml -convert` reads a branch URL as
// notation or pushes a whole Turtle graph to it.
//
// The stack is external and opt-in; see .agents/skills/flexo-interop for how to
// bring it up and what the gate reports.
package flexo

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

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const projectsPath = "/projects/"

// Environment variables read by ConfigFromEnv. FLEXO_INTEROP is the gate: the
// live test skips unless it is set, exactly as the corpus gates skip on an
// absent corpus.
const (
	EnvGate       = "FLEXO_INTEROP"
	EnvLayer1URL  = "FLEXO_LAYER1_URL"
	EnvSysMLV2URL = "FLEXO_SYSMLV2_URL"
	EnvToken      = "FLEXO_INTEROP_TOKEN" // #nosec G101 -- a variable name, not a credential
	EnvOrg        = "FLEXO_SYSMLV2_ORG"
	EnvPlainHTTP  = "FLEXO_ALLOW_PLAIN_HTTP"
)

// Defaults matching flexo-mms-sysmlv2/docker-compose/docker-compose.yml, so a
// stack brought up from that file needs no configuration beyond the token.
const (
	DefaultLayer1URL  = "http://localhost:8080"
	DefaultSysMLV2URL = "http://localhost:8083"
	DefaultOrg        = "sysmlv2"
)

// mediaTurtle is the RDF media type both services speak.
const mediaTurtle = "text/turtle"

// Config addresses one running stack.
type Config struct {
	Layer1URL  string // Layer 1 service, which owns orgs, repos, branches and graphs
	SysMLV2URL string // SysML v2 API, which owns projects, commits and elements
	Token      string // bearer token accepted by both services
	Org        string // Layer 1 org the SysML v2 service keeps its projects under
	Timeout    time.Duration
}

// ConfigFromEnv reads the stack's addresses from the environment, defaulting to
// the published compose file's ports. The token has no default: it is a
// credential and is never written to the repository.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Layer1URL:  envOr(EnvLayer1URL, DefaultLayer1URL),
		SysMLV2URL: envOr(EnvSysMLV2URL, DefaultSysMLV2URL),
		Token:      os.Getenv(EnvToken),
		Org:        envOr(EnvOrg, DefaultOrg),
		Timeout:    60 * time.Second,
	}
	if cfg.Token == "" {
		return cfg, fmt.Errorf("%s is unset, so neither service will authorize the run", EnvToken)
	}
	return cfg, nil
}

// PlaintextError is a bearer token about to cross the network unencrypted: an
// http:// URL whose host is not this machine.
type PlaintextError struct {
	URL string
}

func (e *PlaintextError) Error() string {
	return fmt.Sprintf("%s would send the bearer token in the clear; use https://, or set %s=1 for a stack you trust the network to", e.URL, EnvPlainHTTP)
}

// CheckTransport refuses a token over plaintext to anything but a loopback
// host, unless the environment opts in with FLEXO_ALLOW_PLAIN_HTTP=1.
func (cfg Config) CheckTransport() error {
	if os.Getenv(EnvPlainHTTP) == "1" {
		return nil
	}
	for _, raw := range []string{cfg.SysMLV2URL, cfg.Layer1URL} {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("stack url %q: %w", raw, err)
		}
		if u.Scheme == "http" && !loopback(u.Hostname()) {
			return &PlaintextError{URL: raw}
		}
	}
	return nil
}

// loopback reports a host name that resolves to this machine without a lookup.
func loopback(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// Client talks to one stack. It is safe for sequential use by one test.
type Client struct {
	cfg  Config
	http *http.Client
}

// New returns a client for cfg.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: timeout}}
}

// Config returns the configuration the client was built with.
func (c *Client) Config() Config { return c.cfg }

// statusError is a response the harness did not ask for. It carries the body,
// because both services answer with a diagnostic there.
type statusError struct {
	method string
	url    string
	status int
	body   string
}

func (e *statusError) Error() string {
	body := strings.TrimSpace(e.body)
	if len(body) > 400 {
		body = body[:400] + "..."
	}
	return fmt.Sprintf("%s %s: %s: %s", e.method, e.url, http.StatusText(e.status), body)
}

// Status returns the HTTP status of a failed request, or 0 if err is not one.
func Status(err error) int {
	var se *statusError
	if errors.As(err, &se) {
		return se.status
	}
	return 0
}

// do performs one authorized request and returns the response body, failing on
// any status outside 2xx.
func (c *Client) do(ctx context.Context, method, url string, body []byte, contentType string, headers map[string]string) ([]byte, http.Header, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return content, resp.Header, &statusError{method: method, url: url, status: resp.StatusCode, body: string(content)}
	}
	return content, resp.Header, nil
}

// Reachable reports whether both services answer. The SysML v2 project list is
// the cheapest authorized read on the stack.
func (c *Client) Reachable(ctx context.Context) error {
	if _, _, err := c.do(ctx, http.MethodGet, c.cfg.SysMLV2URL+"/projects", nil, "", nil); err != nil {
		return fmt.Errorf("sysmlv2 at %s: %w", c.cfg.SysMLV2URL, err)
	}
	// The org list, not the org itself: a fresh cluster has no org yet.
	if _, _, err := c.do(ctx, http.MethodGet, c.cfg.Layer1URL+"/orgs", nil, "",
		map[string]string{"Accept": mediaTurtle}); err != nil {
		return fmt.Errorf("layer1 at %s: %w", c.cfg.Layer1URL, err)
	}
	return nil
}

func (c *Client) orgURL() string {
	return c.cfg.Layer1URL + "/orgs/" + url.PathEscape(c.cfg.Org)
}

// EnsureOrg creates the Layer 1 org the SysML v2 service keeps its projects
// under. A freshly initialized cluster does not have it, and every project
// creation fails until it does.
func (c *Client) EnsureOrg(ctx context.Context) error {
	if _, _, err := c.do(ctx, http.MethodGet, c.orgURL(), nil, "", map[string]string{"Accept": mediaTurtle}); err == nil {
		return nil
	}
	body := fmt.Sprintf("<> <http://purl.org/dc/terms/title> %q .\n", c.cfg.Org)
	_, _, err := c.do(ctx, http.MethodPut, c.orgURL(), []byte(body), mediaTurtle, nil)
	return err
}

// Project is the part of a created project the harness uses.
type Project struct {
	ID   string
	Name string
}

// CreateProject creates a project through the SysML v2 API, which also creates
// its default branch and the queries scratch. The id doubles as the Layer 1 repo
// id, so it must satisfy the service's id rules; the default branch is named
// rather than left to the service, which would otherwise mint a uuid the graph
// endpoint's path needs.
func (c *Client) CreateProject(ctx context.Context, id, name, branch string) (Project, error) {
	request := map[string]any{
		"@type":         "Project",
		"@id":           id,
		"name":          name,
		"defaultBranch": map[string]any{"@id": branch},
	}
	body, err := json.Marshal(request)
	if err != nil {
		return Project{}, err
	}
	content, _, err := c.do(ctx, http.MethodPost, c.cfg.SysMLV2URL+"/projects", body, "application/json", nil)
	if err != nil {
		return Project{}, err
	}
	var created struct {
		ID   string `json:"@id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(content, &created); err != nil {
		return Project{}, fmt.Errorf("decode created project: %w", err)
	}
	if created.ID == "" {
		created.ID = id
	}
	return Project{ID: created.ID, Name: created.Name}, nil
}

// branchGraphURL is the Graph Store Protocol endpoint of one branch's model.
func (c *Client) branchGraphURL(project, branch string) string {
	return fmt.Sprintf("%s/orgs/%s/repos/%s/branches/%s/graph",
		c.cfg.Layer1URL, url.PathEscape(c.cfg.Org), url.PathEscape(project), url.PathEscape(branch))
}

// LoadTurtle replaces a branch's model graph unconditionally: PutGraph with
// If-Match *, the precondition the measurement harness loads under.
func (c *Client) LoadTurtle(ctx context.Context, project, branch string, turtle []byte, message string) error {
	_, err := c.PutGraph(ctx, project, branch, turtle, message, "*")
	return err
}

// PutResult is the ETag and Location a graph write's response carries; a
// committed write has both, a refused 412 neither.
type PutResult struct {
	Commit   string
	Location string
}

// PutGraph replaces a branch's model graph, conditional on ifMatch ("*" = any).
// 412 also answers a committed write, so the response headers are returned
// either way.
func (c *Client) PutGraph(ctx context.Context, project, branch string, turtle []byte, message, ifMatch string) (PutResult, error) {
	target := c.branchGraphURL(project, branch)
	if message != "" {
		target += "?message=" + url.QueryEscape(message)
	}
	// Layer 1 parses entity tags quoted; the star qualifier is sent bare.
	if ifMatch != "*" {
		ifMatch = "\"" + ifMatch + "\""
	}
	headers := map[string]string{"If-Match": ifMatch}
	_, header, err := c.do(ctx, http.MethodPut, target, turtle, mediaTurtle, headers)
	return PutResult{
		Commit:   strings.Trim(header.Get("ETag"), `"`),
		Location: header.Get("Location"),
	}, err
}

// BranchETag reads the etag Layer 1's branch resource carries — the entity tag
// a conditional graph write quotes as its If-Match.
func (c *Client) BranchETag(ctx context.Context, project, branch string) (string, error) {
	target := fmt.Sprintf("%s/orgs/%s/repos/%s/branches/%s",
		c.cfg.Layer1URL, url.PathEscape(c.cfg.Org), url.PathEscape(project), url.PathEscape(branch))
	_, header, err := c.do(ctx, http.MethodGet, target, nil, "", map[string]string{"Accept": mediaTurtle})
	if err != nil {
		return "", err
	}
	etag := header.Get("ETag")
	if etag == "" {
		return "", fmt.Errorf("branch %s of %s answered without an ETag, so a conditional write cannot guard the head", branch, project)
	}
	return strings.Trim(etag, "\""), nil
}

// PostChanges commits SysML v2 JSON changes through the service's own commit
// path and returns the commit it made. An empty branch takes the default one.
func (c *Client) PostChanges(ctx context.Context, project, branch string, changes []byte) (Commit, error) {
	target := c.cfg.SysMLV2URL + projectsPath + url.PathEscape(project) + "/commits"
	if branch != "" {
		target += "?branchId=" + url.QueryEscape(branch)
	}
	content, _, err := c.do(ctx, http.MethodPost, target, changes, "application/json", nil)
	if err != nil {
		return Commit{}, err
	}
	var commit Commit
	if err := json.Unmarshal(content, &commit); err != nil {
		return Commit{}, fmt.Errorf("decode commit: %w", err)
	}
	return commit, nil
}

// Commit identifies one commit of a project.
type Commit struct {
	ID   string `json:"@id"`
	Type string `json:"@type"`
}

// Branch is one branch as the SysML v2 API describes it: its id and the
// commit at its head.
type Branch struct {
	ID   string `json:"@id"`
	Head Commit `json:"head"`
}

// Branch reads one branch of a project, head commit included.
func (c *Client) Branch(ctx context.Context, project, branch string) (Branch, error) {
	content, _, err := c.do(ctx, http.MethodGet,
		c.cfg.SysMLV2URL+projectsPath+url.PathEscape(project)+"/branches/"+url.PathEscape(branch), nil, "", nil)
	if err != nil {
		return Branch{}, err
	}
	var read Branch
	if err := json.Unmarshal(content, &read); err != nil {
		return Branch{}, fmt.Errorf("decode branch: %w", err)
	}
	return read, nil
}

// ErrBlankNode is a blank node in a graph read back from the stack; elements
// are IRIs, so a sync has no way to key one.
var ErrBlankNode = errors.New("the graph holds a blank node, which no element id can address")

// sparqlResults is the SPARQL 1.1 Query Results JSON Format, as far as a
// SELECT of triples uses it.
type sparqlResults struct {
	Results struct {
		Bindings []map[string]sparqlTerm `json:"bindings"`
	} `json:"results"`
}

type sparqlTerm struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Datatype string `json:"datatype"`
	Lang     string `json:"xml:lang"`
}

// term maps a result binding onto an RDF term; xsd:string folds into the
// plain literal it is equivalent to, as this project's graphs spell it.
func (t sparqlTerm) term() (rdf.Term, error) {
	switch t.Type {
	case "uri":
		return rdf.IRI(t.Value), nil
	case "literal", "typed-literal":
		switch {
		case t.Lang != "":
			return rdf.Term{Kind: rdf.TermLiteral, Value: t.Value, Lang: t.Lang}, nil
		case t.Datatype == "" || t.Datatype == rdf.XSD+"string":
			return rdf.String(t.Value), nil
		}
		return rdf.TypedLiteral(t.Value, t.Datatype), nil
	case "bnode":
		return rdf.Term{}, ErrBlankNode
	}
	return rdf.Term{}, fmt.Errorf("unknown SPARQL result term type %q", t.Type)
}

// versionGraphURL is Layer 1's SPARQL endpoint over one version of a repo:
// a branch, or the lock the SysML v2 service keeps per commit.
func (c *Client) versionGraphURL(project, version string) string {
	return fmt.Sprintf("%s/orgs/%s/repos/%s/%s/query",
		c.cfg.Layer1URL, url.PathEscape(c.cfg.Org), url.PathEscape(project), version)
}

// CommitGraph reads the model graph as one commit left it, through Layer 1's
// SPARQL endpoint, in the typed form the store holds rather than Turtle's shorthand.
func (c *Client) CommitGraph(ctx context.Context, project, commit string) (*rdf.Graph, error) {
	return c.selectGraph(ctx, c.versionGraphURL(project, "locks/"+url.PathEscape("Commit."+commit)))
}

func (c *Client) selectGraph(ctx context.Context, target string) (*rdf.Graph, error) {
	query := []byte("SELECT ?s ?p ?o WHERE { ?s ?p ?o } ORDER BY ?s ?p ?o")
	content, _, err := c.do(ctx, http.MethodPost, target, query, "application/sparql-query",
		map[string]string{"Accept": "application/sparql-results+json"})
	if err != nil {
		return nil, err
	}
	var results sparqlResults
	if err := json.Unmarshal(content, &results); err != nil {
		return nil, fmt.Errorf("decode SPARQL results: %w", err)
	}
	graph := rdf.NewGraph()
	for _, binding := range results.Results.Bindings {
		var triple rdf.Triple
		for name, into := range map[string]*rdf.Term{"s": &triple.Subject, "p": &triple.Predicate, "o": &triple.Object} {
			bound, ok := binding[name]
			if !ok {
				return nil, fmt.Errorf("SPARQL result binding lacks ?%s", name)
			}
			term, err := bound.term()
			if err != nil {
				return nil, err
			}
			*into = term
		}
		graph.AddTriple(triple)
	}
	return graph, nil
}

// Commits lists a project's commits, newest first as the service returns them.
func (c *Client) Commits(ctx context.Context, project string) ([]Commit, error) {
	content, _, err := c.do(ctx, http.MethodGet,
		c.cfg.SysMLV2URL+projectsPath+url.PathEscape(project)+"/commits", nil, "", nil)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	if err := json.Unmarshal(content, &commits); err != nil {
		return nil, fmt.Errorf("decode commits: %w", err)
	}
	return commits, nil
}

// Element is one element as the SysML v2 API delivers it: property name to raw
// JSON value, kept raw so a value's shape — reference, literal, array — is
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

func (c *Client) commitPath(project, commit string) string {
	return fmt.Sprintf("%s/projects/%s/commits/%s",
		c.cfg.SysMLV2URL, url.PathEscape(project), url.PathEscape(commit))
}

// Listing is one element listing: the elements it delivered, the number of
// responses it took, and whether the service ignored the paging parameters.
type Listing struct {
	Elements      []Element
	Responses     int
	IgnoredPaging bool
}

// Elements reads a commit's elements a page at a time. A page that overruns
// pageSize or repeats an element is a service ignoring paging; the repeat is
// dropped rather than counted twice.
func (c *Client) Elements(ctx context.Context, project, commit string, pageSize int) (Listing, error) {
	var listing Listing
	seen := make(map[string]bool)
	after := ""
	for {
		target := c.commitPath(project, commit) + "/elements?pageSize=" + strconv.Itoa(pageSize)
		if after != "" {
			target += "&pageAfter=" + url.QueryEscape(after)
		}
		content, _, err := c.do(ctx, http.MethodGet, target, nil, "", nil)
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

		switch {
		case len(page) > pageSize, fresh < len(page):
			listing.IgnoredPaging = true
			return listing, nil
		case len(page) < pageSize:
			return listing, nil
		}
		last := page[len(page)-1].ID()
		if last == "" || last == after || listing.Responses > 1000 {
			return listing, nil
		}
		after = last
	}
}

// ElementByID reads one element directly. A non-2xx answer is returned as the
// error, with Status(err) carrying the code: a rejected id is a finding, not a
// harness failure.
func (c *Client) ElementByID(ctx context.Context, project, commit, id string) (Element, error) {
	content, _, err := c.do(ctx, http.MethodGet,
		c.commitPath(project, commit)+"/elements/"+url.PathEscape(id), nil, "", nil)
	if err != nil {
		return nil, err
	}
	var element Element
	if err := json.Unmarshal(content, &element); err != nil {
		return nil, fmt.Errorf("decode element %s: %w", id, err)
	}
	return element, nil
}

// Roots lists the elements the service considers roots of a commit: those with
// neither an owner nor an owning related element.
func (c *Client) Roots(ctx context.Context, project, commit string) ([]Element, error) {
	content, _, err := c.do(ctx, http.MethodGet, c.commitPath(project, commit)+"/roots", nil, "", nil)
	if err != nil {
		return nil, err
	}
	var roots []Element
	if err := json.Unmarshal(content, &roots); err != nil {
		return nil, fmt.Errorf("decode roots: %w", err)
	}
	return roots, nil
}
