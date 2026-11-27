package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// lookupOne resolves the one symbol of a qualified name.
func lookupOne(t *testing.T, m *Model, fqn string) *symbols.Symbol {
	t.Helper()
	found := m.resolver.Index().LookupQualified(fqn)
	if len(found) != 1 {
		t.Fatalf("%s: %d symbols", fqn, len(found))
	}
	return found[0]
}

// The binding of a feature value is a connector with two ends, the first referring to a chain
// through the value and its result (KerML Table 11, checkFeatureValueBindingConnector).
func TestImpliedFeatureValueBindingHoldsConnectorEndsAndChain(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "b.kerml", source.KindKerML,
		`package P { datatype D; class C { feature a : D; feature b : D = a; feature c : D default a; feature d : D; } }`)
	b := lookupOne(t, m, "P::C::b")
	connectors := m.ImpliedBindingConnectors(b)
	if len(connectors) != 1 {
		t.Fatalf("P::C::b owns %d implied binding connectors, want 1", len(connectors))
	}
	connector := connectors[0]
	if again := m.ImpliedBindingConnectors(b); again[0] != connector {
		t.Error("the connector is not the same symbol on a second read")
	}
	if got := m.MetaclassOf(connector); got == nil || got.Name != "BindingConnector" {
		t.Errorf("metaclass of the connector = %v, want BindingConnector", got)
	}
	if got := fqnsOf(m, m.ImpliedSpecializations(connector)); got != "Links::selfLinks" {
		t.Errorf("the connector specializes %v, want Links::selfLinks", got)
	}
	if got := m.ImpliedTypeFeaturings(connector); len(got) != 1 || got[0].Type != lookupOne(t, m, "P::C") || got[0].Snapshots {
		t.Errorf("the connector is featured by %v, want the featuring type of P::C::b", got)
	}
	ends, related, ok := m.ImpliedBinding(connector)
	if !ok {
		t.Fatal("ImpliedBinding does not know the connector")
	}
	for i, want := range []string{"Links::selfLinks::thisThing", "Links::selfLinks::sameThing"} {
		end := ends[i]
		if end.Inclusion == nil || end.Inclusion.Role != symbols.InclusionConnectorEnd || end.Inclusion.Owner != connector || end.Inclusion.Index != i {
			t.Errorf("end %d: inclusion record %+v", i, end.Inclusion)
		}
		if got := fqnsOf(m, m.ImpliedRedefinitions(end)); got != want {
			t.Errorf("end %d redefines %v, want %s", i, got, want)
		}
		if got := m.ImpliedEndReference(end); got != related[i] {
			t.Errorf("end %d refers to %v, want the related feature", i, got)
		}
		if got := m.ImpliedTypeFeaturings(end); len(got) != 1 || got[0].Type != connector {
			t.Errorf("end %d is featured by %v, want its connector", i, got)
		}
	}
	if related[1] != b {
		t.Errorf("the second end refers to %s, want P::C::b", m.resolver.Index().GetFQN(related[1]))
	}
	chain := related[0]
	if chain.Inclusion == nil || chain.Inclusion.Role != symbols.InclusionChain || chain.Inclusion.Owner != ends[0] {
		t.Fatalf("the first end refers to %+v, want a chain it owns", chain.Inclusion)
	}
	if owned := m.ImpliedOwnedFeatures(ends[0]); !slices.Equal(owned, []*symbols.Symbol{chain}) {
		t.Errorf("the first end owns %v, want its chain", owned)
	}
	value := m.ValueExpression(b)
	if links := m.impliedChainingFeatures(chain); len(links) != 2 || links[0] != value || links[1] != m.ExpressionResult(value) {
		t.Errorf("the chain chains %v, want the value and its result", links)
	}
	// A default value binds nothing where it is declared (KerML 8.4.4.11), nor does a feature without a value.
	for _, name := range []string{"P::C::c", "P::C::d"} {
		if got := m.ImpliedBindingConnectors(lookupOne(t, m, name)); len(got) != 0 {
			t.Errorf("%s owns %d implied binding connectors, want none", name, len(got))
		}
	}
}

// What is held for an implied relationship is in no scope and no index, so no name resolves to it
// and nothing that walks the declarations, the default export included, meets it.
func TestImpliedBindingElementsAreInNoScope(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "b.kerml", source.KindKerML,
		`package P { datatype D; class C { feature a : D; feature b : D = a; } }`)
	held := map[*symbols.Symbol]bool{}
	for _, connector := range m.ImpliedBindingConnectors(lookupOne(t, m, "P::C::b")) {
		held[connector] = true
		ends, _, _ := m.ImpliedBinding(connector)
		for _, end := range ends {
			held[end] = true
			for _, chain := range m.ImpliedOwnedFeatures(end) {
				held[chain] = true
			}
		}
	}
	if len(held) != 4 {
		t.Fatalf("held %d elements, want a connector, two ends and a chain", len(held))
	}
	root.WalkDeclarations(func(sym *symbols.Symbol) {
		if held[sym] {
			t.Errorf("a held %v is a declaration of the scope", m.MetaclassOf(sym).Name)
		}
	})
	for sym := range held {
		if fqn := m.resolver.Index().GetFQN(sym); fqn != "" {
			t.Errorf("a held element is indexed as %q", fqn)
		}
		if sym.OwnerScope != nil || sym.Scope != nil {
			t.Error("a held element has a scope")
		}
	}
}

// The binding of an initial value is featured by the chain that.startShot, which it owns (KerML 8.4.4.11).
func TestImpliedInitialValueBindingIsFeaturedByStartShot(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "b.sysml", source.KindSysML,
		`package P { attribute def D; part def C { attribute a : D; attribute b : D := a; } }`)
	connectors := m.ImpliedBindingConnectors(lookupOne(t, m, "P::C::b"))
	if len(connectors) != 1 {
		t.Fatalf("%d implied binding connectors, want 1", len(connectors))
	}
	featuring := m.ImpliedTypeFeaturings(connectors[0])
	if len(featuring) != 1 || featuring[0].Type.Inclusion == nil || featuring[0].Type.Inclusion.Owner != connectors[0] {
		t.Fatalf("the connector is featured by %v, want a chain it owns", featuring)
	}
	if got := fqnsOf(m, m.impliedChainingFeatures(featuring[0].Type)); got != "Base::things::that Occurrences::Occurrence::startShot" {
		t.Errorf("the featuring chain is %v, want that.startShot", got)
	}
	if owned := m.ImpliedOwnedFeatures(connectors[0]); !slices.Equal(owned, []*symbols.Symbol{featuring[0].Type}) {
		t.Errorf("the connector owns %v, want its featuring chain", owned)
	}
}

// A transition owns a binding of its succession to transitionLink and one of its source to its first
// parameter, and its succession's source end refers to the source (SysML Table 33).
func TestImpliedTransitionBindings(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "b.sysml", source.KindSysML,
		`package P { state def S { state a; state b; transition t first a then b; } }`)
	transition := lookupOne(t, m, "P::S::t")
	parts := m.transitionPartsOf(transition)
	connectors := m.ImpliedBindingConnectors(transition)
	if len(connectors) != 2 {
		t.Fatalf("%d implied binding connectors, want 2", len(connectors))
	}
	source := lookupOne(t, m, "P::S::a")
	want := [][2]*symbols.Symbol{{parts.succession, parts.transitionLink}, {source, parts.linkParameters[0]}}
	for i, connector := range connectors {
		if _, related, _ := m.ImpliedBinding(connector); related != want[i] {
			t.Errorf("binding %d relates %v, want %v", i, fqnsOf(m, related[:]), fqnsOf(m, want[i][:]))
		}
	}
	if got := fqnsOf(m, m.ImpliedSpecializations(parts.succession)); got != "Occurrences::happensBeforeLinks" {
		t.Errorf("the succession specializes %v, want Occurrences::happensBeforeLinks", got)
	}
	if got := fqnsOf(m, m.ImpliedRedefinitions(parts.transitionLink)); got != "TransitionPerformances::TransitionPerformance::transitionLink" {
		t.Errorf("transitionLink redefines %v", got)
	}
	if got := fqnsOf(m, m.ImpliedRedefinitions(parts.linkParameters[0])); got != "States::StateTransitionAction::transitionLinkSource" {
		t.Errorf("the first parameter redefines %v", got)
	}
	ends := m.SuccessionEndMembers(parts.succession)
	if got := m.ImpliedSpecializations(ends[0]); !slices.Contains(got, source) {
		t.Errorf("the source end specializes %v, want the transition's source among them", fqnsOf(m, got))
	}
	if got := m.ImpliedSpecializations(ends[1]); slices.Contains(got, lookupOne(t, m, "P::S::b")) {
		t.Error("the target end's written reference is implied")
	}
}

// The result of a behavior invocation is bound to the invocation itself (KerML Table 11, Note 3),
// that of a function invocation is not.
func TestImpliedInvocationBehaviorBinding(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "b.kerml", source.KindKerML,
		`package P { datatype D; behavior B { in x : D; } function F { in x : D; return : D; }
			class C { feature d : D; feature viaB = B(d); feature viaF : D = F(d); } }`)
	for name, want := range map[string]int{"P::C::viaB": 1, "P::C::viaF": 0} {
		invocation := m.ValueExpression(lookupOne(t, m, name))
		connectors := m.ImpliedBindingConnectors(invocation)
		if len(connectors) != want {
			t.Fatalf("%s: the invocation owns %d implied binding connectors, want %d", name, len(connectors), want)
		}
		if want == 1 {
			if _, related, _ := m.ImpliedBinding(connectors[0]); related[0] != invocation || related[1] != m.ExpressionResult(invocation) {
				t.Errorf("%s: the binding relates %v, want the invocation and its result", name, related)
			}
		}
	}
}

// An end owning a cross feature has a CrossSubsetting of a chain through the other end and that
// cross feature, in place of the base it has until the chain exists (KerML Table 9).
func TestImpliedCrossSubsettingTargetsAChain(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "b.kerml", source.KindKerML,
		`package P { class C; assoc AX { end [1] feature xa : C; end [0..*] feature xb : C; } }`)
	xa, xb := lookupOne(t, m, "P::AX::xa"), lookupOne(t, m, "P::AX::xb")
	chain := m.ImpliedCrossedChain(xa)
	if chain == nil || chain.Inclusion == nil || chain.Inclusion.Role != symbols.InclusionChain || chain.Inclusion.Owner != xa {
		t.Fatalf("the crossed chain of xa is %+v, want a chain xa owns", chain)
	}
	if links := m.impliedChainingFeatures(chain); len(links) != 2 || links[0] != xb || links[1] != m.OwnedCrossFeature(xa) {
		t.Errorf("the chain chains %s, want xb and the cross feature of xa", fqnsOf(m, links))
	}
	if got := m.ImpliedSpecializations(xa); !slices.Contains(got, chain) {
		t.Errorf("xa specializes %s, want its crossed chain among them", fqnsOf(m, got))
	}
	if !slices.Contains(m.ImpliedOwnedFeatures(xa), chain) {
		t.Error("xa does not own its crossed chain")
	}
	if m.ImpliedCrossedChain(lookupOne(t, m, "P::C")) != nil {
		t.Error("a class has a crossed chain")
	}
}

// The source parameter of a feature chain expression nests a feature redefining the '.' function's
// target and the feature reached, and the result subsets a chain through both (KerML Table 11, Note 5).
func TestImpliedFeatureChainExpressionTargetAndResult(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "b.kerml", source.KindKerML,
		`package P { datatype D; struct S { feature px : D; } class C { feature ps : S; feature v : D = (ps).px; } }`)
	expr := m.ValueExpression(lookupOne(t, m, "P::C::v"))
	parameters := m.ExpressionParameters(expr)
	if len(parameters) == 0 {
		t.Fatal("the chain expression has no source parameter")
	}
	nested := m.ImpliedOwnedFeatures(parameters[0])
	if len(nested) != 1 || nested[0].Inclusion == nil || nested[0].Inclusion.Role != symbols.InclusionChainTarget {
		t.Fatalf("the source parameter owns %v, want one target feature", nested)
	}
	if got := fqnsOf(m, m.ImpliedRedefinitions(nested[0])); got != "ControlFunctions::.::source::target P::S::px" {
		t.Errorf("the target feature redefines %s", got)
	}
	result := m.ExpressionResult(expr)
	chain := m.impliedSubsettedChain(result)
	if chain == nil || chain.Inclusion.Owner != result {
		t.Fatalf("the result subsets %+v, want a chain it owns", chain)
	}
	if links := m.impliedChainingFeatures(chain); len(links) != 2 || links[0] != parameters[0] || links[1] != nested[0] {
		t.Errorf("the result's chain chains %s, want the source parameter and its target feature", fqnsOf(m, links))
	}
	if got := m.ImpliedSpecializations(result); !slices.Contains(got, chain) || slices.Contains(got, lookupOne(t, m, "P::S::px")) {
		t.Errorf("the result specializes %s, want the chain and not the feature it ends at", fqnsOf(m, got))
	}
}

// A chain read before any binding connector keeps the features it chains once one is read.
func TestImpliedChainSurvivesTheFirstBindingRead(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "b.kerml", source.KindKerML,
		`package P { datatype D; class C { feature a : D; feature b : D = a; }
			assoc AX { end [1] feature xa : C; end [0..*] feature xb : C; } }`)
	chain := m.ImpliedCrossedChain(lookupOne(t, m, "P::AX::xa"))
	if chain == nil {
		t.Fatal("xa has no crossed chain")
	}
	if len(m.ImpliedBindingConnectors(lookupOne(t, m, "P::C::b"))) != 1 {
		t.Fatal("P::C::b owns no implied binding connector")
	}
	if links := m.impliedChainingFeatures(chain); len(links) != 2 {
		t.Errorf("the crossed chain chains %d features after a binding read, want 2", len(links))
	}
}

// A connector, its ends and its chain are distinct elements: none conforms to another or to their owner.
func TestImpliedBindingElementsAreDistinct(t *testing.T) {
	m, _ := buildModelWithLibraryNamedKind(t, "b.kerml", source.KindKerML,
		`package P { datatype D; class C { feature a : D; feature b : D = a; } }`)
	b := lookupOne(t, m, "P::C::b")
	connector := m.ImpliedBindingConnectors(b)[0]
	ends, _, _ := m.ImpliedBinding(connector)
	elements := append([]*symbols.Symbol{b, connector, ends[0], ends[1]}, m.ImpliedOwnedFeatures(ends[0])...)
	for i, one := range elements {
		for j, other := range elements {
			if i != j && symbols.KeyOf(one) == symbols.KeyOf(other) {
				t.Errorf("elements %d and %d share a key", i, j)
			}
		}
	}
	if m.Conforms(ends[0], ends[1]) || m.Conforms(ends[1], ends[0]) || m.Conforms(connector, b) {
		t.Error("distinct held elements conform to one another")
	}
}
