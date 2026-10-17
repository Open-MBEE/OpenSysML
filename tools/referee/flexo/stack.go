// Package flexo measures what a running Flexo MMS stack does with this project's
// RDF, over the production client; the stack is opt-in (.agents/skills/flexo-interop).
package flexo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	mms "github.com/Open-MBEE/OpenSysML/internal/translate/interop/flexo"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/sysmlapi"
)

// EnvGate gates the live tests: they skip unless it is set, and fail once it is.
const EnvGate = "FLEXO_INTEROP"

// mediaTurtle is the RDF media type both services speak.
const mediaTurtle = "text/turtle"

// Stack is one running stack under measurement: the production client plus the
// org, project and unconditional-load operations only the harness needs.
type Stack struct {
	*mms.Client
}

// Connect returns the stack addressed by cfg.
func Connect(cfg mms.Config) *Stack {
	return &Stack{Client: mms.New(cfg)}
}

// Reachable reports whether both services answer; the project list is the
// cheapest authorized read.
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
// under; a fresh cluster lacks it and every project creation fails until it exists.
func (s *Stack) EnsureOrg(ctx context.Context) error {
	if _, _, err := s.API().Do(ctx, http.MethodGet, s.orgURL(), nil, "", map[string]string{"Accept": mediaTurtle}); err == nil {
		return nil
	}
	body := fmt.Sprintf("<> <http://purl.org/dc/terms/title> %q .\n", s.Config().Org)
	_, _, err := s.API().Do(ctx, http.MethodPut, s.orgURL(), []byte(body), mediaTurtle, nil)
	return err
}

// CreateProject creates a project and its named default branch through the
// SysML v2 API; the id doubles as the Layer 1 repo id the graph endpoint's path needs.
func (s *Stack) CreateProject(ctx context.Context, id, name, branch string) (sysmlapi.Project, error) {
	return s.API().CreateProject(ctx, id, name, branch)
}

// LoadTurtle replaces a branch's model graph unconditionally (PutGraph with If-Match *).
func (s *Stack) LoadTurtle(ctx context.Context, project, branch string, turtle []byte, message string) error {
	_, err := s.PutGraph(ctx, project, branch, turtle, message, "*")
	return err
}

// PostChanges commits SysML v2 JSON changes through the service's own commit
// path; an empty branch takes the default one.
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

// ElementByID reads one element directly; a non-2xx answer is returned as the
// error with sysmlapi.Status(err) carrying the code, since a rejected id is a finding.
func (s *Stack) ElementByID(ctx context.Context, project, commit, id string) (sysmlapi.Element, error) {
	return s.API().ElementByID(ctx, project, commit, id)
}

// Roots lists the elements the service considers roots of a commit.
func (s *Stack) Roots(ctx context.Context, project, commit string) ([]sysmlapi.Element, error) {
	return s.API().Roots(ctx, project, commit)
}
