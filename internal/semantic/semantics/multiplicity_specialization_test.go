package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// libraryFeature is the one library element the stand-in index knows under fqn.
func libraryFeature(t *testing.T, model *Model, fqn string) *symbols.Symbol {
	t.Helper()
	found := model.resolver.Index().LookupQualified(fqn)
	if len(found) != 1 || found[0] == nil {
		t.Fatalf("LookupQualified(%s) = %d symbols, want one", fqn, len(found))
	}
	return found[0]
}

// TestMultiplicitySpecializesNaturals covers KerML 1.1 §8.3.3.1.9
// checkMultiplicitySpecialization: a usage's written range and its implied [1..1]
// (a feature of a type's, SysML v2 §7.6.3) each subset Base::naturals, which types
// them ScalarValues::Natural.
func TestMultiplicitySpecializesNaturals(t *testing.T) {
	src := `package MS {
    part def P;
    part def Car {
        part p : P [2];
        part q : P;
        part r : P [0..*];
    }
}`
	for _, fixture := range reflectiveFixtures(t, "multiplicity-naturals.sysml", source.KindSysML, src) {
		model := fixture.model
		naturals := libraryFeature(t, model, "Base::naturals")
		natural := libraryFeature(t, model, "ScalarValues::Natural")
		for _, name := range []string{"p", "q", "r"} {
			usage := relFixtureSymbol(t, fixture, name)
			member := model.MultiplicityMember(usage)
			if member == nil {
				t.Fatalf("MultiplicityMember(%s) = nil", name)
			}
			if fqn, ok := model.MultiplicityBaseFQN(member); !ok || fqn != "Base::naturals" {
				t.Errorf("MultiplicityBaseFQN(%s multiplicity) = %q, %t; want Base::naturals", name, fqn, ok)
			}
			if supers := model.DirectSupertypes(member); len(supers) != 1 || supers[0] != naturals {
				t.Errorf("DirectSupertypes(%s multiplicity) = %v, want [Base::naturals]", name, supers)
			}
			if generals := model.ImplicitGenerals(member); len(generals) != 1 || generals[0] != naturals {
				t.Errorf("ImplicitGenerals(%s multiplicity) = %v, want [Base::naturals]", name, generals)
			}
			if types := model.FeatureTypeSet(member); len(types) != 1 || types[0] != natural {
				t.Errorf("FeatureTypeSet(%s multiplicity) = %v, want [ScalarValues::Natural]", name, types)
			}
			if types, _ := model.ReflectiveElements(member, "type"); len(types) != 1 || types[0] != natural {
				t.Errorf("%s multiplicity type = %v, want [ScalarValues::Natural]", name, types)
			}
		}
		// The usages themselves keep their own bases: naturals is their multiplicity's.
		for _, super := range model.DirectSupertypes(relFixtureSymbol(t, fixture, "q")) {
			if super == naturals {
				t.Error("q itself specializes Base::naturals")
			}
		}
	}
}

// TestImpliedBaseFQNsForMultiplicityMetaclasses covers the metaclass-keyed reading
// a graph element without a symbol gets: Base::naturals for either multiplicity metaclass.
func TestImpliedBaseFQNsForMultiplicityMetaclasses(t *testing.T) {
	model := NewModel(nil)
	for _, metaclass := range []string{"Multiplicity", "MultiplicityRange"} {
		for _, kind := range []source.Kind{source.KindKerML, source.KindSysML} {
			if got := model.ImpliedBaseFQNsForMetaclass(metaclass, kind); len(got) != 1 || got[0] != "Base::naturals" {
				t.Errorf("ImpliedBaseFQNsForMetaclass(%s, %v) = %v, want [Base::naturals]", metaclass, kind, got)
			}
		}
	}
}

// TestKerMLMultiplicityMemberSpecializesNaturals covers the KerML `multiplicity`
// member (KerML 1.1 §8.4.4.12.2): a range subsets Base::naturals, a member that
// declares a subsetting reaching it does not take the implied one too, and the
// library's own ranges specialize it.
func TestKerMLMultiplicityMemberSpecializesNaturals(t *testing.T) {
	src := `package MK {
    classifier C {
        multiplicity m [2];
        multiplicity n subsets Base::exactlyOne;
        multiplicity o subsets m;
        multiplicity p subsets q;
        multiplicity q subsets p;
        multiplicity r subsets r;
    }
}`
	for _, fixture := range reflectiveFixtures(t, "multiplicity-member.kerml", source.KindKerML, src) {
		model := fixture.model
		naturals := libraryFeature(t, model, "Base::naturals")
		natural := libraryFeature(t, model, "ScalarValues::Natural")
		exactlyOne := libraryFeature(t, model, "Base::exactlyOne")
		m := relFixtureSymbol(t, fixture, "m")
		if supers := model.DirectSupertypes(m); len(supers) != 1 || supers[0] != naturals {
			t.Errorf("DirectSupertypes(m) = %v, want [Base::naturals]", supers)
		}
		// A declared `subsets` is the member's general; reaching naturals through it, the
		// member takes no implied one and still inherits the type (KerML 1.1 §8.3.3.3.1).
		for name, general := range map[string]*symbols.Symbol{"n": exactlyOne, "o": m} {
			sym := relFixtureSymbol(t, fixture, name)
			if generals := model.ImplicitGenerals(sym); len(generals) != 0 {
				t.Errorf("ImplicitGenerals(%s) = %v, want none: its declared subsetting reaches Base::naturals", name, generals)
			}
			if supers := model.DirectSupertypes(sym); len(supers) != 1 || supers[0] != general {
				t.Errorf("DirectSupertypes(%s) = %v, want [%s]", name, supers, general.Name)
			}
			if declared := model.DeclaredSupertypes(sym); len(declared) != 1 || declared[0] != general {
				t.Errorf("DeclaredSupertypes(%s) = %v, want [%s]", name, declared, general.Name)
			}
			if !model.Conforms(sym, naturals) {
				t.Errorf("%s does not conform to Base::naturals through its declared subsetting", name)
			}
			if types := model.FeatureTypeSet(sym); len(types) != 1 || types[0] != natural {
				t.Errorf("FeatureTypeSet(%s) = %v, want [ScalarValues::Natural]", name, types)
			}
		}
		// A cycle reaches no base, so each member on it keeps the implied one beside
		// the general it declares (a recorded member answers from its recorded supers).
		for _, name := range []string{"p", "q", "r"} {
			sym := relFixtureSymbol(t, fixture, name)
			if supers := model.DirectSupertypes(sym); !slices.Contains(supers, naturals) {
				t.Errorf("DirectSupertypes(%s) = %v, want Base::naturals among them: its declared chain is a cycle", name, supers)
			}
			if !model.Conforms(sym, naturals) {
				t.Errorf("%s does not conform to Base::naturals", name)
			}
		}
		zeroOrOne := libraryFeature(t, model, "Base::zeroOrOne")
		if supers := model.DirectSupertypes(zeroOrOne); len(supers) != 1 || supers[0] != naturals {
			t.Errorf("DirectSupertypes(Base::zeroOrOne) = %v, want [Base::naturals]", supers)
		}
		if generals := model.ImplicitGenerals(naturals); len(generals) != 0 {
			for _, general := range generals {
				if general == naturals {
					t.Error("Base::naturals specializes itself")
				}
			}
		}
	}
}

// TestIndividualOccurrenceDefinitionMultiplicity covers SysML v2 §8.3.9.3
// checkOccurrenceDefinitionMultiplicitySpecialization: an individual occurrence
// definition is parsed with a Multiplicity, its first owned member, subsetting
// Base::zeroOrOne; a definition that is not individual has none.
func TestIndividualOccurrenceDefinitionMultiplicity(t *testing.T) {
	src := `package IO {
    part def P;
    individual part def I { part x : P; attribute a; }
    individual occurrence def O;
    individual item def J :> P;
    part def Q { part y : P; }
    occurrence def R;
    attribute def A;
    metadata def Tag;
    individual #Tag part def T;
}`
	for _, fixture := range reflectiveFixtures(t, "individual-multiplicity.sysml", source.KindSysML, src) {
		model := fixture.model
		zeroOrOne := libraryFeature(t, model, "Base::zeroOrOne")
		natural := libraryFeature(t, model, "ScalarValues::Natural")
		for _, name := range []string{"I", "O", "J"} {
			def := relFixtureSymbol(t, fixture, name)
			member := model.DefaultMultiplicityMember(def)
			if member == nil {
				t.Fatalf("DefaultMultiplicityMember(%s) = nil, want the Multiplicity it is parsed with", name)
			}
			if again := model.MultiplicityMember(def); again != member {
				t.Errorf("MultiplicityMember(%s) is not its individual multiplicity", name)
			}
			if !IsDefaultMultiplicity(member) || !IsImpliedMultiplicity(member) || !member.Implied() {
				t.Errorf("%s's member is not recognised as an individual multiplicity", name)
			}
			if got := model.fqnOf(model.MetaclassOf(member)); got != "KerML::Core::Multiplicity" {
				t.Errorf("%s multiplicity metaclass = %s, want KerML::Core::Multiplicity", name, got)
			}
			if fqn, ok := model.MultiplicityBaseFQN(member); !ok || fqn != "Base::zeroOrOne" {
				t.Errorf("MultiplicityBaseFQN(%s multiplicity) = %q, %t; want Base::zeroOrOne", name, fqn, ok)
			}
			if supers := model.DirectSupertypes(member); len(supers) != 1 || supers[0] != zeroOrOne {
				t.Errorf("DirectSupertypes(%s multiplicity) = %v, want [Base::zeroOrOne] alone: naturals is reached through it", name, supers)
			}
			if types := model.FeatureTypeSet(member); len(types) != 1 || types[0] != natural {
				t.Errorf("FeatureTypeSet(%s multiplicity) = %v, want [ScalarValues::Natural]", name, types)
			}
			assertRelFeature(t, model, def, "multiplicity", member)
			assertRelFeature(t, model, member, "owner", def)
			if members, _ := model.ReflectiveElements(def, "ownedMember"); len(members) == 0 || members[0] != member {
				t.Errorf("%s ownedMember = %d elements, want the individual multiplicity first", name, len(members))
			}
			if elements, _ := model.ReflectiveElements(def, "ownedElement"); len(elements) == 0 || elements[0] != member {
				t.Errorf("%s ownedElement = %d elements, want the individual multiplicity first", name, len(elements))
			}
			rels := model.OwnedRelationships(def)
			if len(rels) == 0 || rels[0].Implicit == nil || rels[0].Implicit.Member != member {
				t.Fatalf("the first owned relationship of %s is not the individual multiplicity's membership", name)
			}
			if got := relMetaName(model, rels[0]); got != "OwningMembership" {
				t.Errorf("%s membership metaclass = %s, want OwningMembership", name, got)
			}
			for i, membership := range model.OwnedMemberships(def) {
				if membership.Implicit != nil && membership.Implicit.Ordinal != i {
					t.Errorf("%s owned membership %d has ordinal %d", name, i, membership.Implicit.Ordinal)
				}
			}
			if got := model.OwningMembership(member); got != rels[0] {
				t.Errorf("OwningMembership(%s multiplicity) is not the membership %s owns it through", name, name)
			}
		}
		for _, name := range []string{"P", "Q", "R", "A"} {
			def := relFixtureSymbol(t, fixture, name)
			if model.MultiplicityMember(def) != nil {
				t.Errorf("%s owns a multiplicity, want none: it is not individual", name)
			}
			if got, _ := model.ReflectiveElements(def, "multiplicity"); len(got) != 0 {
				t.Errorf("%s multiplicity = %v, want none", name, got)
			}
		}
		x := relFixtureSymbol(t, fixture, "x")
		if member := model.MultiplicityMember(x); member == nil || !IsDefaultMultiplicity(member) {
			t.Error("x, a usage of an individual definition, does not keep its own implicit [1..1]")
		}
		tagged := relFixtureSymbol(t, fixture, "T")
		if rels := model.OwnedRelationships(tagged); len(rels) != 2 || rels[0].Implicit == nil ||
			!IsDefaultMultiplicity(rels[0].Implicit.Member) || !isPrefixAnnotation(rels[1]) {
			t.Errorf("T ownedRelationship = %d elements, want the Multiplicity's membership before the prefix annotation (SysML v2 §8.2.2.9.1)", len(rels))
		}
	}
}

// The clause reads the definition's `multiplicity`, whichever construction minted it: a
// Multiplicity owned by an individual definition as a usage's implicit [1..1] is (the form
// a parser post-processor states it in) subsets Base::zeroOrOne alike, any other Base::naturals.
func TestMultiplicityBaseFollowsTheOwner(t *testing.T) {
	src := `package IOF {
    part def P;
    individual part def I { multiplicity m [2]; part x : P; }
    part a : P; part b : P;
    connection c connect e1 ::> a to e2 ::> b;
}`
	for _, fixture := range reflectiveFixtures(t, "individual-owner-multiplicity.sysml", source.KindSysML, src) {
		model := fixture.model
		individual := relFixtureSymbol(t, fixture, "I")
		asDefault := impliedMultiplicity(individual, individual.DeclSpan, symbols.ImpliedDefaultMultiplicity)
		if fqn, ok := model.MultiplicityBaseFQN(asDefault); !ok || fqn != "Base::zeroOrOne" {
			t.Errorf("MultiplicityBaseFQN(I's default-form multiplicity) = %q, %t; want Base::zeroOrOne", fqn, ok)
		}
		for _, name := range []string{"m", "x"} {
			sym := relFixtureSymbol(t, fixture, name)
			if sym.Kind != symbols.SymbolMultiplicity {
				sym = model.MultiplicityMember(sym)
			}
			if fqn, ok := model.MultiplicityBaseFQN(sym); !ok || fqn != "Base::naturals" {
				t.Errorf("MultiplicityBaseFQN(%s, in I's body) = %q, %t; want Base::naturals: it is not I's multiplicity", name, fqn, ok)
			}
		}
		plain := relFixtureSymbol(t, fixture, "P")
		if fqn, ok := model.MultiplicityBaseFQN(impliedMultiplicity(plain, plain.DeclSpan, symbols.ImpliedDefaultMultiplicity)); !ok || fqn != "Base::naturals" {
			t.Errorf("MultiplicityBaseFQN(P's default-form multiplicity) = %q, %t; want Base::naturals", fqn, ok)
		}
		ends, ok := model.OwnedConnectorEnds(relFixtureSymbol(t, fixture, "c"))
		if !ok || len(ends) != 2 {
			t.Fatalf("c ends = %d, want 2", len(ends))
		}
		for i, end := range ends {
			if fqn, ok := model.MultiplicityBaseFQN(impliedMultiplicity(end, end.DeclSpan, symbols.ImpliedDefaultMultiplicity)); !ok || fqn != "Base::naturals" {
				t.Errorf("MultiplicityBaseFQN(end %d's default multiplicity) = %q, %t; want Base::naturals", i, fqn, ok)
			}
		}
	}
}
