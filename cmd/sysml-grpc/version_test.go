package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/buildinfo"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// Builds with the Makefile's -X flag names, so renaming the metadata variables
// cannot silently leave a released binary reporting "dev".
func TestVersionReportsWhatTheLinkerSet(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sysml-grpc")
	ldflags := "-X main.Version=v9.9.9 -X main.Commit=abc1234 -X main.BuildTime=2026-01-02_03:04:05"
	if out, err := exec.Command("go", gobuild.Args(binary, "-ldflags", ldflags)...).CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	out, err := exec.Command(binary, "-version").CombinedOutput()
	if err != nil {
		t.Fatalf("-version: %v\n%s", err, out)
	}
	for _, want := range []string{"sysml-grpc version v9.9.9", "commit: abc1234", "built: 2026-01-02_03:04:05"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("-version is missing %q:\n%s", want, out)
		}
	}
}

// Builds without -X flags, as `go install` does, so the binary reports the
// commit and time the toolchain recorded from the checkout rather than "unknown".
func TestVersionReportsTheEmbeddedBuildInfoWithoutLinkerStamps(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sysml-grpc")
	if out, err := exec.Command("go", gobuild.Args(binary)...).CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	info, err := gobuild.ReadBuildInfo(binary)
	if err != nil {
		t.Fatal(err)
	}
	want := buildinfo.ResolveFrom(buildinfo.Stamps{}, info)
	if want.Commit == buildinfo.Unknown {
		if exec.Command("git", "rev-parse", "--is-inside-work-tree").Run() != nil {
			t.Skip("not built from a checkout: the toolchain recorded no VCS metadata")
		}
		t.Fatalf("built from a checkout, but resolved %+v from %+v", want, info.Settings)
	}

	out, err := exec.Command(binary, "-version").CombinedOutput()
	if err != nil {
		t.Fatalf("-version: %v\n%s", err, out)
	}
	got := strings.Split(strings.TrimSpace(string(out)), "\n")
	wantLines := []string{"sysml-grpc version " + want.Version, "commit: " + want.Commit, "built: " + want.BuildTime}
	if strings.Join(got, "\n") != strings.Join(wantLines, "\n") {
		t.Errorf("-version printed:\n%s\nwant:\n%s", out, strings.Join(wantLines, "\n"))
	}
}
