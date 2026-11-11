package repl

import (
	"strings"
	"testing"
)

const inlineProjection = `Project(WhereType(Descendants(Observatory::telescope), "PartUsage"), ("name", "mass"))`

func TestRunQueryInlineProjectsRows(t *testing.T) {
	s := docQuerySession(t)
	got := run(t, s, "%run-query inline "+inlineProjection)
	wants(t, got,
		"✓ Inline query returned 3 rows",
		"Columns: name, mass",
		"Row 1: Observatory::telescope::optics",
		`name = "optics"`,
		"mass = 8.5",
		"Row 2: Observatory::telescope::segmentControl",
		"Row 3: Observatory::telescope::mount",
	)
	// The library's names are in scope unqualified, and so are the model's own.
	wants(t, run(t, s, `%run-query inline WhereName(OwnedElements(Observatory::telescope), "startsWith", "m")`),
		"✓ Inline query returned 1 row", "Row 1: Observatory::telescope::mount")
	// A named query runs through the same prompt, and the inline definition is not among the session's names.
	wants(t, run(t, s, "%run-query HeavySubsystems root=telescope"), "✓ Query Observatory::HeavySubsystems returned 2 rows")
	if got := run(t, s, "%search inline"); strings.Contains(got, "<inline query>") {
		t.Errorf("the inline definition leaked into the session's index:\n%s", got)
	}
}

func TestRunInlineDocumentQueryVerdict(t *testing.T) {
	s := docQuerySession(t)
	for _, v := range []Verdict{
		s.RunDocumentQuery("inline " + inlineProjection),
		s.RunInlineDocumentQuery(inlineProjection),
	} {
		if !v.Holds() {
			t.Fatalf("verdict = %s: %v", v.Status, v.Lines)
		}
		if v.Subject != inlineProjection {
			t.Errorf("subject = %q, want the query expression", v.Subject)
		}
		values := map[string]string{}
		for _, nv := range v.Values {
			values[nv.Name] = nv.Value
		}
		if values["rows"] != "3" || values["columns"] != "name, mass" {
			t.Errorf("values = %v", v.Values)
		}
		if v.Table == nil || len(v.Table.Rows) != 3 || v.Table.Rows[2].Element != "Observatory::telescope::mount" {
			t.Errorf("table = %+v", v.Table)
		}
	}
}

func TestRunQueryInlineReportsFaultsInTheExpression(t *testing.T) {
	s := docQuerySession(t)
	// A parse error is positioned within the expression, under a caret, and is no model diagnostic.
	got := run(t, s, `%run-query inline Project(Named("Observatory") ("name"))`)
	wants(t, got, "error: inline query 1:", `Project(Named("Observatory") ("name"))`, "^")
	if strings.Contains(got, "declared in") || strings.Contains(got, "<inline query>") {
		t.Errorf("the fault is reported as the model's:\n%s", got)
	}
	// So is a planning error.
	wants(t, run(t, s, `%run-query inline Union(Named("Observatory"), Frobnicate(Named("Observatory")))`),
		"error: inline query 1:29: query", "unknown operation Frobnicate", "Frobnicate(", "^")
	// An expression declaring no parameter takes no binding, and nothing is a query.
	wants(t, run(t, s, "%run-query inline"), "error:", "inline")
	wants(t, run(t, s, "%run-query inline   "), "error:", "inline")
}

func TestRunQueryInlinePrintsTheOutcomeRecord(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, `%run-query inline Named(qualifiedName = "Observatory::telescop")`),
		"error: inline query 1:1: query", "names no element Observatory::telescop",
		"path error at Named(Observatory::telescop): expected exactly one element with that qualified name",
		"nearest: telescope")
	verdict := s.RunInlineDocumentQuery(`Named(qualifiedName = "Observatory::telescop")`)
	if verdict.Holds() || verdict.Outcome == nil || verdict.Outcome.Kind() != "pathError" {
		t.Fatalf("verdict = %s outcome %+v, want the path error", verdict.Status, verdict.Outcome)
	}
}

func TestRunQueryInlineReadsHeldObjects(t *testing.T) {
	s := objectQuerySession(t)
	wants(t, run(t, s, `%run-query inline Project(Objects(type = "Wheel"), ("pressure"))`), "✓ Inline query returned 0 rows")
	wants(t, run(t, s, "%instantiate Garage::car"), "Created instance")
	wants(t, run(t, s, "%invoke car.wheels[2] inflate delta=5"), "Invoked inflate on object #3")
	wants(t, run(t, s, `%run-query inline Project(Objects(type = "Wheel"), ("pressure"))`),
		"✓ Inline query returned 2 rows",
		"Row 1: Garage::car.wheels[1] (#2)", "pressure = 30",
		"Row 2: Garage::car.wheels[2] (#3)", "pressure = 35")
	// An element named in the expression is the element, as in a declared query's body.
	wants(t, run(t, s, `%run-query inline Project(OwnedElements(Garage::car), ("name", "pressure"))`),
		"✓ Inline query returned 0 rows")
	wants(t, run(t, s, `%run-query inline Project(OwnedElements(Garage::Car), ("name", "pressure"))`),
		"✓ Inline query returned 1 row", "Row 1: Garage::Car::wheels")
}

func TestRunQueryInlineKeywordYieldsToAQueryNamedInline(t *testing.T) {
	s := docQuerySession(t)
	if res := s.Submit(`package Shadow {
	private import DocumentQueries::*;
	private import KerML::Root::*;
	calc def inline :> Query {
		in root : Element;
		WhereType(source = Descendants(source = root), type = "PartUsage")
	}
}`); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	// Bindings name the query inline; a query expression never begins with one.
	wants(t, run(t, s, "%run-query inline root=Observatory::telescope"), "✓ Query Shadow::inline returned 3 rows")
	wants(t, run(t, s, "%run-query Shadow::inline root=Observatory::telescope"), "✓ Query Shadow::inline returned 3 rows")
	wants(t, run(t, s, "%run-query inline "+inlineProjection), "✓ Inline query returned 3 rows")
}
