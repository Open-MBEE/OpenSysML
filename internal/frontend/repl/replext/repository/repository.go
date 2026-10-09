// Package repository links the %repo, %projects, %load and %publish commands to
// a SysML v2 API server: any conformant one through the standard surface, with
// a bearer token from the environment when the server wants one.
package repository

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/modelsync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/sysmlapi"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func init() { replext.RegisterRepository(repository{}) }

type repository struct{}

// NotFoundError is a project or branch the server does not have.
type NotFoundError struct {
	What string // "project" or "branch"
	Name string
}

func (e *NotFoundError) Error() string { return fmt.Sprintf("%s %s doesn't exist", e.What, e.Name) }

// NoHeadError is a branch no commit has reached, which has no elements to load.
type NoHeadError struct {
	Branch string
}

func (e *NoHeadError) Error() string { return fmt.Sprintf("branch %s has no head commit", e.Branch) }

func (repository) DefaultURL() string { return sysmlapi.ConfigFromEnv().BaseURL }

func (repository) CheckURL(base string) error { return sysmlapi.CheckURL(base) }

func client(base string) *sysmlapi.Client {
	cfg := sysmlapi.ConfigFromEnv()
	cfg.BaseURL = base
	return sysmlapi.New(cfg)
}

func (repository) Projects(ctx context.Context, base string) ([]replext.ProjectInfo, error) {
	projects, err := client(base).Projects(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]replext.ProjectInfo, 0, len(projects))
	for _, p := range projects {
		out = append(out, replext.ProjectInfo{ID: p.ID, Name: p.Name})
	}
	return out, nil
}

// findProject resolves a project by id, else by name; a name several projects
// share is refused naming them all.
func findProject(ctx context.Context, c *sysmlapi.Client, id, name string) (sysmlapi.Project, error) {
	if id != "" {
		project, err := c.Project(ctx, id)
		if sysmlapi.Status(err) == http.StatusNotFound {
			return sysmlapi.Project{}, &NotFoundError{What: "project", Name: id}
		}
		return project, err
	}
	projects, err := c.Projects(ctx)
	if err != nil {
		return sysmlapi.Project{}, err
	}
	var matches []sysmlapi.Project
	for _, p := range projects {
		if p.Name == name {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return sysmlapi.Project{}, &NotFoundError{What: "project", Name: name}
	case 1:
		return matches[0], nil
	}
	ids := make([]string, 0, len(matches))
	for _, p := range matches {
		ids = append(ids, p.ID)
	}
	return sysmlapi.Project{}, &modelsync.AmbiguousNameError{Name: name, IDs: ids}
}

// findBranch resolves a branch by name or id, or the project's default branch
// when none is named.
func findBranch(ctx context.Context, c *sysmlapi.Client, project sysmlapi.Project, nameOrID string) (sysmlapi.Branch, error) {
	branches, err := c.Branches(ctx, project.ID)
	if err != nil {
		return sysmlapi.Branch{}, err
	}
	want := nameOrID
	if want == "" {
		want = project.DefaultBranch.ID
	}
	for _, b := range branches {
		if b.ID == want {
			return b, nil
		}
	}
	for _, b := range branches {
		if nameOrID != "" && b.Name == nameOrID {
			return b, nil
		}
	}
	if nameOrID == "" {
		if want == "" && len(branches) == 1 {
			return branches[0], nil
		}
		return sysmlapi.Branch{}, &NotFoundError{What: "default branch", Name: want}
	}
	return sysmlapi.Branch{}, &NotFoundError{What: "branch", Name: nameOrID}
}

func stateOf(base string, project sysmlapi.Project, branch sysmlapi.Branch) replext.ProjectState {
	return replext.ProjectState{
		Base:      base,
		ProjectID: project.ID, ProjectName: project.Name,
		Branch: branch.ID, BranchName: branch.Name,
		LastSeenCommit: branch.Head.ID,
	}
}

func (repository) Load(ctx context.Context, base string, req replext.LoadRequest) (*replext.LoadResult, error) {
	c := client(base)
	project, err := findProject(ctx, c, req.ProjectID, req.Name)
	if err != nil {
		return nil, err
	}
	branch, err := findBranch(ctx, c, project, req.Branch)
	if err != nil {
		return nil, err
	}
	if branch.Head.ID == "" {
		return nil, &NoHeadError{Branch: branch.Name}
	}
	repo := c.Repository(project.ID, branch.ID)
	graph, err := repo.GraphAt(ctx, branch.Head.ID)
	if err != nil {
		return nil, err
	}
	result := &replext.LoadResult{State: stateOf(base, project, branch)}
	scoped := modelsync.Scoped(graph, reposync.Scope{ProjectID: project.ID, Branch: branch.ID})
	result.Notation, err = modelsync.Notation(scoped, func(w string) { result.Warnings = append(result.Warnings, w) })
	if err != nil {
		return nil, fmt.Errorf("the elements of %s could not be written as notation: %w", project.Name, err)
	}
	return result, nil
}

// simpleName is the name a root element publishes under when no project is
// named: its declared name, else the last segment of its qualified name.
func simpleName(graph *rdf.Graph, root rdf.Term, qualified string) string {
	if name, ok := graph.Lexical(root, rdf.SysML+"declaredName"); ok && name != "" {
		return name
	}
	if i := strings.LastIndex(qualified, "::"); i >= 0 {
		return qualified[i+2:]
	}
	return qualified
}

func (repository) Publish(ctx context.Context, base string, req replext.PublishRequest) (*replext.PublishResult, error) {
	whole, err := convert.SysMLToRDF(req.Origin, req.Source)
	if err != nil {
		return nil, err
	}
	cut, err := modelsync.Rooted(whole, req.Root)
	if err != nil {
		return nil, err
	}
	roots := modelsync.Roots(cut)
	if len(roots) != 1 {
		return nil, fmt.Errorf("%s does not cut to one root element", req.Root)
	}
	if !req.Derived {
		cut = modelsync.WithoutDerived(cut)
	}
	name := req.Project
	if name == "" {
		name = simpleName(cut, roots[0], req.Root)
	}

	c := client(base)
	result := &replext.PublishResult{}
	// State from another server is no history of this one, whatever ids match.
	known := req.State != nil && req.State.Base == base
	project, err := findProject(ctx, c, "", name)
	var missing *NotFoundError
	switch {
	case errors.As(err, &missing):
		// The model has to diff cleanly before anything is created on the server.
		if _, err := reposync.Diff(cut, rdf.NewGraph(), reposync.Options{Representation: sysmlapi.Representation{}}); err != nil {
			return nil, err
		}
		if project, err = c.CreateProject(ctx, "", name, req.Branch); err != nil {
			return nil, err
		}
		// The server fills in what the create left to it: the default branch.
		if created, err := c.Project(ctx, project.ID); err == nil {
			project = created
		}
		result.NewProject = true
	case err != nil:
		return nil, err
	}
	fail := func(err error) (*replext.PublishResult, error) {
		if result.NewProject {
			return nil, fmt.Errorf("project %s was created but holds no commit yet; publishing again commits into it: %w", projectLine(project), err)
		}
		return nil, err
	}
	want := req.Branch
	if want == "" && known && req.State.ProjectID == project.ID {
		want = req.State.Branch
	}
	branch, err := findBranch(ctx, c, project, want)
	if err != nil {
		return fail(err)
	}

	repo := c.Repository(project.ID, branch.ID)
	scope := reposync.Scope{ProjectID: project.ID, Branch: branch.ID}
	state := &reposync.State{ProjectID: project.ID, Branch: branch.ID}
	tracked := known && req.State.ProjectID == project.ID && req.State.Branch == branch.ID
	if tracked {
		state.LastSeenCommit = req.State.LastSeenCommit
		repo.Resume(state.LastSeenCommit)
	}
	opts := reposync.Options{Representation: sysmlapi.Representation{}, ConfirmDeletes: tracked}
	if opts.Base, err = modelsync.Baseline(ctx, repo, state); err != nil {
		return fail(err)
	}
	remote, err := repo.Graph(ctx)
	if err != nil {
		return fail(err)
	}
	set, err := reposync.Diff(modelsync.Scoped(cut, scope), remote, opts)
	if err != nil {
		return fail(err)
	}
	confine(set, cut, remote, roots[0], tracked, req.Root, result)
	if err := set.Appliable(); err != nil {
		return fail(fmt.Errorf("refused to publish: %w", err))
	}
	applied, err := modelsync.Apply(ctx, repo, set, state, "sysml %publish "+req.Root)
	if err != nil {
		return fail(err)
	}
	result.Commit = applied.Commit
	result.Created, result.Updated, result.Deleted = applied.Created, applied.Updated, applied.Deleted
	result.State = stateOf(base, project, branch)
	result.State.LastSeenCommit = state.LastSeenCommit
	return result, nil
}

func projectLine(project sysmlapi.Project) string {
	return fmt.Sprintf("%s (%s)", project.Name, project.ID)
}

// confine keeps a publication to its root: the elements under it locally and
// the elements under the same root on the branch. What the branch holds under
// other roots is left as it is, neither deleted nor disputed; and a branch the
// session never loaded keeps everything it holds, since nothing of it was seen.
func confine(set *reposync.ChangeSet, cut, remote *rdf.Graph, root rdf.Term, tracked bool, name string, result *replext.PublishResult) {
	within := map[string]bool{}
	for _, subject := range cut.Subjects() {
		within[rdf.LocalName(subject.Value)] = true
	}
	// The branch may qualify its ids; the root is the subject with the same
	// effective id, as the diff matches elements.
	for _, subject := range remote.Subjects() {
		if rdf.LocalName(subject.Value) == rdf.LocalName(root.Value) {
			for _, owned := range modelsync.RootedAt(remote, subject).Subjects() {
				within[rdf.LocalName(owned.Value)] = true
			}
			break
		}
	}
	kept := set.Changes[:0]
	outside, unseen := 0, 0
	for _, change := range set.Changes {
		switch {
		case !within[change.ID]:
			outside++
		case !tracked && change.Kind == reposync.KindDelete:
			unseen++
		default:
			kept = append(kept, change)
		}
	}
	set.Changes = kept
	if outside > 0 {
		result.Notes = append(result.Notes, fmt.Sprintf("%d element(s) the branch holds outside %s were left in place", outside, name))
	}
	if unseen > 0 {
		result.Notes = append(result.Notes, fmt.Sprintf("%d element(s) of %s the branch holds beyond the model were left in place: the branch was not loaded into this session", unseen, name))
	}
}
