package main

import (
	"os/exec"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/buildinfo"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// A build without linker stamps, as `go install` makes, reports what the
// toolchain embedded: the commit and its time from the checkout it was built
// from, with the version unversioned until a release tag names the build.
func TestVersionReportsTheEmbeddedBuildInfoWithoutLinkerStamps(t *testing.T) {
	binary := serverBinary(t)
	info, err := gobuild.ReadBuildInfo(binary)
	if err != nil {
		t.Fatal(err)
	}
	want := buildinfo.ResolveFrom(buildinfo.Stamps{}, info)
	if setting(info, "vcs.revision") == "" {
		if exec.Command("git", "rev-parse", "--is-inside-work-tree").Run() != nil {
			t.Skip("not built from a checkout: the toolchain recorded no VCS metadata")
		}
		t.Fatal("built from a checkout, but the toolchain recorded no vcs.revision")
	}
	if want.Commit == buildinfo.Unknown || want.BuildTime == buildinfo.Unknown {
		t.Fatalf("resolved %+v from %+v: the VCS metadata was not used", want, info.Settings)
	}

	out, err := exec.Command(binary, "-version").CombinedOutput()
	if err != nil {
		t.Fatalf("-version: %v\n%s", err, out)
	}
	if got := string(out); got != want.Report("sysml-lsp") {
		t.Errorf("-version printed:\n%s\nwant:\n%s", got, want.Report("sysml-lsp"))
	}
}

// initialize reports the version the binary itself prints, not a literal.
func TestInitializeReportsTheVersionOfTheBinary(t *testing.T) {
	out, err := exec.Command(serverBinary(t), "-version").Output()
	if err != nil {
		t.Fatalf("-version: %v", err)
	}
	first, _, _ := strings.Cut(string(out), "\n")
	version, found := strings.CutPrefix(first, "sysml-lsp ")
	if !found || version == "" {
		t.Fatalf("-version first line = %q, want \"sysml-lsp <version>\"", first)
	}

	s := startServer(t)
	res := s.initialize(1)
	result, _ := res["result"].(map[string]any)
	serverInfo, _ := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "sysml-lsp" {
		t.Errorf("serverInfo.name = %v, want %q", serverInfo["name"], "sysml-lsp")
	}
	if serverInfo["version"] != version {
		t.Errorf("serverInfo.version = %v, want %q as -version prints", serverInfo["version"], version)
	}
	s.request(2, "shutdown", nil)
	s.response(2)
	s.notify("exit", nil)
	if status := s.waitStatus(20 * time.Second); status != exitServed {
		t.Errorf("exit status = %d, want %d\nstderr: %s", status, exitServed, s.stderr.String())
	}
}

// setting is the value of one build setting, or "" when it was not recorded.
func setting(info *debug.BuildInfo, key string) string {
	for _, s := range info.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}
