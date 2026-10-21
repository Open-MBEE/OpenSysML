package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

const apiJSONEffectiveNameModel = `package P {
	action def A { in x; out z; in ref y; }
	action def B :> A { in :>> x; }
	action def C { perform P::A; perform P::AWithNestedAction.nested; }
	action def AWithNestedAction { action def nested; }
	part def Q { in port p; }
}`

func TestAPIJSONQualifiedNamesUseNamingFeature(t *testing.T) {
	standard, err := convert.Convert(
		"unnamed-features.sysml",
		[]byte(apiJSONEffectiveNameModel),
		convert.FormatSysML,
		convert.FormatAPIJSON,
	)
	if err != nil {
		t.Fatalf("to standard API JSON: %v", err)
	}
	graph, err := convert.SysMLToRDF("unnamed-features.sysml", []byte(apiJSONEffectiveNameModel))
	if err != nil {
		t.Fatalf("to RDF: %v", err)
	}
	compact, err := export.WriteAPIJSONCompact(graph, export.CompactAPIJSONOptions{})
	if err != nil {
		t.Fatalf("to default compact API JSON: %v", err)
	}

	for _, form := range []struct {
		name     string
		document []byte
	}{
		{name: "standard", document: standard},
		{name: "compact", document: compact},
	} {
		t.Run(form.name, func(t *testing.T) {
			type element struct {
				QualifiedName *string `json:"qualifiedName"`
				Name          *string `json:"name"`
				DeclaredName  *string `json:"declaredName"`
			}
			var elements []element
			if form.name == "compact" {
				var compactDocument struct {
					Elements []element `json:"elements"`
				}
				if err := json.Unmarshal(form.document, &compactDocument); err != nil {
					t.Fatalf("decode compact API JSON: %v", err)
				}
				elements = compactDocument.Elements
			} else if err := json.Unmarshal(form.document, &elements); err != nil {
				t.Fatalf("decode standard API JSON: %v", err)
			}

			got := make(map[string]int, len(elements))
			names := make(map[string]*string, len(elements))
			derivedNames := make(map[string]*string, len(elements))
			for _, element := range elements {
				if element.QualifiedName == nil {
					continue
				}
				name := *element.QualifiedName
				got[name]++
				effectiveName := element.Name
				if effectiveName == nil {
					effectiveName = element.DeclaredName
				}
				names[name] = effectiveName
				derivedNames[name] = element.Name
				if strings.HasSuffix(name, "::kind") {
					t.Errorf("qualifiedName %q ends in the implicit kind marker", name)
				}
			}
			wanted := map[string]string{
				"P::A::x":      "x",
				"P::A::z":      "z",
				"P::A::y":      "y",
				"P::B::x":      "x",
				"P::C::A":      "A",
				"P::C::nested": "nested",
				"P::Q::p":      "p",
			}
			for want, effectiveName := range wanted {
				if got[want] == 0 {
					t.Errorf("qualifiedName %q is missing from %s API JSON", want, form.name)
					continue
				}
				if names[want] == nil || *names[want] != effectiveName {
					t.Errorf("%s name = %v, want evaluator name %q", want, names[want], effectiveName)
				}
				if strings.HasPrefix(want, "P::C::") &&
					(derivedNames[want] == nil || *derivedNames[want] != effectiveName) {
					t.Errorf("%s name property = %v, want evaluator name %q", want, derivedNames[want], effectiveName)
				}
			}
		})
	}

	compactOmitDerived, err := export.WriteAPIJSONCompact(graph, export.CompactAPIJSONOptions{OmitDerived: true})
	if err != nil {
		t.Fatalf("to compact API JSON omitting derived properties: %v", err)
	}
	var compactDocument struct {
		IDs      []string                     `json:"ids"`
		Elements []map[string]json.RawMessage `json:"elements"`
	}
	if err := json.Unmarshal(compactOmitDerived, &compactDocument); err != nil {
		t.Fatalf("decode compact API JSON omitting derived properties: %v", err)
	}
	var defaultCompact struct {
		IDs      []string `json:"ids"`
		Elements []struct {
			Name         *string `json:"name"`
			DeclaredName *string `json:"declaredName"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(compact, &defaultCompact); err != nil {
		t.Fatalf("decode default compact API JSON: %v", err)
	}
	checkedDerivedName := false
	for i, element := range compactDocument.Elements {
		if i >= len(defaultCompact.Elements) || i >= len(compactDocument.IDs) ||
			compactDocument.IDs[i] != defaultCompact.IDs[i] {
			t.Fatal("compact element ids differ between default and OmitDerived forms")
		}
		if defaultCompact.Elements[i].Name != nil && defaultCompact.Elements[i].DeclaredName == nil {
			checkedDerivedName = true
			if _, present := element["name"]; present {
				t.Errorf("compact OmitDerived form retained derived name for %s", compactDocument.IDs[i])
			}
		}
	}
	if !checkedDerivedName {
		t.Error("default compact form did not emit a derived name property")
	}
}
