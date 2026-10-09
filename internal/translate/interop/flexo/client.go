// Package flexo drives a running Flexo MMS stack — the Layer 1 service and the
// SysML v2 API in front of it — so the RDF this project writes can be measured
// against what that service can actually read back, and so a project branch
// can be read and written as a model's repository: `sysml -sync-diff` and
// `-sync-apply` sync element-wise, `sysml -convert` reads a branch URL as
// notation or pushes a whole Turtle graph to it.
//
// The standard API — projects, branches, commits, elements — is package
// sysmlapi's; this package adds what only Flexo has: the Layer 1 org a project
// lives under, its Turtle graph store, and the SPARQL read of a commit's graph.
//
// The stack is external and opt-in; see .agents/skills/flexo-interop for how to
// bring it up and what the gate reports.
package flexo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/sysmlapi"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Environment variables read by ConfigFromEnv. FLEXO_INTEROP is the gate: the
// live test skips unless it is set, exactly as the corpus gates skip on an
// absent corpus.
const (
	EnvGate       = "FLEXO_INTEROP"
	EnvLayer1URL  = "FLEXO_LAYER1_URL"
	EnvSysMLV2URL = sysmlapi.EnvBaseURL
	EnvToken      = sysmlapi.EnvToken
	EnvOrg        = "FLEXO_SYSMLV2_ORG"
	EnvPlainHTTP  = sysmlapi.EnvPlainHTTP
)

// Defaults matching flexo-mms-sysmlv2/docker-compose/docker-compose.yml, so a
// stack brought up from that file needs no configuration beyond the token.
const (
	DefaultLayer1URL  = "http://localhost:8080"
	DefaultSysMLV2URL = sysmlapi.DefaultBaseURL
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
type PlaintextError = sysmlapi.PlaintextError

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
		if u.Scheme == "http" && !sysmlapi.Loopback(u.Hostname()) {
			return &PlaintextError{URL: raw}
		}
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// Client talks to one stack: the standard API through a sysmlapi client, Layer
// 1 directly. It is safe for sequential use by one test.
type Client struct {
	cfg Config
	api *sysmlapi.Client
}

// New returns a client for cfg.
func New(cfg Config) *Client {
	api := sysmlapi.New(sysmlapi.Config{BaseURL: cfg.SysMLV2URL, Token: cfg.Token, Timeout: cfg.Timeout})
	return &Client{cfg: cfg, api: api}
}

// Config returns the configuration the client was built with.
func (c *Client) Config() Config { return c.cfg }

// API is the standard SysML v2 API client the stack is driven through.
func (c *Client) API() *sysmlapi.Client { return c.api }

// Status returns the HTTP status of a failed request, or 0 if err is not one.
func Status(err error) int { return sysmlapi.Status(err) }

// do performs one authorized request against either service and returns the
// response body, failing on any status outside 2xx.
func (c *Client) do(ctx context.Context, method, target string, body []byte, contentType string, headers map[string]string) ([]byte, http.Header, error) {
	return c.api.Do(ctx, method, target, body, contentType, headers)
}

// Reachable reports whether both services answer. The SysML v2 project list is
// the cheapest authorized read on the stack.
func (c *Client) Reachable(ctx context.Context) error {
	if err := c.api.Reachable(ctx); err != nil {
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
type Project = sysmlapi.Project

// CreateProject creates a project through the SysML v2 API, which also creates
// its default branch and the queries scratch. The id doubles as the Layer 1 repo
// id, so it must satisfy the service's id rules; the default branch is named
// rather than left to the service, which would otherwise mint a uuid the graph
// endpoint's path needs.
func (c *Client) CreateProject(ctx context.Context, id, name, branch string) (Project, error) {
	return c.api.CreateProject(ctx, id, name, branch)
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
	return c.api.PostChanges(ctx, project, branch, changes)
}

// Commit identifies one commit of a project.
type Commit = sysmlapi.Commit

// Branch is one branch as the SysML v2 API describes it: its id and the
// commit at its head.
type Branch = sysmlapi.Branch

// Branch reads one branch of a project, head commit included.
func (c *Client) Branch(ctx context.Context, project, branch string) (Branch, error) {
	return c.api.Branch(ctx, project, branch)
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
	return c.api.Commits(ctx, project)
}

// Element is one element as the SysML v2 API delivers it.
type Element = sysmlapi.Element

// Listing is one element listing: the elements it delivered, the number of
// responses it took, and whether the service ignored the paging parameters.
type Listing = sysmlapi.Listing

// Elements reads a commit's elements a page at a time.
func (c *Client) Elements(ctx context.Context, project, commit string, pageSize int) (Listing, error) {
	return c.api.Elements(ctx, project, commit, pageSize)
}

// ElementByID reads one element directly. A non-2xx answer is returned as the
// error, with Status(err) carrying the code: a rejected id is a finding, not a
// harness failure.
func (c *Client) ElementByID(ctx context.Context, project, commit, id string) (Element, error) {
	return c.api.ElementByID(ctx, project, commit, id)
}

// Roots lists the elements the service considers roots of a commit: those with
// neither an owner nor an owning related element.
func (c *Client) Roots(ctx context.Context, project, commit string) ([]Element, error) {
	return c.api.Roots(ctx, project, commit)
}

// Representation is what the service stores of a posted graph: sysml:
// properties only, values as JSON spells them. A diff under it converges with
// an apply.
type Representation = sysmlapi.Representation

// UnrepresentableError is a value the service's JSON commit path has no
// spelling for; the diff refuses it rather than approximating.
type UnrepresentableError = sysmlapi.UnrepresentableError
