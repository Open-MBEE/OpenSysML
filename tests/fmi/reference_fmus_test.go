// Package fmi_test gates the FMI integration end to end: the Modelica
// Reference-FMUs are converted to calc defs and simulated through the
// tool:fmi engine and the FMPy reference runner (client/python). The FMUs are
// not vendored — ./scripts/download-reference-fmus.sh fetches them — so the
// gate skips when they or fmpy are absent and fails when
// OPENSYSML_REQUIRE_REFERENCE_FMUS=1.
package fmi_test

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

const (
	fmusDir    = "../../examples/reference-fmus"
	requireEnv = "OPENSYSML_REQUIRE_REFERENCE_FMUS"
)

var (
	buildOnce sync.Once
	buildPath string
	buildErr  error
)

// buildCLI builds the sysml binary once per test binary, so the gate exercises
// the real command line.
func buildCLI(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "sysml-build")
		if err != nil {
			buildErr = err
			return
		}
		buildPath = filepath.Join(dir, "sysml")
		build := exec.Command("go", gobuild.Args(buildPath)...)
		build.Dir = filepath.Join("..", "..", "cmd", "sysml")
		if out, err := build.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building sysml: %v", buildErr)
	}
	return buildPath
}

// fmuPresent reports whether the Reference-FMUs are downloaded; the gate fails
// under the require variable, skips otherwise.
func fmuPresent(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(fmusDir, "2.0", "BouncingBall.fmu")); err == nil {
		if _, err := os.Stat(filepath.Join(fmusDir, "3.0", "BouncingBall.fmu")); err == nil {
			return
		}
	}
	if os.Getenv(requireEnv) == "1" {
		t.Fatalf("the Reference-FMUs are required but absent; fetch them with ./scripts/download-reference-fmus.sh")
	}
	t.Skip("Reference-FMUs not downloaded (run ./scripts/download-reference-fmus.sh)")
}

// fmpyPresent reports whether python3 can import fmpy, the library the
// reference runner simulates with; same fail-or-skip rule.
func fmpyPresent(t *testing.T) {
	t.Helper()
	if exec.Command("python3", "-c", "import fmpy").Run() == nil {
		return
	}
	if os.Getenv(requireEnv) == "1" {
		t.Fatalf("fmpy is required but python3 cannot import it; pip install opensysml[fmi]")
	}
	t.Skip("fmpy not installed (pip install opensysml[fmi])")
}

// runnerScript writes a shim that runs the reference runner module out of the
// client/python source tree; the minimal tool environment sees only the export
// the script makes itself.
func runnerScript(t *testing.T) string {
	t.Helper()
	client, err := filepath.Abs(filepath.Join("..", "..", "client", "python"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fmi-runner")
	script := "#!/bin/sh\nexport PYTHONPATH=" + strconv.Quote(client) + "\nexec python3 -m opensysml.fmi_runner\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// run drives the CLI to completion, returning its combined output.
func run(t *testing.T, binary string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	out, err := cmd.CombinedOutput()
	status := 0
	if exit, ok := err.(*exec.ExitError); ok {
		status = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("sysml: %v\n%s", err, out)
	}
	return string(out), status
}

// outputValue parses the `name = <value>` line a calc report prints for one of
// its outputs; name "" is the invocation's own `= <value>` return line.
func outputValue(t *testing.T, report, name string) float64 {
	t.Helper()
	want := name + " = "
	if name == "" {
		want = "= "
	}
	for _, line := range strings.Split(report, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, want) {
			f, err := strconv.ParseFloat(strings.TrimPrefix(line, want), 64)
			if err == nil {
				return f
			}
		}
	}
	t.Fatalf("report has no %q line:\n%s", want, report)
	return 0
}

// convert imports the FMU as SysML notation beside the test's model file.
func convert(t *testing.T, binary, fmu, dir string) string {
	t.Helper()
	out := filepath.Join(dir, filepath.Base(fmu)+".sysml")
	report, status := run(t, binary, "-convert", "sysml", "-o", out, fmu)
	if status != 0 {
		t.Fatalf("convert %s: status %d\n%s", fmu, status, report)
	}
	return out
}

// runUsage is a calc usage of the imported def, which reports every output.
const runUsage = `package Run {
	private import BouncingBall::*;
	calc bb : BouncingBall;
}
`

// TestReferenceFMUs converts each BouncingBall.fmu and evaluates the imported
// calc through the tool:fmi engine and the FMPy runner: a simulation to
// stopTime keeps the ball in [0, 1], a stopTime of 0 returns its initial
// height, and the calc usage reports a finite velocity beside the height.
func TestReferenceFMUs(t *testing.T) {
	fmuPresent(t)
	fmpyPresent(t)
	binary := buildCLI(t)
	t.Setenv("OPENSYSML_FMI_RUNNER", runnerScript(t))

	for _, fmiVersion := range []string{"2.0", "3.0"} {
		t.Run(fmiVersion, func(t *testing.T) {
			dir := t.TempDir()
			fmu, err := filepath.Abs(filepath.Join(fmusDir, fmiVersion, "BouncingBall.fmu"))
			if err != nil {
				t.Fatal(err)
			}
			model := convert(t, binary, fmu, dir)
			usage := filepath.Join(dir, "run.sysml")
			if err := os.WriteFile(usage, []byte(runUsage), 0o600); err != nil {
				t.Fatal(err)
			}

			report, status := run(t, binary, model, usage, "-calc", "Run::bb")
			if status != 0 {
				t.Fatalf("-calc Run::bb: status %d\n%s", status, report)
			}
			if h := outputValue(t, report, "h"); h < 0 || h > 1 {
				t.Errorf("h = %v at stopTime, want in [0, 1]\n%s", h, report)
			}
			if v := outputValue(t, report, "v"); math.IsNaN(v) || math.IsInf(v, 0) {
				t.Errorf("v = %v at stopTime, want finite\n%s", v, report)
			}

			// A stopTime of zero answers the start values: h is the initial height.
			report, status = run(t, binary, model, "-calc",
				"BouncingBall::BouncingBall(-9.81, 0.7, 0.0, 0.0, 0.01)")
			if status != 0 {
				t.Fatalf("-calc with stopTime 0: status %d\n%s", status, report)
			}
			if h := outputValue(t, report, ""); h != 1.0 {
				t.Errorf("h = %v at stopTime 0, want the initial height 1.0\n%s", h, report)
			}
		})
	}
}
