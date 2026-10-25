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

// A modified tree reuses one revision for every edit, so it must not identify a build (#491).
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

// The identity has to be file-name safe and stable within one process.
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

// Go 1.24 stamps a dirty tree's main module `<pseudo-version>+dirty`, one value for
// every edit of that commit; accepting it let differently patched builds share records.
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

// A clean checkout is identified by its revision, every other build by the linker's
// build ID; a modified tree's metadata never identifies it.
func TestBuildIdentityOrder(t *testing.T) {
	clean := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}, {Key: "vcs.modified", Value: "false"}}}
	dirty := &debug.BuildInfo{
		Main:     debug.Module{Version: "v0.11.1-0.20261025041301-76c98694c2da+dirty"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}, {Key: "vcs.modified", Value: "true"}},
	}
	released := &debug.BuildInfo{Main: debug.Module{Version: "v0.11.0"}}
	cases := []struct {
		name     string
		info     *debug.BuildInfo
		linkerID string
		want     string
	}{
		{"clean checkout, stamped link", clean, "L1", "gabc123"},
		{"clean checkout, no linker ID", clean, "", "gabc123"},
		{"modified tree", dirty, "L1", "lL1"},
		{"modified tree, no linker ID", dirty, "", ""},
		{"module build", released, "L1", "lL1"},
		{"module build, no linker ID", released, "", "mv0.11.0"},
		{"nothing known", &debug.BuildInfo{}, "", ""},
	}
	for _, c := range cases {
		got, ok := buildIdentity(c.info, c.linkerID)
		if ok != (c.want != "") || got != c.want {
			t.Errorf("%s: buildIdentity = %q, %v; want %q", c.name, got, ok, c.want)
		}
	}
}

// The running test binary carries a linker build ID equal to `go tool buildid`'s, and
// the cache identity is that ID unless the binary records a clean checkout.
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
	want := "l" + id
	if info, ok := debug.ReadBuildInfo(); ok {
		if rev, ok := cleanRevision(info); ok {
			want = "g" + rev
		}
	}
	if got := currentBuildID(); got != want {
		t.Fatalf("currentBuildID() = %q, want %q", got, want)
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

// Differently stamped links of the same code (as the Makefile's -X flags stamp them)
// get distinct linker build IDs, however alike their build metadata is.
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
