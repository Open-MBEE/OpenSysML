package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const triggerBindingModel = `package P {
	part def PQ;
	action def A {
		in c : ScalarValues::Boolean; in t : Time::TimeInstantValue; in d : ISQ::DurationValue; part q : PQ;
		action aw accept when c via q;
		action aat accept at t;
		action aaf accept after d via q;
		action plain accept s : PQ via q;
	}
	action def B :> A { action bw :>> aw; }
	state def S {
		in c : ScalarValues::Boolean; part q : PQ;
		state a; state b;
		transition tr first a accept when c via q then b;
		transition tn first b accept after 5 [SI::s] then a;
	}
}`

// triggerInvocationOf returns the trigger invocation the payload of the accept action owner is bound to.
func triggerInvocationOf(t *testing.T, m *Model, owner *symbols.Symbol) *symbols.Symbol {
	t.Helper()
	_, invocation, _ := m.triggerReceiverBinding(owner)
	if invocation == nil {
		t.Fatalf("%s has no trigger invocation", m.resolver.Index().GetFQN(owner))
	}
	return invocation
}

// A trigger invocation invokes the Triggers function of its kind, which it specializes
// (SysML 8.3.17.17, checkTriggerInvocationExpressionSpecialization).
func TestImpliedTriggerInvocationFunction(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "t.sysml", source.KindSysML, triggerBindingModel)
	for name, want := range map[string]string{
		"P::A::aw": "Triggers::TriggerWhen", "P::A::aat": "Triggers::TriggerAt", "P::A::aaf": "Triggers::TriggerAfter",
	} {
		invocation := triggerInvocationOf(t, m, lookupOne(t, m, name))
		function := m.ExpressionFunction(invocation)
		if got := fqnsOf(m, []*symbols.Symbol{function}); got != want {
			t.Errorf("%s: the trigger invokes %v, want %s", name, got, want)
		}
		if function != m.TriggerFunction(TriggerKind(invocation.Decl)) {
			t.Errorf("%s: ExpressionFunction and TriggerFunction disagree", name)
		}
		if got := m.ImpliedSpecializations(invocation); !slices.Contains(got, function) {
			t.Errorf("%s: the trigger specializes %v, want %s among them", name, fqnsOf(m, got), want)
		}
	}
	if m.TriggerFunction("") != nil || m.TriggerFunction("until") != nil {
		t.Error("TriggerFunction answers a kind that is no TriggerKind")
	}
}

// An accept action owns a binding connector between its receiver parameter and the receiver
// parameter of its trigger invocation (SysML Table 33, checkAcceptActionUsageReceiverBindingConnector).
func TestImpliedTriggerReceiverBinding(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "t.sysml", source.KindSysML, triggerBindingModel)
	for name, want := range map[string][2]string{
		"P::A::aw":  {"", "Triggers::TriggerWhen::receiver"},
		"P::A::aat": {"Transfers::AcceptPerformance::receiver", "Triggers::TriggerAt::receiver"},
		"P::A::aaf": {"", "Triggers::TriggerAfter::receiver"},
	} {
		owner := lookupOne(t, m, name)
		connectors := m.ImpliedBindingConnectors(owner)
		if len(connectors) != 1 {
			t.Errorf("%s owns %d implied binding connectors, want 1", name, len(connectors))
			continue
		}
		connector := connectors[0]
		if got := fqnsOf(m, m.ImpliedSpecializations(connector)); got != "Links::selfLinks" {
			t.Errorf("%s: the connector specializes %v", name, got)
		}
		if got := m.ImpliedTypeFeaturings(connector); len(got) != 1 || got[0].Type != owner || got[0].Snapshots {
			t.Errorf("%s: the connector is featured by %v, want its accept action", name, got)
		}
		ends, related, ok := m.ImpliedBinding(connector)
		if !ok {
			t.Fatalf("%s: ImpliedBinding does not know the connector", name)
		}
		// A receiver written with `via` is a parameter the action holds; without, the one it inherits.
		if written := m.acceptActionParts(owner); written != nil {
			if related[0] != written.receiver {
				t.Errorf("%s: the first end does not refer to the action's receiver parameter", name)
			}
		} else if got := m.resolver.Index().GetFQN(related[0]); got != want[0] {
			t.Errorf("%s: the first end refers to %q, want %s", name, got, want[0])
		}
		chain := related[1]
		if chain.Inclusion == nil || chain.Inclusion.Role != symbols.InclusionChain || chain.Inclusion.Owner != ends[1] {
			t.Fatalf("%s: the second end refers to %+v, want a chain it owns", name, chain.Inclusion)
		}
		links := m.ChainingFeatures(chain)
		if len(links) != 2 || links[0] != triggerInvocationOf(t, m, owner) || m.resolver.Index().GetFQN(links[1]) != want[1] {
			t.Errorf("%s: the chain is %v, want the invocation then %s", name, fqnsOf(m, links), want[1])
		}
		held := []*symbols.Symbol{connector, ends[0], ends[1], chain}
		root.WalkDeclarations(func(sym *symbols.Symbol) {
			for _, element := range held {
				if element == sym {
					t.Errorf("%s: a held %v is a declaration of the scope", name, m.MetaclassOf(sym).Name)
				}
			}
		})
		for _, element := range held {
			if fqn := m.resolver.Index().GetFQN(element); fqn != "" {
				t.Errorf("%s: a held element is indexed as %q", name, fqn)
			}
			if element.OwnerScope != nil || element.Scope != nil {
				t.Errorf("%s: a held element has a scope", name)
			}
		}
		if again := m.ImpliedBindingConnectors(owner); again[0] != connector {
			t.Errorf("%s: the connector is not the same symbol on a second read", name)
		}
	}
}

// An accept action of a payload that is no trigger invocation has no receiver binding, and neither has
// one that only inherits its trigger.
func TestImpliedTriggerReceiverBindingNeedsAnOwnedTrigger(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "t.sysml", source.KindSysML, triggerBindingModel)
	for _, name := range []string{"P::A::plain", "P::B::bw", "P::A", "P::A::q"} {
		if receiver, _, _ := m.triggerReceiverBinding(lookupOne(t, m, name)); receiver != nil {
			t.Errorf("%s has a trigger receiver binding", name)
		}
	}
}

// The accept action of a transition's trigger owns the binding as an accept action usage does.
func TestImpliedTriggerReceiverBindingOfATransitionTrigger(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "t.sysml", source.KindSysML, triggerBindingModel)
	parts := m.transitionPartsOf(lookupOne(t, m, "P::S::tr"))
	if parts == nil || parts.accepter == nil {
		t.Fatal("the transition has no trigger action")
	}
	receiver, invocation, parameter := m.triggerReceiverBinding(parts.accepter)
	if receiver != parts.receiver || invocation == nil {
		t.Fatalf("receiver %v, invocation %v", receiver, invocation)
	}
	if got := m.resolver.Index().GetFQN(parameter); got != "Triggers::TriggerWhen::receiver" {
		t.Errorf("the bound parameter is %q", got)
	}
	if got := len(m.ImpliedBindingConnectors(parts.accepter)); got != 1 {
		t.Errorf("the trigger action owns %d implied binding connectors", got)
	}
	// Without `via` the trigger action binds the receiver it inherits from the accepter it subsets.
	accepter := m.transitionPartsOf(lookupOne(t, m, "P::S::tn")).accepter
	receiver, _, parameter = m.triggerReceiverBinding(accepter)
	if got := m.resolver.Index().GetFQN(receiver); got != "Transfers::AcceptPerformance::receiver" {
		t.Errorf("the inherited receiver is %q", got)
	}
	if got := m.resolver.Index().GetFQN(parameter); got != "Triggers::TriggerAfter::receiver" {
		t.Errorf("the bound parameter is %q", got)
	}
	if got := len(m.ImpliedBindingConnectors(accepter)); got != 1 {
		t.Errorf("the trigger action owns %d implied binding connectors, want 1", got)
	}
}
