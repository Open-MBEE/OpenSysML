// Package reposync diffs a local model's identity-carrying RDF graph against a
// repository-side graph of the same project scope, keyed by effective element
// id, so a rename or a move is an update to the existing element rather than a
// delete plus a create.
package reposync

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Provenance predicate IRIs, matching what the RDF writer records for a
// ProjectRef annotation.
var (
	predProjectID = rdf.OpenSysML + "projectId"
	predBranch    = rdf.OpenSysML + "branch"
	predOrg       = rdf.OpenSysML + "org"
)

// Scope is the project a graph belongs to. Org and project identify it; branch
// selects the version being synced against, not a different identity space.
type Scope struct {
	Org       string
	ProjectID string
	Branch    string
}

// IsZero reports whether the scope carries no project at all.
func (s Scope) IsZero() bool { return s.Org == "" && s.ProjectID == "" && s.Branch == "" }

// String renders the scope for reports and error messages.
func (s Scope) String() string {
	if s.IsZero() {
		return "unbound scope"
	}
	out := "project " + s.ProjectID
	if s.Org != "" {
		out = "org " + s.Org + " " + out
	}
	if s.Branch != "" {
		out += " branch " + s.Branch
	}
	return out
}

// sameProject reports whether two scopes name the same project, branch aside.
func (s Scope) sameProject(other Scope) bool {
	return s.Org == other.Org && s.ProjectID == other.ProjectID
}

// ErrTwoBranches reports elements of one document naming two branches at once:
// a branch selects one version, so one sync reads from exactly one.
var ErrTwoBranches = errors.New("one document cannot sync against two branches at once")

// GraphScope reads a graph's provenance triples into the single scope it syncs
// under. Two projects or two branches in one graph is an error, never a guess.
func GraphScope(g *rdf.Graph) (Scope, error) {
	var scope Scope
	found := false
	for _, subject := range g.Subjects() {
		projectID, _ := g.Lexical(subject, predProjectID)
		branch, _ := g.Lexical(subject, predBranch)
		org, _ := g.Lexical(subject, predOrg)
		if projectID == "" && branch == "" && org == "" {
			continue
		}
		at := Scope{Org: org, ProjectID: projectID, Branch: branch}
		if !found {
			scope, found = at, true
			continue
		}
		if !scope.sameProject(at) {
			return Scope{}, fmt.Errorf("the graph holds two project scopes, %s and %s; sync each project separately", scope, at)
		}
		if scope.Branch != at.Branch {
			return Scope{}, fmt.Errorf("%w: the graph names both branch %q and branch %q", ErrTwoBranches, scope.Branch, at.Branch)
		}
	}
	return scope, nil
}
