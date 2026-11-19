package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestMetaclassOfTheProductionWritten: a declaration is reflected as the metaclass its
// production returns, not the general one its kind keyword names, parsed or recorded.
func TestMetaclassOfTheProductionWritten(t *testing.T) {
	const sysml = `library package Lib {
		package Plain;
		metadata def M;
		state def T;
		use case def U;
		part def Ctx {
			exhibit state exhibited : T;
			state plainState : T;
			include use case included : U;
			include use case untyped;
			use case plainUseCase : U;
		}
		#M attribute keyworded;
		ref plainRef;
	}`
	for _, fixture := range reflectiveFixtures(t, "production.sysml", source.KindSysML, sysml) {
		for name, want := range map[string]string{
			"Lib": "LibraryPackage", "Plain": "Package",
			"exhibited": "ExhibitStateUsage", "plainState": "StateUsage",
			"included": "IncludeUseCaseUsage", "untyped": "IncludeUseCaseUsage", "plainUseCase": "UseCaseUsage",
			"keyworded": "AttributeUsage", "plainRef": "ReferenceUsage",
		} {
			if got := relMetaName(fixture.model, relFixtureSymbol(t, fixture, name)); got != want {
				t.Errorf("%s is a %s, want %s", name, got, want)
			}
		}
	}
	const kerml = `library package Lib {
		feature a { feature x; }
		feature b { feature y; }
		succession flow ordered from a.x to b.y;
		flow plain from a.x to b.y;
	}`
	for _, fixture := range reflectiveFixtures(t, "production.kerml", source.KindKerML, kerml) {
		for name, want := range map[string]string{
			"Lib": "LibraryPackage", "ordered": "SuccessionFlow", "plain": "Flow",
		} {
			if got := relMetaName(fixture.model, relFixtureSymbol(t, fixture, name)); got != want {
				t.Errorf("%s is a %s, want %s", name, got, want)
			}
		}
	}
}
