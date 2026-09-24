package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// recordModel declares the cases the -record-run tests record: one case whose
// subject the model binds, and one without a subject.
const recordModel = `package Demo {
	private import ScalarValues::*;
	part def Probe {
		attribute t : Real = 3.0;
	}
	part probe : Probe;
	analysis def Check {
		subject s : Probe;
		in gain : Real;
		out x : Real = s.t + gain;
	}
	analysis timed : Check { subject s = probe; in gain = 2.0; }
}`

func writeRecordModel(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(recordModel), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRecordRunReportsWhatItRecorded runs a case and records it, and the run
// reports both the case's verdict and the element the record became.
func TestRecordRunReportsWhatItRecorded(t *testing.T) {
	binary := buildCLI(t)
	source := writeRecordModel(t)
	cmd := exec.Command(binary, source, "-record-run", "Demo::timed")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-record-run: %v\n%s", err, out)
	}
	for _, want := range []string{"x = 5.0", "recorded Records::timed_run1"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("-record-run output is missing %q:\n%s", want, out)
		}
	}
}

// TestRecordRunConvertWritesTheRecords converts the session text a -record-run
// produced: the file it writes carries the Records package, validates clean as
// a model of its own, and a second run records _run2 into it.
func TestRecordRunConvertWritesTheRecords(t *testing.T) {
	binary := buildCLI(t)
	source := writeRecordModel(t)
	dir := t.TempDir()
	first := filepath.Join(dir, "first.sysml")
	cmd := exec.Command(binary, source, "-record-run", "Demo::timed", "-convert", "sysml", "-o", first)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("record + convert: %v\n%s", err, out)
	}
	written, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"part timed_run1 : TimedRun", `caseName = "Demo::timed"`, "attribute :>> x = 5.0"} {
		if !strings.Contains(string(written), want) {
			t.Errorf("converted model is missing %q:\n%s", want, written)
		}
	}
	if out, err := exec.Command(binary, first, "-validate").CombinedOutput(); err != nil {
		t.Fatalf("converted model does not validate: %v\n%s", err, out)
	}
	second := filepath.Join(dir, "second.sysml")
	cmd = exec.Command(binary, first, "-record-run", "Demo::timed", "-convert", "sysml", "-o", second)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("re-record + convert: %v\n%s", err, out)
	}
	written, err = os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "part timed_run2 : TimedRun") {
		t.Errorf("converted model is missing the renumbered run:\n%s", written)
	}
}

// TestRecordRunSweepRecordsEveryRow sweeps one input of a case and records one
// run per value the range steps through.
func TestRecordRunSweepRecordsEveryRow(t *testing.T) {
	binary := buildCLI(t)
	source := writeRecordModel(t)
	out := filepath.Join(t.TempDir(), "swept.sysml")
	cmd := exec.Command(binary, source, "-record-run", "Demo::timed", "-sweep", "gain=1..3", "-convert", "sysml", "-o", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("record + sweep + convert: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"part timed_run1 :", "part timed_run2 :", "part timed_run3 :", `attribute :>> kind = "sweep"`, "attribute :>> iteration = 3"} {
		if !strings.Contains(string(written), want) {
			t.Errorf("converted sweep is missing %q:\n%s", want, written)
		}
	}
}

// TestRecordRunMonteCarloRecordsEveryRun samples a MonteCarlo case under -runs
// and records each run it made.
func TestRecordRunMonteCarloRecordsEveryRun(t *testing.T) {
	binary := buildCLI(t)
	model := `package MC {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	part def Probe {
		attribute t : Real;
		action settle { first start; then assign t := uniform(1.0, 5.0); then done; }
	}
	individual def probe :> Probe;
	analysis def Mc :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.settle;
		attribute :>> observed : Real = analysed.t;
		return Mean : Real = mean;
	}
}
`
	source := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(source, []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "runs.sysml")
	cmd := exec.Command(binary, source, "-instantiate", "MC::probe", "-record-run", "MC::Mc MC::probe", "-runs", "2", "-seed", "7", "-convert", "sysml", "-o", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("record + runs + convert: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"part Mc_run1 :", "part Mc_run2 :", `attribute :>> kind = "runs"`, "attribute :>> iteration = 2"} {
		if !strings.Contains(string(written), want) {
			t.Errorf("converted MonteCarlo records are missing %q:\n%s", want, written)
		}
	}
}

// TestRecordRunRenderDocumentSeesTheRecords renders a document over the model
// after a run was recorded, so the document's table lists the record it made.
func TestRecordRunRenderDocumentSeesTheRecords(t *testing.T) {
	binary := buildCLI(t)
	model := `package Demo {
	private import ScalarValues::*;
	private import DocumentQueries::*;
	part def Probe {
		attribute t : Real = 3.0;
	}
	part probe : Probe;
	analysis def Check {
		subject s : Probe;
		in gain : Real;
		out x : Real = s.t + gain;
	}
	analysis timed : Check { subject s = probe; in gain = 2.0; }
	calc def RecordedRuns :> DocumentQueries::Query {
		in names : String[1..*];
		Project(
			source = Named(qualifiedName = names),
			properties = ("name", "kind")
		)
	}
	part def Log :> DocumentQueries::Document {
		attribute redefines title = "Run Log";
		part runs : Table {
			calc rows : RecordedRuns { in names = "Records::timed_run1"; }
		}
	}
}
`
	source := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(source, []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "report.md")
	cmd := exec.Command(binary, source, "-record-run", "Demo::timed", "-render-document", "Demo::Log", "-o", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("record + render-document: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "timed\\_run1") && !strings.Contains(string(written), "timed_run1") {
		t.Errorf("rendered document does not list the record:\n%s", written)
	}
}

// TestRecordRunChecksStayExclusive keeps the guard that a decision and a render
// do not share a run: -render-document with -analysis is still refused.
func TestRecordRunChecksStayExclusive(t *testing.T) {
	binary := buildCLI(t)
	source := writeRecordModel(t)
	cmd := exec.Command(binary, source, "-analysis", "Demo::timed", "-render-document", "Demo::Doc", "-o", filepath.Join(t.TempDir(), "x.md"))
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("-analysis + -render-document succeeded:\n%s", out)
	} else if !strings.Contains(string(out), "-render-document writes a document") {
		t.Errorf("unexpected refusal:\n%s", out)
	}
}

// TestRecordIntoWithoutRecordRunRefused rejects -record-into on its own.
func TestRecordIntoWithoutRecordRunRefused(t *testing.T) {
	binary := buildCLI(t)
	source := writeRecordModel(t)
	cmd := exec.Command(binary, source, "-record-into", "Demo::Log", "-convert", "sysml")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("-record-into alone succeeded:\n%s", out)
	} else if !strings.Contains(string(out), "-record-into accompanies -record-run") {
		t.Errorf("unexpected refusal:\n%s", out)
	}
}
