package reposync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// State is the sync tool's record of what a sync last saw. Commit ids are tool
// state, never model content: they live in this JSON file — <model>.sync.json
// beside the model by default — and are never written into the notation.
type State struct {
	Org            string `json:"org,omitempty"`
	ProjectID      string `json:"projectId"`
	Branch         string `json:"branch,omitempty"`
	LastSeenCommit string `json:"lastSeenCommit,omitempty"`
}

// StatePath is the default state file for a model: its path with .sync.json
// appended, so two models beside each other keep separate sync state.
func StatePath(modelPath string) string { return modelPath + ".sync.json" }

// LoadState reads a state file, returning nil without error when the file does
// not exist yet: a first sync has no last-seen commit.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the user names their own sync state file
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	state := &State{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return state, nil
}

// Save writes the state file.
func (s *State) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// ErrIncompleteApply refuses to advance the state past an apply that left
// changes unapplied: no single commit then describes what the branch holds.
var ErrIncompleteApply = errors.New("the apply did not complete, so the last-seen commit stays where it was")

// Advance records a completed apply's last commit as last seen; an apply that
// wrote nothing leaves the state as it was.
func (s *State) Advance(result *Result) error {
	if !result.Complete() {
		return ErrIncompleteApply
	}
	if commit := result.LastCommit(); commit != "" {
		s.LastSeenCommit = commit
	}
	return nil
}

// Scope returns the scope the state pins the sync to.
func (s *State) Scope() Scope {
	return Scope{Org: s.Org, ProjectID: s.ProjectID, Branch: s.Branch}
}

// Check refuses a state that pins the sync to a different project or branch
// than the document's own scope: that would sync one element against two
// branches at once.
func (s *State) Check(scope Scope) error {
	if scope.IsZero() {
		return nil
	}
	if s.ProjectID != "" && scope.ProjectID != "" && !s.Scope().sameProject(scope) {
		return fmt.Errorf("the sync state pins %s but the document is scoped to %s", s.Scope(), scope)
	}
	if s.Branch != "" && scope.Branch != "" && s.Branch != scope.Branch {
		return fmt.Errorf("%w: the sync state pins branch %q but the document names branch %q", ErrTwoBranches, s.Branch, scope.Branch)
	}
	return nil
}
