package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

const crossFeatures = `package Crossing {
    part def A;
    part def B;
    part def Sub1 specializes A;
    metadata def M;
    connection def C {
        end [0..*] item x : A;
        end [0..1] item cart : A[1];
        end x1[0..1] typed by Sub1 item y : B;
        end [2] #M item z : B;
        end ref plain : A[3];
    }
    part a : A;
    part b : B[2];
    connection c : C connect [1] a to [0..*] b;
}
`

// The `[m]` an end writes ahead of its kind keyword is its cross feature's, so
// the graph carries it on a feature the end owns, never as the end's own bounds.
func TestCrossFeatureIsAFeatureTheEndOwns(t *testing.T) {
	g := turtleOf(t, "crossing", crossFeatures)
	elmt := func(name string) string { return rdf.ElementIRI(name).Value }
	end := elmt("Crossing::C::x")
	cross := elmt("Crossing::C::x::@0")
	if g.HasProperty(iri(end), rdf.SysML+"upperBound") || g.HasProperty(iri(end), rdf.SysML+"lowerBound") {
		t.Errorf("<%s> states the cross feature's bounds as its own", end)
	}
	wantType(t, g, cross, "ReferenceUsage")
	if !g.HasProperty(iri(cross), rdf.SysML+"upperBound") {
		t.Errorf("<%s> states no upper bound", cross)
	}
	if got, ok := g.Object(iri(cross), rdf.SysML+"owner"); !ok || got.Value != end {
		t.Errorf("<%s> is owned by %v, want <%s>", cross, got, end)
	}
	membership := rdf.ElementIRIForID(rdf.OwningMembershipID("Crossing::C::x::@0")).Value
	wantType(t, g, membership, "OwningMembership")
	if g.HasProperty(iri(end), rdf.SysML+"ownedFeatureMembership") {
		t.Errorf("<%s> owns its cross feature through a FeatureMembership", end)
	}

	// An end with a multiplicity of its own keeps it; the crossing one is apart.
	cart := elmt("Crossing::C::cart")
	wantLexical(t, g, rdf.ExpressionIRI(iri(cart), "upperBound").Value, rdf.OpenSysML+"sourceText", "1")
	if g.HasProperty(iri(cart), rdf.SysML+"lowerBound") {
		t.Errorf("<%s> took the cross feature's lower bound", cart)
	}
	cartCross := elmt("Crossing::C::cart::@0")
	wantLexical(t, g, rdf.ExpressionIRI(iri(cartCross), "lowerBound").Value, rdf.OpenSysML+"sourceText", "0")

	// A named cross feature links its type as any feature does.
	x1 := elmt("Crossing::C::y::x1")
	wantLexical(t, g, x1, rdf.SysML+"declaredName", "x1")
	if got, ok := g.Object(iri(x1), rdf.SysML+"type"); !ok || got.Value != elmt("Crossing::Sub1") {
		t.Errorf("<%s> is typed by %v, want Sub1", x1, got)
	}
	if types := g.Objects(iri(elmt("Crossing::C::y")), rdf.SysML+"type"); len(types) != 1 || types[0].Value != elmt("Crossing::B") {
		t.Errorf("the end y is typed by %v, want B alone", types)
	}

	// A connector's positional multiplicities stay on its ends.
	c := elmt("Crossing::c")
	if g.HasProperty(iri(c), rdf.SysML+"upperBound") {
		t.Errorf("<%s> took an end's multiplicity as its own", c)
	}
	wantLexical(t, g, rdf.ExpressionIRI(rdf.ExpressionIRI(iri(c), "end1"), "upperBound").Value, rdf.OpenSysML+"sourceText", "*")
}

// Without the source text, the structural triples alone write every cross
// feature back where it was declared, in both notations.
func TestCrossFeaturesComeBackFromTheGraphAlone(t *testing.T) {
	back := toNotation(t, withoutTriples(t, idTurtle(t, crossFeatures), "sysx:sourceText"))
	if back != crossFeatures {
		t.Errorf("cross features were not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", crossFeatures, back)
	}

	kerml := `package Crossing {
    class A;
    class B;
    class Sub1 subsets A;
    assoc C {
        end [0..*] feature x : A;
        end [0..1] feature cart : A[1];
        end x1[0..1] typed by Sub1 feature y : B;
        end feature plain : A[3];
    }
}
`
	turtle, err := export.Convert("m.kerml", []byte(kerml), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	wantType(t, g, rdf.ElementIRI("Crossing::C::x::@0").Value, "Feature")
	out, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if string(out) != kerml {
		t.Errorf("KerML cross features were not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", kerml, out)
	}
}

const crossFeatureWithID = `package Crossing {
    @IdentityMetadata::ProjectRef { projectId = "proj-1"; }
    part def A;
    part def B;
    connection def C {
        end x1[0..1] typed by A item x : B {
            metadata : IdentityMetadata::ElementId about x1 { id = "stable-x1"; }
        }
        end plain : A[3];
    }
}
`

// A cross feature's declared id is written as an `about` annotation in the body
// of the end that owns it, the head having no place for one, from the graph alone.
func TestCrossFeatureIDComesBackFromTheGraphAlone(t *testing.T) {
	turtle := idTurtle(t, crossFeatureWithID)
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	wantType(t, g, "urn:sysmlv2:element:stable-x1", "ReferenceUsage")
	if back := toNotation(t, withoutSourceText(t, turtle)); back != crossFeatureWithID {
		t.Errorf("the cross feature's id was not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", crossFeatureWithID, back)
	}
}

// A cross feature declaring only a short name is annotated `about` that short
// name, which the end's body looks up as it does a name.
func TestShortNamedCrossFeatureIDComesBackFromTheGraphAlone(t *testing.T) {
	const src = `package Crossing {
    part def A;
    part def B;
    connection def C {
        end <x1>[0..1] typed by A item x : B {
            metadata : IdentityMetadata::ElementId about x1 { id = "stable-x1"; }
        }
    }
}
`
	turtle := idTurtle(t, src)
	if !strings.Contains(string(turtle), "sysml:declaredShortName \"x1\"") || strings.Contains(string(turtle), "sysml:declaredName \"x1\"") {
		t.Fatalf("the cross feature is not declared by short name alone:\n%s", turtle)
	}
	if back := toNotation(t, withoutSourceText(t, turtle)); back != src {
		t.Errorf("the short-named cross feature's id was not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", src, back)
	}
}

// A cross feature the graph gives a body has no notation: the head of its end
// holds no body, so the conversion is refused rather than dropping it.
func TestCrossFeatureWithABodyIsRefused(t *testing.T) {
	stripped := string(withoutSourceText(t, idTurtle(t, crossFeatureWithID)))
	const head = "    sysml:declaredName \"x1\" ;\n"
	if !strings.Contains(stripped, head) {
		t.Fatalf("the graph does not declare the cross feature:\n%s", stripped)
	}
	withBody := strings.Replace(stripped, head, head+"    sysx:hasBody \"true\"^^xsd:boolean ;\n", 1)
	_, err := export.Convert("m.ttl", []byte(withBody), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error is %T, want *export.UnsupportedError: %v", err, err)
	}
	if !strings.Contains(err.Error(), "the cross feature <urn:sysmlv2:element:stable-x1>") ||
		!strings.Contains(err.Error(), "no place for a body") {
		t.Errorf("the refusal does not name the cross feature and what it cannot hold: %v", err)
	}
}

// A cross feature's prefix is stated on the cross feature, never on the end,
// and comes back from the structural triples alone.
func TestCrossFeaturePrefixComesBackFromTheGraphAlone(t *testing.T) {
	sysml := `package Crossing {
    part def A;
    part def B;
    connection def C {
        end in x1[1] typed by A item x : A;
        end derived abstract ref x2[1] item y : B;
        end constant x3[1] typed by B item z : B;
        end ref plain : A[3];
    }
}
`
	g := turtleOf(t, "crossing", sysml)
	elmt := func(name string) string { return rdf.ElementIRI(name).Value }
	wantLexical(t, g, elmt("Crossing::C::x::x1"), rdf.SysML+"direction", "in")
	if g.HasProperty(iri(elmt("Crossing::C::x")), rdf.SysML+"direction") {
		t.Errorf("the end x took its cross feature's direction")
	}
	for _, flag := range []struct{ end, cross, property string }{
		{"Crossing::C::y", "Crossing::C::y::x2", "isDerived"},
		{"Crossing::C::y", "Crossing::C::y::x2", "isAbstract"},
		{"Crossing::C::y", "Crossing::C::y::x2", "isReference"},
		{"Crossing::C::z", "Crossing::C::z::x3", "isConstant"},
	} {
		wantLexical(t, g, elmt(flag.cross), rdf.SysML+flag.property, "true")
		if g.HasProperty(iri(elmt(flag.end)), rdf.SysML+flag.property) {
			t.Errorf("the end <%s> took its cross feature's %s", flag.end, flag.property)
		}
	}
	if back := toNotation(t, withoutTriples(t, idTurtle(t, sysml), "sysx:sourceText")); back != sysml {
		t.Errorf("cross feature prefixes were not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", sysml, back)
	}

	kerml := `package Crossing {
    class A;
    class B {
        portion feature q : A;
    }
    assoc C {
        end var x1[1] typed by A feature x : A;
        end out derived composite x2[1] feature y : B;
        end var [1] feature z : B;
        end portion x4[1] feature w : B;
    }
}
`
	turtle, err := export.Convert("m.kerml", []byte(kerml), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	g, err = rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	for _, portion := range []string{"Crossing::B::q", "Crossing::C::w::x4"} {
		wantLexical(t, g, elmt(portion), rdf.SysML+"isPortion", "true")
		wantLexical(t, g, elmt(portion), rdf.SysML+"isComposite", "true")
	}
	if g.HasProperty(iri(elmt("Crossing::C::y::x2")), rdf.SysML+"isPortion") {
		t.Errorf("the composite cross feature x2 is no portion")
	}
	out, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if string(out) != kerml {
		t.Errorf("KerML cross feature prefixes were not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", kerml, out)
	}
}

// `ordered` and `nonunique` after a cross feature's multiplicity are stated on
// the cross feature, never on the end, and come back from the graph alone.
func TestCrossFeatureOrderingComesBackFromTheGraphAlone(t *testing.T) {
	sysml := `package Crossing {
    part def A;
    part def B;
    item g : A;
    connection def C {
        end x1[1..*] ordered item x : A;
        end [0..*] ordered nonunique subsets g item y : A;
        end nonunique item z : B;
        end [2] nonunique ref w : B;
    }
}
`
	g := turtleOf(t, "crossing", sysml)
	elmt := func(name string) string { return rdf.ElementIRI(name).Value }
	for _, flag := range []struct{ end, cross, property string }{
		{"Crossing::C::x", "Crossing::C::x::x1", "isOrdered"},
		{"Crossing::C::y", "Crossing::C::y::@0", "isOrdered"},
		{"Crossing::C::y", "Crossing::C::y::@0", "isNonunique"},
		{"Crossing::C::z", "Crossing::C::z::@0", "isNonunique"},
		{"Crossing::C::w", "Crossing::C::w::@0", "isNonunique"},
	} {
		wantLexical(t, g, elmt(flag.cross), rdf.SysML+flag.property, "true")
		if g.HasProperty(iri(elmt(flag.end)), rdf.SysML+flag.property) {
			t.Errorf("the end <%s> took its cross feature's %s", flag.end, flag.property)
		}
	}
	if g.HasProperty(iri(elmt("Crossing::C::x::x1")), rdf.SysML+"isNonunique") {
		t.Errorf("the ordered cross feature x1 is not nonunique")
	}
	if back := toNotation(t, withoutTriples(t, idTurtle(t, sysml), "sysx:sourceText")); back != sysml {
		t.Errorf("cross feature ordering was not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", sysml, back)
	}

	kerml := `package Crossing {
    class A;
    feature g : A;
    assoc C {
        end x1[1..*] ordered feature x : A;
        end [0..*] ordered nonunique subsets g feature y : A;
    }
}
`
	turtle, err := export.Convert("m.kerml", []byte(kerml), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	out, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if string(out) != kerml {
		t.Errorf("KerML cross feature ordering was not rebuilt from the graph:\n--- want ---\n%s--- got ---\n%s", kerml, out)
	}
}
