package reposync

import (
	"context"
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Committer is the write side of one live repository branch as an apply sees
// it: a fake in tests, the Flexo adapter for real.
type Committer interface {
	// Commit writes one batch as a single commit and returns the id the
	// repository gave it. A refused batch must leave the branch as it was.
	Commit(ctx context.Context, changes []ElementChange, message string) (string, error)
}

// ElementChange is one element as the repository is to hold it after the
// commit: whole content for a create or update, nothing for a delete.
type ElementChange struct {
	Kind    Kind
	ID      string       // id the repository addresses the element by, minted when one was
	Content []rdf.Triple // subject, type and carried properties; nil for a delete
}

// ApplyOptions steer an apply.
type ApplyOptions struct {
	// BatchSize splits a set over several commits; 0 writes one commit.
	BatchSize int
	// Message is recorded on each commit.
	Message string
}

// Result is what an apply did: its commits, and the fate of each change.
// Failed changes were in a refused batch; Pending ones were never sent.
type Result struct {
	Commits []string
	Applied []Change
	Failed  []Change
	Pending []Change
}

// LastCommit is the newest commit made, the one the sync state advances to.
func (r *Result) LastCommit() string {
	if len(r.Commits) == 0 {
		return ""
	}
	return r.Commits[len(r.Commits)-1]
}

// Complete reports whether every change reached the repository.
func (r *Result) Complete() bool { return len(r.Failed) == 0 && len(r.Pending) == 0 }

// ApplyError reports a batch the repository refused, with the result so far:
// what did reach the repository is committed and stays there.
type ApplyError struct {
	Batch   int // 1-based index of the refused batch
	Batches int
	Result  *Result
	Err     error
}

func (e *ApplyError) Error() string {
	if e.Batches == 1 {
		return fmt.Sprintf("the repository refused the commit; nothing was applied: %v", e.Err)
	}
	return fmt.Sprintf("the repository refused commit %d of %d after %d change(s) were applied in %d commit(s); %d failed, %d never sent: %v",
		e.Batch, e.Batches, len(e.Result.Applied), len(e.Result.Commits), len(e.Result.Failed), len(e.Result.Pending), e.Err)
}

func (e *ApplyError) Unwrap() error { return e.Err }

// UnwritableChangeError is a create or update the diff carried no content for,
// so there is nothing the repository could be asked to hold.
type UnwritableChangeError struct {
	Kind Kind
	ID   string
}

func (e *UnwritableChangeError) Error() string {
	return fmt.Sprintf("%s %s carries no content the repository can hold", e.Kind, e.ID)
}

// ErrUnnamedCommit is a commit the repository accepted but did not name, so
// the sync state cannot advance to it.
var ErrUnnamedCommit = errors.New("the repository accepted the commit but did not name it")

// UnsupportedKindError is a change of a kind no repository write exists for,
// such as a conflict that slipped past Appliable.
type UnsupportedKindError struct {
	Kind Kind
	ID   string
}

func (e *UnsupportedKindError) Error() string {
	return fmt.Sprintf("%s %s has no repository write", e.Kind, e.ID)
}

// Apply pushes a change set to the repository. An unappliable set is refused
// before any write; a rejected batch is reported as an *ApplyError.
func Apply(ctx context.Context, repo Committer, set *ChangeSet, opts ApplyOptions) (*Result, error) {
	if err := set.Appliable(); err != nil {
		return nil, err
	}
	changes, err := elementChanges(set)
	if err != nil {
		return nil, err
	}
	result := &Result{}
	if len(changes) == 0 {
		return result, nil
	}

	size := opts.BatchSize
	if size <= 0 || size > len(changes) {
		size = len(changes)
	}
	batches := (len(changes) + size - 1) / size
	for start, n := 0, 0; start < len(changes); start, n = start+size, n+1 {
		end := min(start+size, len(changes))
		commit, err := repo.Commit(ctx, changes[start:end], opts.Message)
		if err != nil {
			result.Failed = set.Changes[start:end]
			result.Pending = set.Changes[end:]
			return result, &ApplyError{Batch: n + 1, Batches: batches, Result: result, Err: err}
		}
		result.Applied = append(result.Applied, set.Changes[start:end]...)
		if commit == "" {
			result.Pending = set.Changes[end:]
			return result, &ApplyError{Batch: n + 1, Batches: batches, Result: result, Err: ErrUnnamedCommit}
		}
		result.Commits = append(result.Commits, commit)
	}
	return result, nil
}

// elementChanges turns an appliable set into what the repository is asked to
// hold, minted ids applied to subjects and references alike.
func elementChanges(set *ChangeSet) ([]ElementChange, error) {
	mints := set.Mints()
	changes := make([]ElementChange, 0, len(set.Changes))
	for _, change := range set.Changes {
		element := ElementChange{Kind: change.Kind, ID: change.ID}
		switch change.Kind {
		case KindDelete:
		case KindCreate, KindUpdate:
			if len(change.Content) == 0 {
				return nil, &UnwritableChangeError{Kind: change.Kind, ID: change.ID}
			}
			element.ID = rdf.LocalName(mintedTerm(rdf.IRI(change.Subject), mints).Value)
			element.Content = make([]rdf.Triple, 0, len(change.Content))
			for _, triple := range change.Content {
				element.Content = append(element.Content, mintTriple(triple, mints))
			}
		default:
			return nil, &UnsupportedKindError{Kind: change.Kind, ID: change.ID}
		}
		changes = append(changes, element)
	}
	return changes, nil
}
