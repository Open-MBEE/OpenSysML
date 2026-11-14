package export_test

import (
	"bytes"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// A conversion to the element form names the library members only an inherited property
// references without first deriving every property to find them.
func TestAPIJSONElementGraphNamesInheritedLibraryMembers(t *testing.T) {
	inputs := []convert.Input{
		{Name: "defs.sysml", Data: []byte("package Defs {\n part def Wheel { attribute radius : ScalarValues::Real; }\n}")},
		{Name: "uses.sysml", Data: []byte("package Uses {\n private import Defs::*;\n part car { part wheel : Wheel[4]; }\n}")},
	}
	var lazy, eager []export.APIJSONLibraryReference
	got, err := convert.ConvertModel(inputs, convert.FormatAPIJSON, convert.Options{
		LibraryReferences: func(refs []export.APIJSONLibraryReference) { lazy = refs },
	})
	if err != nil {
		t.Fatal(err)
	}
	graph, err := convert.ModelGraph(inputs, convert.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{
		LibraryReferences: func(refs []export.APIJSONLibraryReference) { eager = refs },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("the element graph writes another element array than the Turtle graph")
	}
	named := map[string]export.APIJSONLibraryReference{}
	for _, ref := range lazy {
		named[ref.ID] = ref
	}
	for _, ref := range eager {
		if named[ref.ID] != ref {
			t.Errorf("library reference %s = %+v, want %+v", ref.ID, named[ref.ID], ref)
		}
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(got, &elements); err != nil {
		t.Fatal(err)
	}
	written := map[string]bool{}
	inherited := 0
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		written[id] = true
	}
	reference := regexp.MustCompile(`"@id":\s*"([^"]+)"`)
	for _, element := range elements {
		for _, match := range reference.FindAllSubmatch(element["inheritedMembership"], -1) {
			id := string(match[1])
			if written[id] {
				continue
			}
			inherited++
			if _, ok := named[id]; !ok {
				t.Errorf("inherited library membership %s is referenced and not named", id)
			}
		}
	}
	if inherited == 0 {
		t.Fatal("the model inherits no library membership: the case proves nothing")
	}
}
