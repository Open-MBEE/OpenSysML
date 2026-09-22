package flexo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Repository is one project branch as a sync's repository: read as a graph,
// written through the service's commit path so every element keeps its id.
// It remembers the head its last read stood at and refuses to commit past a
// head that has since moved.
type Repository struct {
	client  *Client
	project string
	branch  string
	seen    string
}

// Repository addresses one project branch for a sync.
func (c *Client) Repository(project, branch string) *Repository {
	return &Repository{client: c, project: project, branch: branch}
}

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

// UnrecordedPushError is a graph write the branch accepted but whose response
// named no commit: nothing can be recorded as the new baseline, and a later
// head re-read might name another writer's commit, so the next push must read
// the branch again.
type UnrecordedPushError struct {
	Project, Branch string
}

func (e *UnrecordedPushError) Error() string {
	return fmt.Sprintf("the graph was written to %s/%s but the response named no commit; read the branch again before the next push",
		e.Project, e.Branch)
}

// Head is the branch's current head commit.
func (r *Repository) Head(ctx context.Context) (string, error) {
	branch, err := r.client.Branch(ctx, r.project, r.branch)
	if err != nil {
		return "", err
	}
	if branch.Head.ID == "" {
		return "", fmt.Errorf("branch %s of %s names no head commit", r.branch, r.project)
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
// commit rather than of a branch that may move under it.
func (r *Repository) Graph(ctx context.Context) (*rdf.Graph, error) {
	head, err := r.Head(ctx)
	if err != nil {
		return nil, err
	}
	graph, err := r.client.CommitGraph(ctx, r.project, head)
	if err != nil {
		return nil, err
	}
	r.seen = head
	return graph, nil
}

// GraphAt reads the branch as one earlier commit left it: the last-seen graph
// a diff needs to tell a repository change from its own.
func (r *Repository) GraphAt(ctx context.Context, commit string) (*rdf.Graph, error) {
	return r.client.CommitGraph(ctx, r.project, commit)
}

// Push replaces the branch's whole model graph with the given Turtle and
// returns the commit made; a head that moved past what was seen is refused.
func (r *Repository) Push(ctx context.Context, turtle []byte, message string) (string, error) {
	// The etag is read first: a head read after it can only race into a 412,
	// never into a write past a moved head.
	etag, err := r.client.BranchETag(ctx, r.project, r.branch)
	if err != nil {
		return "", err
	}
	head, err := r.Head(ctx)
	if err != nil {
		return "", err
	}
	if r.seen != "" && head != r.seen {
		return "", &StaleBranchError{Project: r.project, Branch: r.branch, Seen: r.seen, Head: head}
	}
	committed, err := r.client.PutGraph(ctx, r.project, r.branch, turtle, message, etag)
	if err != nil {
		if Status(err) == http.StatusPreconditionFailed {
			// A committed write also answers 412; the new head tells them apart.
			current, readErr := r.Head(ctx)
			if readErr != nil {
				return "", fmt.Errorf("the branch answered 412 and its head could not be re-read: %w", readErr)
			}
			if committed != "" && committed == current {
				r.seen = current
				return current, nil
			}
			return "", &StaleBranchError{Project: r.project, Branch: r.branch, Seen: head, Head: current}
		}
		return "", err
	}
	// Trust the commit the write itself reported; the head may have moved on.
	if committed != "" {
		r.seen = committed
		return committed, nil
	}
	r.seen = ""
	return "", &UnrecordedPushError{Project: r.project, Branch: r.branch}
}

// Commit writes one batch as one SysML v2 commit. Creates and updates send
// the element whole under its identity; deletes send a null payload. The API
// takes no precondition, so the head is re-read first and a moved one refuses
// the write as a StaleBranchError.
func (r *Repository) Commit(ctx context.Context, changes []reposync.ElementChange, message string) (string, error) {
	body, err := commitRequest(changes, message)
	if err != nil {
		return "", err
	}
	if r.seen != "" {
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
	r.seen = commit.ID
	return commit.ID, nil
}

// commitRequest encodes a batch as the service's CommitRequest.
func commitRequest(changes []reposync.ElementChange, message string) ([]byte, error) {
	versions := make([]map[string]any, 0, len(changes))
	for _, change := range changes {
		version := map[string]any{
			"@type":    "DataVersion",
			"identity": map[string]string{"@id": change.ID},
			"payload":  nil,
		}
		switch change.Kind {
		case reposync.KindDelete:
		case reposync.KindCreate, reposync.KindUpdate:
			content, err := payload(change)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", change.Kind, change.ID, err)
			}
			version["payload"] = content
		default:
			return nil, fmt.Errorf("%s %s: the service has no commit for a %s", change.Kind, change.ID, change.Kind)
		}
		versions = append(versions, version)
	}
	request := map[string]any{"@type": "Commit", "change": versions}
	if message != "" {
		request["description"] = message
	}
	return json.Marshal(request)
}

// ErrUntyped is an element with no single sysml: metaclass, which the
// service's payload cannot state.
var ErrUntyped = errors.New("the element has no single SysML metaclass")

// payload renders an element's content as the service's JSON: @id and @type,
// then each sysml: property as a value or, when multi-valued, an array.
func payload(change reposync.ElementChange) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	values := map[string][]json.RawMessage{}
	metaclass := ""
	for _, triple := range change.Content {
		predicate := triple.Predicate.Value
		if predicate == rdf.RDFType {
			name, ok := strings.CutPrefix(triple.Object.Value, rdf.SysML)
			if !ok || !triple.Object.IsIRI() || metaclass != "" {
				return nil, ErrUntyped
			}
			metaclass = name
			continue
		}
		name, ok := strings.CutPrefix(predicate, rdf.SysML)
		if !ok {
			return nil, &UnrepresentableError{Term: triple.Predicate, Reason: "the service stores sysml: properties only"}
		}
		value, err := jsonValue(triple.Object)
		if err != nil {
			return nil, err
		}
		values[name] = append(values[name], value)
	}
	if metaclass == "" {
		return nil, ErrUntyped
	}
	for name, list := range values {
		if len(list) == 1 {
			out[name] = list[0]
			continue
		}
		array, err := json.Marshal(list)
		if err != nil {
			return nil, err
		}
		out[name] = array
	}
	id, err := json.Marshal(change.ID)
	if err != nil {
		return nil, err
	}
	out["@id"] = id
	if out["@type"], err = json.Marshal(metaclass); err != nil {
		return nil, err
	}
	return out, nil
}

// jsonValue spells one carried term as the service reads it back from JSON.
func jsonValue(term rdf.Term) (json.RawMessage, error) {
	carried, err := carryTerm(term)
	if err != nil {
		return nil, err
	}
	if carried.IsIRI() {
		if carried.Value == rdfNil {
			return json.RawMessage("null"), nil
		}
		return json.Marshal(map[string]string{"@id": rdf.LocalName(carried.Value)})
	}
	switch carried.Datatype {
	case rdf.XSD + "boolean", rdf.XSD + "integer", rdf.XSD + "decimal":
		return json.RawMessage(carried.Value), nil
	}
	return json.Marshal(carried.Value)
}
