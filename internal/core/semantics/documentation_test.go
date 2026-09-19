package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// documentedModel builds a model whose documentation is readable: the source
// lookup hands back the text of the one document the test parsed.
func documentedModel(t *testing.T, src string) (*Model, func(string) []string) {
	t.Helper()
	const name = "t.sysml"
	m, root := buildModelNamed(t, name, src)
	sf := source.New(name, []byte(src))
	m.SetSourceText(func(doc string, span source.Span) string {
		if doc != name {
			t.Fatalf("documentation looked up in %q, want %q", doc, name)
		}
		return sf.Text(span)
	})
	return m, func(key string) []string { return m.DocumentationOf(sym(t, root, key)) }
}

func TestDocumentationOfReadsTheNormalizedBody(t *testing.T) {
	_, docs := documentedModel(t, `
requirement def <'HLR-R001'> CrewSafety {
	doc /* The mission shall safely return
	     * all three crew members to Earth. */
}`)
	want := []string{"The mission shall safely return\nall three crew members to Earth."}
	if got := docs("CrewSafety"); !slices.Equal(got, want) {
		t.Fatalf("DocumentationOf = %q, want %q", got, want)
	}
}

func TestDocumentationOfKeepsEveryBodyInDeclarationOrder(t *testing.T) {
	_, docs := documentedModel(t, `
part def Lander {
	doc Descent /* Descends to the surface. */
	attribute mass : Real;
	doc Ascent /* Ascends back to orbit. */
}`)
	want := []string{"Descends to the surface.", "Ascends back to orbit."}
	if got := docs("Lander"); !slices.Equal(got, want) {
		t.Fatalf("DocumentationOf = %q, want %q", got, want)
	}
}

func TestDocumentationOfIsAbsentWithoutDocOrSourceText(t *testing.T) {
	_, docs := documentedModel(t, `
part def Undocumented { attribute mass : Real; }
part def Commented { comment /* a plain comment is not documentation */ }`)
	if got := docs("Undocumented"); len(got) != 0 {
		t.Fatalf("DocumentationOf(Undocumented) = %q, want none", got)
	}
	if got := docs("Commented"); len(got) != 0 {
		t.Fatalf("DocumentationOf(Commented) = %q, want none: a comment is not a doc", got)
	}

	m, root := buildModel(t, "part def Documented { doc /* text */ }")
	if got := m.DocumentationOf(sym(t, root, "Documented")); len(got) != 0 {
		t.Fatalf("DocumentationOf without source text = %q, want none", got)
	}
}

func TestDocumentationOfFollowsAnAlias(t *testing.T) {
	_, docs := documentedModel(t, `
part def Lander { doc /* Descends to the surface. */ }
alias LM for Lander;`)
	want := []string{"Descends to the surface."}
	if got := docs("LM"); !slices.Equal(got, want) {
		t.Fatalf("DocumentationOf(alias) = %q, want %q", got, want)
	}
}

func TestEffectiveShortNameIsTheDeclaredOne(t *testing.T) {
	m, root := buildModel(t, "part def <LM> Lander; part def Orbiter;")
	if got := m.EffectiveShortNameOf(sym(t, root, "Lander")); got != "LM" {
		t.Fatalf("EffectiveShortNameOf(Lander) = %q, want LM", got)
	}
	if got := m.EffectiveShortNameOf(sym(t, root, "Orbiter")); got != "" {
		t.Fatalf("EffectiveShortNameOf(Orbiter) = %q, want none", got)
	}
}

func TestEffectiveShortNameFollowsTheRedefinedFeature(t *testing.T) {
	m, root := buildModel(t,
		"part def A { part <p1> a; } part def B specializes A { part redefines a; }")
	redefining := sym(t, sym(t, root, "B").Scope, "a")
	if redefining.ShortName != "" {
		t.Fatalf("declared short name = %q, want none", redefining.ShortName)
	}
	if got := m.EffectiveShortNameOf(redefining); got != "p1" {
		t.Fatalf("EffectiveShortNameOf = %q, want p1", got)
	}
}

func TestEffectiveShortNameFollowsTheReferencedFeature(t *testing.T) {
	m, root := buildModel(t, `package P {
		action def Provide;
		action <pp> providePower : Provide;
		part vehicle { perform providePower; }
	}`)
	perform := sym(t, sym(t, sym(t, root, "P").Scope, "vehicle").Scope, "providePower")
	if perform.ShortName != "" {
		t.Fatalf("declared short name = %q, want none", perform.ShortName)
	}
	if got := m.EffectiveShortNameOf(perform); got != "pp" {
		t.Fatalf("EffectiveShortNameOf = %q, want pp", got)
	}
	if got := m.EffectiveNameOf(perform); got != "providePower" {
		t.Fatalf("EffectiveNameOf = %q, want providePower", got)
	}
}

func TestEffectiveShortNameStopsAtADeclaredName(t *testing.T) {
	m, root := buildModel(t, `package P {
		action def Provide;
		action <ss> steer : Provide;
		part def A { part <p1> a; }
		part def B :> A { part b :>> a; }
		part def C :> B { part :>> b; }
		part vehicle { perform action turn references steer; }
	}`)
	p := sym(t, root, "P").Scope
	turn := sym(t, sym(t, p, "vehicle").Scope, "turn")
	if got := m.EffectiveShortNameOf(turn); got != "" {
		t.Fatalf("EffectiveShortNameOf(turn) = %q, want none: turn declares its name", got)
	}
	if got := m.EffectiveNameOf(turn); got != "turn" {
		t.Fatalf("EffectiveNameOf(turn) = %q, want turn", got)
	}
	renamed := sym(t, sym(t, p, "B").Scope, "b")
	if got := m.EffectiveShortNameOf(renamed); got != "" {
		t.Fatalf("EffectiveShortNameOf(B::b) = %q, want none: b declares its name", got)
	}
	unnamed := sym(t, sym(t, p, "C").Scope, "b")
	if unnamed.Naming != symbols.NamedByRedefinition {
		t.Fatal("C's redefinition of b should take the name b")
	}
	if got := m.EffectiveShortNameOf(unnamed); got != "" {
		t.Fatalf("EffectiveShortNameOf(C::b) = %q, want none: B::b has no short name to pass on", got)
	}
}

// The pilot names a plain usage by its first redefinition alone: a `::>`
// reference on it supplies no name (Feature::namingFeature).
func TestEffectiveShortNameOfARedefinitionOutranksItsReference(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A { part <r> redefined; part <s> subsetted; }
		part def B :> A { ref part :>> redefined ::> subsetted; }
	}`)
	b := sym(t, sym(t, root, "P").Scope, "B")
	var feature *symbols.Symbol
	for _, member := range b.Scope.Members() {
		if member.Kind == symbols.SymbolPartUsage {
			feature = member
		}
	}
	if feature == nil {
		t.Fatal("B declares no part usage")
	}
	if got := m.EffectiveShortNameOf(feature); got != "r" {
		t.Fatalf("EffectiveShortNameOf = %q, want r", got)
	}
}
