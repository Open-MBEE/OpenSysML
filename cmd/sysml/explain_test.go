package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// explain runs the binary on arguments alone: an -explain run reads no model.
func explain(t *testing.T, binary string, args ...string) runOutcome {
	t.Helper()
	cmd := exec.Command(binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	status := 0
	if exit, ok := err.(*exec.ExitError); ok {
		status = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return runOutcome{stdout: stdout.String(), stderr: stderr.String(), status: status}
}

// TestExplainPrintsACode checks that -explain <code> and -explain=<code> print
// the catalogue entry of a code without reading a model, that -explain alone
// lists every code with its summary, and that every further word is a code.
func TestExplainPrintsACode(t *testing.T) {
	binary := buildCLI(t)

	got := explain(t, binary, "-explain=unresolved")
	wantReport(t, got, 0, "unresolved: error (name-resolution)", "Specification: KerML")
	if strings.Contains(got.output(), "syntax") {
		t.Errorf("-explain=<code> printed more than the code:\n%s", got.output())
	}

	for _, args := range [][]string{{"-explain"}, {"-explain", "-validate"}} {
		out := explain(t, binary, args...).stdout
		for _, want := range []string{"Syntax\n", "Name resolution\n", "  unresolved ", "  syntax ", "  document-plan-missing-title "} {
			if !strings.Contains(out, want) {
				t.Errorf("%v is missing %q:\n%s", args, want, out)
			}
		}
	}

	for _, args := range [][]string{{"-explain=unresolved", "-explain=syntax"}, {"-explain", "unresolved", "syntax"}, {"-explain", "unresolved", "--", "syntax"}} {
		got = explain(t, binary, args...)
		wantReport(t, got, 0, "unresolved: error", "\n\nsyntax: error (syntax)")
		if strings.Contains(got.stdout, "Syntax\n") {
			t.Errorf("%v listed the catalogue:\n%s", args, got.stdout)
		}
	}
}

// TestExplainRefusesAnUnknownCode checks that a code the catalogue lacks is a
// usage error that names the listing, however the code was written.
func TestExplainRefusesAnUnknownCode(t *testing.T) {
	binary := buildCLI(t)

	for _, args := range [][]string{{"-explain=bogus"}, {"-explain", "bogus"}, {"-explain", "--", "bogus"}, {"-explain", "true"}, {"-explain", "false"}} {
		got := explain(t, binary, args...)
		wantReport(t, got, 2, "unknown diagnostic code", "-explain")
		if got.stdout != "" {
			t.Errorf("%v: stdout = %q, want nothing", args, got.stdout)
		}
	}
}
func TestExplainSkipsNoLineForAnUnknownFirstCode(t *testing.T) {
	var out, errw bytes.Buffer
	if status := explainDiagnostics(&out, &errw, []string{"no-such", "syntax"}); status != 2 {
		t.Fatalf("status = %d", status)
	}
	if strings.HasPrefix(out.String(), "\n") || !strings.HasPrefix(out.String(), "syntax") {
		t.Errorf("output starts %q", out.String()[:20])
	}
}
