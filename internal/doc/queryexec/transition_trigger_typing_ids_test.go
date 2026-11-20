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
	payload := fixture.symbol(t, "S::u::x")
	membership := fixture.model.OwningMembership(payload)
	if membership == nil {
		t.Fatal("named trigger payload reads no owning membership")
	}
	if membership.Implicit.Member != payload {
		t.Fatalf("named trigger payload's owning membership owns %s %q, not the payload", membership.Implicit.Member.Kind, membership.Implicit.Member.Name)
	}
	// The accept owns it, and reads it as its payload parameter: one element, named as declared.
	triggers, _ := fixture.model.ReflectiveElements(fixture.symbol(t, "S::u"), "triggerAction")
	if len(triggers) != 1 {
		t.Fatalf("trigger actions of u = %d, want 1", len(triggers))
	}
	if owners, _ := fixture.model.ReflectiveElements(payload, "owner"); len(owners) != 1 || owners[0] != triggers[0] {
		t.Fatalf("named trigger payload's owner = %v, want the trigger's accept", owners)
	}
	if parameters, _ := fixture.model.ReflectiveElements(triggers[0], "payloadParameter"); len(parameters) != 1 || parameters[0] != payload {
		t.Fatalf("accept's payloadParameter = %v, want the named payload", parameters)
	}
	if meta := fixture.model.MetaclassOf(payload); meta == nil || meta.Name != "ReferenceUsage" {
		t.Fatalf("named trigger payload is a %v, want a ReferenceUsage", meta)
	}
	if owned := fixture.model.OwnedMemberships(triggers[0]); len(owned) != 2 || owned[0] != membership {
		t.Fatalf("accept's ownedMembership = %v, want the payload's owning membership first", owned)
	}
	transitionMembers, _ := fixture.model.ReflectiveElements(fixture.symbol(t, "S::u"), "ownedMember")
	for _, member := range transitionMembers {
		if member == payload {
			t.Fatal("transition owns the named payload its trigger's accept owns")
		}
	}
	if members, _ := fixture.model.ReflectiveElements(membership, "ownedMemberElement"); len(members) != 1 || members[0] != payload {
		t.Fatalf("membership's ownedMemberElement = %v, want the named payload", members)
	}
	walk(fixture.symbol(t, "S"))
	walk(fixture.symbol(t, "A"))
	sort.Strings(typings)
	want := "Observatory__A__a___400_ft0 Observatory__S__p_ft0 Observatory__S__t___40trigger___400_ft0 Observatory__S__u___40trigger__x_ft0"
	if got := strings.Join(typings, " "); got != want {
		t.Fatalf("FeatureTyping ids = %q, want %q", got, want)
	}
}
