package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// An «AdjunctProperty» and its principal are one v2 usage, the principal's, for each kind of principal
// SysML v1 admits; an association end or allocation naming the property names that usage.
func TestAdjunctPropertyIsTheUsageOfItsPrincipal(t *testing.T) {
	r := migrateFixtureFile(t, "adjunct_properties")
	for _, line := range []string{
		"connection feed connect pump to tank;",
		"state Cycling : Cycle {",
		"occurrence priming : Prime;",
		"in level : ScalarValues::Real[1];",
		"private attribute count : ScalarValues::Integer;",
		"action call : Fill;",
		"action empty : Drain;",
		"end call : Fill crosses run.call;",
		"allocate source.empty to target;",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"feed 2", "Cycling 2", "priming 2", "level 2", "count 2", "empty 2", "ref action empty", "ref action call"} {
		wantNoLine(t, r.Notation, line)
	}
	const one = "are one v2 usage, written for the principal; what referred to the property refers to that usage"
	for id, target := range map[string]string{
		"_adjFeed":      "Model::Plant::feed",
		"_adjCycle":     "Model::Plant::Modes::Cycling",
		"_adjPriming":   "Model::Plant::Startup::priming",
		"_adjCall":      "Model::Run::call",
		"_adjNamed":     "Model::Run::empty",
		"_adjParam":     "Model::Run::level",
		"_adjParamNode": "Model::Run::level",
		"_adjVar":       "Model::Run::count",
	} {
		wantNote(t, r, id, migrate.Mapped, one)
		if es := entriesFor(r, id); len(es) == 1 && es[0].Target != target {
			t.Errorf("%s is written as %q, want %q", id, es[0].Target, target)
		}
	}
	wantNote(t, r, "_adjNamedDoc", migrate.Unmapped, "owned by an «AdjunctProperty» that is written as the usage of its principal, which has no place for a Comment")
	if es := entriesFor(r, "_adjNamedLo"); len(es) != 0 {
		t.Errorf("the multiplicity of a collapsed property is reported: %+v", es)
	}
	if n := strings.Count(string(r.Notation), `stereotype = "AdjunctProperty"`); n != 5 {
		t.Errorf("%d «AdjunctProperty» applications are kept as metadata, want the 5 of the properties written apart", n)
	}
	wantClean(t, "adjunct_properties.sysml", r)
}

// An adjunct property the collapse cannot cover is written as it would be without the stereotype,
// and its entry says which v1 constraint on the principal failed.
func TestAdjunctPropertyWrittenApartSaysWhy(t *testing.T) {
	r := migrateFixtureFile(t, "adjunct_properties")
	const apart = "«AdjunctProperty» is not collapsed into its principal: "
	for id, why := range map[string]string{
		"_adjGone":   "its principal is not in the document",
		"_adjKind":   "its principal 'note' is a OpaqueAction, which SysML v1 admits as no principal",
		"_adjOwner":  "its principal 'empty' is owned by Run, neither its owner nor a generalization of it",
		"_adjType":   "its type is neither that of its principal 'refill' nor a generalization of it",
		"_adjBuffer": "its principal 'queue' is not written as a v2 usage",
	} {
		wantNote(t, r, id, migrate.Approximated, apart+why+"; the property is written as a usage of its own")
		wantNote(t, r, id, migrate.Approximated, "«AdjunctProperty» from http://www.omg.org/spec/SysML/20181001/SysML is a SysML stereotype the migration gives no v2 form; the application is kept as metadata")
	}
	wantNote(t, r, "_adjStray", migrate.Unmapped, "no lifeline stands for; "+apart+"its principal is not in the document")
	for _, line := range []string{
		"ref action lost : Fill {",
		"ref action note : Fill {",
		"ref action borrowed : Drain {",
		"ref action refill : Drain {",
		"action 'refill 2' : Fill;",
		"ref part queue : Tank[0..*] {",
	} {
		wantLine(t, r.Notation, line)
	}
}

// A diagram that draws an adjunct property exposes and lays out the usage of its principal,
// and the association whose end the property is routes to it.
func TestAdjunctPropertyLayoutIsAboutThePrincipalUsage(t *testing.T) {
	r := migrateStream(t, "adjunct_layout")
	for _, line := range []string{
		"action call : Fill;",
		"action empty : Drain;",
		"end call : Fill crosses run.call;",
		"expose Behavior::Run::empty;",
		"metadata DiagramLayout::Layout about Behavior::Run::empty { x = 55; y = 60; width = 90; height = 30; }",
		"metadata DiagramLayout::Route about FillToRun { points = (100, 200, 100, 110); }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "ref action")
	if l := r.Report.Layout; l == nil || l.PlacementsUnexposed != 0 || l.RoutesWritten != 1 {
		t.Errorf("layout summary: %+v", l)
	}
	wantClean(t, "adjunct_layout.sysml", r)
}

// A principal nested in a structured node, one that is a private variable, and a property carrying a
// second stereotype: the association end is written so it resolves, and the report says what gave way.
func TestAdjunctPropertyOfANestedOrHiddenPrincipal(t *testing.T) {
	r := migrateFixtureFile(t, "adjunct_nested")
	for _, line := range []string{
		"ref item held : Tank;",
		"end held : Tank crosses batch.held;",
		"end rinse : Rinse;",
		"ref item spare : Tank;",
		"end spare : Tank crosses rebatch.spare;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "private ref item")
	wantNote(t, r, "_spare", migrate.Approximated, "the variable is not written private: the connection def of the association 'RebatchSpare' in Wash crosses it")
	wantNote(t, r, "_held", migrate.Approximated, "the variable is not written private: the connection def of the association 'BatchHeld' in Wash crosses it")
	wantNote(t, r, "_batchRinse", migrate.Approximated, "end rinse crosses no property: the end property is the usage of its «AdjunctProperty» principal, which is nested in 'stage' of Batch")
	wantNote(t, r, "_adjRinse", migrate.Approximated, "«moe» applied to the property is not written")
	for _, d := range errorsMode(t, "adjunct_nested.sysml", r.Notation, diag.ConformanceStrict) {
		t.Errorf("%v", d)
	}
}
