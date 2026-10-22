package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A Requirement Table the tool stored no rows for — a representation with a
// binaryObject naming no stream and no used elements — exposes the requirements
// written under its owner, nested and contained, and the note says the view is
// reconstructed from the owning scope.
func TestUnstoredRequirementTableListsTheOwnersRequirements(t *testing.T) {
	r := migrateFixtureFile(t, "unstored_requirement_tables")
	wantClean(t, "unstored_requirement_tables.sysml", r)
	notation := string(r.Notation)
	wantInOrder(t, "requirement table view", notation,
		"view 'Requirement Table' {",
		"expose 'Flow Requirement';",
		"expose Requirements::'Flow Requirement'::'Flow Margin';",
		"expose 'Seal Requirement';",
		"expose Requirements::Mass::'Mass Requirement';",
		"render Views::asElementTable;")
	view := notationSectionOf(notation, "view 'Requirement Table' {")
	if strings.Contains(view, "Pump") {
		t.Errorf("the requirement table exposes the block Pump:\n%s", view)
	}
	wantNote(t, r, "_diag_req_table", migrate.Approximated,
		"the tool stored no rows for this table; reconstructed from the owning scope: 4 requirements exposed")
}

// A Refine Requirement Matrix the tool stored no cells for exposes the
// connections written for the «Refine»s under its owner and the usages they
// join; a «Trace» under the same owner is no refinement and is not exposed.
func TestUnstoredRefineMatrixListsTheOwnersRefinements(t *testing.T) {
	r := migrateFixtureFile(t, "unstored_requirement_tables")
	notation := string(r.Notation)
	wantInOrder(t, "refine matrix view", notation,
		"view 'Refine Requirement Matrix' {",
		"expose 'Pump Fluid refines Flow Requirement';",
		"expose 'Pump Fluid';",
		"expose Requirements::'Flow Requirement';",
		"expose 'Seal Valve refines Seal Requirement';",
		"expose 'Seal Valve';",
		"expose Requirements::'Seal Requirement';",
		"render Views::asElementTable;")
	view := notationSectionOf(notation, "view 'Refine Requirement Matrix' {")
	if strings.Contains(view, "traces") || strings.Contains(view, "Mass Requirement") {
		t.Errorf("the refine matrix exposes the trace or its end:\n%s", view)
	}
	wantNote(t, r, "_diag_refine_matrix", migrate.Approximated,
		"the tool stored no cells for this matrix; reconstructed from the owning scope: 2 refinements exposed with the usages they join")
}

// An unstored Requirement Table owned by a package writing no requirement is
// still a view, exposing nothing, and the note says why.
func TestUnstoredRequirementTableOfNoRequirementsExposesNothing(t *testing.T) {
	r := migrateFixtureFile(t, "unstored_requirement_tables")
	wantInOrder(t, "empty requirement table view", string(r.Notation),
		"view 'Use Case Requirements' {",
		"render Views::asElementTable;")
	wantNote(t, r, "_diag_uc_reqs", migrate.Approximated,
		"the tool stored no rows for this table; reconstructed from the owning scope: no requirement is written under Use Cases, so the view exposes nothing")
}

// A Requirement Table that lists its rows as used elements is migrated from
// what it lists, as before: nothing is reconstructed for it.
func TestStoredRequirementTableIsNotReconstructed(t *testing.T) {
	r := migrateFixtureFile(t, "tables")
	for _, e := range entriesFor(r, "_diag_reqs") {
		if strings.Contains(e.Note, "reconstructed") {
			t.Errorf("the stored Requirement Table is reconstructed: %+v", e)
		}
	}
}

// notationSectionOf is the block opened by head up to its closing brace at
// the same indentation, or "" when the notation lacks it.
func notationSectionOf(notation, head string) string {
	i := strings.Index(notation, head)
	if i < 0 {
		return ""
	}
	indent := notation[strings.LastIndex(notation[:i], "\n")+1 : i]
	rest := notation[i:]
	if j := strings.Index(rest, "\n"+indent+"}"); j >= 0 {
		return rest[:j]
	}
	return rest
}
