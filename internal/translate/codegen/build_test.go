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
	got, err := goCommand("go")
	if err != nil || got != want {
		t.Fatalf("goCommand() = %q, %v; want %q", got, err, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("goCommand() = %q is not an absolute path", got)
	}

	if got, err := goCommand("no-such-go-command-for-opensysml"); err == nil {
		t.Fatalf("goCommand() = %q for a missing override, want an error", got)
	}
}

func TestCompilerName(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv(CCompilerEnvVar, "")
		t.Setenv(GoCommandEnvVar, "")
		if got := compilerName(TargetC); got != "cc" {
			t.Errorf("compilerName(TargetC) = %q, want cc", got)
		}
		if got := compilerName(TargetGo); got != "go" {
			t.Errorf("compilerName(TargetGo) = %q, want go", got)
		}
	})

	t.Run("environment overrides", func(t *testing.T) {
		t.Setenv(CCompilerEnvVar, "custom-cc")
		t.Setenv(GoCommandEnvVar, "custom-go")
		if got := compilerName(TargetC); got != "custom-cc" {
			t.Errorf("compilerName(TargetC) = %q, want custom-cc", got)
		}
		if got := compilerName(TargetGo); got != "custom-go" {
			t.Errorf("compilerName(TargetGo) = %q, want custom-go", got)
		}
	})
}
