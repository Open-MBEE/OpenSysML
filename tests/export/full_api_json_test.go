package export_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

var updateFullAPIJSON = flag.Bool("update-full-api-json", false, "rewrite the full API JSON goldens")

func TestFullAPIJSONGoldens(t *testing.T) {
	for _, name := range []string{"names", "enums", "nested", "parts"} {
		t.Run(name, func(t *testing.T) {
			sourcePath := filepath.Join("testdata", "convert", name+".sysml")
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			full, err := convert.ConvertWith(sourcePath, source, convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
				APIJSON: export.APIJSONFull,
			})
			if err != nil {
				t.Fatalf("full API JSON: %v", err)
			}
			goldenPath := filepath.Join("testdata", "convert", name+".full.golden.json")
			if *updateFullAPIJSON {
				if err := os.WriteFile(goldenPath, full, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("%v (run with -update-full-api-json to create it)", err)
			}
			if !bytes.Equal(full, want) {
				t.Errorf("%s differs\n--- want ---\n%s\n--- got ---\n%s", goldenPath, want, full)
			}
		})
	}
}

func TestFullAPIJSONKeepsCompactGraphValues(t *testing.T) {
	const source = `package P {
		part def Base { attribute size : Integer; }
		part def Derived specializes Base { attribute mass : Integer; }
		part vehicle : Derived;
	}`
	compact, err := convert.Convert("m.sysml", []byte(source), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	full, err := convert.ConvertWith("m.sysml", []byte(source), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	var compactElements, fullElements []map[string]json.RawMessage
	if err := json.Unmarshal(compact, &compactElements); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(full, &fullElements); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]json.RawMessage, len(fullElements))
	for _, element := range fullElements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		byID[id] = element
	}
	for _, element := range compactElements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		fullElement := byID[id]
		for key, value := range element {
			if !bytes.Equal(value, fullElement[key]) {
				t.Errorf("%s[%q] changed: compact %s, full %s", id, key, value, fullElement[key])
			}
		}
	}
}

func TestFullAPIJSONUsesGraphMembershipIDs(t *testing.T) {
	const source = `package P {
		part def Base { part size; }
		part def Derived specializes Base { part mass; }
	}`
	full, err := convert.ConvertWith("m.sysml", []byte(source), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(full, &elements); err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]bool, len(elements))
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		ids[id] = true
	}
	for _, element := range elements {
		var qualifiedName string
		_ = json.Unmarshal(element["qualifiedName"], &qualifiedName)
		if qualifiedName != "P" {
			continue
		}
		var memberships []map[string]string
		if err := json.Unmarshal(element["membership"], &memberships); err != nil {
			t.Fatal(err)
		}
		if len(memberships) == 0 {
			t.Fatal("package P has no membership references")
		}
		for _, membership := range memberships {
			if !ids[membership["@id"]] {
				t.Errorf("membership %q has no graph element", membership["@id"])
			}
		}
		return
	}
	t.Fatal("full API JSON omitted package P")
}

func TestFullAPIJSONRejectsGraphInputs(t *testing.T) {
	_, err := convert.ConvertWith("m.ttl", []byte("@prefix sysml: <https://www.omg.org/spec/SysML/> ."), convert.FormatTurtle, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err == nil || err.Error() != "full API JSON form requires SysML or KerML input; graph inputs are not supported" {
		t.Fatalf("full form on a graph input = %v, want a clear unsupported-input error", err)
	}
}

func TestFullAPIJSONModelUsesSharedDocumentModel(t *testing.T) {
	out, err := convert.ConvertModel([]convert.Input{
		{Name: "types.sysml", Data: []byte(`package Types { part def Vehicle; }`)},
		{Name: "uses.sysml", Data: []byte(`package Uses { part car : Types::Vehicle; }`)},
	}, convert.FormatAPIJSON, convert.Options{APIJSON: export.APIJSONFull})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, element := range elements {
		var name string
		_ = json.Unmarshal(element["name"], &name)
		if name != "car" {
			continue
		}
		var types []map[string]string
		if err := json.Unmarshal(element["type"], &types); err != nil {
			t.Fatal(err)
		}
		if len(types) == 0 || types[0]["@id"] == "" {
			t.Error("car.type has no reference id")
		}
		found = true
	}
	if !found {
		t.Fatal("full API JSON omitted the car usage")
	}
}
