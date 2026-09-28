package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

const verifyModel = `package V {
    requirement r;
    requirement def Q;
    verification def T { objective { verify r; } }
    verification def U { objective o { verify requirement v : Q; } }
    requirement def P { requirement req : Q; }
    verification def W { objective : P { verify r :>> req; } }
    verification def X { objective { verify requirement :> r; } }
    part def S;
    part s : S;
    requirement q : Q;
    satisfy q by s;
}
`

// An objective's `verify r;`, `verify r :>> req;`, `verify requirement :> r;` or
// `verify requirement v : Q;` is the
// RequirementUsage a RequirementVerificationMembership owns (SysML-textual-bnf
// RequirementVerificationMember), not a SatisfyRequirementUsage, and comes back
// as `verify` from the graph alone; a `satisfy` stays a SatisfyRequirementUsage.
func TestVerifyIsARequirementVerificationMembership(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "v.sysml", []byte(verifyModel))
	graph := string(turtle)
	for _, want := range []string{
		"elmt:V__T___400___400\n    a sysml:RequirementUsage ;",
		"elmt:V__T___400___400_om\n    a sysml:RequirementVerificationMembership ;",
		"elmt:V__U__o__v\n    a sysml:RequirementUsage ;",
		"elmt:V__U__o__v_om\n    a sysml:RequirementVerificationMembership ;",
		// A reference's subsetting is a ReferenceSubsetting, a declaration's `:>` a Subsetting.
		"elmt:V__W___400___400_rs\n    a sysml:ReferenceSubsetting ;",
		"elmt:V__X___400___400_ss0\n    a sysml:Subsetting ;",
		"sysml:ownedRequirement elmt:V__T___400___400 ;",
		`sysml:kind "requirement" ;`,
		"a sysml:SatisfyRequirementUsage ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	if strings.Count(graph, "a sysml:SatisfyRequirementUsage ;") != 1 {
		t.Errorf("only the `satisfy` is a SatisfyRequirementUsage:\n%s", graph)
	}
	notation := string(back)
	for _, want := range []string{"verify r;\n", "verify requirement v : Q;\n", "verify r redefines req;\n", "verify requirement subsets r;\n", "satisfy q by s;\n"} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
	if strings.Contains(notation, "satisfy r") || strings.Contains(notation, "satisfy requirement") ||
		strings.Contains(notation, "verify requirement subsets r redefines") {
		t.Errorf("`verify r` came back as a satisfy:\n%s", notation)
	}
}

// toolkitCompact rewrites this tool's element form the way the toolkit's
// compact element form states a model: UUID ids, only the owning links and the
// relationship ends, no collapsed properties and no sysx: ones.
func toolkitCompact(t *testing.T, src string) []byte {
	t.Helper()
	full, err := convert.ConvertWith("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{ID: export.IDUUID})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(full, &elements); err != nil {
		t.Fatal(err)
	}
	keep := map[string]bool{
		"@id": true, "@type": true, "elementId": true, "declaredName": true,
		"ownedRelationship": true, "owningRelationship": true, "ownedRelatedElement": true, "owningRelatedElement": true,
		"referencedFeature": true, "subsettingFeature": true, "subsettedFeature": true,
		"redefinedFeature": true, "redefiningFeature": true, "type": true, "typedFeature": true,
		"general": true, "specific": true, "memberElement": true, "kind": true,
	}
	compact := make([]map[string]any, 0, len(elements))
	for _, element := range elements {
		out := map[string]any{"isImpliedIncluded": false}
		for key, value := range element {
			if !keep[key] {
				continue
			}
			if one, isObject := value.(map[string]any); isObject && (key == "ownedRelationship" || key == "ownedRelatedElement") {
				value = []any{one}
			}
			out[key] = value
		}
		compact = append(compact, out)
	}
	document, err := json.Marshal(compact)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

// The toolkit's compact form states a `verify` by its membership and a
// ReferenceSubsetting alone, with no sysx:endForm: each form still reads back
// as written (review finding).
func TestToolkitVerifyFormsDecode(t *testing.T) {
	back, err := convert.Convert("m.json", toolkitCompact(t, verifyModel), convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("toolkit compact decode: %v", err)
	}
	notation := string(back)
	for _, want := range []string{"verify r;\n", "verify requirement v : Q;\n", "verify r redefines req;\n", "verify requirement subsets r;\n", "satisfy q by s;\n"} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
	if strings.Contains(notation, "references r") {
		t.Errorf("a verification reference came back as a declaration:\n%s", notation)
	}
}
