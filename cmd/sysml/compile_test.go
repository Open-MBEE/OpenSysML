package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/codegen"
)

const compileModel = `package Compiled {
    private import ScalarValues::*;
    calc def Fib { in n : Integer; return : Integer = if n < 2 ? n else Fib(n - 1) + Fib(n - 2); }
}
`

// -source writes the generated program for each target where -o names and stops
// there, so the calc is compiled without a C or Go toolchain being driven.
func TestCompileWritesTheSourceForEachTarget(t *testing.T) {
	binary := buildCLI(t)
	for _, target := range codegen.Targets() {
		out := filepath.Join(t.TempDir(), "fib"+codegen.SourceExtension(target))
		got := runStreams(t, binary, compileModel, "-compile", "Compiled::Fib", "-source", "-target", string(target), "-o", out)
		if got.status != exitHolds {
			t.Fatalf("%s: exit status = %d, want %d\n%s", target, got.status, exitHolds, got.output())
		}
		src, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if !strings.Contains(string(src), "Compiled::Fib") {
			t.Errorf("%s: the source does not name Compiled::Fib:\n%s", target, src)
		}
	}
}

// What stops a compilation is reported and the run decides nothing.
func TestCompileRefusals(t *testing.T) {
	binary := buildCLI(t)
	out := filepath.Join(t.TempDir(), "fib")
	cases := []struct {
		name  string
		model string
		args  []string
		want  string
	}{
		{"unknown-target", compileModel, []string{"-compile", "Compiled::Fib", "-target", "rust", "-o", out}, "unknown target"},
		{"unknown-calc", compileModel, []string{"-compile", "Compiled::Missing", "-source", "-o", out}, "Compiled::Missing"},
		{"model-with-errors", "package Broken { part p : Nope::Missing; }\n", []string{"-compile", "Broken::p", "-source", "-o", out}, "did not analyse cleanly"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runStreams(t, binary, tc.model, tc.args...)
			if got.status != exitUnevaluable {
				t.Errorf("exit status = %d, want %d\n%s", got.status, exitUnevaluable, got.output())
			}
			if !strings.Contains(got.stderr, tc.want) {
				t.Errorf("stderr is missing %q:\n%s", tc.want, got.stderr)
			}
		})
	}
}

// A compilation request combined with another mode is refused, never silently
// dropped in favour of the other mode — whichever mode dispatches first.
func TestCompileMutualExclusions(t *testing.T) {
	binary := buildCLI(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"sync-diff", []string{"-compile", "Mission::Fall", "-sync-diff", "graph.ttl"}, "cannot be combined"},
		{"sync-apply", []string{"-compile", "Mission::Fall", "-sync-apply", "http://localhost/api"}, "cannot be combined"},
		{"render-documents", []string{"-compile", "Mission::Fall", "-render-documents", "out"}, "cannot be combined"},
		{"render-all", []string{"-compile", "Mission::Fall", "-render-all", "out"}, "cannot be combined"},
		{"render-document", []string{"-compile", "Mission::Fall", "-render-document", "Mission::Doc"}, "cannot be combined"},
		{"target-without-compile", []string{"-target", "go", "-render-all", "out"}, "apply to -compile"},
		{"source-without-compile", []string{"-source", "-sync-diff", "graph.ttl"}, "apply to -compile"},
		{"empty-sync-diff", []string{"-compile", "Mission::Fall", "-o", "fall", "-sync-diff", ""}, "-sync-diff is empty"},
		{"empty-sync-apply", []string{"-compile", "Mission::Fall", "-o", "fall", "-sync-apply", ""}, "-sync-apply is empty"},
		{"sync-base", []string{"-compile", "Mission::Fall", "-o", "fall", "-sync-base", "base.ttl"}, "not to -compile"},
		{"sync-state", []string{"-compile", "Mission::Fall", "-o", "fall", "-sync-state", "state.json"}, "not to -compile"},
		{"sync-confirm-deletes", []string{"-compile", "Mission::Fall", "-o", "fall", "-sync-confirm-deletes"}, "not to -compile"},
		{"sync-mint-ids", []string{"-compile", "Mission::Fall", "-o", "fall", "-sync-mint-ids"}, "not to -compile"},
		{"sync-annotate", []string{"-compile", "Mission::Fall", "-o", "fall", "-sync-annotate", "notes.sysml"}, "not to -compile"},
		{"check", []string{"-compile", "Mission::Fall", "-o", "fall", "-validate"}, "check it in its own run"},
		{"no-output", []string{"-compile", "Mission::Fall"}, "needs -o"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runStreams(t, binary, behaviorModel, tc.args...)
			if got.status != exitUnevaluable {
				t.Errorf("exit status = %d, want %d\n%s", got.status, exitUnevaluable, got.output())
			}
			if !strings.Contains(got.stderr, tc.want) {
				t.Errorf("stderr is missing %q:\n%s", tc.want, got.stderr)
			}
		})
	}
}
