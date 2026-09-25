package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// MagicDraw's «typeModifier» on a property or parameter writes its collection
// shape as a multiplicity: [] is [0..*], [n] is [n], both ordered nonunique.
func TestTypeModifierCollectionsBecomeMultiplicities(t *testing.T) {
	r := migrateFixtureFile(t, "type_modifiers")
	wantLine(t, r.Notation, "attribute samples : ScalarValues::Real[0..*] ordered nonunique;")
	wantLine(t, r.Notation, "attribute row : ScalarValues::Real[3] ordered nonunique;")
	wantLine(t, r.Notation, "attribute taps : ScalarValues::Real[4] ordered nonunique;")
	wantLine(t, r.Notation, "in values : ScalarValues::Real[0..*] ordered nonunique;")
	wantLine(t, r.Notation, "in window : ScalarValues::Real[4] ordered nonunique;")
	wantLine(t, r.Notation, "out result : ScalarValues::Real[0..*] ordered nonunique;")
	for _, id := range []string{"_samples", "_row", "_taps", "_par_values", "_par_window"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Mapped || es[0].Note != "" {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	// A modifier the declaration writes is not repeated as a comment: the []
	// comments left are history's and stray's, whose [] is refused.
	if n := strings.Count(string(r.Notation), "typeModifier = [] */"); n != 2 {
		t.Errorf("%d comments of typeModifier = [], want 2", n)
	}
	wantNoLine(t, r.Notation, "typeModifier = [3]")
	wantNoLine(t, r.Notation, "typeModifier = [4]")
}

// * and & make a part or item a reference usage; on a value they have no form.
func TestTypeModifierPointersBecomeReferences(t *testing.T) {
	r := migrateFixtureFile(t, "type_modifiers")
	wantLine(t, r.Notation, "ref part drive : Actuator;")
	wantLine(t, r.Notation, "ref part probe : Sensor;")
	for _, id := range []string{"_drive", "_probe"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	wantLine(t, r.Notation, "attribute level : ScalarValues::Real {")
	wantLine(t, r.Notation, "/* applied stereotype «typeModifier»: typeModifier = * */")
	wantNote(t, r, "_level", migrate.Approximated, "«typeModifier» * is kept as a comment: the type modifier * has no v2 form: only a part or item is held by reference, not an attribute")
	wantLine(t, r.Notation, "in handle : ScalarValues::Real {")
	wantNote(t, r, "_par_handle", migrate.Approximated, "«typeModifier» * is kept as a comment: the type modifier * has no v2 form: a parameter is not held by reference")
}

// A shape with two dimensions, one over a declared collection or a malformed
// multiplicity, or one the migrator does not read stays a comment, and the
// report says why.
func TestTypeModifierRefusalsStayComments(t *testing.T) {
	r := migrateFixtureFile(t, "type_modifiers")
	wantLine(t, r.Notation, "attribute grid : ScalarValues::Real {")
	wantLine(t, r.Notation, "/* applied stereotype «typeModifier»: typeModifier = [][] */")
	wantNote(t, r, "_grid", migrate.Approximated, "«typeModifier» [][] is kept as a comment: the type modifier [][] has no v2 form: a multiplicity has one dimension")
	wantLine(t, r.Notation, "/* applied stereotype «typeModifier»: typeModifier = [2*3] */")
	wantNote(t, r, "_rect", migrate.Approximated, "«typeModifier» [2*3] is kept as a comment: the type modifier [2*3] has no v2 form: a multiplicity has one dimension")
	wantLine(t, r.Notation, "attribute history : ScalarValues::Real[0..*] {")
	wantNote(t, r, "_history", migrate.Approximated, "«typeModifier» [] is kept as a comment: the type modifier [] has no v2 form: the declared multiplicity [0..*] is already a collection, and a collection of collections has no multiplicity")
	wantLine(t, r.Notation, "attribute stray : ScalarValues::Real {")
	wantNote(t, r, "_stray", migrate.Approximated, "multiplicity n..n is not a range of natural numbers and is not written; «typeModifier» [] is kept as a comment: the type modifier [] has no v2 form: the declared multiplicity n..n is not a range of natural numbers and is not written")
	wantNote(t, r, "_odd", migrate.Approximated, "«typeModifier» [2x] is kept as a comment: the type modifier [2x] is not one the migrator reads")
	wantNote(t, r, "_blank", migrate.Approximated, "an empty «typeModifier» is kept as a comment: it says nothing of the type")
	for _, id := range []string{"_grid", "_rect", "_history", "_stray", "_odd", "_blank", "_level", "_par_handle"} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != migrate.Approximated {
			t.Errorf("%s entries = %+v", id, es)
		}
		if strings.Contains(es[0].Note, "profile the document does not define") {
			t.Errorf("%s note repeats the comment verdict: %s", id, es[0].Note)
		}
	}
}

// A user stereotype named typeModifier outside the tool's profile is model
// content: a metadata usage, read for no shape.
func TestTypeModifierHomonymIsNotTheToolMarker(t *testing.T) {
	r := migrateFixtureFile(t, "type_modifiers")
	wantLine(t, r.Notation, "metadata def typeModifier {")
	wantLine(t, r.Notation, "attribute count : ScalarValues::Integer {")
	wantLine(t, r.Notation, "@'Shapes Profile'::typeModifier {")
	wantLine(t, r.Notation, "typeModifier = \"[]\";")
	wantNoLine(t, r.Notation, "attribute count : ScalarValues::Integer[0..*]")
	if es := entriesFor(r, "_count"); len(es) != 1 || es[0].Verdict != migrate.Mapped || es[0].Note != "" {
		t.Errorf("_count entries = %+v", es)
	}
}
