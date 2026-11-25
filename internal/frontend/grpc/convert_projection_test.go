package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const projectionModelSource = `package P {
  part def Plain;
  part def Faceted;
  metadata def Facet;
  metadata facet : Facet about Faceted { note = "n"; }
}
`

func convertProjected(t *testing.T, srv *Service, projection *pb.ConvertProjection, req *pb.ConvertRequest) *pb.ConvertResponse {
	t.Helper()
	if req == nil {
		req = &pb.ConvertRequest{
			Source:     &pb.ConvertRequest_Content{Content: projectionModelSource},
			FromFormat: "sysml",
		}
	}
	req.ToFormat = "api-json"
	req.Projection = projection
	resp, err := srv.Convert(context.Background(), req)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("Convert reported %q", resp.Error)
	}
	return resp
}

func projectedIDs(t *testing.T, content string) []json.RawMessage {
	t.Helper()
	var elements []json.RawMessage
	if err := json.Unmarshal([]byte(content), &elements); err != nil {
		t.Fatalf("api-json is not an array: %v", err)
	}
	return elements
}

func rawField(t *testing.T, element json.RawMessage, key string) json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(element, &object); err != nil {
		t.Fatal(err)
	}
	return object[key]
}

func rawID(t *testing.T, element json.RawMessage) string {
	t.Helper()
	var id string
	if err := json.Unmarshal(rawField(t, element, "@id"), &id); err != nil {
		t.Fatal(err)
	}
	return id
}

func assertProjectedElementsMatchWhole(t *testing.T, whole, projected string) {
	t.Helper()
	byID := map[string]json.RawMessage{}
	for _, element := range projectedIDs(t, whole) {
		byID[rawID(t, element)] = element
	}
	for _, element := range projectedIDs(t, projected) {
		original, ok := byID[rawID(t, element)]
		if !ok {
			t.Errorf("projection emitted %s, absent from the whole corpus", rawID(t, element))
			continue
		}
		if !bytes.Equal(original, element) {
			t.Errorf("%s is not byte-identical:\nwhole: %s\nprojected: %s", rawID(t, element), original, element)
		}
	}
}

func TestConvertProjectionCapabilityIsAdvertised(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	if !slices.Contains(Capabilities(), CapabilityConvertProjection) {
		t.Errorf("capabilities %v omit %q", Capabilities(), CapabilityConvertProjection)
	}
}

func TestConvertProjectionByQualifiedName(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	whole := convertProjected(t, srv, nil, nil)
	projection := &pb.ConvertProjection{QualifiedNames: []string{"P::Faceted"}}
	reply := convertProjected(t, srv, projection, nil)
	if len(reply.Content) >= len(whole.Content) {
		t.Errorf("projected reply is %d bytes, whole is %d: not smaller", len(reply.Content), len(whole.Content))
	}
	elements := projectedIDs(t, reply.Content)
	if len(elements) != 1 || rawID(t, elements[0]) != "P__Faceted" {
		t.Fatalf("projection by qualified name emitted %s", reply.Content)
	}
	assertProjectedElementsMatchWhole(t, whole.Content, reply.Content)
}

func TestConvertProjectionByMetaclass(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	whole := convertProjected(t, srv, nil, nil)
	reply := convertProjected(t, srv, &pb.ConvertProjection{Metaclasses: []string{"PartDefinition"}}, nil)
	elements := projectedIDs(t, reply.Content)
	if len(elements) != 2 {
		t.Fatalf("projection by metaclass emitted %d elements, want 2:\n%s", len(elements), reply.Content)
	}
	for _, element := range elements {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(element, &object); err != nil {
			t.Fatal(err)
		}
		var typ string
		if err := json.Unmarshal(object["@type"], &typ); err != nil || typ != "PartDefinition" {
			t.Errorf("emitted @type = %q, want PartDefinition", typ)
		}
	}
	assertProjectedElementsMatchWhole(t, whole.Content, reply.Content)
}

func TestConvertProjectionByAnnotationType(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	whole := convertProjected(t, srv, nil, nil)
	for _, name := range []string{"P::Facet", "Facet"} {
		reply := convertProjected(t, srv, &pb.ConvertProjection{AnnotationTypes: []string{name}}, nil)
		ids := map[string]bool{}
		for _, element := range projectedIDs(t, reply.Content) {
			ids[rawID(t, element)] = true
		}
		for _, want := range []string{"P__Faceted", "P__facet", "P__facet_an0"} {
			if !ids[want] {
				t.Errorf("projection by %q did not emit %s: %v", name, want, ids)
			}
		}
		if ids["P__Plain"] {
			t.Errorf("projection by %q emitted the unannotated declaration", name)
		}
		assertProjectedElementsMatchWhole(t, whole.Content, reply.Content)
	}
}

// A selected annotation must carry the owned members and value expressions
// that let a caller follow its usage to the literal a metadata feature binds.
func TestConvertProjectionByAnnotationTypeCarriesBindings(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	whole := convertProjected(t, srv, nil, nil)
	reply := convertProjected(t, srv, &pb.ConvertProjection{AnnotationTypes: []string{"Facet"}}, nil)
	byID := map[string]json.RawMessage{}
	for _, element := range projectedIDs(t, reply.Content) {
		byID[rawID(t, element)] = element
	}
	for _, want := range []string{
		"P__facet", "P__facet_an0", "P__Faceted",
		"P__facet__note", "P__facet__note_om", "P__facet__note_pvalue", "P__facet__note_pvalue_om",
	} {
		if _, ok := byID[want]; !ok {
			t.Errorf("projection by Facet did not emit the binding element %s: %v", want, byID)
		}
	}
	reference := func(element json.RawMessage, key string) string {
		t.Helper()
		raw := rawField(t, element, key)
		var refs []struct {
			ID string `json:"@id"`
		}
		if err := json.Unmarshal(raw, &refs); err == nil {
			if len(refs) != 1 {
				t.Fatalf("%s holds %d references, want one: %s", key, len(refs), raw)
			}
			return refs[0].ID
		}
		var one struct {
			ID string `json:"@id"`
		}
		if err := json.Unmarshal(raw, &one); err != nil {
			t.Fatalf("%s is not an @id reference: %v", key, err)
		}
		return one.ID
	}
	memberID := reference(byID["P__facet"], "ownedMember")
	if memberID != "P__facet__note" {
		t.Fatalf("usage owns %q, want P__facet__note", memberID)
	}
	literalID := reference(byID[memberID], "value")
	if literalID != "P__facet__note_pvalue" {
		t.Fatalf("member's value is %q, want P__facet__note_pvalue", literalID)
	}
	var bound string
	if err := json.Unmarshal(rawField(t, byID[literalID], "value"), &bound); err != nil || bound != "n" {
		t.Errorf("bound literal reads %q (%v), want %q", bound, err, "n")
	}
	assertProjectedElementsMatchWhole(t, whole.Content, reply.Content)
}

func TestConvertProjectionUnknownNamesAreInvalidArgument(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	for _, tc := range []struct {
		name       string
		projection *pb.ConvertProjection
	}{
		{"qualified_names", &pb.ConvertProjection{QualifiedNames: []string{"P::Missing"}}},
		{"metaclasses", &pb.ConvertProjection{Metaclasses: []string{"NoSuchMetaclass"}}},
		{"annotation_types", &pb.ConvertProjection{AnnotationTypes: []string{"NoSuchFacet"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.Convert(context.Background(), &pb.ConvertRequest{
				Source: &pb.ConvertRequest_Content{Content: projectionModelSource}, FromFormat: "sysml",
				ToFormat: "api-json", Projection: tc.projection,
			})
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("Convert status = %s, want INVALID_ARGUMENT: %v", connect.CodeOf(err), err)
			}
			if !strings.Contains(err.Error(), tc.name) || !strings.Contains(err.Error(), "does not carry") {
				t.Errorf("error %q does not name the selector and the unmatched name", err)
			}
		})
	}
}

func TestConvertProjectionRefusals(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	projection := &pb.ConvertProjection{QualifiedNames: []string{"P::Faceted"}}
	t.Run("other target", func(t *testing.T) {
		_, err := srv.Convert(context.Background(), &pb.ConvertRequest{
			Source: &pb.ConvertRequest_Content{Content: projectionModelSource}, FromFormat: "sysml",
			ToFormat: "ttl", Projection: projection,
		})
		if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "api-json") {
			t.Fatalf("Convert status = %s, want api-json refusal: %v", connect.CodeOf(err), err)
		}
	})
	t.Run("capability withheld", func(t *testing.T) {
		without := mustNewServiceWithout(t, CapabilityConvertProjection)
		_, err := without.Convert(context.Background(), &pb.ConvertRequest{
			Source: &pb.ConvertRequest_Content{Content: projectionModelSource}, FromFormat: "sysml",
			ToFormat: "api-json", Projection: projection,
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityConvertProjection) {
			t.Fatalf("Convert status = %s, want UNIMPLEMENTED naming the capability: %v", connect.CodeOf(err), err)
		}
	})
}

func TestConvertProjectionOfSeveralDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := documentsModel(t, srv)
	whole := convertProjected(t, srv, nil, &pb.ConvertRequest{Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}})
	reply := convertProjected(t, srv, &pb.ConvertProjection{QualifiedNames: []string{"App::e"}}, &pb.ConvertRequest{Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}})
	elements := projectedIDs(t, reply.Content)
	if len(elements) != 1 || rawID(t, elements[0]) != "App__e" {
		t.Fatalf("model projection emitted %s", reply.Content)
	}
	if len(reply.Content) >= len(whole.Content) {
		t.Errorf("projected reply is %d bytes, whole is %d: not smaller", len(reply.Content), len(whole.Content))
	}
	assertProjectedElementsMatchWhole(t, whole.Content, reply.Content)
}

func TestConvertProjectionWithDocumentsAndCompact(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := documentsModel(t, srv)
	source := &pb.ConvertRequest_ModelHash{ModelHash: hash}
	standard := convertProjected(t, srv, &pb.ConvertProjection{QualifiedNames: []string{"App::e"}}, &pb.ConvertRequest{Source: source})
	compact := convertProjected(t, srv, &pb.ConvertProjection{QualifiedNames: []string{"App::e"}}, &pb.ConvertRequest{Source: source, Compact: true})
	var document struct {
		ElementCount int               `json:"elementCount"`
		Elements     []json.RawMessage `json:"elements"`
	}
	if err := json.Unmarshal([]byte(compact.Content), &document); err != nil {
		t.Fatalf("compact document: %v", err)
	}
	if document.ElementCount != 1 || len(document.Elements) != 1 {
		t.Fatalf("compact projection wrote %d of %d elements: %s", document.ElementCount, len(document.Elements), compact.Content)
	}
	if len(standard.Content) == 0 {
		t.Fatal("standard projection wrote nothing")
	}
}

// Body annotations are owned members rather than about relationships; their
// metaclass and metadata-definition names still select them and their owners.
func TestConvertProjectionBodyAnnotationTypes(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	source := &pb.ConvertRequest_Content{Content: `package P {
  metadata def Facet;
  part def Vehicle {
    doc /* a vehicle */
    rep language "markdown" /* body */
    @Facet { note = "n"; }
  }
  #Facet part def Hashed;
}
`}
	request := &pb.ConvertRequest{Source: source, FromFormat: "sysml"}
	whole := convertProjected(t, srv, nil, request)
	for _, tc := range []struct {
		name string
		ids  []string
	}{
		{"Documentation", []string{"P__Vehicle___400", "P__Vehicle"}},
		{"TextualRepresentation", []string{"P__Vehicle___401", "P__Vehicle"}},
		{"Facet", []string{"P__Vehicle___402", "P__Hashed___400", "P__Vehicle", "P__Hashed"}},
		{"P::Facet", []string{"P__Vehicle___402", "P__Hashed___400", "P__Vehicle", "P__Hashed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reply := convertProjected(t, srv, &pb.ConvertProjection{AnnotationTypes: []string{tc.name}}, request)
			ids := map[string]bool{}
			for _, element := range projectedIDs(t, reply.Content) {
				ids[rawID(t, element)] = true
			}
			for _, want := range tc.ids {
				if !ids[want] {
					t.Errorf("projection by %q did not emit %s: %v", tc.name, want, ids)
				}
			}
			assertProjectedElementsMatchWhole(t, whole.Content, reply.Content)
		})
	}
}

// A metadata definition in an excluded document is a bare reference in the
// document-filtered graph; the selector still resolves against the full model.
func TestConvertProjectionAnnotationTypeAcrossDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments(
			"defs.sysml", "package Defs { metadata def Facet; }\n",
			"app.sysml", "package App { private import Defs::*; part def Faceted; metadata facet : Facet about Faceted { note = \"n\"; } }\n",
		),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	source := &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}
	whole := convertProjected(t, srv, nil, &pb.ConvertRequest{Source: source, Documents: []string{"app.sysml"}})
	reply := convertProjected(t, srv, &pb.ConvertProjection{AnnotationTypes: []string{"Defs::Facet"}}, &pb.ConvertRequest{Source: source, Documents: []string{"app.sysml"}})
	ids := map[string]bool{}
	for _, element := range projectedIDs(t, reply.Content) {
		ids[rawID(t, element)] = true
	}
	for _, want := range []string{"App__facet", "App__facet_an0"} {
		if !ids[want] {
			t.Errorf("projection across documents did not emit %s: %v", want, ids)
		}
	}
	if ids["Defs__Facet"] {
		t.Errorf("projection wrote %s from the excluded document", "Defs__Facet")
	}
	assertProjectedElementsMatchWhole(t, whole.Content, reply.Content)
}

// A graph that types a declaration with a legacy KerML name is written as its
// normalized metaclass; the selector matches the @type the reply carries.
func TestConvertProjectionMatchesWrittenMetaclassFromTTL(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	converted, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: "package P { part def Plain; part def Faceted; }\n"},
		FromFormat: "sysml", ToFormat: "ttl",
	})
	if err != nil || converted.Error != "" {
		t.Fatalf("Convert to ttl: %v %s", err, converted.Error)
	}
	old := "elmt:P__Plain\n    a sysml:PartDefinition ;"
	newText := "elmt:P__Plain\n    a sysml:StructureUsage ;"
	ttl := strings.Replace(converted.Content, old, newText, 1)
	if ttl == converted.Content {
		t.Fatal("the ttl does not state P__Plain's rdf:type as expected")
	}
	source := &pb.ConvertRequest_Content{Content: ttl}
	whole := convertProjected(t, srv, nil, &pb.ConvertRequest{Source: source, FromFormat: "ttl"})
	var wholeObject map[string]json.RawMessage
	for _, element := range projectedIDs(t, whole.Content) {
		if rawID(t, element) == "P__Plain" {
			wholeObject = map[string]json.RawMessage{}
			if err := json.Unmarshal(element, &wholeObject); err != nil {
				t.Fatal(err)
			}
		}
	}
	var writtenType string
	if err := json.Unmarshal(wholeObject["@type"], &writtenType); err != nil || writtenType != "Structure" {
		t.Fatalf("P__Plain is written as %q, want Structure", writtenType)
	}
	projected := convertProjected(t, srv, &pb.ConvertProjection{Metaclasses: []string{"Structure"}}, &pb.ConvertRequest{Source: source, FromFormat: "ttl"})
	ids := map[string]bool{}
	for _, element := range projectedIDs(t, projected.Content) {
		ids[rawID(t, element)] = true
	}
	if !ids["P__Plain"] || ids["P__Faceted"] {
		t.Fatalf("projection by written metaclass wrote %v", ids)
	}
	assertProjectedElementsMatchWhole(t, whole.Content, projected.Content)
}

// The writer derives qualifiedName when a graph does not state it; selecting
// by that name must work on an api-json source too.
func TestConvertProjectionMatchesDerivedQualifiedNameFromAPIJSON(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	whole := convertProjected(t, srv, nil, nil)
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(whole.Content), &elements); err != nil {
		t.Fatal(err)
	}
	for i := range elements {
		delete(elements[i], "qualifiedName")
	}
	source, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	fromGraph := convertProjected(t, srv, nil, &pb.ConvertRequest{
		Source: &pb.ConvertRequest_Content{Content: string(source)}, FromFormat: "api-json",
	})
	want := ""
	for _, element := range projectedIDs(t, fromGraph.Content) {
		if rawID(t, element) == "P__Faceted" {
			if err := json.Unmarshal(rawField(t, element, "qualifiedName"), &want); err != nil {
				t.Fatal(err)
			}
		}
	}
	if want != "P::Faceted" {
		t.Fatalf("P__Faceted is written as %q, want P::Faceted", want)
	}
	projected := convertProjected(t, srv, &pb.ConvertProjection{QualifiedNames: []string{want}}, &pb.ConvertRequest{
		Source: &pb.ConvertRequest_Content{Content: string(source)}, FromFormat: "api-json",
	})
	ids := map[string]bool{}
	for _, element := range projectedIDs(t, projected.Content) {
		ids[rawID(t, element)] = true
	}
	if !ids["P__Faceted"] || len(ids) != 1 {
		t.Fatalf("projection by derived qualified name wrote %v", ids)
	}
	assertProjectedElementsMatchWhole(t, fromGraph.Content, projected.Content)
}

func TestConvertProjectionEmptyIsTheWholeCorpus(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	whole := convertProjected(t, srv, nil, nil)
	for _, projection := range []*pb.ConvertProjection{nil, {}, {QualifiedNames: []string{}, Metaclasses: []string{}}} {
		reply := convertProjected(t, srv, projection, nil)
		if reply.Content != whole.Content {
			t.Errorf("empty projection %v changed the reply", projection)
		}
	}
}
