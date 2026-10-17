// Package flexo measures what a running Flexo MMS stack — the Layer 1 service
// and the SysML v2 API in front of it — does with the RDF this project writes:
// which elements and properties a graph load delivers back through the standard
// API, against what the service's own commit path delivers, and what the
// identity-keyed sync's apply leaves behind.
//
// The client the measurements drive is the product's own, package
// internal/translate/interop/flexo; this package adds the operations only a
// measurement needs. The stack is external and opt-in; see
// .agents/skills/flexo-interop for how to bring it up and what the gate reports.
package flexo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	mms "github.com/Open-MBEE/OpenSysML/internal/translate/interop/flexo"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/sysmlapi"
)

// EnvGate is the gate: the live tests skip unless it is set, exactly as the
// corpus gates skip on an absent corpus, and fail rather than skip once it is.
const EnvGate = "FLEXO_INTEROP"

// mediaTurtle is the RDF media type both services speak.
const mediaTurtle = "text/turtle"

// Stack is one running stack under measurement: the product's client, plus the
// project creation, org creation and unconditional graph load the harness needs
// and nothing the product does.
type Stack struct {
	*mms.Client
}

// Connect returns the stack addressed by cfg.
func Connect(cfg mms.Config) *Stack {
	return &Stack{Client: mms.New(cfg)}
}

// Reachable reports whether both services answer. The SysML v2 project list is
// the cheapest authorized read on the stack.
func (s *Stack) Reachable(ctx context.Context) error {
	cfg := s.Config()
	if err := s.API().Reachable(ctx); err != nil {
		return fmt.Errorf("sysmlv2 at %s: %w", cfg.SysMLV2URL, err)
	}
	// The org list, not the org itself: a fresh cluster has no org yet.
	if _, _, err := s.API().Do(ctx, http.MethodGet, cfg.Layer1URL+"/orgs", nil, "",
		map[string]string{"Accept": mediaTurtle}); err != nil {
		return fmt.Errorf("layer1 at %s: %w", cfg.Layer1URL, err)
	}
	return nil
}

func (s *Stack) orgURL() string {
	cfg := s.Config()
	return cfg.Layer1URL + "/orgs/" + url.PathEscape(cfg.Org)
}

// EnsureOrg creates the Layer 1 org the SysML v2 service keeps its projects
// under. A freshly initialized cluster does not have it, and every project
// creation fails until it does.
func (s *Stack) EnsureOrg(ctx context.Context) error {
	if _, _, err := s.API().Do(ctx, http.MethodGet, s.orgURL(), nil, "", map[string]string{"Accept": mediaTurtle}); err == nil {
		return nil
	}
	body := fmt.Sprintf("<> <http://purl.org/dc/terms/title> %q .\n", s.Config().Org)
	_, _, err := s.API().Do(ctx, http.MethodPut, s.orgURL(), []byte(body), mediaTurtle, nil)
	return err
}

// CreateProject creates a project through the SysML v2 API, which also creates
// its default branch and the queries scratch. The id doubles as the Layer 1 repo
// id, so it must satisfy the service's id rules; the default branch is named
// rather than left to the service, which would otherwise mint a uuid the graph
// endpoint's path needs.
func (s *Stack) CreateProject(ctx context.Context, id, name, branch string) (sysmlapi.Project, error) {
	return s.API().CreateProject(ctx, id, name, branch)
}

// LoadTurtle replaces a branch's model graph unconditionally: PutGraph with
// If-Match *, the precondition the measurement loads under.
func (s *Stack) LoadTurtle(ctx context.Context, project, branch string, turtle []byte, message string) error {
	_, err := s.PutGraph(ctx, project, branch, turtle, message, "*")
	return err
}

// PostChanges commits SysML v2 JSON changes through the service's own commit
// path and returns the commit it made. An empty branch takes the default one.
func (s *Stack) PostChanges(ctx context.Context, project, branch string, changes []byte) (sysmlapi.Commit, error) {
	return s.API().PostChanges(ctx, project, branch, changes)
}

// Commits lists a project's commits, newest first as the service returns them.
func (s *Stack) Commits(ctx context.Context, project string) ([]sysmlapi.Commit, error) {
	return s.API().Commits(ctx, project)
}

// Elements reads a commit's elements a page at a time.
func (s *Stack) Elements(ctx context.Context, project, commit string, pageSize int) (sysmlapi.Listing, error) {
	return s.API().Elements(ctx, project, commit, pageSize)
}

// ElementByID reads one element directly. A non-2xx answer is returned as the
// error, with sysmlapi.Status(err) carrying the code: a rejected id is a
// finding, not a harness failure.
func (s *Stack) ElementByID(ctx context.Context, project, commit, id string) (sysmlapi.Element, error) {
	return s.API().ElementByID(ctx, project, commit, id)
}

// Roots lists the elements the service considers roots of a commit: those with
// neither an owner nor an owning related element.
func (s *Stack) Roots(ctx context.Context, project, commit string) ([]sysmlapi.Element, error) {
	return s.API().Roots(ctx, project, commit)
}
