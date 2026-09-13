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

// checkModel states conditions that decide both ways, so a check's exit status
// is exercised rather than only its report.
const checkModel = `package Rover {
    part def Battery {
        attribute capacity;
        attribute charge;
    }

    part pack : Battery {
        attribute :>> capacity = 100.0;
        attribute :>> charge = 80.0;
        constraint notOvercharged { charge <= capacity }
    }

    constraint MassBudget { 180.0 <= 200.0 }
    constraint TooHeavy { 210.0 <= 200.0 }

    requirement PowerMargin {
        require constraint { 600.0 >= 450.0 }
    }

    part def Lander {
        attribute verticalSpeed;
    }
    requirement def TouchdownRequirement {
        subject lander : Lander;
        attribute maxVerticalSpeed;
        require constraint {
            lander.verticalSpeed <= maxVerticalSpeed
        }
    }
    requirement touchdown : TouchdownRequirement {
        attribute :>> maxVerticalSpeed = 1.5;
    }
    part slowLander : Lander {
        attribute :>> verticalSpeed = 1.2;
    }
    part fastLander : Lander {
        attribute :>> verticalSpeed = 2.4;
    }
    part analysisContext {
        assert satisfy touchdown by slowLander;
        assert satisfy touchdown by fastLander;
    }
}
`

// runOutcome is what a build step sees of a model check.
type runOutcome struct {
	status         int
	stdout, stderr string
}

func (r runOutcome) output() string { return r.stdout + r.stderr }

// check runs the binary on a model written to a temporary file and reports the
// exit status without failing the test, since the status is what is under test.
func check(t *testing.T, binary, model string, args ...string) runOutcome {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, append(args, path)...)
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

func wantReport(t *testing.T, got runOutcome, status int, substrings ...string) {
	t.Helper()
	if got.status != status {
		t.Errorf("exit status = %d, want %d\n%s", got.status, status, got.output())
	}
	for _, want := range substrings {
		if !strings.Contains(got.output(), want) {
			t.Errorf("report is missing %q:\n%s", want, got.output())
		}
	}
}

// TestCheckExitStatus checks the contract a build step depends on: a model that
// holds succeeds, a verdict against it fails with 1, and a check that could not
// be made at all is distinguished as 2.
func TestCheckExitStatus(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::MassBudget", "-requirement", "Rover::PowerMargin"),
		0, "✓ Constraint Rover::MassBudget passed", "✓ Requirement Rover::PowerMargin satisfied")

	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::TooHeavy"),
		1, "✗ Constraint Rover::TooHeavy failed", "Assertion evaluated to false: 210.0 <= 200.0")

	// A verdict that failed is reported even when a later check holds.
	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::TooHeavy", "-constraint", "Rover::MassBudget"),
		1, "✗ Constraint Rover::TooHeavy failed", "✓ Constraint Rover::MassBudget passed")

	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::nosuch"),
		2, "unresolved reference: Rover::nosuch")

	// Not deciding outranks a failed verdict: the model was not fully checked.
	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::TooHeavy", "-requirement", "nosuch"),
		2, "✗ Constraint Rover::TooHeavy failed", "unresolved reference: nosuch")

	// A requirement whose subject nothing binds decided nothing about the model,
	// so the report says so rather than contradicting the status a script reads.
	undecided := check(t, binary, checkModel, "-requirement", "Rover::touchdown")
	wantReport(t, undecided, 2,
		"? Requirement Rover::touchdown could not be evaluated", "lander subject is unbound")
	rejectReport(t, undecided, "✗ Requirement Rover::touchdown failed")
}

// rejectReport checks that a report does not say something, which is how wording
// that contradicts the exit status is caught.
func rejectReport(t *testing.T, got runOutcome, substrings ...string) {
	t.Helper()
	for _, unwanted := range substrings {
		if strings.Contains(got.output(), unwanted) {
			t.Errorf("report says %q:\n%s", unwanted, got.output())
		}
	}
}

// TestEvalAfterInstantiateThroughCLI: `-instantiate p -e f` answers about the
// object of p, as a check of a condition over f does, rather than about the
// declared default.
func TestEvalAfterInstantiateThroughCLI(t *testing.T) {
	binary := buildCLI(t)
	const model = `package P {
    part def Sensor { attribute reading default = 0.0; }
    part hot : Sensor { attribute :>> reading = 140.0; }
}
`
	wantReport(t, check(t, binary, model, "-instantiate", "P::hot", "-e", "P::Sensor::reading"),
		0, "(on P::hot ID: 1)", "= 140.0")

	// With no object the declared default is the answer, claiming no object.
	answered := check(t, binary, model, "-e", "P::Sensor::reading")
	wantReport(t, answered, 0, "= 0.0")
	rejectReport(t, answered, "(on ")
}

// TestCheckOfInheritedConstraintAfterInstantiate checks what `-instantiate p
// -constraint C` promises: the verdict, and so the exit status a build step reads,
// is about the object of p rather than about C's declared defaults.
func TestCheckOfInheritedConstraintAfterInstantiate(t *testing.T) {
	binary := buildCLI(t)
	const model = `package P {
    part def Sensor {
        attribute reading default = 0.0;
        constraint inRange { reading <= 100.0 }
    }
    part hot : Sensor { attribute :>> reading = 140.0; }
}
`
	wantReport(t, check(t, binary, model, "-instantiate", "P::hot", "-constraint", "P::Sensor::inRange"),
		1, "✗ Constraint P::Sensor::inRange failed (on P::hot ID: 1)")

	// With no object of the constraint's type the check is about declared
	// defaults, which hold.
	wantReport(t, check(t, binary, model, "-constraint", "P::Sensor::inRange"),
		0, "✓ Constraint P::Sensor::inRange passed")
}

// TestCheckSatisfyThroughCLI checks the requirement-traceability gate, both over
// the whole model and for one element's assertions.
func TestCheckSatisfyThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, checkModel, "-satisfy"), 1,
		"✓ satisfy touchdown by slowLander holds",
		"✗ satisfy touchdown by fastLander fails")

	wantReport(t, check(t, binary, checkModel, "-satisfy=Rover::analysisContext"), 1,
		"✗ satisfy touchdown by fastLander fails")

	// An element stating no assertion decided nothing, so the check failed to run.
	wantReport(t, check(t, binary, checkModel, "-satisfy=Rover::touchdown"), 2,
		"no satisfaction assertion in Rover::touchdown")
}

// TestCheckModelSplitAcrossFiles checks that a reference from one file to a
// declaration in another resolves, whatever order the files are named in: the
// analysis gate is about the model, not about each file as it is read.
func TestCheckModelSplitAcrossFiles(t *testing.T) {
	binary := buildCLI(t)

	dir := t.TempDir()
	user := filepath.Join(dir, "user.sysml")
	defined := filepath.Join(dir, "defined.sysml")
	if err := os.WriteFile(user, []byte("package A {\n    part rover : B::Rover;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defined, []byte("package B {\n    part def Rover;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, order := range [][]string{{user, defined}, {defined, user}} {
		cmd := exec.Command(binary, append([]string{"-validate"}, order...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("-validate %v = %v, want the model to analyse cleanly\n%s", order, err, out)
		}
		if strings.Contains(string(out), "error:") {
			t.Errorf("-validate %v reported an error it went on to accept:\n%s", order, out)
		}
	}

	// A reference no file declares is still a finding, named in the file that
	// makes it so the reader knows which of them to go to.
	broken := filepath.Join(dir, "broken.sysml")
	if err := os.WriteFile(broken, []byte("package C {\n    part probe : D::Probe;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(binary, "-validate", defined, broken).CombinedOutput()
	if err == nil {
		t.Errorf("-validate accepted an unresolved reference:\n%s", out)
	}
	for _, want := range []string{"broken.sysml:2:18: error: unresolved reference: D::Probe", "did not analyse cleanly"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("report is missing %q:\n%s", want, out)
		}
	}
}

// TestCheckFilesOpeningTheSamePackage checks that files of one model that open
// the same package accumulate: neither the elements nor the findings of the
// earlier file are superseded by the later one declaring that package too.
func TestCheckFilesOpeningTheSamePackage(t *testing.T) {
	binary := buildCLI(t)

	dir := t.TempDir()
	first := filepath.Join(dir, "first.sysml")
	second := filepath.Join(dir, "second.sysml")
	if err := os.WriteFile(first, []byte("package M {\n    constraint Held { 1.0 <= 2.0 }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("package M {\n    constraint Also { 2.0 <= 3.0 }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(binary, "-constraint", "M::Held", "-constraint", "M::Also", first, second).CombinedOutput()
	if err != nil {
		t.Errorf("a constraint of each file = %v, want both checked\n%s", err, out)
	}

	// An error in the first file must still stop the check, rather than being
	// dropped along with the file the second one redeclared.
	if err := os.WriteFile(first, []byte("package M {\n    part x : Nope::Missing;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = exec.Command(binary, "-validate", first, second).CombinedOutput()
	if err == nil {
		t.Errorf("-validate accepted a model whose first file has an error:\n%s", out)
	}
	if !strings.Contains(string(out), "first.sysml:2:14: error: unresolved reference: Nope::Missing") {
		t.Errorf("the first file's error was not reported:\n%s", out)
	}
}

// TestCheckHonoursVerbosity checks that a check reports at the verbosity asked
// for: -quiet reports errors only, and -debug names the pass behind a finding.
func TestCheckHonoursVerbosity(t *testing.T) {
	binary := buildCLI(t)

	const warns = "package W {\n    attribute flag = 1 == \"one\";\n}\n"
	quiet := check(t, binary, warns, "-quiet", "-validate")
	wantReport(t, quiet, 0, "no errors")
	if strings.Contains(quiet.output(), "warning:") {
		t.Errorf("-quiet reported a warning it was asked to suppress:\n%s", quiet.output())
	}
	wantReport(t, check(t, binary, warns, "-validate"), 0, "warning: comparing Natural with String")
	wantReport(t, check(t, binary, warns, "-debug", "-validate"), 0, "[type/type.expr]")

	// An error is what stops the check, so no verbosity hides it.
	wantReport(t, check(t, binary, "package E {\n    part x : Nope::Missing;\n}\n", "-quiet", "-validate"),
		2, "error: unresolved reference: Nope::Missing")
}

// TestConvertAndCheckAreSeparateRuns checks that a check asked for alongside a
// conversion is reported as a misuse, rather than the conversion silently
// answering nothing about the model.
func TestConvertAndCheckAreSeparateRuns(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, checkModel, "-convert", "ttl", "-validate")
	wantReport(t, got, 2, "-convert writes the model out")
	if strings.Contains(got.stdout, "@prefix") {
		t.Errorf("the model was converted anyway:\n%s", got.output())
	}
}

// TestSatisfyCanBeTurnedOff checks that the off spelling of a flag declared
// boolean asks for no satisfaction check, rather than for an element named
// "false", so -satisfy=$on works in a script.
func TestSatisfyCanBeTurnedOff(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, checkModel, "-satisfy=false", "-constraint", "Rover::MassBudget")
	wantReport(t, got, 0, "✓ Constraint Rover::MassBudget passed")
	if strings.Contains(got.output(), "satisfy touchdown") {
		t.Errorf("-satisfy=false checked satisfaction anyway:\n%s", got.output())
	}

	// With nothing else asked for, that leaves no check to make, which is reported
	// rather than leaving the script at a prompt it cannot answer.
	wantReport(t, check(t, binary, checkModel, "-satisfy=false"), 2, "no check was named")
}

// TestCheckAgainstInstantiatedObject checks that -instantiate makes a following
// verdict be about that object, which is the only way a part's own constraint
// reaches concrete feature values.
func TestCheckAgainstInstantiatedObject(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, checkModel, "-instantiate", "Rover::pack", "-constraint", "Rover::pack::notOvercharged"),
		0, "✓ Created instance of Rover::pack",
		"✓ Constraint Rover::pack::notOvercharged passed (on Rover::pack ID: 1)")

	wantReport(t, check(t, binary, checkModel, "-instantiate", "Rover::nosuch", "-constraint", "Rover::MassBudget"),
		2, "unresolved reference: Rover::nosuch")
}

// TestCheckReportsVerdictsOnStdout checks that a verdict is a result on stdout,
// while a check that could not be made is reported on stderr like any other
// failure to run.
func TestCheckReportsVerdictsOnStdout(t *testing.T) {
	binary := buildCLI(t)

	failed := check(t, binary, checkModel, "-constraint", "Rover::TooHeavy")
	if !strings.Contains(failed.stdout, "✗ Constraint Rover::TooHeavy failed") {
		t.Errorf("a verdict was not reported on stdout:\n%s", failed.output())
	}

	unresolved := check(t, binary, checkModel, "-constraint", "nosuch")
	if !strings.Contains(unresolved.stderr, "unresolved reference: nosuch") {
		t.Errorf("an unmade check was not reported on stderr:\n%s", unresolved.output())
	}
}

// TestCheckWithoutModelFile checks that a check asked for with nothing to check
// exits 2 rather than waiting at a prompt a build step cannot answer.
func TestCheckWithoutModelFile(t *testing.T) {
	binary := buildCLI(t)
	cmd := exec.Command(binary, "-constraint", "Rover::MassBudget")
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("exit = %v, want status 2\n%s", err, out)
	}
	if !strings.Contains(string(out), "no model to check") {
		t.Errorf("report does not say a model is needed:\n%s", out)
	}
}

// TestCheckFlagsAreOptional checks that the modes that existed before checking
// was added are unchanged: a bare expression still evaluates and succeeds.
func TestCheckFlagsAreOptional(t *testing.T) {
	binary := buildCLI(t)
	out, err := exec.Command(binary, "-e", "5 + 3").CombinedOutput()
	if err != nil {
		t.Fatalf("-e: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "8") {
		t.Errorf("-e did not evaluate:\n%s", out)
	}
}

// TestCheckReportsDiagramLayoutFindings checks that a check reports the
// DiagramLayout findings the way a build step reads them: a misplaced
// annotation is a warning that leaves the model checkable, an ill-formed one an
// error that stops the check.
func TestCheckReportsDiagramLayoutFindings(t *testing.T) {
	binary := buildCLI(t)

	const warns = `package W {
    private import DiagramLayout::*;
    part def Pump {
        @Route { points = (0, 0, 10, 10); }
    }
}
`
	wantReport(t, check(t, binary, warns, "-validate"), 0,
		"model.sysml:4:9: warning: Route steers part def W::Pump, which no rendering draws as an edge",
		"no errors")

	const errs = `package E {
    private import DiagramLayout::*;
    part def Pump {
        @Canvas { width = 400; }
    }
    part def Loop {
        part pump : Pump;
        part tank : Pump;
        connection supply connect pump to tank {
            @Route { points = (0, 0, 10); }
        }
    }
}
`
	wantReport(t, check(t, binary, errs, "-validate"), 2,
		"model.sysml:4:9: error: Canvas annotates part def E::Pump, which is no view; a Canvas belongs in the body of the view it sizes",
		"model.sysml:10:38: error: connection E::Loop::supply: Route binds 3 values to points; waypoints are x, y pairs, so the count must be even")
}
