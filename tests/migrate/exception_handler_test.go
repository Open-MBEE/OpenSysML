package migrate_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// testdata/xmi/exception_handlers.xmi: the call 'capture' owns three UML
// exception handlers whose bodies no edge leads to: two structured nodes and a
// raise exception action; an object flow feeds one body's pin and an observation
// times another. SysML v2 has no exception handler, so each handler is diagnosed
// and its body is written as a reference action usage: nothing starts it and the
// activity does not perform it, where it used to start with the activity.
func TestExceptionHandlerBodyIsNeverStarted(t *testing.T) {
	r := migrateStream(t, "exception_handlers")
	checkGolden(t, "testdata/xmi/exception_handlers.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/exception_handlers.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "exception_handlers.sysml", r.Notation) {
		t.Errorf("%v", d)
	}

	notation := string(r.Notation)
	wantLine(t, r.Notation, "/* not migrated: ExceptionHandler (_eh_h1) — SysML v2 has no exception handler: UML runs the handler body 'Image Capture Fails' only when its protected node 'capture' raises a matching exception of type 'CaptureFault'; the body is written as a reference action usage the activity does not perform */")
	wantLine(t, r.Notation, "/* not migrated: ExceptionHandler (_eh_h2) — SysML v2 has no exception handler: UML runs the handler body 'Position Not Correct' only when its protected node 'capture' raises a matching exception; no exception type: catches nothing in UML */")
	wantLine(t, r.Notation, "ref action 'Image Capture Fails' {")
	wantLine(t, r.Notation, "ref action 'Position Not Correct' {")
	wantLine(t, r.Notation, "ref action rethrow;")
	for _, body := range []string{"'Image Capture Fails'", "'Position Not Correct'", "rethrow"} {
		for _, start := range []string{"first start then " + body, "then " + body + ";", "then " + body + "\n", "succession 'start to " + strings.Trim(body, "'") + "'", "succession flow", "stamp"} {
			if strings.Contains(notation, start) {
				t.Errorf("%q starts the handler body %s:\n%s", start, body, notation)
			}
		}
	}
	// The raise in a handler body gets no terminate of its own: only the finals'.
	if n := strings.Count(notation, " terminate;"); n != 2 || strings.Contains(notation, "action terminate ") {
		t.Errorf("%d terminate actions, want the two activity finals' alone:\n%s", n, notation)
	}
	// The object flow into the body's pin is not written, and forks nothing off store.
	wantNote(t, r, "_eh_e6", migrate.Unmapped, "the edge leads to 'Position Not Correct', the handler body of (_eh_h2)")
	if strings.Contains(notation, "fork") {
		t.Errorf("a fork is written for the unwritable edge:\n%s", notation)
	}
	// The bodies' own steps and the main flow survive, nested as before.
	wantLine(t, r.Notation, "succession 'retry to log failure' first retry then 'log failure';")
	wantLine(t, r.Notation, "succession 'adjust to recapture' first adjust then recapture;")
	wantLine(t, r.Notation, "succession 'start to capture' first start then capture;")

	wantNote(t, r, "_eh_h1", migrate.Unmapped, "of type 'CaptureFault'")
	wantNote(t, r, "_eh_h2", migrate.Unmapped, "no exception type: catches nothing in UML")
	wantNote(t, r, "_eh_faultIn", migrate.Unmapped, "the exception input of (_eh_h1)")
	wantNote(t, r, "_eh_fails", migrate.Approximated, "the handler body of (_eh_h1): written as a reference action usage, which the activity does not perform")
	wantNote(t, r, "_eh_pos", migrate.Approximated, "the handler body of (_eh_h2)")
	wantNote(t, r, "_eh_h3", migrate.Unmapped, "the handler body 'rethrow'")
	wantNote(t, r, "_eh_rethrow", migrate.Approximated, "raising ends nothing")
	wantNote(t, r, "_eh_obs", migrate.Unmapped, "is the handler body of (_eh_h1), which the activity does not perform")
	for _, id := range []string{"_eh_fails", "_eh_pos"} {
		for _, e := range entriesFor(r, id) {
			if strings.Contains(e.Note, "starts with the activity") {
				t.Errorf("%s is reported as starting with the activity: %q", id, e.Note)
			}
		}
	}
	if r.Report.Layout == nil {
		t.Fatal("no layout summary")
	}
	found := false
	for _, k := range r.Report.Layout.RoutesByKind {
		if k.Kind == "ExceptionHandler" {
			found = true
			if k.Reason != "no v2 member" || k.Count != 3 {
				t.Errorf("ExceptionHandler routes = %+v, want 3 no v2 member", k)
			}
		}
	}
	if !found {
		t.Errorf("no ExceptionHandler route kind in %+v", r.Report.Layout.RoutesByKind)
	}
	if !strings.Contains(r.Report.Summary(), "unmapped") {
		t.Errorf("summary %q counts no unmapped element", r.Report.Summary())
	}
}

// Executing the migrated Inspect performs the main flow and never a handler body.
func TestExceptionHandlerBodyIsNeverPerformed(t *testing.T) {
	r := migrateStream(t, "exception_handlers")
	p := parser.New(source.New("exception_handlers.sysml", r.Notation))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse the migrated notation: %v", p.Diagnostics[0])
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("exception_handlers.sysml", root)
	idx.ExpandWildcardImports()
	res := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(passes.NewTypedModel(res), res), 1_000_000)
	tr := runtime.NewTraceRecorder()
	ctx.SetTrace(tr)
	syms := idx.LookupQualified("Model::Imaging::Camera::Inspect")
	if len(syms) != 1 {
		t.Fatalf("Inspect names %d symbols, want one", len(syms))
	}
	if _, err := ctx.ExecuteAction(syms[0]); err != nil {
		t.Fatalf("execute Inspect: %v", err)
	}
	trace := tr.String()
	for _, step := range []string{"capture", "store"} {
		if !strings.Contains(trace, step) {
			t.Errorf("the trace does not perform %s:\n%s", step, trace)
		}
	}
	for _, step := range []string{"Image Capture Fails", "retry", "log failure", "Position Not Correct", "adjust", "recapture", "rethrow"} {
		if strings.Contains(trace, step) {
			t.Errorf("the trace performs the handler body step %s:\n%s", step, trace)
		}
	}
}
