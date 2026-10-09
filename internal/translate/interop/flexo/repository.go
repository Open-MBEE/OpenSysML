package flexo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/sysmlapi"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Repository is one project branch as a sync's repository: read as a graph
// through Layer 1, written through the service's commit path so every element
// keeps its id, or replaced whole by a Turtle push. It remembers the head its
// last read stood at and refuses to commit past a head that has since moved.
type Repository struct {
	*sysmlapi.Repository
	client *Client
}

// Repository addresses one project branch for a sync.
func (c *Client) Repository(project, branch string) *Repository {
	return &Repository{Repository: c.api.Repository(project, branch), client: c}
}

// StaleBranchError is a branch whose head moved between the read a change set
// was computed against and the commit that would have written it; nothing was
// written, and the change set has to be recomputed against the new head.
type StaleBranchError = sysmlapi.StaleBranchError

// UnrecordedPushError is a graph write the branch accepted but whose response
// named no commit; a later head read could be another writer's, so none is recorded.
type UnrecordedPushError struct {
	Project, Branch string
}

func (e *UnrecordedPushError) Error() string {
	return fmt.Sprintf("the graph was written to %s/%s but the response named no commit; read the branch again before the next push",
		e.Project, e.Branch)
}

// SupersededPushError is a committed graph write, named by its Location, whose
// branch head has since moved on; the baseline must come from reading the branch again.
type SupersededPushError struct {
	Project, Branch, Commit, Head string
}

func (e *SupersededPushError) Error() string {
	return fmt.Sprintf("the graph was committed to %s/%s as %s but the branch has since moved to %s; read it again before the next push",
		e.Project, e.Branch, e.Commit, e.Head)
}

// Head is the branch's current head commit; a branch naming none is an error,
// as every Flexo branch stands at a commit.
func (r *Repository) Head(ctx context.Context) (string, error) {
	head, err := r.Repository.Head(ctx)
	if err != nil {
		return "", err
	}
	if head == "" {
		return "", fmt.Errorf("branch %s of %s names no head commit", r.Branch(), r.Project())
	}
	return head, nil
}

// Graph reads the branch as its head commit left it, so the read is of one
// commit rather than of a branch that may move under it.
func (r *Repository) Graph(ctx context.Context) (*rdf.Graph, error) {
	head, err := r.Head(ctx)
	if err != nil {
		return nil, err
	}
	graph, err := r.client.CommitGraph(ctx, r.Project(), head)
	if err != nil {
		return nil, err
	}
	r.Resume(head)
	return graph, nil
}

// GraphAt reads the branch as one earlier commit left it, through Layer 1's
// SPARQL endpoint: the last-seen graph a diff needs to tell a repository
// change from its own.
func (r *Repository) GraphAt(ctx context.Context, commit string) (*rdf.Graph, error) {
	return r.client.CommitGraph(ctx, r.Project(), commit)
}

// Push replaces the branch's whole model graph with the given Turtle and
// returns the commit made; a head that moved past what was seen is refused.
func (r *Repository) Push(ctx context.Context, turtle []byte, message string) (string, error) {
	project, branch := r.Project(), r.Branch()
	// The etag is read first: a head read after it can only race into a 412,
	// never into a write past a moved head.
	etag, err := r.client.BranchETag(ctx, project, branch)
	if err != nil {
		return "", err
	}
	head, err := r.Head(ctx)
	if err != nil {
		return "", err
	}
	if seen := r.Seen(); seen != "" && head != seen {
		return "", &StaleBranchError{Project: project, Branch: branch, Seen: seen, Head: head}
	}
	res, err := r.client.PutGraph(ctx, project, branch, turtle, message, etag)
	if err != nil {
		if Status(err) == http.StatusPreconditionFailed {
			current, readErr := r.Head(ctx)
			if readErr != nil {
				return "", fmt.Errorf("the branch answered 412 and its head could not be re-read: %w", readErr)
			}
			// A committed write answers 412 too; its Location proves the commit,
			// and only a head still at it counts it as ours.
			if c := committedFrom(res); c != "" {
				if c == current {
					r.Resume(c)
					return c, nil
				}
				return "", &SupersededPushError{Project: project, Branch: branch, Commit: c, Head: current}
			}
			return "", &StaleBranchError{Project: project, Branch: branch, Seen: head, Head: current}
		}
		return "", err
	}
	// Trust the commit the write itself reported; the head may have moved on.
	if res.Commit != "" {
		r.Resume(res.Commit)
		return res.Commit, nil
	}
	r.Resume("")
	return "", &UnrecordedPushError{Project: project, Branch: branch}
}

// committedFrom trusts an ETag only when the response's Location names the
// same commit under /commits/; a refused 412 carries neither.
func committedFrom(res PutResult) string {
	if res.Commit == "" {
		return ""
	}
	u, err := url.Parse(res.Location)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 && parts[len(parts)-2] == "commits" && parts[len(parts)-1] == res.Commit {
		return res.Commit
	}
	return ""
}
