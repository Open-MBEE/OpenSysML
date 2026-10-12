package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compactDocument is the compact form as a client reads it.
type compactDocument struct {
	Format           string           `json:"format"`
	IDs              []string         `json:"ids"`
	ElementCount     int              `json:"elementCount"`
	ElementIDsAreIDs bool             `json:"elementIdsAreIds"`
	ReferenceKeys    []string         `json:"referenceKeys"`
	Elements         []map[string]any `json:"elements"`
}

// expand reads a compact document back as the standard element array: the id
// table puts each element's "@id" and each handle's id back.
func (d compactDocument) expand(t *testing.T) []map[string]any {
	t.Helper()
	bare := map[string]bool{}
	for _, key := range d.ReferenceKeys {
		bare[key] = true
	}
	var restore func(key string, value any) any
	restore = func(key string, value any) any {
		switch v := value.(type) {
		case float64:
			if bare[key] {
				return map[string]any{"@id": d.IDs[int(v)]}
			}
		case []any:
			out := make([]any, len(v))
			for i, item := range v {
				out[i] = restore(key, item)
			}
			return out
		case map[string]any:
			if handle, ok := v["@id"].(float64); ok {
				return map[string]any{"@id": d.IDs[int(handle)]}
			}
		}
		return value
	}
	out := make([]map[string]any, 0, len(d.Elements))
	for i, element := range d.Elements {
		expanded := map[string]any{"@id": d.IDs[i]}
		if d.ElementIDsAreIDs {
			expanded["elementId"] = d.IDs[i]
		}
		for key, value := range element {
			expanded[key] = restore(key, value)
		}
		out = append(out, expanded)
	}
	return out
}

func convertCompact(t *testing.T, srv *Service, req *pb.ConvertRequest) (compactDocument, string) {
	t.Helper()
	req.ToFormat = "api-json"
	req.Compact = true
	resp, err := srv.Convert(context.Background(), req)
	if err != nil || resp.Error != "" {
		t.Fatalf("Convert compact: %v %s", err, resp.GetError())
	}
	var doc compactDocument
	if err := json.Unmarshal([]byte(resp.Content), &doc); err != nil {
		t.Fatalf("the compact document is not JSON: %v", err)
	}
	return doc, resp.Content
}

func standardElements(t *testing.T, srv *Service, hash string, documents ...string) []map[string]any {
	t.Helper()
	resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, ToFormat: "api-json", Documents: documents,
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("Convert: %v %s", err, resp.GetError())
	}
	var elements []map[string]any
	if err := json.Unmarshal([]byte(resp.Content), &elements); err != nil {
		t.Fatal(err)
	}
	return elements
}

// Expanded, the compact form is the standard element array, element by element
// and key by key; for a document of a model, the elements it references but does
// not write are in the id table after the elements.
func TestConvertCompactExpandsToTheStandardForm(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := documentsModel(t, srv)
	for _, documents := range [][]string{nil, {"app.sysml"}} {
		doc, _ := convertCompact(t, srv, &pb.ConvertRequest{Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, Documents: documents})
		if doc.Format != "api-json-compact/1" {
			t.Errorf("format = %q", doc.Format)
		}
		want := standardElements(t, srv, hash, documents...)
		if doc.ElementCount != len(want) || len(doc.Elements) != len(want) {
			t.Fatalf("documents %v: %d elements written, the standard form has %d", documents, len(doc.Elements), len(want))
		}
		got := doc.expand(t)
		for i := range want {
			// The standard form round-trips through JSON, so numbers are float64 in both.
			if !reflect.DeepEqual(got[i], want[i]) {
				wantText, _ := json.Marshal(want[i])
				gotText, _ := json.Marshal(got[i])
				t.Fatalf("documents %v, element %d: the standard form writes\n%s\nthe compact form expands to\n%s", documents, i, wantText, gotText)
			}
		}
		if len(doc.IDs) < doc.ElementCount {
			t.Errorf("the id table holds %d ids for %d elements", len(doc.IDs), doc.ElementCount)
		}
	}
}

// The compact form is smaller than the standard one, and unindented.
func TestConvertCompactIsSmaller(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := documentsModel(t, srv)
	_, compact := convertCompact(t, srv, &pb.ConvertRequest{Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}})
	standard, err := srv.Convert(context.Background(), &pb.ConvertRequest{Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, ToFormat: "api-json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(compact)*2 > len(standard.Content) {
		t.Errorf("compact is %d bytes against the standard %d: not under half", len(compact), len(standard.Content))
	}
}

// omit_derived leaves out derived properties, except the ones kept.
func TestConvertCompactOmitsDerived(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := documentsModel(t, srv)
	source := &pb.ConvertRequest_ModelHash{ModelHash: hash}
	all, _ := convertCompact(t, srv, &pb.ConvertRequest{Source: source})
	lean, _ := convertCompact(t, srv, &pb.ConvertRequest{Source: source, OmitDerived: true})
	kept, _ := convertCompact(t, srv, &pb.ConvertRequest{Source: source, OmitDerived: true, KeepDerived: []string{"owner", "qualifiedName"}})
	if !all.ElementIDsAreIDs {
		t.Error("elementIdsAreIds is not set, though every element's elementId is its @id")
	}
	for _, element := range all.Elements {
		if _, written := element["elementId"]; written {
			t.Fatal("elementId is written though it is the element's id")
		}
	}
	keys := func(d compactDocument) map[string]int {
		counts := map[string]int{}
		for _, element := range d.Elements {
			for key := range element {
				counts[key]++
			}
		}
		return counts
	}
	allKeys, leanKeys, keptKeys := keys(all), keys(lean), keys(kept)
	if allKeys["owner"] == 0 || allKeys["qualifiedName"] == 0 {
		t.Fatalf("the standard properties lack owner or qualifiedName: %v", allKeys)
	}
	if leanKeys["owner"] != 0 || leanKeys["qualifiedName"] != 0 {
		t.Errorf("omit_derived still writes owner or qualifiedName: %v", leanKeys)
	}
	if keptKeys["owner"] != allKeys["owner"] || keptKeys["qualifiedName"] != allKeys["qualifiedName"] {
		t.Errorf("keep_derived writes owner %d and qualifiedName %d times, want %d and %d",
			keptKeys["owner"], keptKeys["qualifiedName"], allKeys["owner"], allKeys["qualifiedName"])
	}
	// What is not derived is untouched.
	for _, key := range []string{"declaredName", "ownedRelationship", "@type"} {
		if leanKeys[key] != allKeys[key] {
			t.Errorf("omit_derived changes %s: %d against %d", key, leanKeys[key], allKeys[key])
		}
	}
	if len(lean.Elements) != len(all.Elements) || !slices.Equal(lean.IDs[:lean.ElementCount], all.IDs[:all.ElementCount]) {
		t.Error("omit_derived changes which elements are written")
	}
}

// compact and its two companions are refused where they do not apply.
func TestConvertCompactRefusals(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	content := &pb.ConvertRequest_Content{Content: "package P;"}
	for name, req := range map[string]*pb.ConvertRequest{
		"to turtle":                    {Source: content, FromFormat: "sysml", ToFormat: "ttl", Compact: true},
		"to sysml":                     {Source: content, FromFormat: "sysml", ToFormat: "sysml", Compact: true},
		"from api-json":                {Source: &pb.ConvertRequest_Content{Content: "[]"}, FromFormat: "api-json", ToFormat: "api-json", Compact: true},
		"omit_derived without compact": {Source: content, FromFormat: "sysml", ToFormat: "api-json", OmitDerived: true},
		"keep_derived without compact": {Source: content, FromFormat: "sysml", ToFormat: "api-json", KeepDerived: []string{"owner"}},
		"keep_derived without omit":    {Source: content, FromFormat: "sysml", ToFormat: "api-json", Compact: true, KeepDerived: []string{"owner"}},
		"keep_derived not derived":     {Source: content, FromFormat: "sysml", ToFormat: "api-json", Compact: true, OmitDerived: true, KeepDerived: []string{"declaredName"}},
		"keep_derived unknown":         {Source: content, FromFormat: "sysml", ToFormat: "api-json", Compact: true, OmitDerived: true, KeepDerived: []string{"nonsense"}},
	} {
		if _, err := srv.Convert(context.Background(), req); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: err = %v, want INVALID_ARGUMENT", name, err)
		}
	}
}

// A model of one document converts the same way.
func TestConvertCompactOfOneDocument(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	doc, _ := convertCompact(t, srv, &pb.ConvertRequest{
		Source: &pb.ConvertRequest_Content{Content: "package P { part def A { attribute n : ScalarValues::Integer = 3; } }"}, FromFormat: "sysml",
	})
	want, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_Content{Content: "package P { part def A { attribute n : ScalarValues::Integer = 3; } }"}, FromFormat: "sysml", ToFormat: "api-json",
	})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal([]byte(want.Content), &elements); err != nil {
		t.Fatal(err)
	}
	if got := doc.expand(t); !reflect.DeepEqual(got, elements) {
		t.Errorf("one document expands to %d elements, differently from the standard form's %d", len(got), len(elements))
	}
}

// The document conforms to the JSON Schema the documentation publishes, and the
// invariants a schema cannot state hold: every handle is in the table, the
// elements are the table's first elementCount entries, and a reference key
// carries no number.
func TestConvertCompactConformsToItsSchema(t *testing.T) {
	file, err := os.ReadFile("../../../docs/reference/api-json-compact.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("compact.schema.json", schemaDoc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("compact.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	// The schema is not vacuous: it refuses a document whose element names its own @id.
	for name, bad := range map[string]string{
		"an element with an @id":   `{"format":"api-json-compact/1","ids":["a"],"elementCount":1,"referenceKeys":[],"elements":[{"@type":"Namespace","@id":"a"}]}`,
		"another format":           `{"format":"api-json-compact/2","ids":[],"elementCount":0,"referenceKeys":[],"elements":[]}`,
		"a negative handle":        `{"format":"api-json-compact/1","ids":["a"],"elementCount":1,"referenceKeys":[],"elements":[{"@type":"Namespace","x":{"@id":-1}}]}`,
		"a missing reference list": `{"format":"api-json-compact/1","ids":[],"elementCount":0,"elements":[]}`,
	} {
		instance, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(bad)))
		if err != nil {
			t.Fatal(err)
		}
		if schema.Validate(instance) == nil {
			t.Errorf("the schema accepts %s", name)
		}
	}
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := documentsModel(t, srv)
	for _, req := range []*pb.ConvertRequest{
		{Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}},
		{Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, Documents: []string{"app.sysml"}},
		{Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, OmitDerived: true, KeepDerived: []string{"qualifiedName"}},
	} {
		doc, text := convertCompact(t, srv, req)
		instance, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(text)))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(instance); err != nil {
			t.Fatalf("the document does not conform to its schema: %v", err)
		}
		if doc.ElementCount > len(doc.IDs) || doc.ElementCount != len(doc.Elements) {
			t.Errorf("elementCount %d, %d ids, %d elements", doc.ElementCount, len(doc.IDs), len(doc.Elements))
		}
		bare := map[string]bool{}
		for _, key := range doc.ReferenceKeys {
			bare[key] = true
		}
		var check func(key string, value any)
		check = func(key string, value any) {
			switch v := value.(type) {
			case float64:
				if !bare[key] {
					return
				}
				if v < 0 || int(v) >= len(doc.IDs) || v != float64(int(v)) {
					t.Errorf("%s: handle %v is not in the id table of %d", key, v, len(doc.IDs))
				}
			case []any:
				for _, item := range v {
					check(key, item)
				}
			case map[string]any:
				if h, ok := v["@id"].(float64); ok && (int(h) < 0 || int(h) >= len(doc.IDs)) {
					t.Errorf("%s: handle %v is not in the id table", key, h)
				}
			}
		}
		for _, element := range doc.Elements {
			for key, value := range element {
				check(key, value)
			}
		}
	}
}
