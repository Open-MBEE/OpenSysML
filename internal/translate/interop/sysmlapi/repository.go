package sysmlapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// elementsPageSize is how many elements one page of a branch read asks for.
const elementsPageSize = 500

// Repository is one project branch as a sync's repository: read as the graph
// its elements spell, written through the commit path so every element keeps
// its id. It remembers the head its last read stood at — empty for a branch
// read before its first commit — and refuses to commit past a head that has
// since moved.
type Repository struct {
	client  *Client
	project string
	branch  string
	seen    string
	read    bool // whether seen records a head this repository read or wrote
}

// Repository addresses one project branch for a sync.
func (c *Client) Repository(project, branch string) *Repository {
	return &Repository{client: c, project: project, branch: branch}
}

// Client is the client the repository reads and writes through.
func (r *Repository) Client() *Client { return r.client }

// Project is the id of the project the repository is a branch of.
func (r *Repository) Project() string { return r.project }

// Branch is the id of the branch the repository reads and writes.
func (r *Repository) Branch() string { return r.branch }

// StaleBranchError is a branch whose head moved between the read a change set
// was computed against and the commit that would have written it; nothing was
// written, and the change set has to be recomputed against the new head.
type StaleBranchError struct {
	Project, Branch, Seen, Head string
}

func (e *StaleBranchError) Error() string {
	return fmt.Sprintf("branch %s of %s moved from commit %s to %s since the change set was computed; nothing was written, so diff again against the new head",
		e.Branch, e.Project, e.Seen, e.Head)
}

// Head is the branch's current head commit; empty for a branch no commit has
// reached yet, as a freshly created project's is on some servers.
func (r *Repository) Head(ctx context.Context) (string, error) {
	branch, err := r.client.Branch(ctx, r.project, r.branch)
	if err != nil {
		return "", err
	}
	return branch.Head.ID, nil
}

// Seen is the head commit the last Graph read stood at, or the last commit
// this repository wrote; empty before either.
func (r *Repository) Seen() string { return r.seen }

// Resume records the last-seen commit a saved sync state carries, so a head
// that has since moved is refused instead of written past.
func (r *Repository) Resume(commit string) { r.seen = commit }

// Graph reads the branch as its head commit left it, so the read is of one
// commit rather than of a branch that may move under it. A branch with no
// commit yet reads as an empty graph.
func (r *Repository) Graph(ctx context.Context) (*rdf.Graph, error) {
	head, err := r.Head(ctx)
	if err != nil {
		return nil, err
	}
	if head == "" {
		r.seen, r.read = "", true
		return rdf.NewGraph(), nil
	}
	graph, err := r.GraphAt(ctx, head)
	if err != nil {
		return nil, err
	}
	r.seen, r.read = head, true
	return graph, nil
}

// GraphAt reads the branch as one commit left it: its elements, paged through
// and read as the graph their JSON spells.
func (r *Repository) GraphAt(ctx context.Context, commit string) (*rdf.Graph, error) {
	listing, err := r.client.Elements(ctx, r.project, commit, elementsPageSize)
	if err != nil {
		return nil, err
	}
	return ElementsGraph(listing.Elements)
}

// ElementsGraph reads a commit's elements as the RDF graph their API JSON
// spells, the inverse of what a commit's payloads were written from.
func ElementsGraph(elements []Element) (*rdf.Graph, error) {
	if len(elements) == 0 {
		return rdf.NewGraph(), nil
	}
	data, err := json.Marshal(elements)
	if err != nil {
		return nil, err
	}
	return export.ReadAPIJSON(data)
}

// Commit writes one batch as one SysML v2 commit. The API takes no
// precondition, so once a head is known — read by Graph, resumed or written —
// it is re-read first and a moved one refuses the write as a StaleBranchError;
// a branch read without a head has to be still without one.
func (r *Repository) Commit(ctx context.Context, changes []reposync.ElementChange, message string) (string, error) {
	body, err := CommitRequest(changes, message)
	if err != nil {
		return "", err
	}
	if r.read || r.seen != "" {
		head, err := r.Head(ctx)
		if err != nil {
			return "", err
		}
		if head != r.seen {
			return "", &StaleBranchError{Project: r.project, Branch: r.branch, Seen: r.seen, Head: head}
		}
	}
	commit, err := r.client.PostChanges(ctx, r.project, r.branch, body)
	if err != nil {
		return "", err
	}
	r.seen, r.read = commit.ID, true
	return commit.ID, nil
}
