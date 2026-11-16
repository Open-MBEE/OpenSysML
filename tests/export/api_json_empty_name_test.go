package export_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The empty unrestricted name is a name (KerML 1.1 §8.2.2.3): the element states
// it, is identified by it and is qualified by its escaped form (§8.3.2.1.2).
func TestAPIJSONEmptyNameIsAName(t *testing.T) {
	path := filepath.Join("testdata", "convert", "empty_names.sysml")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := cachedAPIJSON(path, source, "standard", "graph")
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	byQualifiedName := make(map[string]map[string]json.RawMessage)
	unnamed := 0
	for _, element := range elements {
		if string(element["@type"]) == `"PartDefinition"` && string(element["declaredName"]) == `"Short"` {
			if got := string(element["declaredShortName"]); got != `""` {
				t.Errorf("Short: declaredShortName = %s, want the empty string", got)
			}
			if got := string(element["shortName"]); got != `""` {
				t.Errorf("Short: shortName = %s, want the empty string", got)
			}
		}
		if string(element["@type"]) != `"PartUsage"` {
			continue
		}
		var qualified *string
		if raw, stated := element["qualifiedName"]; stated {
			if err := json.Unmarshal(raw, &qualified); err != nil {
				t.Fatal(err)
			}
		}
		if qualified == nil {
			unnamed++
			if got := string(element["declaredName"]); got != "null" {
				t.Errorf("unnamed part: declaredName = %s, want null", got)
			}
			continue
		}
		byQualifiedName[*qualified] = element
	}
	if unnamed != 1 {
		t.Errorf("%d parts without a qualified name, want the one unnamed part", unnamed)
	}
	for _, c := range []struct{ qualified, name, id string }{
		{"'Empty Names'::''", `""`, `"Empty_20Names___27_27"`},
		{"'Empty Names'::'   '", `"   "`, `"Empty_20Names___20_20_20"`},
	} {
		element, ok := byQualifiedName[c.qualified]
		if !ok {
			t.Errorf("no part is qualified %s", c.qualified)
			continue
		}
		for key, want := range map[string]string{"declaredName": c.name, "name": c.name, "elementId": c.id} {
			if got := string(element[key]); got != want {
				t.Errorf("%s: %s = %s, want %s", c.qualified, key, got, want)
			}
		}
	}
}
