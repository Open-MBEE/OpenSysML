package export_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
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
	turtle, err := convert.Convert("m.sysml", []byte(enumVariationModel), convert.FormatSysML, convert.FormatTurtle)
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
	turtle, err := convert.Convert("m.sysml", []byte(enumVariationModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	foreign := strings.ReplaceAll(string(withoutSourceText(t, turtle)), "sysml:VariantMembership", "sysml:OwningMembership")
	foreign = string(withoutTriples(t, []byte(foreign), "sysml:ownedVariantUsage"))
	back, err := convert.Convert("m.ttl", []byte(foreign), convert.FormatTurtle, convert.FormatSysML)
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

// variantReferenceModel names existing features as variants, by a lone name, a
// qualified name and a feature chain; B declares a second e1 for the refusal.
const variantReferenceModel = "package P {\n    part def E;\n    part def B {\n        part e1 : E;\n    }\n    part def Q {\n        part k : E;\n    }\n" +
	"    part q : Q;\n    part e1 : E;\n    part e2 : E;\n    variation part def V {\n        variant e1;\n        variant P::e2;\n        variant q.k;\n    }\n}\n"

// A bare `variant x;` is a VariantReference (SysML-textual-bnf :343-345): a
// ReferenceUsage whose owned ReferenceSubsetting references the feature x names
// outside the variation, or the chain of features, declaring no name of its own.
func TestVariantReferenceOwnsAReferenceSubsetting(t *testing.T) {
	if diagnostics := diagnosticMessages("m.sysml", []byte(variantReferenceModel)); len(diagnostics) > 0 {
		t.Fatalf("the model should analyse clean:\n%s", strings.Join(diagnostics, "\n"))
	}
	turtle, err := convert.Convert("m.sysml", []byte(variantReferenceModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for _, want := range []string{
		"elmt:P__V__e1\n    a sysml:ReferenceUsage",
		"sysml:referencedFeature elmt:P__e1",
		"sysml:referencedFeature elmt:P__e2",
		"sysml:chainingFeature elmt:P__q, elmt:P__Q__k",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	for _, block := range strings.Split(graph, "\n\n") {
		if strings.HasPrefix(block, "elmt:P__V_") && !strings.HasPrefix(block, "elmt:P__V__e1_") &&
			(strings.Contains(block, "a sysml:PartUsage") || strings.Contains(block, "sysml:declaredName")) {
			t.Errorf("a variant reference should declare no usage or name of its own:\n%s", block)
		}
	}
	if n := strings.Count(graph, "a sysml:ReferenceSubsetting"); n != 3 {
		t.Errorf("each of the three variants should own a ReferenceSubsetting, found %d:\n%s", n, graph)
	}
	back, err := convert.Convert("m.ttl", withoutSourceText(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v\n%s", err, turtle)
	}
	if string(back) != variantReferenceModel {
		t.Fatalf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", variantReferenceModel, back)
	}

	doc, err := convert.Convert("m.sysml", []byte(variantReferenceModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(doc, &elements); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, e := range elements {
		id, _ := e["@id"].(string)
		byID[id] = e
	}
	ref := func(v any) string {
		id, _ := v.(map[string]any)["@id"].(string)
		return id
	}
	variants := 0
	for _, e := range elements {
		if e["@type"] != "VariantMembership" {
			continue
		}
		variants++
		usage := byID[ref(e["ownedVariantUsage"])]
		if usage["@type"] != "ReferenceUsage" || usage["declaredName"] != nil {
			t.Errorf("%v: a %v declaring %v, want an unnamed ReferenceUsage", usage["@id"], usage["@type"], usage["declaredName"])
		}
		subsetting := byID[ref(usage["ownedReferenceSubsetting"])]
		if subsetting == nil || byID[ref(subsetting["referencedFeature"])] == nil {
			t.Errorf("%v: ownedReferenceSubsetting %v does not reference a feature in the output", usage["@id"], usage["ownedReferenceSubsetting"])
		}
	}
	if variants != 3 {
		t.Errorf("want three variant memberships, found %d", variants)
	}
	if _, err := convert.Convert("m.json", doc, convert.FormatAPIJSON, convert.FormatSysML); err != nil {
		t.Errorf("the API JSON did not read back: %v", err)
	}

	// A variant referencing a feature its name does not reach from the
	// variation is refused rather than written as `variant e1;` naming another.
	blocks := strings.Split(string(withoutSourceText(t, turtle)), "\n\n")
	for i, block := range blocks {
		if strings.HasPrefix(block, "elmt:P__V__e1\n") || strings.HasPrefix(block, "elmt:P__V__e1_rs\n") {
			block = strings.ReplaceAll(block, "elmt:P__e1 ", "elmt:P__B__e1 ")
			blocks[i] = strings.ReplaceAll(block, `\"P__e1\"`, `\"P__B__e1\"`)
		}
	}
	retargeted := strings.Join(blocks, "\n\n")
	if !strings.Contains(retargeted, "sysml:references elmt:P__B__e1 ;") {
		t.Fatalf("the variant was not retargeted:\n%s", retargeted)
	}
	if _, err := convert.Convert("m.ttl", []byte(retargeted), convert.FormatTurtle, convert.FormatSysML); err == nil ||
		!strings.Contains(err.Error(), "`variant e1` does not name P::B::e1") {
		t.Errorf("expected the retargeted variant to be refused, got %v", err)
	}
}

// A `variant x;` is a VariantReference by its syntax: one whose name resolves
// to nothing keeps x as the name it references, the way an unresolved
// reference is kept, rather than becoming a part usage declaring x.
func TestUnresolvedVariantReferenceKeepsItsName(t *testing.T) {
	src := "package P {\n    variation part def V {\n        variant missing;\n    }\n}\n"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	var variant string
	for _, block := range strings.Split(string(turtle), "\n\n") {
		if strings.HasPrefix(block, "elmt:P__V__missing\n") {
			variant = block
		}
	}
	for _, want := range []string{"a sysml:ReferenceUsage", `sysml:references "missing"`} {
		if !strings.Contains(variant, want) {
			t.Errorf("the variant should state %q:\n%s", want, variant)
		}
	}
	if strings.Contains(variant, "sysml:declaredName") {
		t.Errorf("the variant should declare no name:\n%s", variant)
	}
	back, err := convert.Convert("m.ttl", withoutSourceText(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v", err)
	}
	if string(back) != src {
		t.Fatalf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", src, back)
	}
}

// An anonymous variant whose reference the graph keeps as a single name it
// links to nothing is refused: `variant e2;` would be a variant named e2, and
// a single name has no reference form.
func TestAnonymousVariantWithASingleNameLiteralIsRefused(t *testing.T) {
	turtle, err := convert.Convert("m.sysml", []byte(variantReferenceModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	// The `variant P::e2;` member is anonymous; its reference becomes the literal "e2".
	blocks := strings.Split(string(withoutSourceText(t, turtle)), "\n\n")
	for i, block := range blocks {
		if strings.HasPrefix(block, "elmt:P__V___401\n") || strings.HasPrefix(block, "elmt:P__V___401_rs\n") {
			block = strings.ReplaceAll(block, "elmt:P__e2 ", `"e2" `)
			blocks[i] = strings.ReplaceAll(block, `{\"@id\":\"P__e2\"}`, `\"e2\"`)
		}
	}
	literal := strings.Join(blocks, "\n\n")
	if !strings.Contains(literal, `sysml:references "e2" ;`) {
		t.Fatalf("the variant was not given a literal reference:\n%s", literal)
	}
	_, err = convert.Convert("m.ttl", []byte(literal), convert.FormatTurtle, convert.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), "a single name has no reference form") {
		t.Errorf("expected the literal variant to be refused, got %v", err)
	}
}

// A VariantReference takes no usage prefix: `variant ref x;` declares the
// reference usage x (SysML-textual-bnf VariantUsageElement → ReferenceUsage),
// keeping its name and its `ref`.
func TestPrefixedVariantDeclaresItsName(t *testing.T) {
	src := "package P {\n    part def E;\n    part x : E;\n    variation part def V {\n        variant ref x;\n    }\n}\n"
	if diagnostics := diagnosticMessages("m.sysml", []byte(src)); len(diagnostics) > 0 {
		t.Fatalf("the model should analyse clean:\n%s", strings.Join(diagnostics, "\n"))
	}
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	if !strings.Contains(graph, `sysml:declaredName "x"`) || strings.Contains(graph, "a sysml:ReferenceSubsetting") {
		t.Errorf("`variant ref x;` should declare x and reference nothing:\n%s", graph)
	}
	back, err := convert.Convert("m.ttl", withoutSourceText(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v", err)
	}
	if string(back) != src {
		t.Fatalf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", src, back)
	}
}
