package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// The two ends a transition's succession owns, and the target end's ReferenceSubsetting, read the ids
// the conversion mints under the succession (transitionSuccession): `<succession>_pend0`, `_pend1`, `_pend1_prs`.
func TestTransitionSuccessionEndsReadTheConversionIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
    state def S {
        state idle;
        state busy;
        transition go first idle then busy;
    }
    state def T {
        state idle;
        then busy;
        state busy;
    }
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) string {
		id, err := ElementIDOf(read, fixture.index, sym)
		if err != nil {
			t.Fatalf("%v id: %v", sym.Decl, err)
		}
		return id
	}
	transition := fixture.symbol(t, "S::go")
	successions, _ := fixture.model.ReflectiveElements(transition, "succession")
	if len(successions) != 1 {
		t.Fatalf("successions = %d, want 1", len(successions))
	}
	ends, _ := fixture.model.ReflectiveElements(successions[0], "ownedEndFeature")
	want := "Observatory__S__go_succession_pend0 Observatory__S__go_succession_pend1"
	if got := strings.Join(elementIDs(elementID, ends), " "); got != want {
		t.Fatalf("end ids = %q, want %q", got, want)
	}
	owned, _ := fixture.model.ReflectiveElements(successions[0], "ownedRelationship")
	want = "Observatory__S__go_succession_pend0_om Observatory__S__go_succession_pend1_om"
	if got := strings.Join(elementIDs(elementID, owned), " "); got != want {
		t.Fatalf("ownedRelationship ids = %q, want %q", got, want)
	}
	references, _ := fixture.model.ReflectiveElements(ends[1], "ownedReferenceSubsetting")
	want = "Observatory__S__go_succession_pend1_prs"
	if got := strings.Join(elementIDs(elementID, references), " "); got != want {
		t.Fatalf("target end ownedReferenceSubsetting ids = %q, want %q", got, want)
	}
	// A state body's `then busy;` after a state is written as a transition (encodeTargetTransition): the same ids off its.
	var stateBody *symbols.Symbol
	for _, rel := range fixture.model.OwnedRelationships(fixture.symbol(t, "T")) {
		if member := rel.Implicit.Member; member != nil && member.DeclaresTransition() {
			stateBody = member
		}
	}
	if stateBody == nil {
		t.Fatal("T owns no transition for its `then busy;`")
	}
	successions, _ = fixture.model.ReflectiveElements(stateBody, "succession")
	if len(successions) != 1 {
		t.Fatalf("state-body successions = %d, want 1", len(successions))
	}
	ends, _ = fixture.model.ReflectiveElements(successions[0], "ownedEndFeature")
	ids := elementIDs(elementID, ends)
	if len(ids) != 2 || !strings.HasSuffix(ids[0], "_succession_pend0") || !strings.HasSuffix(ids[1], "_succession_pend1") || !strings.HasPrefix(ids[0], "Observatory__T__") {
		t.Fatalf("state-body end ids = %q, want Observatory__T__…_succession_pend0/_pend1", ids)
	}
}
