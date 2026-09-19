package codegen

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGoCommandResolvesTheToolchainBeforeRunningIt(t *testing.T) {
	want, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command on PATH")
	}
	t.Setenv(GoCommandEnvVar, "")
	got, err := goCommand()
	if err != nil || got != want {
		t.Fatalf("goCommand() = %q, %v; want %q", got, err, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("goCommand() = %q is not an absolute path", got)
	}

	t.Setenv(GoCommandEnvVar, "no-such-go-command-for-opensysml")
	if got, err := goCommand(); err == nil {
		t.Fatalf("goCommand() = %q for a missing override, want an error", got)
	}
}
