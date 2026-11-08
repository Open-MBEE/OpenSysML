package grpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// agreementExamples are the committed single-document models under examples/;
// the downloaded corpora have gates of their own.
func agreementExamples(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..", "examples")
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "pilot-corpora" {
			return filepath.SkipDir
		}
		if ext := filepath.Ext(path); !entry.IsDir() && (ext == ".sysml" || ext == ".kerml") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

// agreementAdjudicated reads testdata/element_properties_agreement_adjudicated.txt:
// the (model, element, property) rows GetElementProperties answers and the export
// omits, each with the clause the answer rests on. A row the run no longer
// produces is stale and fails, like a ratchet.
func agreementAdjudicated(t *testing.T) map[[3]string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "element_properties_agreement_adjudicated.txt"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[[3]string]string{}
	for number, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("element_properties_agreement_adjudicated.txt:%d: want model, element, property and clause separated by tabs", number+1)
		}
		rows[[3]string{fields[0], fields[1], fields[2]}] = fields[3]
	}
	return rows
}

// GetElementProperties answers what the api-json export writes, for every
// (element, metamodel property) pair of every example: the same names, the same
// values in the same order, element values by the same id; the export's `sysx:`
// extension keys are no metamodel properties. The export spells an empty or
// null derived value, and one it has no rule for, by leaving the key out
// (completeDerived), so an omitted key agrees with an empty or unsupported
// answer; an omitted key against a value is a disagreement the adjudicated
// file must name. A hygiene check, not a baseline.
func TestElementPropertiesAgreeWithTheAPIJSONExport(t *testing.T) {
	paths := agreementExamples(t)
	if len(paths) == 0 {
		t.Fatal("no example models")
	}
	adjudicated := agreementAdjudicated(t)
	seen := map[[3]string]bool{}
	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	converted, elements, pairs := 0, 0, 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		exported, err := convert.Convert(filepath.Base(path), data, convert.FormatSysML, convert.FormatAPIJSON)
		if err != nil {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_FilePath{FilePath: absolute}})
		if err != nil {
			t.Fatalf("%s: ParseFile: %v", path, err)
		}
		var objects []map[string]any
		decoder := json.NewDecoder(strings.NewReader(string(exported)))
		decoder.UseNumber()
		if err := decoder.Decode(&objects); err != nil {
			t.Fatalf("%s: api-json: %v", path, err)
		}
		converted++
		for _, object := range objects {
			id, _ := object["@id"].(string)
			resp, err := srv.GetElementProperties(ctx, &pb.GetElementPropertiesRequest{ModelHash: parsed.ModelHash, ElementId: id})
			if err != nil {
				t.Fatalf("%s: GetElementProperties(%s): %v", path, id, err)
			}
			if resp.Error != "" {
				t.Errorf("%s: GetElementProperties(%s): %s", path, id, resp.Error)
				continue
			}
			if metaclass, _ := object["@type"].(string); resp.Element.Metaclass != metaclass {
				t.Errorf("%s: %s is a %s in the export and a %s in the answer", path, id, metaclass, resp.Element.Metaclass)
			}
			elements++
			answered := map[string]*pb.ElementProperty{}
			for _, property := range resp.Properties {
				answered[property.Name] = property
			}
			for name, value := range object {
				if name == "@id" || name == "@type" || strings.HasPrefix(name, "sysx:") {
					continue
				}
				pairs++
				property, ok := answered[name]
				if !ok {
					t.Errorf("%s: %s.%s is exported but not answered", path, id, name)
					continue
				}
				delete(answered, name)
				if want, got := apiJSONSpelling(value), propertySpelling(property); want != got {
					t.Errorf("%s: %s.%s exported as %s, answered as %s", path, id, name, want, got)
				}
			}
			for name, property := range answered {
				if property.Unsupported != "" || len(property.Values) == 0 {
					continue
				}
				model, err := filepath.Rel(filepath.Join("..", ".."), path)
				if err != nil {
					t.Fatal(err)
				}
				row := [3]string{filepath.ToSlash(model), id, name}
				if _, ok := adjudicated[row]; ok {
					seen[row] = true
					continue
				}
				t.Errorf("%s: %s.%s is answered as %s but not exported", path, id, name, propertySpelling(property))
			}
		}
	}
	for row, clause := range adjudicated {
		if !seen[row] {
			t.Errorf("element_properties_agreement_adjudicated.txt: %s %s.%s (%s) no longer differs; remove the row", row[0], row[1], row[2], clause)
		}
	}
	if converted == 0 || elements == 0 {
		t.Fatalf("no example converts to api-json (%d models, %d elements)", converted, elements)
	}
	t.Logf("%d of %d examples convert; %d elements, %d (element, property) pairs agree", converted, len(paths), elements, pairs)
}

// apiJSONSpelling flattens an exported property value to one comparable text:
// element references by id, literals by value, in collection order.
func apiJSONSpelling(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []any:
		parts := make([]string, 0, len(v))
		for _, member := range v {
			parts = append(parts, apiJSONSpelling(member))
		}
		return strings.Join(parts, " | ")
	case map[string]any:
		id, _ := v["@id"].(string)
		return "element " + id
	case json.Number:
		return numberSpelling(v.String())
	case string:
		return "string " + v
	case bool:
		return "bool " + strconv.FormatBool(v)
	}
	return fmt.Sprintf("%T %v", value, value)
}

// propertySpelling flattens an answered property the way apiJSONSpelling does the export.
func propertySpelling(property *pb.ElementProperty) string {
	if property.Unsupported != "" {
		return "unsupported: " + property.Unsupported
	}
	parts := make([]string, 0, len(property.Values))
	for _, value := range property.Values {
		switch v := value.Value.(type) {
		case *pb.ElementPropertyValue_Element:
			parts = append(parts, "element "+v.Element.Id)
		case *pb.ElementPropertyValue_StringValue:
			parts = append(parts, "string "+v.StringValue)
		case *pb.ElementPropertyValue_BoolValue:
			parts = append(parts, "bool "+strconv.FormatBool(v.BoolValue))
		case *pb.ElementPropertyValue_IntValue:
			parts = append(parts, numberSpelling(strconv.FormatInt(v.IntValue, 10)))
		case *pb.ElementPropertyValue_RealValue:
			parts = append(parts, numberSpelling(strconv.FormatFloat(v.RealValue, 'g', -1, 64)))
		case *pb.ElementPropertyValue_Number:
			parts = append(parts, numberSpelling(v.Number))
		default:
			parts = append(parts, fmt.Sprintf("%T", value.Value))
		}
	}
	return strings.Join(parts, " | ")
}

// numberSpelling normalizes a decimal text so 2.0 and 2 compare equal.
func numberSpelling(text string) string {
	if f, err := strconv.ParseFloat(text, 64); err == nil {
		return "number " + strconv.FormatFloat(f, 'g', -1, 64)
	}
	return "number " + text
}
