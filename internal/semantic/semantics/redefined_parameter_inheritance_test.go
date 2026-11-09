package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// redefinedParameterSource is the model whose symbol surfaces listed inherited
// library parameters their declared or inherited parameters redefine.
const redefinedParameterSource = `package P {
	private import ScalarValues::*;
	calc def Calc { in x : Real; return r : Real; }
	constraint def Cons { in x : Real; x > 0 }
	verification def Ver { subject s; }
	action def Act { in a : Real; }
}`

// libSym is the one standard-library symbol the qualified name denotes.
func libSym(t *testing.T, m *Model, fqn string) *symbols.Symbol {
	t.Helper()
	syms := m.resolver.Index().LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(%s) = %v, want one library symbol", fqn, syms)
	}
	return syms[0]
}

func effectiveMemberNames(m *Model, sym *symbols.Symbol) []string {
	var names []string
	for _, member := range m.Members(sym) {
		if member.Symbol.Kind == symbols.SymbolDocumentation {
			continue
		}
		names = append(names, m.EffectiveNameOf(member.Symbol))
	}
	return names
}

// A behavior with several generals inherits the parameters none of its own
// redefines from each of them (KerML 7.4.7.2, 8.3.3.1): Calculation, which
// specializes Action and Evaluation, has Evaluation's result parameter.
func TestBehaviorWithSeveralGeneralsInheritsTheirParameters(t *testing.T) {
	m, _ := buildModelWithStdlib(t, redefinedParameterSource)
	calculation := libSym(t, m, "Calculations::Calculation")
	result := libSym(t, m, "Performances::Evaluation::result")
	if got := m.ResultParameterOf(calculation); got != result {
		t.Fatalf("ResultParameterOf(Calculation) = %v, want Evaluation::result", got)
	}
	m2, root2 := buildModel(t, `package Q {
		behavior A { in a1; out a2; }
		behavior B { in b1; out b2; return br; }
		behavior C specializes A, B { in c1; }
	}`)
	c := nestedSym(t, root2, "Q::C")
	var positional []string
	for _, p := range m2.BehaviorParametersOf(c) {
		positional = append(positional, p.Symbol.Owner().Name+"::"+p.Symbol.Name)
	}
	// c1 redefines a1 and b1 by position; the rest follow in general order.
	if want := []string{"C::c1", "A::a2", "B::b2", "B::br"}; !slices.Equal(positional, want) {
		t.Fatalf("BehaviorParametersOf(C) = %v, want %v", positional, want)
	}
	if got := m2.ImplicitParameterRedefinitions(nestedSym(t, root2, "Q::C::c1")); len(got) != 2 {
		t.Fatalf("ImplicitParameterRedefinitions(C::c1) = %v, want A::a1 and B::b1", got)
	}
}

// A parameter reached through two generals, one redefining the other's, is
// inherited once, as the redefining one.
func TestParameterInheritedThroughTwoGeneralsIsListedOnce(t *testing.T) {
	m, root := buildModel(t, `package Q {
		behavior Base { in p; }
		behavior Sub specializes Base { in q :>> p; }
		behavior Both specializes Sub, Base;
	}`)
	both := nestedSym(t, root, "Q::Both")
	params := m.BehaviorParametersOf(both)
	if len(params) != 1 || params[0].Symbol != nestedSym(t, root, "Q::Sub::q") {
		t.Fatalf("BehaviorParametersOf(Both) = %v, want Sub::q alone", params)
	}
}

// A declared result parameter redefines the result a library behavior reaches
// through several generals, so the redefined parameter is not inherited, in
// the members and in the mask alike (KerML 8.3.3.1, 8.4.4.6).
func TestDeclaredResultMasksTheLibraryResultReachedThroughGenerals(t *testing.T) {
	m, root := buildModelWithStdlib(t, redefinedParameterSource)
	calc := nestedSym(t, root, "P::Calc")
	r := nestedSym(t, root, "P::Calc::r")
	result := libSym(t, m, "Performances::Evaluation::result")
	if got := m.MaskedParameterRedefinitions(r); len(got) != 1 || got[0] != result {
		t.Fatalf("MaskedParameterRedefinitions(Calc::r) = %v, want Evaluation::result", got)
	}
	if !m.InheritanceMasked(calc, result) {
		t.Fatalf("Calc still inherits Evaluation::result")
	}
	if names := effectiveMemberNames(m, calc); len(names) < 2 || names[0] != "x" || names[1] != "r" || slices.Contains(names, "result") {
		t.Fatalf("Members(Calc) = %v, want x, r and no result", names)
	}
	if m.ResultParameterOf(calc) != r {
		t.Fatalf("ResultParameterOf(Calc) is not r")
	}
	act := nestedSym(t, root, "P::Act")
	if params := m.BehaviorParametersOf(act); len(params) != 1 || params[0].Symbol != nestedSym(t, root, "P::Act::a") {
		t.Fatalf("BehaviorParametersOf(Act) = %v, want a alone", params)
	}
}

// An inherited redefiner masks too: Ver inherits VerificationCase::verdict,
// which redefines Case::result, itself redefining Evaluation::result by
// position; Case's subject and objective mask their generals' roles the same way.
func TestInheritedRedefinerMasksTheWholeRedefinitionChain(t *testing.T) {
	m, root := buildModelWithStdlib(t, redefinedParameterSource)
	ver := nestedSym(t, root, "P::Ver")
	caseResult := libSym(t, m, "Cases::Case::result")
	result := libSym(t, m, "Performances::Evaluation::result")
	if got := m.MaskedParameterRedefinitions(caseResult); len(got) != 1 || got[0] != result {
		t.Fatalf("MaskedParameterRedefinitions(Case::result) = %v, want Evaluation::result", got)
	}
	for _, masked := range []*symbols.Symbol{caseResult, result, libSym(t, m, "Cases::Case::subj"), libSym(t, m, "Cases::Case::obj")} {
		if !m.InheritanceMasked(ver, masked) {
			t.Errorf("Ver still inherits %s::%s", masked.Owner().Name, masked.Name)
		}
	}
	names := effectiveMemberNames(m, ver)
	if len(names) < 2 || names[0] != "s" || !slices.Contains(names, "verdict") || slices.Contains(names, "result") {
		t.Fatalf("Members(Ver) = %v, want s, verdict and no result", names)
	}
	params := m.BehaviorParametersOf(ver)
	if len(params) != 1 || params[0].Symbol != libSym(t, m, "VerificationCases::VerificationCase::verdict") {
		t.Fatalf("BehaviorParametersOf(Ver) = %v, want verdict alone", params)
	}
}

// BooleanEvaluation's unnamed return redefines Evaluation::result by position,
// so it answers to that name (KerML 8.2.4, 8.3.3.3): its effective and
// qualified names, the members of a constraint def and a member lookup.
func TestUnnamedLibraryReturnIsQualifiedByTheRedefinedName(t *testing.T) {
	m, root := buildModelWithStdlib(t, redefinedParameterSource)
	cons := nestedSym(t, root, "P::Cons")
	boolEval := libSym(t, m, "Performances::BooleanEvaluation")
	ret := m.ResultParameterOf(boolEval)
	if ret == nil || ret.Name != "" || ret.Owner() != boolEval {
		t.Fatalf("ResultParameterOf(BooleanEvaluation) = %v, want its unnamed return", ret)
	}
	if !m.PositionallyNamed(ret) || m.EffectiveNameOf(ret) != "result" {
		t.Fatalf("the return is not positionally named result: %v %q", m.PositionallyNamed(ret), m.EffectiveNameOf(ret))
	}
	if got := m.QualifiedNameOf(ret); got != "Performances::BooleanEvaluation::result" {
		t.Fatalf("QualifiedNameOf(return) = %q", got)
	}
	if got, ok := m.LookupMember(cons, "result"); !ok || got != ret {
		t.Fatalf("LookupMember(Cons, result) = %v, %v; want the return", got, ok)
	}
	if names := effectiveMemberNames(m, cons); len(names) < 1 || names[0] != "x" || !slices.Contains(names, "result") {
		t.Fatalf("Members(Cons) = %v, want x then the inherited result", names)
	}
	named := libSym(t, m, "Performances::Evaluation::result")
	if m.PositionallyNamed(named) || m.QualifiedNameOf(named) != "Performances::Evaluation::result" {
		t.Fatalf("a declared name is not positional: %v %q", m.PositionallyNamed(named), m.QualifiedNameOf(named))
	}
	if m.QualifiedNameOf(nestedSym(t, root, "P::Cons::x")) != "P::Cons::x" {
		t.Fatalf("QualifiedNameOf(Cons::x) is not the index's")
	}
}
