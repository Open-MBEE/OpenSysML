package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// enumVariationModel holds an enumeration definition beside an explicit
// variation, so the derived and the declared readings sit in one graph.
const enumVariationModel = `package V {
    enum def Level {
        enum low;
        high;
    }
    variation part def Engine {
        variant part gas : Engine;
        variant part electric : Engine;
    }
    part def Car {
        attribute level : Level;
    }
}
`

// wantObject requires subject to state property with object among its values.
func wantObject(t *testing.T, g *rdf.Graph, subject rdf.Term, property string, object rdf.Term) {
	t.Helper()
	for _, o := range g.Objects(subject, rdf.SysML+property) {
		if o == object {
			return
		}
	}
	t.Errorf("<%s> does not state %s <%s>", subject.Value, property, object.Value)
}

// An enumeration definition is a variation by the metamodel, not by the
// `variation` keyword, so the graph states sysml:isVariation for it as it does
// for a declared variation.
func TestEnumerationDefinitionIsAVariationInRDF(t *testing.T) {
	g := turtleOf(t, "enum-variation", enumVariationModel)
	for _, def := range []string{"V::Level", "V::Engine"} {
		wantLexical(t, g, rdf.ElementIRI(def).Value, rdf.SysML+"isVariation", "true")
	}
	if _, ok := g.Lexical(rdf.ElementIRI("V::Car"), rdf.SysML+"isVariation"); ok {
		t.Errorf("a plain part definition states isVariation")
	}
}

// An enumerated value is a variant of its enumeration definition: owned through
// a VariantMembership, listed among the definition's variants, and flagged
// sysml:isVariant, exactly as a declared `variant` is.
func TestEnumeratedValueIsAVariantInRDF(t *testing.T) {
	g := turtleOf(t, "enum-variant", enumVariationModel)
	for _, tc := range []struct{ owner, member string }{
		{"V::Level", "V::Level::low"},
		{"V::Level", "V::Level::high"},
		{"V::Engine", "V::Engine::gas"},
		{"V::Engine", "V::Engine::electric"},
	} {
		owner, member := rdf.ElementIRI(tc.owner), rdf.ElementIRI(tc.member)
		membership := rdf.OwningMembershipIRI(tc.member)
		wantLexical(t, g, member.Value, rdf.SysML+"isVariant", "true")
		wantType(t, g, membership.Value, "VariantMembership")
		wantObject(t, g, membership, "ownedVariantUsage", member)
		wantObject(t, g, owner, "variant", member)
		wantObject(t, g, owner, "variantMembership", membership)
	}
	level := rdf.ElementIRI("V::Car::level")
	if _, ok := g.Lexical(level, rdf.SysML+"isVariant"); ok {
		t.Errorf("an attribute typed by an enumeration states isVariant")
	}
	wantType(t, g, rdf.OwningMembershipIRI("V::Car::level").Value, "FeatureMembership")
}

// Reading the graph back writes neither `variation` before an `enum def` nor
// `variant` before an enumerated value — the enumeration grammar has no such
// keywords — while a declared variation and its variants keep theirs; the
// structure alone, source text stripped, carries the same notation.
func TestEnumerationVariationComesBackFromTheGraphAlone(t *testing.T) {
	checkRoundTrip(t, enumVariationModel)
	turtle, err := export.Convert("m.sysml", []byte(enumVariationModel), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back := structuralRoundTrip(t, "enum-variation", turtle)
	for _, want := range []string{
		"enum def Level {",
		"enum low;",
		"\n        high;",
		"variation part def Engine {",
		"variant part gas : Engine;",
		"variant part electric : Engine;",
		"attribute level : Level;",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the graph alone did not write %q:\n%s", want, back)
		}
	}
	for _, forbidden := range []string{"variation enum def", "variant enum"} {
		if strings.Contains(string(back), forbidden) {
			t.Errorf("the graph alone wrote %q:\n%s", forbidden, back)
		}
	}
}

// A graph from another tool may flag an enumerated value sysml:isVariant under
// a plain OwningMembership; the flag is still not written back as `variant`.
func TestForeignEnumeratedValueFlagWritesNoKeyword(t *testing.T) {
	turtle, err := export.Convert("m.sysml", []byte(enumVariationModel), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	foreign := strings.ReplaceAll(string(withoutSourceText(t, turtle)), "sysml:VariantMembership", "sysml:OwningMembership")
	foreign = string(withoutTriples(t, []byte(foreign), "sysml:ownedVariantUsage"))
	back, err := export.Convert("m.ttl", []byte(foreign), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{"enum low;", "\n        high;", "variant part gas : Engine;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("did not write %q:\n%s", want, back)
		}
	}
	if strings.Contains(string(back), "variant enum") || strings.Contains(string(back), "variant high") {
		t.Errorf("an enumerated value came back as a `variant`:\n%s", back)
	}
}

// An enumeration definition that specializes, nests inside a package or another
// definition, or carries metadata is still read back as an enumeration.
func TestNestedEnumerationVariationRoundTrips(t *testing.T) {
	checkRoundTrip(t, `package N {
    metadata def Tag;
    part def Box {
        enum def Size {
            #Tag small;
            large;
        }
        attribute size : Size = Size::small;
    }
    enum def Wide :> Box::Size;
}
`)
}
