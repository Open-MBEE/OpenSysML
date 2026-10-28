package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

type compactRulesDocument struct {
	IDs              []string         `json:"ids"`
	ElementCount     int              `json:"elementCount"`
	ElementIDsAreIDs bool             `json:"elementIdsAreIds"`
	ReferenceKeys    []string         `json:"referenceKeys"`
	Elements         []map[string]any `json:"elements"`
}

var compactRulesSeeds = []uint64{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
	10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
	20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
	30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
	40, 41, 42, 43, 44, 45, 46, 47, 48, 49,
}

func expandByTheDocumentedRules(text string) ([]map[string]any, error) {
	var document compactRulesDocument
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if document.ElementCount != len(document.Elements) || document.ElementCount > len(document.IDs) {
		return nil, fmt.Errorf("elementCount %d, %d elements and %d ids", document.ElementCount, len(document.Elements), len(document.IDs))
	}
	referenceKeys := make(map[string]bool, len(document.ReferenceKeys))
	for _, key := range document.ReferenceKeys {
		referenceKeys[key] = true
	}
	handle := func(key string, value any) (map[string]any, error) {
		number, ok := value.(json.Number)
		if !ok {
			return nil, fmt.Errorf("%s value %#v is not a handle", key, value)
		}
		index, err := number.Int64()
		if err != nil || index < 0 || index >= int64(len(document.IDs)) {
			return nil, fmt.Errorf("%s handle %#v is outside the id table", key, value)
		}
		return map[string]any{"@id": document.IDs[index]}, nil
	}
	var expandOther func(string, any) (any, error)
	expandOther = func(key string, value any) (any, error) {
		switch typed := value.(type) {
		case []any:
			expanded := make([]any, len(typed))
			for i, item := range typed {
				result, err := expandOther(key, item)
				if err != nil {
					return nil, err
				}
				expanded[i] = result
			}
			return expanded, nil
		case map[string]any:
			if len(typed) == 1 {
				if raw, exists := typed["@id"]; exists {
					if number, isNumber := raw.(json.Number); isNumber {
						if _, integer := new(big.Int).SetString(number.String(), 10); integer {
							return handle(key, raw)
						}
					}
				}
			}
			expanded := make(map[string]any, len(typed))
			for nestedKey, nestedValue := range typed {
				result, err := expandOther(key, nestedValue)
				if err != nil {
					return nil, err
				}
				expanded[nestedKey] = result
			}
			return expanded, nil
		default:
			return value, nil
		}
	}
	expanded := make([]map[string]any, len(document.Elements))
	for i, element := range document.Elements {
		if i >= len(document.IDs) {
			return nil, fmt.Errorf("element %d has no id table entry", i)
		}
		result := make(map[string]any, len(element)+2)
		result["@id"] = document.IDs[i]
		if document.ElementIDsAreIDs {
			result["elementId"] = document.IDs[i]
		}
		for key, value := range element {
			if referenceKeys[key] {
				switch typed := value.(type) {
				case nil:
					result[key] = nil
				case json.Number:
					reference, err := handle(key, typed)
					if err != nil {
						return nil, err
					}
					result[key] = reference
				case []any:
					references := make([]any, len(typed))
					for i, item := range typed {
						reference, err := handle(key, item)
						if err != nil {
							return nil, err
						}
						references[i] = reference
					}
					result[key] = references
				default:
					return nil, fmt.Errorf("%s value %#v is neither null nor a handle or array of handles", key, value)
				}
				continue
			}
			expandedValue, err := expandOther(key, value)
			if err != nil {
				return nil, err
			}
			result[key] = expandedValue
		}
		expanded[i] = result
	}
	return expanded, nil
}

func TestConvertCompactExpandsByTheDocumentedRulesOnly(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	for _, seed := range compactRulesSeeds {
		documents := compactRulesTrial(seed)
		namedDocuments := make([]string, len(documents))
		namedContent := make([]string, 0, len(documents)*2)
		for i, document := range documents {
			namedDocuments[i] = document.name
			namedContent = append(namedContent, document.name, document.text)
		}
		documentText := strings.Join(namedContent, "\n")
		parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
			Documents: inlineDocuments(namedContent...),
		})
		if err != nil {
			t.Fatalf("seed %d documents:\n%s\nParseSources: %v", seed, documentText, err)
		}
		source := &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}
		cases := []struct {
			name        string
			documents   []string
			omitDerived bool
			keepDerived []string
		}{
			{name: "all-documents"},
			{name: "one-document", documents: []string{"d1.sysml"}},
			{
				name:        "omit-derived",
				omitDerived: true,
				keepDerived: []string{"qualifiedName", "isLibraryElement", "importedMembership"},
			},
		}
		for _, test := range cases {
			request := &pb.ConvertRequest{
				Source: source, ToFormat: "api-json", Compact: true, Documents: test.documents,
				OmitDerived: test.omitDerived, KeepDerived: test.keepDerived,
			}
			compactResponse, err := srv.Convert(context.Background(), request)
			if err != nil || compactResponse.GetError() != "" {
				t.Fatalf("seed %d documents:\n%s\n%s compact conversion: %v %s",
					seed, documentText, test.name, err, compactResponse.GetError())
			}
			expanded, err := expandByTheDocumentedRules(compactResponse.Content)
			if err != nil {
				t.Fatalf("seed %d documents:\n%s\n%s documented-rules reader: %v\ncompact:\n%s",
					seed, documentText, test.name, err, compactResponse.Content)
			}
			if test.name == "omit-derived" {
				full, err := srv.Convert(context.Background(), &pb.ConvertRequest{
					Source: source, ToFormat: "api-json", Compact: true,
				})
				if err != nil || full.GetError() != "" {
					t.Fatalf("seed %d documents:\n%s\nfull compact conversion: %v %s",
						seed, documentText, err, full.GetError())
				}
				fullExpanded, err := expandByTheDocumentedRules(full.Content)
				if err != nil {
					t.Fatalf("seed %d documents:\n%s\nfull documented-rules reader: %v",
						seed, documentText, err)
				}
				assertLeanCompactExpansion(t, seed, documentText, expanded, fullExpanded)
				continue
			}

			standardResponse, err := srv.Convert(context.Background(), &pb.ConvertRequest{
				Source: source, ToFormat: "api-json", Documents: test.documents,
			})
			if err != nil || standardResponse.GetError() != "" {
				t.Fatalf("seed %d documents:\n%s\n%s standard conversion: %v %s",
					seed, documentText, test.name, err, standardResponse.GetError())
			}
			standard, err := decodeAPIJSONElementsUseNumber(standardResponse.Content)
			if err != nil {
				t.Fatalf("seed %d documents:\n%s\n%s standard JSON: %v",
					seed, documentText, test.name, err)
			}
			if !reflect.DeepEqual(expanded, standard) {
				for i := 0; i < len(expanded) && i < len(standard); i++ {
					if !reflect.DeepEqual(expanded[i], standard[i]) {
						t.Fatalf("seed %d documents:\n%s\n%s element %d differs:\nexpanded: %#v\nstandard: %#v",
							seed, documentText, test.name, i, expanded[i], standard[i])
					}
				}
				t.Fatalf("seed %d documents:\n%s\n%s element count differs: expanded %d, standard %d",
					seed, documentText, test.name, len(expanded), len(standard))
			}
		}
	}
}

type compactRulesSource struct {
	name string
	text string
}

func compactRulesTrial(seed uint64) []compactRulesSource {
	random := rand.New(rand.NewPCG(seed+1, seed^0x9e3779b97f4a7c15))
	documentCount := 2 + random.IntN(3)
	counts := make([]int, documentCount)
	total := 0
	for i := range counts {
		counts[i] = 1 + random.IntN(6)
		total += counts[i]
	}
	if counts[0] < 4 {
		total += 4 - counts[0]
		counts[0] = 4
	}
	for total < 5 {
		for {
			index := random.IntN(documentCount)
			if counts[index] < 6 {
				counts[index]++
				total++
				break
			}
		}
	}

	type attribute struct {
		name  string
		value string
	}
	documents := make([]compactRulesSource, documentCount)
	for documentIndex, count := range counts {
		attributes := make([]attribute, 0, count)
		for i := 0; i < count; i++ {
			name := fmt.Sprintf("a%d", i)
			if documentIndex == 0 && i == 2 {
				attributes = append(attributes, attribute{name: "r", value: "a0"})
				continue
			}
			var value string
			switch {
			case documentIndex == 0 && i == 0:
				value = compactRulesInteger(random)
			case documentIndex == 0 && i == 1:
				value = compactRulesReal(random)
			case documentIndex == 0 && i == 3:
				value = compactRulesBoolean(random)
			case documentIndex == 1:
				value = compactRulesString(random)
			default:
				value = compactRulesLiteral(random)
			}
			attributes = append(attributes, attribute{name: name, value: value})
		}
		random.Shuffle(len(attributes), func(i, j int) {
			attributes[i], attributes[j] = attributes[j], attributes[i]
		})
		body := make([]string, 0, len(attributes))
		for _, item := range attributes {
			body = append(body, fmt.Sprintf("attribute %s = %s;", item.name, item.value))
		}
		var prefix, suffix string
		if documentIndex > 0 && random.IntN(2) == 1 {
			imported := random.IntN(documentIndex)
			prefix = fmt.Sprintf("private import D%d::*;", imported)
			if random.IntN(2) == 1 {
				suffix = fmt.Sprintf("part q%d :> D%d::p%d;", documentIndex, imported, imported)
			}
		}
		source := fmt.Sprintf(
			"package D%d { %s part def T%d { %s } part p%d : T%d; %s }",
			documentIndex, prefix, documentIndex, strings.Join(body, " "), documentIndex, documentIndex, suffix,
		)
		documents[documentIndex] = compactRulesSource{
			name: fmt.Sprintf("d%d.sysml", documentIndex),
			text: source,
		}
	}
	return documents
}

func compactRulesLiteral(random *rand.Rand) string {
	switch random.IntN(4) {
	case 0:
		return compactRulesString(random)
	case 1:
		return compactRulesInteger(random)
	case 2:
		return compactRulesReal(random)
	default:
		return compactRulesBoolean(random)
	}
}

func compactRulesString(random *rand.Rand) string {
	return strconv.Quote([]string{"3", "0", "true", "", " spaces ", `escaped " quote`}[random.IntN(6)])
}

func compactRulesInteger(random *rand.Rand) string {
	return []string{"0", "3", "-1", "-12"}[random.IntN(4)]
}

func compactRulesReal(random *rand.Rand) string {
	return []string{"2.5", "1.0E3", "0.1"}[random.IntN(3)]
}

func compactRulesBoolean(random *rand.Rand) string {
	return []string{"true", "false"}[random.IntN(2)]
}

func decodeAPIJSONElementsUseNumber(text string) ([]map[string]any, error) {
	var elements []map[string]any
	decoder := json.NewDecoder(bytes.NewBufferString(text))
	decoder.UseNumber()
	if err := decoder.Decode(&elements); err != nil {
		return nil, err
	}
	return elements, nil
}

func assertLeanCompactExpansion(
	t *testing.T,
	seed uint64,
	documentText string,
	lean, full []map[string]any,
) {
	t.Helper()
	if len(lean) != len(full) {
		t.Fatalf("seed %d documents:\n%s\nomit-derived has %d elements, full compact has %d",
			seed, documentText, len(lean), len(full))
	}
	for i := range lean {
		if lean[i]["@id"] != full[i]["@id"] {
			t.Fatalf("seed %d documents:\n%s\nelement %d id differs: lean %#v, full %#v",
				seed, documentText, i, lean[i]["@id"], full[i]["@id"])
		}
		for key, value := range lean[i] {
			if !reflect.DeepEqual(value, full[i][key]) {
				t.Fatalf("seed %d documents:\n%s\nelement %d key %s differs: lean %#v, full %#v",
					seed, documentText, i, key, value, full[i][key])
			}
		}
	}
}
