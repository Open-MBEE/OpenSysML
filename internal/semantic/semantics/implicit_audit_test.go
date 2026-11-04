package semantics

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// auditLibraryBases walks a `::`-separated path from root and returns the symbol.
func auditLibraryBases(t *testing.T, root *symbols.Scope, path string) *symbols.Symbol {
	t.Helper()
	scope := root
	var s *symbols.Symbol
	for _, name := range strings.Split(path, "::") {
		if scope == nil {
			t.Fatalf("%s: no scope at %q", path, name)
		}
		s, _ = scope.LookupLocal(name)
		if s == nil {
			t.Fatalf("%s: %q not found", path, name)
		}
		scope = s.Scope
	}
	return s
}

// impliedLibraryFQNs are the qualified names of the library generals and the
// nested owner feature the implementation implies for sym.
func impliedLibraryFQNs(m *Model, sym *symbols.Symbol) []string {
	var out []string
	for _, s := range slices.Concat(m.DirectSupertypes(sym), m.ImplicitSubsettings(sym)) {
		out = append(out, m.resolver.Index().GetFQN(s))
	}
	return out
}

// Every row of the implicit-specialization audit the implementation fixes: the
// implied library general is present (SysML v2 1.1 Beta 2 §8.4, KerML 1.1 Beta 2 §8.4)
// and a member the library general supplies resolves from the usage.
func TestImpliedSpecializationAuditRows(t *testing.T) {
	sysml := `package P {
		attribute def AD; part def PD; occurrence def OD { event occurrence ev; }
		individual occurrence def IOD; individual occurrence io : IOD;
		action def Ad { in x : AD; action s1; fork f; join j; merge mg; decide dc;
			action acc accept sig : AD; action snd send x to s1; action stop terminate;
			transition tr first s1 then acc; }
		state def Sd { state s1; state s2; transition t first s1 accept sig : AD then s2; }
		state def Pl parallel { state a; state b; }
		part def Prt { event occurrence pev; }
		requirement def RD { actor ac : PD; stakeholder sh : PD; frame concern fc; }
		constraint def CD;
		part psat { satisfy requirement sq : RD by this; assert not satisfy requirement nsq : RD by this;
			assert constraint ac1 : CD; assert not constraint ac2 : CD; }
		view def VD { viewpoint vp; }
		verification case def VC { objective { verify requirement vr : RD; } actor va : PD; }
		flow def FD { end src; end tgt; }
		part pA { part pB; }
		flow fl1 of AD from pA.pB to pA; flow fnoends : FD; message msg of AD from pA to pA;
		connection def Tri { end a : PD; end b : PD; end c : PD; }
	}`
	m, root := buildModelWithLibraryNamedKind(t, "audit.sysml", source.KindSysML, sysml)
	kerml := `package K {
		class C { composite feature cf : C; portion feature pf : C; }
		struct S { composite feature sf : S; }
		assoc A3 { end a1 : C; end a2 : C; end a3 : C; }
		metadata MF : Metaobjects::Metaobject;
		behavior B { step s2; flow fk from s2 to s2; flow bare; succession flow sfk from s2 to s2; }
	}`
	mk, rootk := buildModelWithLibraryNamedKind(t, "audit.kerml", source.KindKerML, kerml)
	cases := []struct {
		model  *Model
		root   *symbols.Scope
		path   string
		want   string // implied library general
		member string // a member that general supplies
		absent string // a general the row must not imply
	}{
		{m, root, "P::Ad::f", "Actions::Action::forks", "start", ""},
		{m, root, "P::Ad::j", "Actions::Action::joins", "start", ""},
		{m, root, "P::Ad::mg", "Actions::Action::merges", "start", ""},
		{m, root, "P::Ad::dc", "Actions::Action::decisions", "start", ""},
		{m, root, "P::Ad::acc", "Actions::acceptActions", "payload", ""},
		{m, root, "P::Ad::acc", "Actions::Action::acceptSubactions", "payload", ""},
		{m, root, "P::Ad::snd", "Actions::sendActions", "receiver", ""},
		{m, root, "P::Ad::snd", "Actions::Action::sendSubactions", "receiver", ""},
		{m, root, "P::Ad::stop", "Actions::terminateActions", "terminatedOccurrence", ""},
		{m, root, "P::Ad::stop", "Actions::Action::terminateSubactions", "terminatedOccurrence", ""},
		{m, root, "P::Ad::tr", "Actions::Action::decisionTransitions", "effect", ""},
		{m, root, "P::Sd::t", "States::StateAction::stateTransitions", "accepter", ""},
		{m, root, "P::Sd::s1", "States::StateAction::exclusiveStates", "substates", "States::StateAction::substates"},
		{m, root, "P::Pl::a", "States::StateAction::substates", "substates", "States::StateAction::exclusiveStates"},
		{m, root, "P::OD::ev", "Occurrences::Occurrence::timeEnclosedOccurrences", "portionOf", ""},
		{m, root, "P::Prt::pev", "Occurrences::Occurrence::timeEnclosedOccurrences", "portionOf", ""},
		{m, root, "P::io", "Occurrences::occurrences", "portionOf", "Occurrences::Life"},
		{m, root, "P::RD::ac", "Requirements::RequirementCheck::actors", "start", ""},
		{m, root, "P::RD::sh", "Requirements::RequirementCheck::stakeholders", "start", ""},
		{m, root, "P::RD::fc", "Requirements::RequirementCheck::concerns", "subj", "Requirements::RequirementCheck::subrequirements"},
		{m, root, "P::VC::va", "Cases::Case::actors", "start", ""},
		{m, root, "P::psat::sq", "Requirements::satisfiedRequirementChecks", "subj", ""},
		{m, root, "P::psat::nsq", "Requirements::notSatisfiedRequirementChecks", "subj", "Requirements::satisfiedRequirementChecks"},
		{m, root, "P::psat::ac1", "Constraints::assertedConstraintChecks", "result", ""},
		{m, root, "P::psat::ac2", "Constraints::negatedConstraintChecks", "result", "Constraints::assertedConstraintChecks"},
		{m, root, "P::VD::vp", "Views::View::viewpointSatisfactions", "subj", ""},
		{m, root, "P::fl1", "Flows::flows", "payload", "Flows::Message"},
		{m, root, "P::fnoends", "Flows::messages", "payload", "Flows::flows"},
		{m, root, "P::msg", "Flows::messages", "payload", "Flows::flows"},
		{m, root, "P::Tri::a", "Links::Link::participant", "self", ""},
		{mk, rootk, "K::C::cf", "Occurrences::Occurrence::suboccurrences", "portionOf", ""},
		{mk, rootk, "K::C::pf", "Occurrences::Occurrence::portions", "portionOf", ""},
		{mk, rootk, "K::S::sf", "Objects::Object::subobjects", "subobjects", ""},
		{mk, rootk, "K::A3::a1", "Links::Link::participant", "self", ""},
		{mk, rootk, "K::MF", "Metaobjects::metaobjects", "annotatedElement", "Objects::objects"},
		{mk, rootk, "K::B::fk", "Transfers::flowTransfers", "payload", "Flows::Message"},
		{mk, rootk, "K::B::bare", "Transfers::transfers", "payload", "Transfers::flowTransfers"},
		{mk, rootk, "K::B::sfk", "Transfers::flowTransfersBefore", "payload", "Transfers::flowTransfers"},
	}
	for _, tc := range cases {
		sym := auditLibraryBases(t, tc.root, tc.path)
		got := impliedLibraryFQNs(tc.model, sym)
		if !slices.Contains(got, tc.want) {
			t.Errorf("%s: implied generals %v, want %s among them", tc.path, got, tc.want)
		}
		if tc.absent != "" && slices.Contains(got, tc.absent) {
			t.Errorf("%s: implied generals %v must not include %s", tc.path, got, tc.absent)
		}
		if member, ok := tc.model.LookupMember(sym, tc.member); !ok || member == nil {
			t.Errorf("%s: member %q does not resolve through %s", tc.path, tc.member, tc.want)
		}
	}
	// The objective's `verify` is a requirement check verified by the case.
	vr := auditLibraryBases(t, root, "P::VC").Scope.AllMembers()
	for _, obj := range vr {
		if obj.Name != "" {
			continue
		}
		for _, v := range obj.Scope.AllMembers() {
			got := impliedLibraryFQNs(m, v)
			for _, want := range []string{"Requirements::requirementChecks", "VerificationCases::VerificationCase::obj::requirementVerifications"} {
				if !slices.Contains(got, want) {
					t.Errorf("verify requirement: implied generals %v, want %s", got, want)
				}
			}
			if _, ok := m.LookupMember(v, "subj"); !ok {
				t.Errorf("verify requirement: member subj does not resolve")
			}
		}
	}
}

// The public base queries agree with the implied edges: a typed KerML metadata
// feature's base is metaobjects, and the AST-only ImpliedBaseFQNs gives an
// individual occurrence usage an occurrence, not a Life.
func TestImpliedSpecializationAuditQueriesAgree(t *testing.T) {
	mk, rootk := buildModelWithLibraryNamedKind(t, "q.kerml", source.KindKerML, `package K {
		struct S; metadata typed : S; metadata untyped : Metaobjects::Metaobject; }`)
	for _, name := range []string{"typed", "untyped"} {
		sym := auditLibraryBases(t, rootk, "K::"+name)
		if fqn, ok := mk.FeatureBaseFQN(sym); !ok || fqn != "Metaobjects::metaobjects" {
			t.Errorf("FeatureBaseFQN(%s) = %q, %v, want Metaobjects::metaobjects", name, fqn, ok)
		}
		if got := impliedLibraryFQNs(mk, sym); !slices.Contains(got, "Metaobjects::metaobjects") || slices.Contains(got, "Objects::objects") {
			t.Errorf("%s: implied generals %v, want Metaobjects::metaobjects and no Objects::objects", name, got)
		}
	}
	m, root := buildModelWithLibraryNamedKind(t, "q.sysml", source.KindSysML, `package P {
		individual occurrence def IOD; individual occurrence x : IOD; }`)
	sym := auditLibraryBases(t, root, "P::x")
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("P::x is %T, want *ast.Usage", sym.Decl)
	}
	if got := m.ImpliedBaseFQNs(usage, source.KindSysML); !slices.Equal(got, []string{"Occurrences::occurrences"}) {
		t.Errorf("ImpliedBaseFQNs(individual occurrence x) = %v, want [Occurrences::occurrences]", got)
	}
	def := auditLibraryBases(t, root, "P::IOD")
	if got := m.ImpliedBaseFQNs(def.Decl, source.KindSysML); !slices.Equal(got, []string{"Occurrences::Life"}) {
		t.Errorf("ImpliedBaseFQNs(individual occurrence def) = %v, want [Occurrences::Life]", got)
	}
}

// A flow whose ends are owned `end` members has end features like `from a to b`
// does (checkFlowUsageFlowSpecialization): it is a flow, not only a message.
func TestImpliedSpecializationAuditFlowWithOwnedEnds(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "fe.sysml", source.KindSysML, `package P {
		part def A { port p; } part a : A; part b : A;
		flow f : Transfers::MessageTransfer { end :>> source ::> a.p; end :>> target ::> b.p; }
		flow g : Transfers::MessageTransfer; }`)
	if got := impliedLibraryFQNs(m, auditLibraryBases(t, root, "P::f")); !slices.Contains(got, "Flows::flows") {
		t.Errorf("flow with owned ends: implied %v, want Flows::flows", got)
	}
	if got := impliedLibraryFQNs(m, auditLibraryBases(t, root, "P::g")); slices.Contains(got, "Flows::flows") || !slices.Contains(got, "Flows::messages") {
		t.Errorf("flow without ends: implied %v, want Flows::messages and no Flows::flows", got)
	}
}
