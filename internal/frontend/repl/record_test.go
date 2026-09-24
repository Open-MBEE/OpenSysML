package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
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

// A sweep that could not run records nothing and does not hold: its verdict is
// the sweep's own error, not a report over an empty table.
func TestRecordSweepErrorRecordsNothing(t *testing.T) {
	s := recordSession(t)
	before := s.text()
	v := s.RecordSweep("Demo::timed", []string{"missing=1..3"}, "", "%record Demo::timed")
	if v.Status == VerdictHolds {
		t.Errorf("a sweep that could not run holds:\n%s", strings.Join(v.Lines, "\n"))
	}
	for _, line := range v.Lines {
		if strings.Contains(line, "recorded") {
			t.Errorf("a failed sweep reported a record: %q", line)
		}
	}
	if s.text() != before {
		t.Error("a failed sweep record changed the buffer")
	}
}

// `into` splits only outside quoted names, string literals and parentheses.
func TestSplitRecordArgsLeavesIntoInNames(t *testing.T) {
	inv, into, err := splitRecordArgs("'step into space'")
	if err != nil || into != "" || inv.name != "'step into space'" {
		t.Errorf("'step into space': inv %+v, into %q, err %v", inv, into, err)
	}
	inv, into, err = splitRecordArgs(`c("go into it") into P`)
	if err != nil || into != "P" || inv.name != "c" || inv.argText != `"go into it"` {
		t.Errorf(`c("go into it") into P: inv %+v, into %q, err %v`, inv, into, err)
	}
}

// Each sweep row's values spell in its own context: instance ids restart per
// row, so one row's object means nothing read through another row's.
func TestRecordSweepSpellsObjectsInTheirOwnContext(t *testing.T) {
	s := NewSession()
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	if errs := errorDiagnostics(s.Submit(`package Demo {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	part def Probe { attribute t : Real = 1.0; }
	part a : Probe;
	part b : Probe;
	analysis def Pick {
		subject s : Probe;
		in n : Real;
		out chosen : Probe = 'if'(n < 2.0, a, b);
	}
	analysis pick : Pick { subject s = a; in n = 1.0; }
}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	v := s.RecordSweep("Demo::pick", []string{"n=1..2"}, "", "%record Demo::pick")
	if v.Status != VerdictHolds {
		t.Fatalf("record sweep: %v", v.Lines)
	}
	text := s.text()
	for _, want := range []string{"ref :>> chosen = Demo::a;", "ref :>> chosen = Demo::b;"} {
		if !strings.Contains(text, want) {
			t.Errorf("recorded model is missing %q:\n%s", want, text)
		}
	}
}

// Record numbers fill the gaps a package's earlier records leave rather than
// renumbering on from the first free prefix.
func TestRecordNumbersIntoTheGaps(t *testing.T) {
	s := recordSession(t)
	if errs := errorDiagnostics(s.Submit(`package Records { part timed_run2 : Demo::Probe; }`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	v := s.RecordSweep("Demo::timed", []string{"gain=1..2"}, "", "%record Demo::timed")
	if v.Status != VerdictHolds {
		t.Fatalf("record sweep: %v", v.Lines)
	}
	for _, want := range []string{"part timed_run1 :", "part timed_run3 :"} {
		if !strings.Contains(s.text(), want) {
			t.Errorf("recorded model is missing %q:\n%s", want, s.text())
		}
	}
}

// A Monte Carlo run whose declared output errors is not recorded; the report
// says which run and why.
func TestRecordMonteCarloSkipsOutputErrors(t *testing.T) {
	s := NewSession()
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	if errs := errorDiagnostics(s.Submit(`package MC {
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
		out Bad : Real = 1.0 / 0.0;
	}
}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	run(t, s, "%instantiate MC::probe")
	seed := uint64(7)
	v := s.RecordMonteCarlo("MC::Mc MC::probe", 1, &seed, "", "%record MC::Mc")
	out := strings.Join(v.Lines, "\n")
	for _, want := range []string{"run 1 not recorded:", "sample not recorded:", "nothing recorded: no run completed"} {
		if !strings.Contains(out, want) {
			t.Errorf("the skipped run is not reported (%q missing):\n%s", want, out)
		}
	}
	if strings.Contains(s.text(), "Mc_run") {
		t.Errorf("the model holds a record of an unreadable run:\n%s", s.text())
	}
}

// A Monte Carlo sample records each iteration under kind "runs" and its
// conclusion once under kind "sample", carrying the statistics and result
// the rows cannot.
func TestRecordMonteCarloRecordsTheSample(t *testing.T) {
	s := monteCarloSession(t)
	run(t, s, "%instantiate MC::probe")
	seed := uint64(7)
	v := s.RecordMonteCarlo("MC::Mc MC::probe", 3, &seed, "", "%record MC::Mc")
	out := strings.Join(v.Lines, "\n")
	if !strings.Contains(out, "recorded 4 runs") {
		t.Errorf("three iterations and the sample were not recorded:\n%s", out)
	}
	text := s.text()
	for _, want := range []string{
		`attribute :>> kind = "runs"`, `attribute :>> kind = "sample"`,
		"attribute :>> Mean", "attribute :>> Deviation", "attribute :>> N = 3",
		"attribute :>> iteration = 3",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("recorded model is missing %q:\n%s", want, text)
		}
	}

	path := filepath.Join(t.TempDir(), "model.sysml")
	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	fresh := recordSession(t)
	res := fresh.SubmitFiles([]SourceFile{{Name: path, Text: mustRead(t, path)}})
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("saved model has errors: %v", errs)
	}
}

// A sample no run of which completed records nothing and says why instead of
// reporting a generator error.
func TestRecordMonteCarloRecordsNothingWhenNoRunCompletes(t *testing.T) {
	s := monteCarloSession(t)
	run(t, s, "%instantiate MC::probe")
	before := s.text()
	seed := uint64(7)
	v := s.RecordMonteCarlo("MC::Crashing MC::probe", 2, &seed, "", "%record MC::Crashing")
	out := strings.Join(v.Lines, "\n")
	if !strings.Contains(out, "nothing recorded: no run completed") {
		t.Errorf("the empty sample is not explained:\n%s", out)
	}
	if strings.Contains(out, "record") && strings.Contains(out, "nothing to record") {
		t.Errorf("the generator's own error reported instead:\n%s", out)
	}
	if s.text() != before {
		t.Errorf("the model changed:\n%s", s.text())
	}
}

// A sweep whose every row failed records nothing and says why instead of
// reporting a generator error.
func TestRecordSweepRecordsNothingWhenEveryRowFails(t *testing.T) {
	s := recordSession(t)
	if errs := errorDiagnostics(s.Submit(`package Demo {
	analysis def Breakable { subject s : Probe; in n : Real; out x : Real = 3.0 / n; }
	analysis breakable : Breakable { subject s = probe; }
}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	before := s.text()
	v := s.RecordSweep("Demo::breakable", []string{"n=0..0"}, "", "%record Demo::breakable")
	out := strings.Join(v.Lines, "\n")
	if !strings.Contains(out, "nothing recorded: every row failed") {
		t.Errorf("the empty sweep is not explained:\n%s", out)
	}
	if strings.Contains(out, "nothing to record") {
		t.Errorf("the generator's own error reported instead:\n%s", out)
	}
	if s.text() != before {
		t.Errorf("the model changed:\n%s", s.text())
	}
}

// A case whose name needs quoting records under quoted names; the saved model
// numbers the next record on.
func TestRecordRunQuotesNames(t *testing.T) {
	s := NewSession()
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	const model = `package Demo {
		private import ScalarValues::*;
		analysis def Bound { out y : Real = 3.0; }
		analysis 'fuel budget' : Bound;
	}`
	if errs := errorDiagnostics(s.Submit(model).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%record Demo::'fuel budget'"),
		"recorded Records::fuel budget_run1 (Records::Fuel budgetRun)")
	for _, want := range []string{
		"part def 'Fuel budgetRun' :> AnalysisRecords::AnalysisRun",
		`attribute :>> caseName default = "Demo::fuel budget";`,
		"part 'fuel budget_run1' : 'Fuel budgetRun'",
	} {
		if !strings.Contains(s.text(), want) {
			t.Errorf("session text is missing %q:\n%s", want, s.text())
		}
	}

	path := filepath.Join(t.TempDir(), "model.sysml")
	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	fresh := recordSession(t)
	res := fresh.SubmitFiles([]SourceFile{{Name: path, Text: mustRead(t, path)}})
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("saved model has errors: %v", errs)
	}
	fresh.SubmitFiles([]SourceFile{{Name: path, Text: mustRead(t, path)}})
	wants(t, run(t, fresh, "%record Demo::'fuel budget'"),
		"recorded Records::fuel budget_run2")
}

// A sibling case of the same short name does not take over the first case's
// record definition: its own definition is named from its owner, in the model
// and again after a save.
func TestRecordRunPrefixesASiblingsDefinition(t *testing.T) {
	s := NewSession()
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	const model = `package Demo {
		private import ScalarValues::*;
		package A {
			analysis def Check { out x : Real = 1.0; }
			analysis check : Check;
		}
		package B {
			analysis def Check { out x : Real = 2.0; }
			analysis check : Check;
		}
	}`
	if errs := errorDiagnostics(s.Submit(model).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%record Demo::A::check"),
		"recorded Demo::Records::check_run1 (Demo::Records::CheckRun)")
	wants(t, run(t, s, "%record Demo::B::check"),
		"recorded Demo::Records::B_check_run1 (Demo::Records::B_checkRun)")

	path := filepath.Join(t.TempDir(), "model.sysml")
	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	fresh := recordSession(t)
	res := fresh.SubmitFiles([]SourceFile{{Name: path, Text: mustRead(t, path)}})
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("saved model has errors: %v", errs)
	}
	fresh.SubmitFiles([]SourceFile{{Name: path, Text: mustRead(t, path)}})
	wants(t, run(t, fresh, "%record Demo::B::check"),
		"recorded Demo::Records::B_check_run2")
}

// A second record settles a member the first left ScalarValue: the existing
// definition is reused and the new record carries the concrete value.
func TestRecordSettlesAnEarlierUnsetMember(t *testing.T) {
	s := recordSession(t)
	if errs := errorDiagnostics(s.Submit(`package Demo {
	analysis def Settle { subject s : Probe; in m : Real[0..1]; out x : Real[0..1] = m; }
	analysis settle : Settle { subject s = probe; }
}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%record Demo::settle"), "recorded Records::settle_run1")
	if !strings.Contains(s.text(), "attribute x : ScalarValues::ScalarValue") {
		t.Fatalf("the unset member is not ScalarValue:\n%s", s.text())
	}
	wants(t, run(t, s, "%record Demo::settle(2.0)"), "recorded Records::settle_run2")
	if !strings.Contains(s.text(), "attribute :>> x = 2.0;") {
		t.Errorf("the settled run did not record:\n%s", s.text())
	}
}

// A verification case's record carries the verdict its body decided and a
// VerdictRecord row for it, beside whatever the body's checks returned.
func TestRecordVerificationRun(t *testing.T) {
	s := NewSession()
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	res := s.Submit(`package Demo {
	private import ScalarValues::*;
	part def Engine { attribute thrust : Real; }
	part engine : Engine { attribute :>> thrust = 2800.0; }
	verification def Fire {
		subject e : Engine;
		VerificationCases::PassIf(e.thrust >= 3000.0)
	}
	verification fire : Fire { subject e = engine; }
}`)
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%record Demo::fire"), "recorded Records::fire_run1")
	text := s.text()
	for _, want := range []string{
		`attribute :>> verdict = "fail"`,
		`attribute :>> kind = "verification"`,
		`attribute :>> status = "fail"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("session text is missing %q:\n%s", want, text)
		}
	}
	// The record parses and validates clean on reload.
	path := filepath.Join(t.TempDir(), "demo.sysml")
	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	fresh := NewSession()
	if errs := errorDiagnostics(fresh.SubmitFiles([]SourceFile{{Name: path, Text: mustRead(t, path)}}).Diagnostics); len(errs) > 0 {
		t.Fatalf("saved model has errors: %v", errs)
	}
}

// A diagnostic the model already reports is a record's fault only when the
// record adds an occurrence of it.
func TestNewProblemsCountsOccurrences(t *testing.T) {
	err := func(msg string) diag.Diagnostic {
		return diag.Diagnostic{Severity: diag.SeverityError, Message: msg}
	}
	warn := diag.Diagnostic{Severity: diag.SeverityWarning, Message: "w"}
	before := map[string]int{"old": 2}
	got := newProblems(before, []diag.Diagnostic{err("old"), err("old"), warn})
	if len(got) != 0 {
		t.Errorf("reported %v for what the model already had", got)
	}
	if got := newProblems(map[string]int{"old": 1}, []diag.Diagnostic{err("old"), err("old")}); len(got) != 1 || got[0] != "old" {
		t.Errorf("a second occurrence is the record's: %v", got)
	}
	if got := newProblems(map[string]int{"old": 1}, []diag.Diagnostic{err("new")}); len(got) != 1 || got[0] != "new" {
		t.Errorf("a new message is the record's: %v", got)
	}
}

// An inout is one parameter: the record carries the value the run left in it
// as the member, and the value it was bound with as an <name>In companion.
func TestRecordInoutRun(t *testing.T) {
	s := NewSession()
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	res := s.Submit(`package Demo {
	private import ScalarValues::*;
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	part def Probe;
	part probe : Probe;
	analysis def Doubling {
		subject s : Probe;
		inout counter : Integer = 3;
		return doubled : Integer = counter * 2;
	}
	analysis tick : Doubling { subject s = probe; }
	calc def Counts :> DocumentQueries::Query {
		in root : Element;
		Project(source = WhereMetadata(
			source = Descendants(source = root, maxDepth = 10),
			'metadata' = "AnalysisRecords::RecordedRun"),
			properties = ("counter", "counterIn", "doubled"))
	}
}`)
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%record Demo::tick into Demo::Log"), "recorded Demo::Log::tick_run1")
	text := s.text()
	for _, want := range []string{
		"attribute counter : ScalarValues::Integer;",
		"attribute counterIn : ScalarValues::Integer;",
		"attribute :>> counter = 3;",
		"attribute :>> counterIn = 3;",
		"attribute :>> doubled = 6;",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("session text is missing %q:\n%s", want, text)
		}
	}
	wants(t, run(t, s, "%run-query Counts root=Demo"),
		"counter = 3", "counterIn = 3", "doubled = 6")
	path := filepath.Join(t.TempDir(), "demo.sysml")
	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	fresh := NewSession()
	if errs := errorDiagnostics(fresh.SubmitFiles([]SourceFile{{Name: path, Text: mustRead(t, path)}}).Diagnostics); len(errs) > 0 {
		t.Fatalf("saved model has errors: %v", errs)
	}
}

// Two files reopening one package: the record goes into the file that already
// holds the target package, not the first file opening the shared ancestor.
func TestRecordMergesIntoTheFileHoldingTheTargetPackage(t *testing.T) {
	s := NewSession()
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	res := s.SubmitFiles([]SourceFile{
		{Name: "one.sysml", Text: `package A {
	private import ScalarValues::*;
	package Cases { analysis def Bound { out y : Real = 1.0; } analysis check : Bound; }
}`},
		{Name: "two.sysml", Text: `package A { package Records { attribute keep : ScalarValues::Integer; } }`},
	})
	if errs := errorDiagnostics(res.Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%record A::Cases::check"), "recorded A::Records::check_run1")
	if n := strings.Count(s.text(), "package Records"); n != 1 {
		t.Fatalf("the record made a second A::Records:\n%s", s.text())
	}
	if errs := errorDiagnostics(s.diagnostics()); len(errs) > 0 {
		t.Fatalf("recording left errors: %v", errs)
	}
}
