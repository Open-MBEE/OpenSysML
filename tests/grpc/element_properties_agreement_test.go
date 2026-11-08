package grpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
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
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
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
var updateAgreementRatchet = flag.Bool("update-agreement-ratchet", false, "re-record the counts of tests/grpc/testdata/element_properties_agreement_ratchet.txt, keeping each row's side and clause")

const agreementRatchetPath = "testdata/element_properties_agreement_ratchet.txt"

// agreementRow is one class of disagreement between the api-json export and
// GetElementProperties: a metamodel property of a metaclass (or its @id / @type) and
// the kind of disagreement.
type agreementRow struct{ class, kind string }

// agreementRatchet is the recorded count of a class with the side that is correct
// (symbol or export) and the KerML/SysML clause it rests on.
type agreementRatchet struct {
	count        int
	side, clause string
}

func agreementRatchetRows(t *testing.T) map[agreementRow]agreementRatchet {
	t.Helper()
	out := map[agreementRow]agreementRatchet{}
	data, err := os.ReadFile(agreementRatchetPath)
	if errors.Is(err, os.ErrNotExist) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 5 {
			t.Fatalf("%s: want class, kind, count, side, clause; got %q", agreementRatchetPath, line)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			t.Fatalf("%s: %q: %v", agreementRatchetPath, line, err)
		}
		if fields[3] != "symbol" && fields[3] != "export" {
			t.Fatalf("%s: %q: the side is symbol or export", agreementRatchetPath, line)
		}
		out[agreementRow{fields[0], fields[1]}] = agreementRatchet{count, fields[3], fields[4]}
	}
	return out
}

func writeAgreementRatchet(t *testing.T, recorded map[agreementRow]agreementRatchet, counts map[agreementRow]int) {
	t.Helper()
	rows := make([]agreementRow, 0, len(counts))
	for row := range counts {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].class != rows[j].class {
			return rows[i].class < rows[j].class
		}
		return rows[i].kind < rows[j].kind
	})
	var b strings.Builder
	b.WriteString("# Classes of disagreement between the api-json export and GetElementProperties over\n")
	b.WriteString("# the committed examples, counted per metamodel property (Metaclass::property, the\n")
	b.WriteString("# export's metaclass) and kind: unresolved-id (the export's id the RPC does not\n")
	b.WriteString("# resolve), metaclass (the two sides classify the element differently), value (both\n")
	b.WriteString("# answer, differently), exported-not-answered, answered-not-exported. Each row names the\n")
	b.WriteString("# side that is correct and the KerML/SysML clause it rests on. Tab-separated:\n")
	b.WriteString("# class, kind, count, side, clause. TestElementPropertiesAgreeWithTheAPIJSONExport fails\n")
	b.WriteString("# on a row not here, on a count above the one here, and on a row here the run no longer\n")
	b.WriteString("# produces; a count that fell is re-recorded with -update-agreement-ratchet.\n")
	for _, row := range rows {
		side, clause := "?", "?"
		if prior, ok := recorded[row]; ok {
			side, clause = prior.side, prior.clause
		}
		fmt.Fprintf(&b, "%s\t%s\t%d\t%s\t%s\n", row.class, row.kind, counts[row], side, clause)
	}
	if err := os.WriteFile(agreementRatchetPath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// metamodelProperty reports whether name is a property the metamodel declares on
// class or an ancestor (KerML/SysML abstract syntax), as opposed to a key the
// export writes for its own reader (`redefines`, `specializes`, `subsets`, `references`).
func metamodelProperty(class, name string) bool {
	for _, definition := range ontology.Properties() {
		if definition.Name == name && ontology.IsAncestorOrSelf(class, definition.DefiningClass) {
			return true
		}
	}
	return false
}

// TestElementPropertiesAgreeWithTheAPIJSONExport converts every committed example to
// the api-json form and asks GetElementProperties for each element the form writes,
// by the form's id, comparing every metamodel property the form states with the
// answer. Each disagreement is counted in its class (Metaclass::property, kind) and
// held against testdata/element_properties_agreement_ratchet.txt, where every class
// carries the side that is correct and the clause it rests on: a class not recorded,
// or one that grew, fails.
func TestElementPropertiesAgreeWithTheAPIJSONExport(t *testing.T) {
	paths := agreementExamples(t)
	if len(paths) == 0 {
		t.Fatal("no example models")
	}
	recorded := agreementRatchetRows(t)
	counts := map[agreementRow]int{}
	examples := map[agreementRow]string{}
	note := func(row agreementRow, example string) {
		counts[row]++
		if _, ok := examples[row]; !ok {
			examples[row] = example
		}
	}
	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	converted, elements, pairs, agreed := 0, 0, 0, 0
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
			metaclass, _ := object["@type"].(string)
			resp, err := srv.GetElementProperties(ctx, &pb.GetElementPropertiesRequest{ModelHash: parsed.ModelHash, ElementId: id})
			if err != nil {
				t.Fatalf("%s: GetElementProperties(%s): %v", path, id, err)
			}
			if resp.Error != "" {
				note(agreementRow{metaclass + "::@id", "unresolved-id"}, fmt.Sprintf("%s %s: %s", path, id, resp.Error))
				continue
			}
			elements++
			if resp.Element.Metaclass != metaclass {
				note(agreementRow{metaclass + "::@type", "metaclass"}, fmt.Sprintf("%s %s is a %s in the export and a %s in the answer", path, id, metaclass, resp.Element.Metaclass))
			}
			answered := map[string]*pb.ElementProperty{}
			for _, property := range resp.Properties {
				if !metamodelProperty(resp.Element.Metaclass, property.Name) {
					t.Errorf("%s: %s answers %s, which the metamodel declares on no ancestor of %s", path, id, property.Name, resp.Element.Metaclass)
				}
				answered[property.Name] = property
			}
			for name, value := range object {
				if name == "@id" || name == "@type" || strings.HasPrefix(name, "sysx:") || !metamodelProperty(metaclass, name) {
					continue
				}
				pairs++
				row := agreementRow{class: metaclass + "::" + name}
				property, ok := answered[name]
				if !ok {
					row.kind = "exported-not-answered"
					note(row, fmt.Sprintf("%s %s.%s", path, id, name))
					continue
				}
				delete(answered, name)
				if want, got := apiJSONSpelling(value), propertySpelling(property); want != got {
					row.kind = "value"
					note(row, fmt.Sprintf("%s %s.%s exported as %s, answered as %s", path, id, name, want, got))
					continue
				}
				agreed++
			}
			for name, property := range answered {
				if property.Unsupported != "" || len(property.Values) == 0 {
					continue
				}
				note(agreementRow{metaclass + "::" + name, "answered-not-exported"}, fmt.Sprintf("%s %s.%s answered as %s", path, id, name, propertySpelling(property)))
			}
		}
	}
	if converted == 0 || elements == 0 {
		t.Fatalf("no example converts to api-json (%d models, %d elements)", converted, elements)
	}
	if *updateAgreementRatchet {
		writeAgreementRatchet(t, recorded, counts)
	}
	fell := 0
	for row, count := range counts {
		prior, ok := recorded[row]
		switch {
		case !ok:
			t.Errorf("%s %s: %d new disagreements, e.g. %s", row.class, row.kind, count, examples[row])
		case count > prior.count:
			t.Errorf("%s %s: %d disagreements, %d recorded (%s is correct: %s); e.g. %s", row.class, row.kind, count, prior.count, prior.side, prior.clause, examples[row])
		case count < prior.count:
			fell++
		}
	}
	for row, prior := range recorded {
		if prior.side == "?" || prior.clause == "?" {
			t.Errorf("%s: %s %s carries no side or clause", agreementRatchetPath, row.class, row.kind)
		}
		if _, ok := counts[row]; !ok {
			t.Errorf("%s: %s %s no longer differs; remove the row", agreementRatchetPath, row.class, row.kind)
		}
	}
	if fell > 0 && !*updateAgreementRatchet {
		t.Logf("%d classes fell below their recorded count; re-record with -update-agreement-ratchet", fell)
	}
	t.Logf("%d of %d examples convert; %d elements, %d (element, property) pairs compared, %d agree, %d classes of disagreement", converted, len(paths), elements, pairs, agreed, len(counts))
}

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
