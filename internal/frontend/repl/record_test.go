package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recordModel declares cases %record runs: one binding its own subject, one
// without one, and an action a debugging session drives.
const recordModel = `package Demo {
	private import ScalarValues::*;
	part def Probe {
		attribute t : Real = 3.0;
		action tick { first start; then assign t := t + 1.0; then done; }
	}
	part probe : Probe;
	analysis def Check {
		subject s : Probe;
		in gain : Real;
		out x : Real = s.t + gain;
	}
	analysis def Bound {
		out y : Real = 1.0 + 2.0;
	}
	analysis timed : Check { subject s = probe; in gain = 2.0; }
	calc def Sum { in a : Real; return : Real = a; }
}`

func recordSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	if errs := errorDiagnostics(s.Submit(recordModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	return s
}

// A recorded run is written into the model beside the case's package and
// saved with it; re-recording in the saved model numbers the record on.
func TestRecordRunSavesAndRenumbers(t *testing.T) {
	s := recordSession(t)
	wants(t, run(t, s, "%record Demo::timed"),
		"✓ Demo::timed", "x = 5.0", "recorded Records::timed_run1 (Records::TimedRun)")
	text := s.text()
	for _, want := range []string{
		"package Records", "part def TimedRun :> AnalysisRecords::AnalysisRun",
		"part timed_run1 : TimedRun", "@AnalysisRecords::RecordedRun",
		"caseName = \"Demo::timed\"", "attribute :>> gain = 2.0",
		"attribute :>> x = 5.0", "runAt = \"2026-01-01T00:00:00Z\"",
		"ref :>> 'subject' = Demo::probe",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("session text is missing %q:\n%s", want, text)
		}
	}

	path := filepath.Join(t.TempDir(), "model.sysml")
	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), "part timed_run1 : TimedRun") {
		t.Fatalf("saved file lacks the record: %v", err)
	}

	fresh := recordSession(t)
	res := fresh.SubmitFiles([]SourceFile{{Name: path, Text: mustRead(t, path)}})
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("saved model has errors: %v", errs)
	}
	// A second record merges into the Records package the file declared.
	fresh.SubmitFiles([]SourceFile{{Name: path, Text: mustRead(t, path)}})
	wants(t, run(t, fresh, "%record Demo::timed"),
		"recorded Records::timed_run2")
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// `into` records into the package named, nesting it under a package already
// in the buffer.
func TestRecordIntoNestedPackage(t *testing.T) {
	s := recordSession(t)
	wants(t, run(t, s, "%record Demo::Bound into Demo::Log"),
		"recorded Demo::Log::Bound_run1 (Demo::Log::BoundRun)")
	if !strings.Contains(s.text(), "package Log") {
		t.Errorf("the records package is missing:\n%s", s.text())
	}
}

// A record that cannot be made leaves the buffer exactly as it was.
func TestRecordLeavesModelOnFailure(t *testing.T) {
	for _, line := range []string{
		"%record Demo::Gone",                   // no such case
		"%record Demo::Sum",                    // a calc is not a case
		"%record Demo::Check(1.0)",             // the run binds no subject
		"%record Demo::Bound into Demo::Probe", // into names a part def
	} {
		s := recordSession(t)
		before := s.text()
		out := run(t, s, line)
		if got := s.text(); got != before {
			t.Errorf("%s changed the model:\nbefore:\n%s\nafter:\n%s\nout:%s", line, before, got, out)
		}
	}
}

// Records a document query finds by their RecordedRun metadata and reads the
// objective they carry.
func TestRecordRunQueryable(t *testing.T) {
	s := recordSession(t)
	if errs := errorDiagnostics(s.Submit(`package Demo {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	calc def RecordedRuns :> Query {
		in root : Element;
		Project(source = WhereMetadata(
			source = Descendants(source = root, maxDepth = 10),
			'metadata' = "AnalysisRecords::RecordedRun"),
			properties = ("name", "caseName", "kind"))
	}
}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("query has errors: %v", errs)
	}
	run(t, s, "%record Demo::Bound into Demo::Log")
	out := run(t, s, "%run-query RecordedRuns root=Demo")
	wants(t, out, "Bound_run1", "Demo::Bound", "run")
}

// A document query filters records by the features they carry and projects the
// values the run bound, so the run is readable as model data.
func TestRecordRunQueryableByFeature(t *testing.T) {
	s := recordSession(t)
	if errs := errorDiagnostics(s.Submit(`package Demo {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	calc def TimedRuns :> Query {
		in root : Element;
		Project(source = WhereFeature(
			source = WhereMetadata(
				source = Descendants(source = root, maxDepth = 10),
				'metadata' = "AnalysisRecords::RecordedRun"),
			'feature' = "caseName",
			operator = "=",
			value = "Demo::timed"),
			properties = ("name", "gain", "x"))
	}
}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("query has errors: %v", errs)
	}
	run(t, s, "%record Demo::timed into Demo::Log")
	out := run(t, s, "%run-query TimedRuns root=Demo")
	wants(t, out, "timed_run1", "gain = 2.0", "x = 5.0")
}

// The records land beside the package enclosing the case's: a case in a
// nested package's Records is made under its parent, and one nested in a
// part lands under a top-level Records.
func TestRecordPackageFollowsTheCasesPackage(t *testing.T) {
	s := recordSession(t)
	if errs := errorDiagnostics(s.Submit(`package A {
	package Descent {
		analysis def C { out k : ScalarValues::Real = 1.0; }
		analysis c : C;
	}
}
package P {
	part def H {
		analysis def Inner { out k : ScalarValues::Real = 1.0; }
		analysis inner : Inner;
	}
}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%record A::Descent::c"), "recorded A::Records::c_run1")
	wants(t, run(t, s, "%record P::H::inner"), "recorded Records::inner_run1")
}

// A record submission that would drop a declaration restores the buffer, and
// the objects and debugging sessions it holds, as they were.
func TestRecordFailureKeepsObjectsAndDebugSession(t *testing.T) {
	s := recordSession(t)
	run(t, s, "%instantiate Demo::probe")
	wants(t, run(t, s, "%action Demo::Probe::tick #1"), "Started action executor")
	// A differently-headed Records package cannot be merged into, so the
	// record's package would supersede it and drop its member.
	if errs := errorDiagnostics(s.Submit(`package 'Records' {
	part keep : Demo::Probe;
}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	before := s.text()
	out := run(t, s, "%record Demo::timed")
	wants(t, out, "recording the run failed", "drop")
	if s.text() != before {
		t.Error("a failed record changed the buffer:\n" + s.text())
	}
	if s.actionExec == nil {
		t.Fatal("the debugging session was ended by the failed record")
	}
	wants(t, run(t, s, "%step"), "Step")
	if len(s.instances) == 0 {
		t.Error("the instantiated object was lost by the failed record")
	}
}

// Recording a run does not end a debugging session over an unrelated
// declaration.
func TestRecordKeepsDebugSession(t *testing.T) {
	s := recordSession(t)
	run(t, s, "%instantiate Demo::probe")
	wants(t, run(t, s, "%action Demo::Probe::tick #1"), "Started action executor")
	run(t, s, "%record Demo::timed")
	if s.actionExec == nil {
		t.Fatal("the debugging session was ended by a record submission")
	}
	wants(t, run(t, s, "%step"), "Step")
}
