package migrate_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestViewpointConcernMigration(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/viewpoint_concerns.xmi")
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.Migrate("viewpoint_concerns.xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	for _, line := range []string{
		"view def 'Review Viewpoint' {",
		"viewpoint 'review Viewpoint' {",
		"subject;",
		"require constraint {",
		"doc /* review purpose */",
		"rendering {",
		"doc /* language: English",
		" * presentation: dashboard",
		"frame 'concern 2';",
		"frame ViewsModel::'concern';",
		"doc /* legacy concern */",
		"concern 'concern' {",
		"concern 'concern 2' {",
		"view 'Generalized View' : 'Review Viewpoint';",
		"view 'Tagged View' : 'Safety Viewpoint';",
		"view 'Conforming View' : 'Review Viewpoint';",
		"view 'Combined View' : 'Review Viewpoint' {",
		"/* conforms to 'Safety Viewpoint' */",
		"individual view def review :> 'Review Viewpoint';",
		"doc /* Package concern */",
		"doc /* Viewpoint concern */",
		"doc /* Root concern */",
	} {
		wantLine(t, r.Notation, line)
	}
	rootConcern := strings.Index(string(r.Notation), "concern 'concern' {\n    doc /* Root concern */")
	viewsPackage := strings.Index(string(r.Notation), "package ViewsModel {")
	if rootConcern < 0 || viewsPackage < 0 || rootConcern > viewsPackage {
		t.Errorf("root-owned concern was not written before its nested package:\n%s", r.Notation)
	}
	rendering := string(r.Notation)
	renderAt := strings.Index(rendering, "rendering {")
	for _, action := range []string{"action : createView;", "action : makeView;", "action : 'Tag Method';"} {
		next := strings.Index(rendering[renderAt:], action)
		if next < 0 {
			t.Errorf("rendering lacks %q:\n%s", action, rendering)
			continue
		}
		renderAt += next + len(action)
	}
	if count := strings.Count(string(r.Notation), "stakeholder reviewer : Reviewer;"); count != 2 {
		t.Errorf("stakeholder parameters appear %d times, want the viewpoint and shared concern usage", count)
	}
	if strings.Contains(string(r.Notation), "concernList") {
		t.Fatalf("concernList should not be emitted as a stakeholder tag:\n%s", r.Notation)
	}
	if strings.Contains(string(r.Notation), "method:") {
		t.Fatalf("written method tags should be rendering actions, not documentation:\n%s", r.Notation)
	}
	if diagnostics := errors(t, "viewpoint_concerns.sysml", r.Notation); len(diagnostics) != 0 {
		t.Fatalf("migrated notation has %d errors: %v", len(diagnostics), diagnostics)
	}

	for _, id := range []string{"_cPkg", "_shared", "_cRoot"} {
		entries := entriesFor(r, id)
		if len(entries) != 1 || entries[0].Verdict != migrate.Mapped {
			t.Errorf("concern %s entries = %+v, want one mapped entry", id, entries)
		}
	}
	for _, id := range []string{"_cEnum", "_cVp"} {
		entries := entriesFor(r, id)
		if len(entries) != 1 || entries[0].Verdict != migrate.Approximated {
			t.Errorf("fallback concern %s entries = %+v, want one approximated entry", id, entries)
		}
	}
	if entries := entriesFor(r, "_cVp"); len(entries) == 1 &&
		!strings.Contains(entries[0].Note, "a v2 concern annotates nothing, so the comment's annotations are not carried") {
		t.Errorf("annotated concern note = %q, want annotation-loss explanation", entries[0].Note)
	}
	for _, id := range []string{"_g", "_conform"} {
		entries := entriesFor(r, id)
		if len(entries) != 1 || entries[0].Verdict != migrate.Mapped {
			t.Errorf("single-viewpoint conformance %s entries = %+v, want one mapped entry", id, entries)
		}
	}
	entries := entriesFor(r, "_bothView")
	if len(entries) != 1 || entries[0].Verdict != migrate.Approximated ||
		!strings.Contains(entries[0].Note, "v2 types a view by one view definition, so its conformance to Views::Safety Viewpoint is kept as a comment") {
		t.Errorf("multiple-viewpoint conformance entry = %+v, want the extra viewpoint approximated as a comment", entries)
	}

	idx := libs.NewModelIndex()
	idx.AddDocument("viewpoint_concerns.sysml", parser.New(source.New("viewpoint_concerns.sysml", r.Notation)).ParseFile())
	idx.ExpandWildcardImports()
	sem := semantics.NewModel(resolve.New(idx))
	for _, name := range []string{"Generalized View", "Tagged View", "Conforming View", "Combined View"} {
		view := symbolByName(t, idx, "ViewsModel::"+name)
		sats := sem.SatisfyMembersOf(view)
		if len(sats) != 1 {
			t.Errorf("%s inherits %d viewpoint satisfy members, want one", name, len(sats))
		}
	}
	viewpoint := symbolByName(t, idx, "ViewsModel::Review Viewpoint")
	var viewpointUsage *symbols.Symbol
	for _, member := range sem.MembersOf(viewpoint) {
		if usage, ok := member.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageViewpoint {
			viewpointUsage = member
			break
		}
	}
	if viewpointUsage == nil {
		t.Fatal("Review Viewpoint has no viewpoint usage")
	}
	if concerns := sem.FramedConcernsOf(viewpointUsage); len(concerns) != 6 {
		t.Errorf("viewpoint usage has %d framed concerns, want five listed comments and the deprecated concern tag", len(concerns))
	}
}

func symbolByName(t *testing.T, idx *symbols.Index, name string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(name)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", name, len(matches))
	}
	return matches[0]
}
