package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const extensionLibraryModel = `package Radios {
    private import MOSA::*;
    #consensusStandard item ethernet : ConsensusStandard {
        :>> organization = "IEEE";
    }
    #majorSystemComponent part radio;
}
`

// A reference into one of OpenSysML's own libraries links by the element's
// fixed extension id — never a {"@ref": name} — and the Turtle names the
// element under that id as it names a standard library element.
func TestReferencesIntoOpenSysMLLibrariesLinkByFixedID(t *testing.T) {
	catalog := identity.LibraryCatalog(libs.NewModelIndex())
	idOf := func(fqn string) string {
		t.Helper()
		el, ok := catalog.ElementNamed(fqn)
		if !ok || el.Language != identity.OpenSysML || el.ID != identity.ElementID(identity.OpenSysML, fqn) {
			t.Fatalf("%s: catalogued as %+v, want a fixed OpenSysML id", fqn, el)
		}
		return el.ID
	}
	out, err := convert.Convert("r.sysml", []byte(extensionLibraryModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"@ref"`) {
		t.Fatalf("the element form still references a library element by name:\n%s", out)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	refs := map[string][]string{} // metaclass.property → referenced ids
	for _, e := range elements {
		metaclass, _ := e["@type"].(string)
		for _, property := range []string{"redefinedFeature", "type", "importedNamespace"} {
			switch v := e[property].(type) {
			case map[string]any:
				refs[metaclass+"."+property] = append(refs[metaclass+"."+property], v["@id"].(string))
			case []any:
				for _, item := range v {
					refs[metaclass+"."+property] = append(refs[metaclass+"."+property], item.(map[string]any)["@id"].(string))
				}
			}
		}
	}
	for key, want := range map[string]string{
		"Redefinition.redefinedFeature":     idOf("MOSA::Standard::organization"),
		"FeatureTyping.type":                idOf("MOSA::ConsensusStandard"),
		"MetadataUsage.type":                idOf("MOSA::MajorSystemComponentMetadata"),
		"NamespaceImport.importedNamespace": idOf("MOSA"),
	} {
		found := false
		for _, id := range refs[key] {
			found = found || id == want
		}
		if !found {
			t.Errorf("%s: references %v, want %s among them", key, refs[key], want)
		}
	}

	turtle, err := convert.Convert("r.sysml", []byte(extensionLibraryModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	organization := idOf("MOSA::Standard::organization")
	stub := "elmt:" + organization + "\n"
	i := strings.Index(string(turtle), stub)
	if i < 0 {
		t.Fatalf("the graph does not name the redefined library feature %s", organization)
	}
	block := string(turtle)[i:]
	block = block[:strings.Index(block, " .\n")]
	for _, want := range []string{
		`sysml:elementId "` + organization + `"`,
		`sysml:qualifiedName "MOSA::Standard::organization"`,
		`sysml:isLibraryElement "true"^^xsd:boolean`,
	} {
		if !strings.Contains(block, want) {
			t.Errorf("the library stub lacks %s:\n%s", want, block)
		}
	}
	if strings.Contains(string(turtle), `sysml:redefinedFeature "organization"`) {
		t.Error("the graph still carries the redefined library feature as text")
	}

	for _, tc := range []struct {
		name string
		in   []byte
		from convert.Format
	}{
		{"turtle", turtle, convert.FormatTurtle},
		{"api-json", out, convert.FormatAPIJSON},
	} {
		back, err := convert.Convert("r."+tc.name, tc.in, tc.from, convert.FormatSysML)
		if err != nil {
			t.Fatalf("%s back to notation: %v", tc.name, err)
		}
		for _, want := range []string{"#consensusStandard item ethernet : ConsensusStandard", ":>> organization = \"IEEE\";", "#majorSystemComponent part radio;"} {
			if !strings.Contains(string(back), want) {
				t.Errorf("%s back to notation lacks %q:\n%s", tc.name, want, back)
			}
		}
	}
}
