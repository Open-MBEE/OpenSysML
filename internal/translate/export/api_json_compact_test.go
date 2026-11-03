package export

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestCompactReferenceKeysAreExact(t *testing.T) {
	tests := []struct {
		name             string
		source           string
		literalType      string
		literalValue     any
		referenceLiteral bool
	}{
		{
			name:         "string",
			source:       `package P { attribute a = "x"; }`,
			literalType:  "LiteralString",
			literalValue: "x",
		},
		{
			name:         "integer",
			source:       `package P { attribute a = 3; }`,
			literalType:  "LiteralInteger",
			literalValue: json.Number("3"),
		},
		{
			name:         "boolean",
			source:       `package P { attribute a = true; }`,
			literalType:  "LiteralBoolean",
			literalValue: true,
		},
		{
			name:             "reference",
			source:           `package P { attribute b; attribute a = b; }`,
			referenceLiteral: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := source.New("compact.sysml", []byte(test.source))
			graph, err := ToRDF(file, parser.New(file).ParseFile())
			if err != nil {
				t.Fatal(err)
			}
			document, err := WriteAPIJSONCompact(graph, CompactAPIJSONOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var compact struct {
				IDs           []string         `json:"ids"`
				ReferenceKeys []string         `json:"referenceKeys"`
				Elements      []map[string]any `json:"elements"`
			}
			decoder := json.NewDecoder(bytes.NewReader(document))
			decoder.UseNumber()
			if err := decoder.Decode(&compact); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(compact.ReferenceKeys, "ownedRelatedElement") {
				t.Errorf("referenceKeys %v omit ownedRelatedElement", compact.ReferenceKeys)
			}

			var literalID string
			var featureValue any
			for i, element := range compact.Elements {
				elementType, _ := element["@type"].(string)
				if test.referenceLiteral && strings.HasPrefix(elementType, "Literal") {
					if _, carriesValue := element["value"]; carriesValue {
						t.Errorf("%s literal carries value: %#v", elementType, element)
					}
				}
				if elementType == test.literalType {
					if i < len(compact.IDs) {
						literalID = compact.IDs[i]
					}
					if element["value"] != test.literalValue {
						t.Errorf("%s.value = %#v, want %#v", test.literalType, element["value"], test.literalValue)
					}
				}
				if element["@type"] == "FeatureValue" {
					featureValue = element["value"]
				}
			}
			if !test.referenceLiteral {
				if slices.Contains(compact.ReferenceKeys, "value") {
					t.Errorf("referenceKeys includes value for a literal: %v", compact.ReferenceKeys)
				}
				if literalID == "" {
					t.Fatalf("no %s element in compact output", test.literalType)
				}
				reference, ok := featureValue.(map[string]any)
				handle, handleOK := reference["@id"].(json.Number)
				if !ok || len(reference) != 1 || !handleOK {
					t.Fatalf("FeatureValue.value = %#v, want an @id handle", featureValue)
				}
				index, err := handle.Int64()
				if err != nil || index < 0 || int(index) >= len(compact.IDs) {
					t.Fatalf("FeatureValue.value handle %q is invalid", handle)
				}
				if compact.IDs[index] != literalID {
					t.Errorf("FeatureValue.value points to %q, want %q", compact.IDs[index], literalID)
				}
				return
			}

			if !slices.Contains(compact.ReferenceKeys, "value") {
				t.Errorf("referenceKeys omits value when it carries only references: %v", compact.ReferenceKeys)
			}
			handle, ok := featureValue.(json.Number)
			if !ok {
				t.Fatalf("FeatureValue.value = %#v, want a bare handle", featureValue)
			}
			index, err := handle.Int64()
			if err != nil || index < 0 || int(index) >= len(compact.IDs) {
				t.Fatalf("FeatureValue.value handle %q is invalid", handle)
			}
		})
	}
}
