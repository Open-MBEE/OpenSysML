package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// unresolvedModel does not resolve, so a diagnostic names the model it is about.
const unresolvedModel = "package Bad { part rover : Undefined; }\n"

// TestStdinIsRead checks that a lone "-" names standard input for every mode
// that takes a model path, and that diagnostics call what was read <stdin>.
func TestStdinIsRead(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name   string
		model  string
		args   []string
		status int
		stdout []string
		stderr []string
	}{{
		name:   "a validated model read from standard input is named <stdin>",
		model:  unresolvedModel,
		args:   []string{"-validate", "-"},
		status: exitUnevaluable,
		stderr: []string{"<stdin>:1:28: error[unresolved]: unresolved reference: Undefined", "sysml: <stdin> did not analyse cleanly"},
	}, {
		name:   "a model read from standard input that analyses validates clean",
		model:  checkModel,
		args:   []string{"-validate", "-"},
		status: exitHolds,
		stdout: []string{"✓ <stdin>: no errors"},
	}, {
		name:   "a check runs over a model read from standard input",
		model:  checkModel,
		args:   []string{"-constraint", "Rover::MassBudget", "-"},
		status: exitHolds,
		stdout: []string{"✓ Constraint Rover::MassBudget passed"},
	}, {
		name:   "an evaluation runs over a model read from standard input",
		model:  checkModel,
		args:   []string{"-e", "1+1", "-"},
		status: exitHolds,
		stdout: []string{"= 2"},
	}, {
		name:   "a conversion reads the model from standard input",
		model:  sampleModel,
		args:   []string{"-convert", "ttl", "-from", "sysml", "-"},
		status: exitHolds,
		stdout: []string{"@prefix sysml:"},
	}, {
		// Standard input carries no extension, so the format cannot be
		// inferred and the flag that names it is asked for.
		name:   "a conversion from standard input without -from is reported",
		model:  sampleModel,
		args:   []string{"-convert", "ttl", "-"},
		status: exitUnevaluable,
		stderr: []string{"sysml: standard input carries no file name to take the format from", "-from sysml"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runStdin(t, binary, tc.model, tc.args...)
			if got.status != tc.status {
				t.Errorf("exit status = %d, want %d\n%s", got.status, tc.status, got.output())
			}
			for _, want := range tc.stdout {
				if !strings.Contains(got.stdout, want) {
					t.Errorf("stdout is missing %q:\n%s", want, got.output())
				}
			}
			for _, want := range tc.stderr {
				if !strings.Contains(got.stderr, want) {
					t.Errorf("stderr is missing %q:\n%s", want, got.output())
				}
			}
		})
	}
}

// TestAFileNamedDashIsStillRead checks that a file really called "-" is read by
// naming it ./-, so making "-" mean the stream takes no file out of reach.
func TestAFileNamedDashIsStillRead(t *testing.T) {
	binary := buildCLI(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "-"), []byte(checkModel), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, "-validate", "./-")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("")
	got := runCommand(t, cmd)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if !strings.Contains(got.stdout, "✓ ./-: no errors") {
		t.Errorf("the file named ./- was not validated:\n%s", got.output())
	}
}

// TestStdinFromADeviceIsRead checks that a "-" whose standard input is /dev/null
// — what a CI runner or a supervisor commonly leaves it as — is read as the
// empty model it is, rather than mistaken for a terminal.
func TestStdinFromADeviceIsRead(t *testing.T) {
	binary := buildCLI(t)

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	cmd := exec.Command(binary, "-validate", "-")
	cmd.Stdin = devNull
	got := runCommand(t, cmd)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if !strings.Contains(got.stdout, "✓ <stdin>: no errors") {
		t.Errorf("standard input from %s was not read:\n%s", os.DevNull, got.output())
	}
}

// runStdin runs the binary with the model on standard input rather than in a
// file, which is what a "-" on the command line asks it to read.
func runStdin(t *testing.T, binary, model string, args ...string) runOutcome {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Stdin = strings.NewReader(model)
	return runCommand(t, cmd)
}

// runCommand runs cmd and reports its streams and status without failing the
// test, since the status is part of what is under test.
func runCommand(t *testing.T, cmd *exec.Cmd) runOutcome {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := runOutcome{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		result.status = exit.ExitCode()
	default:
		t.Fatalf("%v: %v\n%s", cmd.Args, err, result.output())
	}
	return result
}
