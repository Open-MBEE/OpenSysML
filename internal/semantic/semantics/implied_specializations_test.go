package semantics

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// impliedRowDifferences are the declarations of the fixtures under
// testdata/implied_specializations whose implied Specializations differ from the pilot's, with what is answered instead.
var impliedRowDifferences = map[string]struct {
	generals []string
	reason   string
}{
	"R::OOwner::sn": {[]string{"Occurrences::Occurrence::snapshots"},
		"one feature of the owner is derived per usage: checkOccurrenceUsageSuboccurrenceSpecialization is not added beside the portion's"},
	"R::OOwner::ts": {[]string{"Occurrences::Occurrence::timeSlices"},
		"one feature of the owner is derived per usage: checkOccurrenceUsageSuboccurrenceSpecialization is not added beside the portion's"},
	"R::ReOwner::sr": {[]string{"Views::Rendering::subrenderings"},
		"one feature of the owner is derived per usage: checkPartUsageSubpartSpecialization is not added beside the rendering's"},
	"R::ViOwner::sv": {[]string{"Views::View::subviews"},
		"one feature of the owner is derived per usage: checkPartUsageSubpartSpecialization is not added beside the view's"},
	"R::AOwner::pa": {[]string{"Actions::Action::subactions"},
		"a perform action usage is derived as a composite subaction, where it is referential and an enclosed performance"},
	"R::Owner::npo": {[]string{"Occurrences::Occurrence::suboccurrences", "Ports::ports"},
		"checkOccurrenceUsageSuboccurrenceSpecialization holds of a composite port usage of an item definition; the pilot implies nothing for it"},
	"K::bd": {[]string{"Links::selfLinks"},
		"checkBindingConnectorSpecialization names Links::selfLinks; the pilot implies Links::links for a binding connector with no ends"},
	"K::su": {[]string{"Occurrences::happensBeforeLinks"},
		"checkSuccessionSpecialization names Occurrences::happensBeforeLinks; the pilot implies Links::links for a succession with no ends"},
	"K::I": {[]string{"Links::Link", "Performances::Performance"},
		"an interaction is a behavior too (checkBehaviorSpecialization) and a Link is not a Performance; the pilot implies Links::Link alone"},
	"K::Owner::os": {[]string{"Objects::objects", "Occurrences::Occurrence::suboccurrences"},
		"checkFeatureSuboccurrenceSpecialization holds of a composite feature typed by a structure in a class; the pilot implies nothing for it"},
	"K::Owner::ost": {[]string{"Performances::Performance::enclosedPerformances"},
		"a step of a class is derived as an enclosed performance, where checkStepEnclosedPerformanceSpecialization asks for a behavior or step as owner"},
	"K::BOwner::bex": {[]string{"Performances::evaluations"},
		"one feature of the owner is derived per usage: checkStepEnclosedPerformanceSpecialization is not added beside the expression's base"},
}

// Every named declaration of the fixtures owns the implied Specializations the pilot's including
// export states for it (rows.*.pilot.txt: owner, tab, general), but for the listed differences.
func TestImpliedSpecializationsAgreeWithThePilotPerRow(t *testing.T) {
	declared := map[string]bool{}
	for name, kind := range map[string]source.Kind{"rows.sysml": source.KindSysML, "rows.kerml": source.KindKerML} {
		dir := filepath.Join("testdata", "implied_specializations")
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		oracle, err := os.ReadFile(filepath.Join(dir, name+".pilot.txt"))
		if err != nil {
			t.Fatal(err)
		}
		pilot := map[string][]string{}
		for _, line := range strings.Split(strings.TrimSpace(string(oracle)), "\n") {
			owner, general, ok := strings.Cut(line, "\t")
			if !ok {
				t.Fatalf("%s.pilot.txt: malformed row %q", name, line)
			}
			pilot[owner] = append(pilot[owner], general)
		}
		m, root := buildModelWithLibraryNamedKind(t, name, kind, string(src))
		idx := m.resolver.Index()
		root.WalkDeclarations(func(sym *symbols.Symbol) {
			if sym.Name == "" || sym.Kind == symbols.SymbolPackage {
				return
			}
			owner := idx.GetFQN(sym)
			declared[owner] = true
			var got []string
			for _, general := range m.ImpliedSpecializations(sym) {
				got = append(got, idx.GetFQN(general))
				if !slices.Contains(m.DirectSupertypes(sym), general) && !slices.Contains(m.ImplicitSubsettings(sym), general) {
					t.Errorf("%s: %s is read by neither inheritance nor ImplicitSubsettings", owner, idx.GetFQN(general))
				}
			}
			slices.Sort(got)
			want := slices.Sorted(slices.Values(pilot[owner]))
			diff, differs := impliedRowDifferences[owner]
			switch {
			case !differs && !slices.Equal(got, want):
				t.Errorf("%s: implied generals %v, the pilot's %v", owner, got, want)
			case differs && slices.Equal(got, want):
				t.Errorf("%s: agrees with the pilot now; drop it from impliedRowDifferences", owner)
			case differs && !slices.Equal(got, diff.generals):
				t.Errorf("%s: implied generals %v, want %v (%s)", owner, got, diff.generals, diff.reason)
			}
		})
		for owner := range pilot {
			if !declared[owner] {
				t.Errorf("%s.pilot.txt names %s, which the fixture does not declare", name, owner)
			}
		}
	}
	for owner := range impliedRowDifferences {
		if !declared[owner] {
			t.Errorf("impliedRowDifferences names %s, which no fixture declares", owner)
		}
	}
}

// KerML §8.4.2: an implied Specialization is left out when another one, owned or implied, has
// the same general or a subtype of it, and members of a specialization cycle each keep their base.
func TestImpliedSpecializationsRedundancyRules(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "redundancy.sysml", source.KindSysML, `package P {
		part def A; part def B :> A; part def C :> Parts::Part;
		part def X :> Y; part def Y :> X;
		part top; part typed : A; part sub :> top;
		part def O { part nested; part stated :> Items::Item::subparts; part viaSibling :> nested; part viaChain :> viaSibling; }
		action def Act { action x; action y :> x; }
		individual part def I; individual occurrence def J;
		metadata def tagged :> Metaobjects::SemanticMetadata { :>> baseType = A meta SysML::PartDefinition; }
		#tagged individual part def TI; #tagged part def TN;
	}`)
	idx := m.resolver.Index()
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"P::A", []string{"Parts::Part"}},
		{"P::B", nil},
		{"P::C", nil},
		{"P::X", []string{"Parts::Part"}},
		{"P::Y", []string{"Parts::Part"}},
		{"P::top", []string{"Parts::parts"}},
		{"P::typed", []string{"Parts::parts"}},
		{"P::sub", nil},
		{"P::O::nested", []string{"Items::Item::subparts"}},
		{"P::O::stated", nil},
		{"P::O::viaSibling", nil},
		{"P::O::viaChain", nil},
		{"P::Act::y", nil},
		{"P::TI", []string{"Occurrences::Life"}},
		{"P::TN", nil},
		{"P::I", []string{"Parts::Part", "Occurrences::Life"}},
		{"P::J", []string{"Occurrences::Life"}},
	} {
		sym := auditLibraryBases(t, root, tc.path)
		var got []string
		for _, general := range m.ImpliedSpecializations(sym) {
			got = append(got, idx.GetFQN(general))
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: implied generals %v, want %v", tc.path, got, tc.want)
		}
		// What is left out is still reached, so inheritance loses nothing by the rules.
		for _, general := range m.ImplicitGenerals(sym) {
			if !slices.Contains(m.ImpliedSpecializations(sym), general) && !slices.ContainsFunc(m.DirectSupertypes(sym), func(other *symbols.Symbol) bool {
				return other != general && m.Conforms(other, general)
			}) && !slices.ContainsFunc(m.ImpliedSpecializations(sym), func(other *symbols.Symbol) bool { return m.Conforms(other, general) }) {
				t.Errorf("%s: %s is left out and no other general reaches it", tc.path, idx.GetFQN(general))
			}
		}
	}
}
