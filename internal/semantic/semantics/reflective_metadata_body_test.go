package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A metadata body's `mission = "Hubble"` restates the metadata type's feature
// mission: a nameless ReferenceUsage whose implied Redefinition names it
// (SysML.xtext MetadataBodyUsage), as the pilot exports it.
func TestMetadataBodyFeatureRedefinesByNameWithoutDeclaringOne(t *testing.T) {
	const src = `package P {
		part def T; part telescope : T;
		metadata def Heritage { attribute mission : String; attribute year : Integer; }
		@Heritage about telescope { mission = "Hubble"; year = 1990; }
	}`
	for _, fixture := range reflectiveFixtures(t, "metadata-body.sysml", source.KindSysML, src) {
		model := fixture.model
		if nestedSym(t, fixture.root, "P::telescope").Recorded() {
			// A recorded index keeps no body for an unnamed metadata usage; the
			// recorded leg has nothing to answer here.
			continue
		}
		var body []*symbols.Symbol
		for _, member := range model.Members(nestedSym(t, fixture.root, "P")) {
			if member.Symbol.Kind == symbols.SymbolMetadataUsage {
				body = model.membersIncludingAnonymous(member.Symbol)
			}
		}
		if len(body) != 2 {
			t.Fatalf("metadata body members = %d, want mission and year", len(body))
		}
		mission, year := body[0], body[1]
		if got, _ := model.ReflectiveFeatureValues(mission, "declaredName"); len(got) != 0 {
			t.Errorf("mission declaredName = %v, want none: the name is the redefined feature's", got)
		}
		if got, _ := model.ReflectiveFeatureValues(mission, "name"); len(got) != 1 || got[0].Str != "mission" {
			t.Errorf("mission name = %v, want mission, through the Redefinition", got)
		}
		if got, _ := model.ReflectiveFeatureValues(year, "declaredName"); len(got) != 0 {
			t.Errorf("year declaredName = %v, want none", got)
		}
		for _, sym := range body {
			rels, _ := model.ReflectiveElements(sym, "ownedRelationship")
			var kinds []string
			for _, rel := range rels {
				kinds = append(kinds, relMetaName(model, rel))
			}
			if !slices.Equal(kinds, []string{"Redefinition", "FeatureValue"}) {
				t.Errorf("%s ownedRelationship = %v, want [Redefinition FeatureValue]", sym.Name, kinds)
				continue
			}
			redefined, _ := model.ReflectiveElements(rels[0], "redefinedFeature")
			if len(redefined) != 1 || redefined[0] != nestedSym(t, fixture.root, "P::Heritage::"+sym.Name) {
				t.Errorf("%s Redefinition redefinedFeature = %v, want Heritage::%s", sym.Name, redefined, sym.Name)
			}
			redefining, _ := model.ReflectiveElements(rels[0], "redefiningFeature")
			if len(redefining) != 1 || redefining[0] != sym {
				t.Errorf("%s Redefinition redefiningFeature = %v, want the body feature", sym.Name, redefining)
			}
		}
	}
}

// A restatement by short name (`m = …` for `attribute <m> mission`) takes both
// identifiers from the feature it redefines, not the token it was written with.
func TestMetadataBodyRestatementByShortNameIsNamedByItsTarget(t *testing.T) {
	const src = `package P {
		part def T; part telescope : T;
		metadata def Heritage { attribute <m> mission : String; }
		@Heritage about telescope { m = "Hubble"; }
	}`
	for _, fixture := range reflectiveFixtures(t, "metadata-body-short.sysml", source.KindSysML, src) {
		model := fixture.model
		if nestedSym(t, fixture.root, "P::telescope").Recorded() {
			continue
		}
		var body []*symbols.Symbol
		for _, member := range model.Members(nestedSym(t, fixture.root, "P")) {
			if member.Symbol.Kind == symbols.SymbolMetadataUsage {
				body = model.membersIncludingAnonymous(member.Symbol)
			}
		}
		if len(body) != 1 {
			t.Fatalf("metadata body members = %d, want the restatement", len(body))
		}
		if got, _ := model.ReflectiveFeatureValues(body[0], "declaredName"); len(got) != 0 {
			t.Errorf("declaredName = %v, want none", got)
		}
		if got, _ := model.ReflectiveFeatureValues(body[0], "name"); len(got) != 1 || got[0].Str != "mission" {
			t.Errorf("name = %v, want mission, the redefined feature's", got)
		}
		if got, _ := model.ReflectiveFeatureValues(body[0], "shortName"); len(got) != 1 || got[0].Str != "m" {
			t.Errorf("shortName = %v, want m, the redefined feature's", got)
		}
	}
}

// The predicate answers for an absent symbol rather than dereferencing it.
func TestRestatesMetadataBodyFeatureOfNoSymbolIsFalse(t *testing.T) {
	fixture := reflectiveFixtures(t, "metadata-body-nil.sysml", source.KindSysML, `package P;`)[0]
	if fixture.model.RestatesMetadataBodyFeature(nil) {
		t.Fatal("nil symbol restates a metadata body feature")
	}
}
