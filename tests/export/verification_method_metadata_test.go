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
		`sysx:positionalName "S8::T::@0::kind"`,
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

// The API element form carries the same facts: no declaredName and an
// ownedRedefinition of the library's kind feature.
func TestVerificationMethodBodyInAPIJSON(t *testing.T) {
	doc, err := convert.Convert("m.sysml", []byte(verificationMethodModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(doc, &elements); err != nil {
		t.Fatalf("api-json: %v", err)
	}
	// The ids are the pinned pilot library's elementIds of
	// VerificationCases::VerificationMethod and its kind feature.
	const methodID, kindID = "0066c1c7-55af-52b8-8197-37c386a91db1", "d25bc7f4-eb89-5940-be2a-3a17de3e856b"
	byID := make(map[string]map[string]any, len(elements))
	for _, el := range elements {
		if id, _ := el["@id"].(string); id != "" {
			byID[id] = el
		}
	}
	referenceID := func(value any) string {
		switch reference := value.(type) {
		case map[string]any:
			id, _ := reference["@id"].(string)
			return id
		case string:
			return reference
		}
		return ""
	}
	referenceIDs := func(value any) []string {
		if references, ok := value.([]any); ok {
			ids := make([]string, 0, len(references))
			for _, reference := range references {
				if id := referenceID(reference); id != "" {
					ids = append(ids, id)
				}
			}
			return ids
		}
		if id := referenceID(value); id != "" {
			return []string{id}
		}
		return nil
	}
	metadataIDs := make(map[string]bool)
	redefinitionIDs := make(map[string]bool)
	for _, el := range elements {
		switch el["@type"] {
		case "MetadataUsage":
			if types := referenceIDs(el["type"]); len(types) == 1 && types[0] == methodID {
				metadataIDs[referenceID(el)] = true
			}
		case "Redefinition":
			if referenceID(el["redefinedFeature"]) == kindID {
				redefinitionIDs[referenceID(el)] = true
			}
		}
	}
	if len(metadataIDs) == 0 {
		t.Errorf("no MetadataUsage typed by VerificationMethod (%s)", methodID)
	}
	if len(redefinitionIDs) == 0 {
		t.Errorf("no Redefinition of VerificationMethod::kind (%s)", kindID)
	}
	var member map[string]any
	for _, el := range elements {
		if el["@type"] != "ReferenceUsage" {
			continue
		}
		ownsKindRedefinition := false
		for _, id := range referenceIDs(el["ownedRedefinition"]) {
			ownsKindRedefinition = ownsKindRedefinition || redefinitionIDs[id]
		}
		if !ownsKindRedefinition {
			continue
		}
		membership := byID[referenceID(el["owningMembership"])]
		if membership == nil || !metadataIDs[referenceID(membership["owner"])] {
			continue
		}
		member = el
		break
	}
	if member == nil {
		t.Fatalf("no ReferenceUsage under the VerificationMethod MetadataUsage redefines kind in %s", doc)
	}
	if declaredName, ok := member["declaredName"]; !ok || declaredName != nil {
		t.Errorf("the body member declaredName = %v, want present null", declaredName)
	}
	if _, ok := member["ownedRedefinition"]; !ok {
		t.Errorf("no ownedRedefinition on the body member: %v", member)
	}
	if member["@type"] != "ReferenceUsage" {
		t.Errorf("@type = %v, want ReferenceUsage", member["@type"])
	}
	if qualifiedName, ok := member["qualifiedName"]; ok && qualifiedName != nil {
		t.Errorf("the body member qualifiedName = %v, want absent because its owning namespace has null qualifiedName", qualifiedName)
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

// A foreign graph's implicit member — a redefines with no
// sysx:isRedefinitionImplicit flag — decodes to the canonical `redefines`
// spelling, like a pilot-shaped ownedRedefinition.
func TestUnflaggedMetadataBodyMemberReadsBackAsRedefines(t *testing.T) {
	stripped := verificationMethodTurtle(t)
	for _, prop := range []string{"sysx:sourceText", "sysx:sourceTail", "sysx:isRedefinitionImplicit"} {
		stripped = withoutTriples(t, stripped, prop)
	}
	back, err := convert.Convert("m.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{
		"redefines kind = VerificationMethodKind::test;",
		"redefines kind = (VerificationMethodKind::test, VerificationMethodKind::analyze);",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("notation lacks %q:\n%s", want, back)
		}
	}
	// The written model validates and re-exports stably.
	second, err := convert.Convert("m.sysml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("second hop to turtle: %v", err)
	}
	third, err := convert.Convert("m.ttl", withoutTriples(t, withoutTriples(t, second, "sysx:sourceText"), "sysx:sourceTail"), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("third hop to notation: %v", err)
	}
	if string(third) != string(back) {
		t.Errorf("the third hop differs:\n%s\n---\n%s", back, third)
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
