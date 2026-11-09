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

// updateAgreementRatchet re-records the counts under testdata/element_properties_agreement/,
// keeping each row's clause key.
var updateAgreementRatchet = flag.Bool("update-agreement-ratchet", false, "re-record the counts of tests/grpc/testdata/element_properties_agreement/<kind>.txt, keeping each row's clause key")

// agreementRatchetDir holds clauses.txt (key, side, clause: the adjudications) and one
// <kind>.txt per kind of disagreement (class, count, clause key).
const agreementRatchetDir = "testdata/element_properties_agreement"

var agreementKinds = []string{"unresolved-id", "metaclass", "value", "exported-not-answered", "answered-not-exported"}

// agreementRow is one class of disagreement between the api-json export and GetElementProperties:
// a metamodel property of a metaclass (or its @id / @type) and the kind of disagreement.
type agreementRow struct{ class, kind string }

// agreementRatchet is the recorded count of a class with the adjudication it rests on:
// the clause's key, the side that is correct (symbol or export) and the KerML/SysML clause.
type agreementRatchet struct {
	count             int
	key, side, clause string
}

type agreementClause struct{ side, text string }

func agreementClauses(t *testing.T) map[string]agreementClause {
	t.Helper()
	path := filepath.Join(agreementRatchetDir, "clauses.txt")
	out := map[string]agreementClause{}
	data, err := os.ReadFile(path)
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
		if len(fields) != 3 {
			t.Fatalf("%s: want key, side, clause; got %q", path, line)
		}
		if fields[1] != "symbol" && fields[1] != "export" {
			t.Fatalf("%s: %q: the side is symbol or export", path, line)
		}
		if _, dup := out[fields[0]]; dup {
			t.Fatalf("%s: clause %s twice", path, fields[0])
		}
		out[fields[0]] = agreementClause{fields[1], fields[2]}
	}
	return out
}

func agreementRatchetRows(t *testing.T) map[agreementRow]agreementRatchet {
	t.Helper()
	clauses := agreementClauses(t)
	out := map[agreementRow]agreementRatchet{}
	for _, kind := range agreementKinds {
		path := filepath.Join(agreementRatchetDir, kind+".txt")
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) != 3 {
				t.Fatalf("%s: want class, count, clause key; got %q", path, line)
			}
			count, err := strconv.Atoi(fields[1])
			if err != nil {
				t.Fatalf("%s: %q: %v", path, line, err)
			}
			row := agreementRow{fields[0], kind}
			if _, dup := out[row]; dup {
				t.Fatalf("%s: %s twice", path, fields[0])
			}
			recorded := agreementRatchet{count: count, key: fields[2], side: "?", clause: "?"}
			if clause, ok := clauses[fields[2]]; ok {
				recorded.side, recorded.clause = clause.side, clause.text
			} else if fields[2] != "?" {
				t.Fatalf("%s: %s: no clause %s in clauses.txt", path, fields[0], fields[2])
			}
			out[row] = recorded
		}
	}
	return out
}

func writeAgreementRatchet(t *testing.T, recorded map[agreementRow]agreementRatchet, counts map[agreementRow]int) {
	t.Helper()
	byKind := map[string][]agreementRow{}
	for row := range counts {
		byKind[row.kind] = append(byKind[row.kind], row)
	}
	for _, kind := range agreementKinds {
		rows := byKind[kind]
		sort.Slice(rows, func(i, j int) bool { return rows[i].class < rows[j].class })
		var b strings.Builder
		fmt.Fprintf(&b, "# %s disagreements between the api-json export and GetElementProperties over the\n", kind)
		b.WriteString("# committed examples, counted per metamodel property (Metaclass::property, the\n")
		b.WriteString("# export's metaclass). Tab-separated: class, count, key of the clause in clauses.txt\n")
		b.WriteString("# (the side that is correct and the KerML/SysML clause it rests on).\n")
		b.WriteString("# TestElementPropertiesAgreeWithTheAPIJSONExport fails on a row not here, on a count\n")
		b.WriteString("# above the one here, on a row here the run no longer produces, and on a ? key; a count\n")
		b.WriteString("# that fell is re-recorded with -update-agreement-ratchet.\n")
		for _, row := range rows {
			key := "?"
			if prior, ok := recorded[row]; ok {
				key = prior.key
			}
			fmt.Fprintf(&b, "%s\t%d\t%s\n", row.class, counts[row], key)
		}
		if err := os.WriteFile(filepath.Join(agreementRatchetDir, kind+".txt"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// metamodelProperty reports whether name is a property the metamodel declares on class or an ancestor,
// as opposed to a key the export writes for its own reader (`redefines`, `specializes`, `subsets`, `references`).
func metamodelProperty(class, name string) bool {
	for _, definition := range ontology.Properties() {
		if definition.Name == name && ontology.IsAncestorOrSelf(class, definition.DefiningClass) {
			return true
		}
	}
	return false
}

// TestElementPropertiesAgreeWithTheAPIJSONExport asks GetElementProperties for every element the api-json form
// writes and ratchets each disagreement class (Metaclass::property, kind) in testdata/element_properties_agreement_ratchet.txt.
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
			if strings.HasPrefix(metaclass, "sysx:") {
				// An export-only node (a sysx:Note holding a bare comment) reflects no
				// metamodel element, so the RPC has none to answer for it.
				continue
			}
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
			t.Errorf("%s/%s.txt: %s carries no clause key", agreementRatchetDir, row.kind, row.class)
		}
		if _, ok := counts[row]; !ok {
			t.Errorf("%s/%s.txt: %s no longer differs; remove the row", agreementRatchetDir, row.kind, row.class)
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
