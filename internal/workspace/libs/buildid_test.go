package libs

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
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

// Since Go 1.24 a dirty tree's main module is stamped `<pseudo-version>+dirty`,
// one version for every edit of that commit; accepting it in place of the
// rejected revision let two differently patched builds share every record.
func TestBuildIDFromInfoRejectsDirtyModuleVersion(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.11.1-0.20261025041301-76c98694c2da+dirty"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "76c98694c2da097e31d5e253d77e3c1e362d5d57"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	if got, ok := buildIDFromInfo(info); ok {
		t.Fatalf("dirty tree identified by its module version as %q", got)
	}
	unstamped := &debug.BuildInfo{Main: debug.Module{Version: "v0.11.1-0.20261025041301-76c98694c2da+dirty"}}
	if got, ok := buildIDFromInfo(unstamped); ok {
		t.Fatalf("+dirty version without VCS settings identified as %q", got)
	}
}

// The running test binary carries a linker build ID, and that is what the cache
// key is derived from: the same value `go tool buildid` prints for it.
func TestCurrentBuildIDIsTheLinkerBuildID(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	id, err := linkerBuildID(exe)
	if err != nil {
		t.Fatalf("linkerBuildID(%s): %v", exe, err)
	}
	if id == "" {
		t.Fatalf("no linker build ID in %s", exe)
	}
	if got := currentBuildID(); got != "l"+id {
		t.Fatalf("currentBuildID() = %q, want the linker build ID %q", got, "l"+id)
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go tool not on PATH: %v", err)
	}
	out, err := exec.Command(goTool, "tool", "buildid", exe).Output()
	if err != nil {
		t.Fatalf("go tool buildid: %v", err)
	}
	if want := strings.TrimSpace(string(out)); id != want {
		t.Fatalf("linkerBuildID = %q, go tool buildid = %q", id, want)
	}
}

// Two binaries of the same code share a linker build ID; differently stamped
// ones (as the Makefile's -X version flags stamp them) do not, however alike
// their build metadata is.
func TestLinkerBuildIDTellsBuildsApart(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a command twice")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go tool not on PATH: %v", err)
	}
	dir := t.TempDir()
	ids := make([]string, 2)
	for i := range ids {
		out := filepath.Join(dir, "sysml"+strconv.Itoa(i))
		build := exec.Command("go", "build", "-ldflags", "-X main.Version=build"+strconv.Itoa(i), "-o", out, "github.com/Open-MBEE/OpenSysML/cmd/sysml")
		if msg, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build: %v\n%s", err, msg)
		}
		id, err := linkerBuildID(out)
		if err != nil || id == "" {
			t.Fatalf("linkerBuildID(%s) = %q, %v", out, id, err)
		}
		ids[i] = id
	}
	if ids[0] == ids[1] {
		t.Fatalf("two differently stamped builds share linker build ID %q", ids[0])
	}
}

func TestLinkerBuildIDOfOtherFiles(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, []byte("not an executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	if id, err := linkerBuildID(plain); id != "" || err != nil {
		t.Fatalf("linkerBuildID(plain file) = %q, %v; want \"\", nil", id, err)
	}
	marked := filepath.Join(dir, "marked")
	text := append([]byte("MZ header bytes "), goBuildIDPrefix...)
	text = append(text, []byte("abcd/efgh/ijkl/mnop")...)
	text = append(text, goBuildIDSuffix...)
	if err := os.WriteFile(marked, text, 0o600); err != nil {
		t.Fatal(err)
	}
	if id, err := linkerBuildID(marked); id != "abcd/efgh/ijkl/mnop" || err != nil {
		t.Fatalf("linkerBuildID(marked text) = %q, %v; want the marked ID", id, err)
	}
	if _, err := linkerBuildID(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("linkerBuildID(missing file) reported no error")
	}
}
