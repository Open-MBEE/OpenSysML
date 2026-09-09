package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/testutil/gobuild"
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
