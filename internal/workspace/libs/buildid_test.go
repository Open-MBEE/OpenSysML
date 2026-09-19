package libs

import (
	"runtime/debug"
	"testing"
)

func TestBuildIDFromInfoUsesCleanRevision(t *testing.T) {
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "abc123"},
		{Key: "vcs.modified", Value: "false"},
	}}
	got, ok := buildIDFromInfo(info)
	if !ok || got != "gabc123" {
		t.Fatalf("clean revision: got %q, %v; want \"gabc123\", true", got, ok)
	}
}

// A modified tree reuses one revision for every edit, so it must not identify a
// build: that is the stale-hit case that made the differential oracle disagree
// between machines (#491).
func TestBuildIDFromInfoRejectsModifiedTree(t *testing.T) {
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "abc123"},
		{Key: "vcs.modified", Value: "true"},
	}}
	if got, ok := buildIDFromInfo(info); ok {
		t.Fatalf("modified tree identified as %q", got)
	}
}

func TestBuildIDFromInfoFallsBackToModuleVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}
	got, ok := buildIDFromInfo(info)
	if !ok || got != "mv1.2.3" {
		t.Fatalf("released version: got %q, %v; want \"mv1.2.3\", true", got, ok)
	}
	devel := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}
	if got, ok := buildIDFromInfo(devel); ok {
		t.Fatalf("(devel) identified as %q", got)
	}
}

// The fallback identity has to be usable in a file name and stable within one
// process, or every Load would miss the record Persist just wrote.
func TestCurrentBuildIDIsStableAndFileNameSafe(t *testing.T) {
	first, second := buildID(), buildID()
	if first != second {
		t.Fatalf("build ID changed within one process: %q then %q", first, second)
	}
	if first == "" {
		t.Fatal("build ID is empty")
	}
	for _, r := range first {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			t.Fatalf("build ID %q is not hex, so it is not safe in a file name", first)
		}
	}
}
