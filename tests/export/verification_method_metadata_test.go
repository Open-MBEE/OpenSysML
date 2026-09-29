package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// A bare `kind = …` member of a metadata body is a MetadataBodyUsage: it
// declares no name but owns a Redefinition of the metadata definition's
// feature (SysML.xtext MetadataBodyUsage).
const verificationMethodModel = `package S8 {
	private import VerificationCases::*;
	verification def T {
		metadata VerificationMethod { kind = VerificationMethodKind::test; }
	}
	verification def U {
		@VerificationMethod { kind = (VerificationMethodKind::test, VerificationMethodKind::analyze); }
	}
}
`

func verificationMethodTurtle(t *testing.T) []byte {
	t.Helper()
	turtle, err := convert.Convert("m.sysml", []byte(verificationMethodModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	return turtle
}

func TestVerificationMethodBodyIsAMetadataBodyUsage(t *testing.T) {
	turtle := string(verificationMethodTurtle(t))
	for _, want := range []string{
		"a sysml:MetadataUsage",
		"a sysml:ReferenceUsage",
		"a sysml:Redefinition",
		"sysx:isRedefinitionImplicit",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("graph lacks %q:\n%s", want, turtle)
		}
	}
	// The body member declares no name: it is the feature it redefines, not a
	// declaration of `kind`.
	for _, line := range strings.Split(turtle, "\n") {
		if strings.Contains(line, `sysml:declaredName "kind"`) {
			t.Errorf("the body member declares a name: %s", line)
		}
	}
	// The Redefinition targets VerificationCases::VerificationMethod::kind,
	// the member's qualifiedName keeps the effective name, and the type is the
	// metadata definition's normative IRI.
	for _, want := range []string{
		`sysml:qualifiedName "S8::T::@0::kind"`,
		`sysml:redefines elmt:`,
		`sysml:redefinedFeature elmt:`,
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("graph lacks %q:\n%s", want, turtle)
		}
	}
	// Both enum literals of U's value land as referents.
	if n := strings.Count(turtle, "a sysml:Redefinition"); n < 2 {
		t.Errorf("Redefinitions = %d, want at least two (one per body member)", n)
	}
}

// The API element form carries the same facts: no declaredName, an
// ownedRedefinition, a qualifiedName ending ::kind.
func TestVerificationMethodBodyInAPIJSON(t *testing.T) {
	doc, err := convert.Convert("m.sysml", []byte(verificationMethodModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(doc, &elements); err != nil {
		t.Fatalf("api-json: %v", err)
	}
	var member map[string]any
	for _, el := range elements {
		if qn, _ := el["qualifiedName"].(string); strings.HasSuffix(qn, "::kind") {
			member = el
		}
	}
	if member == nil {
		t.Fatalf("no member qualified *::kind in %s", doc)
	}
	if _, ok := member["declaredName"]; ok {
		t.Errorf("the body member declares a name: %v", member["declaredName"])
	}
	if _, ok := member["ownedRedefinition"]; !ok {
		t.Errorf("no ownedRedefinition on the body member: %v", member)
	}
	if member["@type"] != "ReferenceUsage" {
		t.Errorf("@type = %v, want ReferenceUsage", member["@type"])
	}
	// The ids are the pinned pilot library's elementIds of
	// VerificationCases::VerificationMethod and its kind feature.
	const methodID, kindID = "0066c1c7-55af-52b8-8197-37c386a91db1", "d25bc7f4-eb89-5940-be2a-3a17de3e856b"
	var meta, redefined bool
	for _, el := range elements {
		switch el["@type"] {
		case "MetadataUsage":
			types, _ := el["type"].([]any)
			if len(types) == 1 && types[0].(map[string]any)["@id"] == methodID {
				meta = true
			}
		case "Redefinition":
			if target, _ := el["redefinedFeature"].(map[string]any); target["@id"] == kindID {
				redefined = true
			}
		}
	}
	if !meta {
		t.Errorf("no MetadataUsage typed by VerificationMethod (%s)", methodID)
	}
	if !redefined {
		t.Errorf("no Redefinition of VerificationMethod::kind (%s)", kindID)
	}
}

// The stripped graph — no sourceText — rebuilds both metadata forms bare, and
// the second hop's graph equals the first's.
func TestVerificationMethodBodyComesBackFromTheGraphAlone(t *testing.T) {
	first := verificationMethodTurtle(t)
	stripped := withoutTriples(t, first, "sysx:sourceText")
	stripped = withoutTriples(t, stripped, "sysx:sourceTail")
	back, err := convert.Convert("m.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{
		"@VerificationMethod {",
		"kind = VerificationMethodKind::test;",
		"kind = (VerificationMethodKind::test, VerificationMethodKind::analyze);",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("notation lacks %q:\n%s", want, back)
		}
	}
	// A stripped graph canonicalizes the `metadata` member to `@`, so the
	// graphs converge on the second hop: compare the second and the third.
	second, err := convert.Convert("m.sysml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("second hop to turtle: %v", err)
	}
	third, err := convert.Convert("m.ttl", withoutTriples(t, withoutTriples(t, second, "sysx:sourceText"), "sysx:sourceTail"), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("third hop to notation: %v", err)
	}
	thirdGraph, err := convert.Convert("m.sysml", third, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("third hop to turtle: %v", err)
	}
	secondStripped := withoutTriples(t, withoutTriples(t, second, "sysx:sourceText"), "sysx:sourceTail")
	thirdStripped := withoutTriples(t, withoutTriples(t, thirdGraph, "sysx:sourceText"), "sysx:sourceTail")
	if string(secondStripped) != string(thirdStripped) {
		t.Errorf("the third hop's stripped graph differs from the second's")
	}
	// The same round trip through the API element form.
	apiDoc, err := convert.Convert("m.sysml", []byte(verificationMethodModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	backJSON, err := convert.Convert("m.json", apiDoc, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from api-json: %v", err)
	}
	for _, want := range []string{"kind = VerificationMethodKind::test;", "@VerificationMethod {"} {
		if !strings.Contains(string(backJSON), want) {
			t.Errorf("api-json notation lacks %q:\n%s", want, backJSON)
		}
	}
}

// A body name that resolves to nothing keeps the old shape: a declared name
// and no redefinition.
func TestUnresolvedMetadataBodyNameKeepsDeclaredName(t *testing.T) {
	src := `package P {
		metadata def M { attribute a : ScalarValues::Integer; }
		part def C {
			@M { b = 1; }
		}
	}
`
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	text := string(turtle)
	if !strings.Contains(text, `sysml:declaredName "b"`) {
		t.Errorf("an unresolved body name lost its declaredName:\n%s", text)
	}
	if strings.Contains(text, "isRedefinitionImplicit") {
		t.Errorf("an unresolved body name was written as implicit:\n%s", text)
	}
}
