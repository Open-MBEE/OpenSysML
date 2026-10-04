package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// testdata/xmi/variables.xmi, activity Variables: variables declared as private
// features and the variable actions written as assignments to them.
func TestVariablesFixture(t *testing.T) {
	r := migrateFixtureFile(t, "variables")
	wantClean(t, "variables.xmi", r)

	// A data-typed variable is an attribute, any other classifier a ref item,
	// an untyped one a bare ref.
	wantLine(t, r.Notation, "private attribute count2 : ScalarValues::Integer[0..*] ordered;")
	wantLine(t, r.Notation, "private ref item ball2 : Ball;")
	wantLine(t, r.Notation, "private ref any2;")
	wantLine(t, r.Notation, "private attribute bag2 : ScalarValues::Integer[0..*] nonunique;")
	wantNote(t, r, "_vCount", migrate.Mapped, "")
	wantNote(t, r, "_vBall", migrate.Mapped, "")
	wantNote(t, r, "_vAny", migrate.Mapped, "")
	wantNote(t, r, "_vBag", migrate.Mapped, "")

	// A structured node's variable is declared inside its block.
	wantLine(t, r.Notation, "private attribute inner2 : ScalarValues::Integer;")
	wantLine(t, r.Notation, "out result : ScalarValues::Integer[1] = inner2;")
	wantNote(t, r, "_vInner", migrate.Mapped, "")
	wantNote(t, r, "_nestRead", migrate.Mapped, "")

	// Read binds its result pin to the variable; clear empties it.
	wantLine(t, r.Notation, "out count : ScalarValues::Integer[0..*] = count2;")
	wantNote(t, r, "_read", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign count2 := null;")
	wantNote(t, r, "_clear", migrate.Mapped, "")

	// Add: re-adding to a unique variable removes it first; insertAt inserts
	// at the position; isReplaceAll assigns the value.
	wantLine(t, r.Notation, "assign count2 := SequenceFunctions::including(SequenceFunctions::excluding(count2, value), value);")
	wantNote(t, r, "_add", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign count2 := SequenceFunctions::includingAt(SequenceFunctions::excluding(count2, value), value, insertAt);")
	wantNote(t, r, "_addAt", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign count2 := value;")
	wantNote(t, r, "_addAll", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign bag2 := SequenceFunctions::including(bag2, value);")
	wantNote(t, r, "_addBag", migrate.Mapped, "")

	// Remove: excluding at a position, or excluding the value — approximated
	// when a nonunique variable repeats it and only the first should go.
	wantLine(t, r.Notation, "assign count2 := SequenceFunctions::excluding(count2, value);")
	wantNote(t, r, "_remove", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign count2 := SequenceFunctions::excludingAt(count2, removeAt);")
	wantNote(t, r, "_removeAt", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign bag2 := SequenceFunctions::excluding(bag2, value);")
	wantNote(t, r, "_removeBag", migrate.Approximated, "every occurrence of the value is removed, not only the first")

	// A variable reference resolving to nothing, or to a variable no enclosing
	// scope declared, keeps the action a placeholder; so does a pin named for it.
	wantNote(t, r, "_orphan", migrate.Unmapped, "the variable has no v2 declaration")
	wantNote(t, r, "_clash", migrate.Unmapped, "a pin of the action shares the variable's name")
	wantNote(t, r, "_after", migrate.Unmapped, "the variable has no v2 declaration")
}
