package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// includeModel writes both forms of SysML.xtext IncludeUseCaseUsage: the
// reference form `include pay;` that owns a ReferenceSubsetting to the use case
// it includes, and the declaration form `include use case ... : Pay` that
// declares a use case of its own and owns none.
const includeModel = `package Inc {
    use case def Pay;
    use case pay : Pay;
    use case def Checkout {
        include pay;
        include use case payAgain : Pay;
    }
}
`

func includeGraph(t *testing.T) (string, *rdf.Graph) {
	t.Helper()
	turtle, err := convert.Convert("inc.sysml", []byte(includeModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	return string(turtle), g
}

// `include pay;` is an IncludeUseCaseUsage whose ownedReferenceSubsetting is
// a ReferenceSubsetting to pay (SysML.xtext IncludeUseCaseUsage:
// OwnedReferenceSubsetting), which derives its useCaseIncluded; the
// declaration form `include use case payAgain : Pay` owns no such subsetting.
func TestIncludeUseCaseReferenceIsMetamodelStructure(t *testing.T) {
	_, g := includeGraph(t)
	included := element("Inc__Checkout___400")
	if got := g.Type(included); got != rdf.SysML+"IncludeUseCaseUsage" {
		t.Fatalf("`include pay;` is a %s", got)
	}
	subsetting, ok := g.Object(included, rdf.SysML+"ownedReferenceSubsetting")
	if !ok {
		t.Fatalf("`include pay;` states no sysml:ownedReferenceSubsetting")
	}
	if got := g.Type(subsetting); got != rdf.SysML+"ReferenceSubsetting" {
		t.Fatalf("`include pay;`'s owned reference subsetting is a %s", got)
	}
	if referenced, _ := g.Object(subsetting, rdf.SysML+"referencedFeature"); referenced != element("Inc__pay") {
		t.Errorf("the ReferenceSubsetting's referencedFeature is %v, want Inc::pay", referenced)
	}
	if referencing, _ := g.Object(subsetting, rdf.SysML+"referencingFeature"); referencing != included {
		t.Errorf("the ReferenceSubsetting's referencingFeature is %v, want the include", referencing)
	}
	if owned := g.Objects(included, rdf.SysML+"ownedRelationship"); !containsTerm(owned, subsetting) {
		t.Errorf("`include pay;` does not own its ReferenceSubsetting")
	}
	if useCase, _ := g.Object(included, rdf.SysML+"useCaseIncluded"); useCase != element("Inc__pay") {
		t.Errorf("`include pay;` states useCaseIncluded %v, want Inc::pay", useCase)
	}

	declared := element("Inc__Checkout__payAgain")
	if got := g.Type(declared); got != rdf.SysML+"IncludeUseCaseUsage" {
		t.Fatalf("`include use case payAgain : Pay` is a %s", got)
	}
	for _, property := range []string{"ownedReferenceSubsetting", "useCaseIncluded", "references"} {
		if g.HasProperty(declared, rdf.SysML+property) {
			t.Errorf("`include use case payAgain : Pay` declares a use case and must state no sysml:%s", property)
		}
	}
	for _, rel := range g.Objects(declared, rdf.SysML+"ownedRelationship") {
		if g.Type(rel) == rdf.SysML+"ReferenceSubsetting" {
			t.Errorf("`include use case payAgain : Pay` owns a ReferenceSubsetting %v", rel)
		}
	}
}

// The structure alone, source text stripped, writes both forms back: the
// convenience `includes` and the derived useCaseIncluded may both go, the
// ReferenceSubsetting carries `include pay;`. A graph whose convenience
// properties disagree with the ReferenceSubsetting is refused.
func TestIncludeUseCaseReadsFromStructureAlone(t *testing.T) {
	turtle, _ := includeGraph(t)
	back := backFromTheGraphAlone(t, turtle)
	for _, want := range []string{"include pay;", "include use case payAgain : Pay;"} {
		if !strings.Contains(back, want) {
			t.Errorf("the graph alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}

	structural := withoutSourceText(t, []byte(turtle))
	for _, convenience := range []string{"sysml:includes", "sysml:useCaseIncluded"} {
		structural = withoutTriples(t, structural, convenience)
	}
	if !strings.Contains(string(structural), "sysml:ownedReferenceSubsetting") {
		t.Fatal("the stripped graph lost the ReferenceSubsetting it was to keep")
	}
	again, err := convert.Convert("inc.ttl", structural, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back without the convenience properties: %v", err)
	}
	for _, want := range []string{"include pay;", "include use case payAgain : Pay;"} {
		if !strings.Contains(string(again), want) {
			t.Errorf("without the convenience properties the graph did not write %q\n--- notation ---\n%s", want, again)
		}
	}

	for _, disagreeing := range []string{"sysml:includes elmt:Inc__pay ;", "sysml:useCaseIncluded elmt:Inc__pay ;"} {
		if !strings.Contains(turtle, disagreeing) {
			t.Fatalf("the graph does not state %q", disagreeing)
		}
		other := strings.Replace(turtle, disagreeing, strings.Replace(disagreeing, "Inc__pay", "Inc__Pay", 1), 1)
		if _, err := convert.Convert("inc.ttl", []byte(other), convert.FormatTurtle, convert.FormatSysML); err == nil {
			t.Errorf("%q naming another element than the ReferenceSubsetting was accepted", disagreeing)
		}
	}
}

// The API element form carries the same structure under the metaclass's own
// properties, and reads back from it with the source text dropped.
func TestIncludeUseCaseAPIJSON(t *testing.T) {
	_, g := includeGraph(t)
	document, err := export.WriteAPIJSON(g)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	included := apiJSONElement(t, elements, "Inc__Checkout___400")
	if got := string(included["@type"]); got != `"IncludeUseCaseUsage"` {
		t.Errorf("`include pay;` @type = %s", got)
	}
	compact := func(raw json.RawMessage) string { return strings.Join(strings.Fields(string(raw)), "") }
	if got := compact(included["ownedReferenceSubsetting"]); got != `{"@id":"Inc__Checkout___400_rs"}` {
		t.Errorf("`include pay;` ownedReferenceSubsetting = %s", got)
	}
	if got := compact(included["useCaseIncluded"]); got != `{"@id":"Inc__pay"}` {
		t.Errorf("`include pay;` useCaseIncluded = %s", got)
	}
	subsetting := apiJSONElement(t, elements, "Inc__Checkout___400_rs")
	if got := string(subsetting["@type"]); got != `"ReferenceSubsetting"` {
		t.Errorf("the owned reference subsetting's @type = %s", got)
	}
	if got := compact(subsetting["referencedFeature"]); got != `{"@id":"Inc__pay"}` {
		t.Errorf("the owned reference subsetting's referencedFeature = %s", got)
	}
	for i := range elements {
		delete(elements[i], "sysx:sourceText")
		delete(elements[i], "sysx:sourceTail")
	}
	stripped, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	back, err := convert.Convert("inc.json", stripped, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back from the element form alone: %v", err)
	}
	for _, want := range []string{"include pay;", "include use case payAgain : Pay;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the element form alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}
}

// The toolkit's compact element form states `include pay;` by its
// ReferenceSubsetting alone — no `includes`, no `references` — as the pilot
// writes it, and it reads back as written.
func TestIncludeUseCaseToolkitCompactDecodes(t *testing.T) {
	back, err := convert.Convert("inc.json", toolkitCompact(t, includeModel), convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("toolkit compact decode: %v", err)
	}
	for _, want := range []string{"include pay;", "include use case payAgain : Pay;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the ReferenceSubsetting alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}
	if strings.Contains(string(back), "references pay") {
		t.Errorf("the inclusion came back as a reference declaration:\n%s", back)
	}
}
