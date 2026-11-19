package queryexec

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// The typing of a trigger's payload is one FeatureTyping, owned by the payload parameter alone: neither the
// trigger's accept nor the transition, whose scope declares a named payload, reads a second one at its id.
func TestTransitionTriggerPayloadTypingReadsOnce(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
    item def Sig;
    port def PD;
    state def S {
        port p : PD;
        state idle;
        state busy;
        transition t first idle accept Sig then busy;
        transition u first idle accept x : Sig via p then busy;
    }
    action def A {
        action a accept Sig;
    }
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	var typings []string
	seen := map[symbols.ElementKey]bool{}
	var walk func(sym *symbols.Symbol)
	walk = func(sym *symbols.Symbol) {
		if key := symbols.KeyOf(sym); seen[key] {
			return
		} else {
			seen[key] = true
		}
		if sym.ImplicitWrittenRelationship() && sym.Implicit.Kind == ast.RelTyping {
			id, err := ElementIDOf(read, fixture.index, sym)
			if err != nil {
				t.Fatalf("typing id: %v", err)
			}
			typings = append(typings, id)
		}
		for _, rel := range fixture.model.OwnedRelationships(sym) {
			walk(rel)
		}
		related, _ := fixture.model.ReflectiveElements(sym, "ownedRelatedElement")
		for _, element := range related {
			walk(element)
		}
	}
	// The accept shares the payload's node, and reads its own id rather than the payload's.
	accepts, _ := fixture.model.ReflectiveElements(fixture.symbol(t, "S::t"), "triggerAction")
	if len(accepts) != 1 {
		t.Fatalf("trigger actions = %d, want 1", len(accepts))
	}
	if id, err := ElementIDOf(read, fixture.index, accepts[0]); err != nil || id != "Observatory__S__t___40trigger" {
		t.Fatalf("trigger accept id = %q, %v, want Observatory__S__t___40trigger", id, err)
	}
	// A named payload, declared in the transition's scope, is owned through the accept's membership.
	if fixture.model.OwningMembership(fixture.symbol(t, "S::u::x")) == nil {
		t.Fatal("named trigger payload reads no owning membership")
	}
	walk(fixture.symbol(t, "S"))
	walk(fixture.symbol(t, "A"))
	sort.Strings(typings)
	want := "Observatory__A__a___400_ft0 Observatory__S__p_ft0 Observatory__S__t___40trigger___400_ft0 Observatory__S__u___40trigger__x_ft0"
	if got := strings.Join(typings, " "); got != want {
		t.Fatalf("FeatureTyping ids = %q, want %q", got, want)
	}
}
