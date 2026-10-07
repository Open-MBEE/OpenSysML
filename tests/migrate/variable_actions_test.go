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
	// an untyped one a bare ref; each keeps its v1 name.
	wantLine(t, r.Notation, "private attribute count : ScalarValues::Integer[0..*] ordered;")
	wantLine(t, r.Notation, "private ref item ball : Ball;")
	wantLine(t, r.Notation, "private ref any;")
	wantLine(t, r.Notation, "private attribute bag : ScalarValues::Integer[0..*] nonunique;")
	wantNote(t, r, "_vCount", migrate.Mapped, "")
	wantNote(t, r, "_vBall", migrate.Mapped, "")
	wantNote(t, r, "_vAny", migrate.Mapped, "")
	wantNote(t, r, "_vBag", migrate.Mapped, "")

	// A structured node's variable is declared inside its block.
	wantLine(t, r.Notation, "private attribute inner : ScalarValues::Integer;")
	wantLine(t, r.Notation, "out result : ScalarValues::Integer[1] = inner;")
	wantNote(t, r, "_vInner", migrate.Mapped, "")
	wantNote(t, r, "_nestRead", migrate.Mapped, "")

	// An inner member named for an outer variable is renamed, so an
	// unqualified name still resolves to the outer variable; an inner opaque
	// body resolves the inner declaration, an outer one the outer's.
	wantLine(t, r.Notation, "private attribute count2 : ScalarValues::Integer;")
	wantNote(t, r, "_vShadow", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign count2 := value;")
	wantNote(t, r, "_nestAddOuter", migrate.Approximated, "never fires")
	wantNote(t, r, "_nestAddInner", migrate.Approximated, "never fires")
	wantLine(t, r.Notation, "action 'count 2' {")
	wantLine(t, r.Notation, "assign count2 := count2 + 1;")
	wantLine(t, r.Notation, "assign rate := rate + 1;")
	wantNote(t, r, "_vRate", migrate.Mapped, "")
	wantNote(t, r, "_bump", migrate.Mapped, "the JavaScript body is translated to v2")

	// A variable whose owner declares no variables stays unmapped, and a body
	// naming it resolves nothing.
	wantNote(t, r, "_vCalcVar", migrate.Unmapped, "not a place for a Variable")
	wantNote(t, r, "_ghost", migrate.Approximated, "nothing visible from Vars::Variables::ghost is called calcVar")

	// Read binds its result pin to the variable; clear empties it. A read with
	// no result pin comments the read and still maps.
	wantLine(t, r.Notation, "out result : ScalarValues::Integer[0..*] = count;")
	wantNote(t, r, "_read", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign count := null;")
	wantNote(t, r, "_clear", migrate.Mapped, "")
	wantLine(t, r.Notation, "/* reads count, which flows nowhere */")
	wantNote(t, r, "_readless", migrate.Mapped, "")

	// Add: re-adding to a unique variable removes it first; insertAt inserts
	// at the position; isReplaceAll assigns the value.
	wantLine(t, r.Notation, "assign count := SequenceFunctions::including(SequenceFunctions::excluding(count, value), value);")
	wantNote(t, r, "_add", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign count := SequenceFunctions::includingAt(SequenceFunctions::excluding(count, value), value, insertAt);")
	wantNote(t, r, "_addAt", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign count := value;")
	wantNote(t, r, "_addAll", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign bag := SequenceFunctions::including(bag, value);")
	wantNote(t, r, "_addBag", migrate.Mapped, "")

	// Remove: excluding at a position, or excluding the value — approximated
	// when a nonunique variable repeats it and only the first should go.
	wantLine(t, r.Notation, "assign count := SequenceFunctions::excluding(count, value);")
	wantNote(t, r, "_remove", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign count := SequenceFunctions::excludingAt(count, removeAt);")
	wantNote(t, r, "_removeAt", migrate.Mapped, "")
	wantLine(t, r.Notation, "assign bag := SequenceFunctions::excluding(bag, value);")
	wantNote(t, r, "_removeBag", migrate.Approximated, "every occurrence of the value is removed, not only the first")

	// A variable reference resolving to nothing, or to a variable no enclosing
	// scope declared, keeps the action a placeholder; so does a pin named for it.
	wantNote(t, r, "_orphan", migrate.Unmapped, "the variable has no v2 declaration")
	wantNote(t, r, "_clash", migrate.Unmapped, "a pin of the action shares the variable's name")
	wantNote(t, r, "_after", migrate.Unmapped, "the variable has no v2 declaration")
}
