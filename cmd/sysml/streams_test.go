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

// brokenModel does not parse, so nothing about it can be decided.
const brokenModel = "package Bad { part def A { attribute x : ; } }\n"

// TestStreamsAndStatus checks the contract every non-interactive run keeps: what
// was asked for on stdout, what went wrong on stderr under the command's own
// prefix, and a status that says whether it was carried out.
func TestStreamsAndStatus(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name      string
		model     string
		args      []string
		status    int
		stdout    []string // must be on stdout
		stderr    []string // must be on stderr, and not on stdout
		absent    []string // must be on neither stream
		emptyOut  bool     // stdout must be empty
		emptyErrs bool     // stderr must be empty
	}{{
		name:      "an evaluation reports its value",
		model:     checkModel,
		args:      []string{"-e", "1+1"},
		status:    exitHolds,
		stdout:    []string{"✓ 1+1", "= 2"},
		emptyErrs: true,
	}, {
		name:   "a model that does not analyse stops an evaluation",
		model:  brokenModel,
		args:   []string{"-e", "1+1"},
		status: exitUnevaluable,
		stderr: []string{"error: expected a name", "did not analyse cleanly"},
	}, {
		name:   "an unresolved expression is reported",
		model:  checkModel,
		args:   []string{"-e", "nope"},
		status: exitUnevaluable,
		stdout: []string{"✓ package Rover"},
		stderr: []string{"sysml: unresolved reference: nope"},
		absent: []string{"error: unresolved reference"},
	}, {
		name:   "a model that does not analyse is a failed load",
		model:  brokenModel,
		args:   nil,
		status: exitUnevaluable,
		stderr: []string{"error: expected a name", "did not analyse cleanly"},
	}, {
		name:   "a calculation reports what it computed",
		model:  behaviorModel,
		args:   []string{"-calc", "Mission::Fall(3, 2)"},
		status: exitHolds,
		stdout: []string{"= 18"},
	}, {
		name:   "a calculation that could not be invoked is reported",
		model:  behaviorModel,
		args:   []string{"-calc", "Mission::nosuch(1)"},
		status: exitUnevaluable,
		stderr: []string{"sysml: unresolved reference: Mission::nosuch"},
		absent: []string{"error: unresolved reference"},
	}, {
		name:   "an action that could not be run is reported",
		model:  behaviorModel,
		args:   []string{"-action", "Mission::nosuch"},
		status: exitUnevaluable,
		stderr: []string{"nosuch", "sysml: unresolved reference: Mission::nosuch"},
		absent: []string{"error: unresolved reference"},
	}, {
		name:   "a state machine that could not be run is reported",
		model:  behaviorModel,
		args:   []string{"-state", "Mission::nosuch"},
		status: exitUnevaluable,
		stderr: []string{"nosuch", "sysml: unresolved reference: Mission::nosuch"},
		absent: []string{"error: unresolved reference"},
	}, {
		name:   "a constraint that could not be checked is reported",
		model:  checkModel,
		args:   []string{"-constraint", "Rover::nosuch"},
		status: exitUnevaluable,
		stderr: []string{"sysml: unresolved reference: Rover::nosuch"},
		absent: []string{"error: unresolved reference"},
	}, {
		// The line goes through the same boundary as the other undecided
		// verdicts, so the command reports it under the command's prefix.
		name:   "a model that states no satisfaction assertion decides nothing",
		model:  warningModel,
		args:   []string{"-satisfy"},
		status: exitUnevaluable,
		stderr: []string{"no satisfaction assertion in the session"},
	}, {
		name:   "a check the model decided false stays status 1",
		model:  checkModel,
		args:   []string{"-constraint", "Rover::TooHeavy"},
		status: exitFailed,
		stdout: []string{"✗ Constraint Rover::TooHeavy failed"},
	}, {
		name:   "a check that holds succeeds",
		model:  checkModel,
		args:   []string{"-constraint", "Rover::MassBudget"},
		status: exitHolds,
		stdout: []string{"✓ Constraint Rover::MassBudget passed"},
	}, {
		name:   "a warning is reported on stderr and decides nothing",
		model:  warningModel,
		args:   []string{"-constraint", "Rover::MassBudget"},
		status: exitHolds,
		stdout: []string{"✓ Constraint Rover::MassBudget passed"},
		stderr: []string{"warning:"},
	}, {
		name:   "a validated model reports its diagnostics on stderr",
		model:  brokenModel,
		args:   []string{"-validate"},
		status: exitUnevaluable,
		stderr: []string{"error: expected a name", "did not analyse cleanly"},
	}, {
		name:   "an RDF conversion writes only the model out, and its status on stderr",
		model:  sampleModel,
		args:   []string{"-convert", "ttl"},
		status: exitHolds,
		stdout: []string{"@prefix sysml:"},
		stderr: []string{"note: RDF conversion is experimental"},
	}, {
		name:      "a notation conversion writes only the model out",
		model:     sampleModel,
		args:      []string{"-convert", "sysml"},
		status:    exitHolds,
		stdout:    []string{"package"},
		emptyErrs: true,
	}, {
		name:     "a failed conversion writes nothing out",
		model:    brokenModel,
		args:     []string{"-convert", "ttl"},
		status:   exitUnevaluable,
		stderr:   []string{"expected a name", "sysml: "},
		emptyOut: true,
	}, {
		name:     "a format that is not a format is reported under the command's prefix",
		model:    sampleModel,
		args:     []string{"-convert", "nosuchformat"},
		status:   exitUnevaluable,
		stderr:   []string{"sysml: unknown format \"nosuchformat\""},
		emptyOut: true,
	}, {
		name:      "the help asked for is a result on stdout",
		model:     checkModel,
		args:      []string{"-h"},
		status:    exitHolds,
		stdout:    []string{"Usage: sysml [options] [file...]", "-convert string"},
		emptyErrs: true,
	}, {
		name:     "a flag that is not defined is reported with the usage on stderr",
		model:    checkModel,
		args:     []string{"--nosuchflag"},
		status:   exitUnevaluable,
		stderr:   []string{"flag provided but not defined: -nosuchflag", "Usage: sysml [options] [file...]"},
		emptyOut: true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runStreams(t, binary, tc.model, tc.args...)
			if got.status != tc.status {
				t.Errorf("exit status = %d, want %d\n%s", got.status, tc.status, got.output())
			}
			for _, want := range tc.stdout {
				if !strings.Contains(got.stdout, want) {
					t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
				}
			}
			for _, want := range tc.stderr {
				if !strings.Contains(got.stderr, want) {
					t.Errorf("stderr is missing %q:\n%s", want, got.stderr)
				}
				if strings.Contains(got.stdout, want) {
					t.Errorf("%q was reported on stdout:\n%s", want, got.stdout)
				}
			}
			for _, unwanted := range tc.absent {
				if strings.Contains(got.output(), unwanted) {
					t.Errorf("%q was reported:\n%s", unwanted, got.output())
				}
			}
			if tc.emptyOut && got.stdout != "" {
				t.Errorf("stdout should be empty, got:\n%s", got.stdout)
			}
			if tc.emptyErrs && got.stderr != "" {
				t.Errorf("stderr should be empty, got:\n%s", got.stderr)
			}
		})
	}
}

// TestConvertToFileStaysClean checks that a conversion written to a file holds
// the model and nothing about the run, which is reported on stderr.
func TestConvertToFileStaysClean(t *testing.T) {
	binary := buildCLI(t)

	out := filepath.Join(t.TempDir(), "model.ttl")
	got := runStreams(t, binary, sampleModel, "-convert", "ttl", "-o", out)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if !strings.Contains(got.stderr, "wrote "+out) {
		t.Errorf("stderr should name the file written, got:\n%s", got.stderr)
	}
	written, err := os.ReadFile(out) // #nosec G304 -- the test wrote this path.
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "@prefix sysml:") {
		t.Errorf("conversion is missing from the file:\n%s", written)
	}
	for _, unwanted := range []string{"sysml:", "wrote ", "✓"} {
		if strings.Contains(string(written), "\n"+unwanted) {
			t.Errorf("the file carries a line about the run (%q):\n%s", unwanted, written)
		}
	}
}

// TestPromptDoesNotExitOnABadLine checks that the prompt keeps taking lines
// after one it could not carry out, and that %quit ends the session cleanly.
func TestPromptDoesNotExitOnABadLine(t *testing.T) {
	binary := buildCLI(t)

	got := runPiped(t, binary, "%eval nope\npart def Wheel;\n%quit\n", checkModel)
	if got.status != exitHolds {
		t.Errorf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"unresolved reference: nope", "✓ part def Wheel"} {
		if !strings.Contains(got.output(), want) {
			t.Errorf("the session is missing %q:\n%s", want, got.output())
		}
	}
}

// TestPipedSessionGatesOnTheModel checks that a session driven from a pipe
// reports the model it was given: an unusable one leaves the run undecided,
// while %quit over a model that analysed exits 0.
func TestPipedSessionGatesOnTheModel(t *testing.T) {
	binary := buildCLI(t)

	broken := runPiped(t, binary, "%quit\n", brokenModel)
	if broken.status != exitUnevaluable {
		t.Errorf("exit status = %d, want %d\n%s", broken.status, exitUnevaluable, broken.output())
	}
	if strings.Contains(broken.stdout, "error: expected a name") {
		t.Errorf("the diagnostic was reported on stdout:\n%s", broken.stdout)
	}

	clean := runPiped(t, binary, "%quit\n", checkModel)
	if clean.status != exitHolds {
		t.Errorf("exit status = %d, want %d\n%s", clean.status, exitHolds, clean.output())
	}
}

// runStreams runs the binary over a model with nothing on stdin, so the run is
// the non-interactive one a build step makes.
func runStreams(t *testing.T, binary, model string, args ...string) runOutcome {
	t.Helper()
	return runWith(t, binary, "", model, args...)
}

// runPiped runs the binary over a model with lines on stdin, as a script driving
// the prompt does.
func runPiped(t *testing.T, binary, stdin, model string, args ...string) runOutcome {
	t.Helper()
	return runWith(t, binary, stdin, model, args...)
}

func runWith(t *testing.T, binary, stdin, model string, args ...string) runOutcome {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(model), 0o600); err != nil {
		t.Fatal(err)
	}
	return runBinary(t, binary, stdin, append(args, path))
}

// runFiles runs the binary over model files already on disk, named after the
// flags, with nothing on stdin.
func runFiles(t *testing.T, binary string, files []string, args ...string) runOutcome {
	t.Helper()
	return runBinary(t, binary, "", append(args, files...))
}

func runBinary(t *testing.T, binary, stdin string, args []string) runOutcome {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Stdin = strings.NewReader(stdin)
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
		t.Fatalf("%v: %v\n%s", args, err, result.output())
	}
	return result
}
