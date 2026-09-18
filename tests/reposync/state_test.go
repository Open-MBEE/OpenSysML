package reposync_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/interop/reposync"
)

func TestStateRoundTrips(t *testing.T) {
	path := reposync.StatePath(filepath.Join(t.TempDir(), "model.sysml"))
	if !strings.HasSuffix(path, "model.sysml.sync.json") {
		t.Fatalf("state path %q does not sit beside the model", path)
	}
	missing, err := reposync.LoadState(path)
	if err != nil || missing != nil {
		t.Fatalf("an absent state file must load as nil: %v, %v", missing, err)
	}
	state := &reposync.State{ProjectID: "proj-1", Branch: "main", LastSeenCommit: "c0ffee"}
	if err := state.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := reposync.LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if *loaded != *state {
		t.Errorf("state round trip changed it: %+v != %+v", loaded, state)
	}
}

func TestStateRefusesAnotherBranch(t *testing.T) {
	state := &reposync.State{ProjectID: "proj-1", Branch: "main"}
	if err := state.Check(reposync.Scope{ProjectID: "proj-1", Branch: "main"}); err != nil {
		t.Errorf("matching scope refused: %v", err)
	}
	if err := state.Check(reposync.Scope{ProjectID: "proj-1", Branch: "dev"}); err == nil {
		t.Error("a second branch was not refused")
	}
	if err := state.Check(reposync.Scope{ProjectID: "proj-2", Branch: "main"}); err == nil {
		t.Error("another project was not refused")
	}
}
