package grpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A query's elementId is the elementId Convert writes for the same element, so a
// query result joins the converted graph: across documents, for an unnamed
// member (by its positional @id), a quoted name, a declared ElementId and
// elements of two identity scopes, whose API JSON @id the scope qualifies while
// elementId stays the element's own (#733).
func TestQueryElementIDIsTheIDConvertWrites(t *testing.T) {
	for _, tc := range []struct {
		name      string
		documents []string
		scoped    bool
	}{
		{name: "two documents", documents: []string{
			"lib.sysml", "package Lib { part def Engine { attribute mass : ScalarValues::Real; } }\n",
			"app.sysml", "package App { private import Lib::*; part e : Engine; }\n",
		}},
		{name: "unnamed and quoted", documents: []string{
			"q.sysml", "package Q { part def 'Fuel Pump'; part def A { part : 'Fuel Pump'; part p : 'Fuel Pump'; } }\n",
		}},
		{name: "declared id", documents: []string{
			"d.sysml", "package D { @IdentityMetadata::ProjectRef { projectId = \"d\"; branch = \"main\"; } part def A { @IdentityMetadata::ElementId { id = \"a-declared-id\"; } } }\n",
		}},
		{name: "two scopes", scoped: true, documents: []string{
			"one.sysml", "package P { @IdentityMetadata::ProjectRef { projectId = \"one\"; branch = \"main\"; } part def A; }\n",
			"two.sysml", "package R { @IdentityMetadata::ProjectRef { projectId = \"two\"; branch = \"main\"; } part def B; }\n",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := mustNewService(t, 10)
			defer srv.Close()
			parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments(tc.documents...)})
			if err != nil {
				t.Fatalf("ParseSources: %v", err)
			}
			if len(parsed.Diagnostics) > 0 {
				t.Fatalf("the model does not parse cleanly: %v", parsed.Diagnostics)
			}
			converted, err := srv.Convert(context.Background(), &pb.ConvertRequest{
				Source:   &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash},
				ToFormat: "api-json",
			})
			if err != nil || converted.Error != "" {
				t.Fatalf("Convert: %v %s", err, converted.GetError())
			}
			var elements []map[string]any
			if err := json.Unmarshal([]byte(converted.Content), &elements); err != nil {
				t.Fatal(err)
			}
			written, atIDs := map[string]string{}, map[string]string{}
			for _, e := range elements {
				if name, ok := e["qualifiedName"].(string); ok {
					written[name], _ = e["elementId"].(string)
					atIDs[name], _ = e["@id"].(string)
				}
			}
			queried, err := srv.Query(context.Background(), &pb.QueryRequest{
				ModelHash: parsed.ModelHash,
				Query:     &pb.Query{Select: []string{"@id", "elementId"}},
			})
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if len(queried.Elements) == 0 {
				t.Fatal("the query selected nothing")
			}
			for _, e := range queried.Elements {
				want, ok := written[e.Id]
				if !ok {
					t.Errorf("%s: Convert writes no element of that name", e.Id)
					continue
				}
				got := e.Properties["elementId"]
				if got != want {
					t.Errorf("%s: elementId %q, Convert writes elementId %q", e.Id, got, want)
				}
				// Where no identity scope qualifies the IRIs, the API JSON @id
				// is the elementId, so a result joins the graph by @id too.
				if !tc.scoped && got != atIDs[e.Id] {
					t.Errorf("%s: elementId %q, Convert writes @id %q", e.Id, got, atIDs[e.Id])
				}
			}
		})
	}
}

// A standard library element's elementId is its normative id, the id every
// reference to it in a converted graph carries.
func TestQueryElementIDOfALibraryElementIsItsNormativeID(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("p.sysml", "package P { attribute speed : ScalarValues::Real; }\n"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	queried, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{Scope: []string{"ScalarValues::Real"}, Select: []string{"elementId"}},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(queried.Elements) == 0 {
		t.Fatal("the query selected nothing")
	}
	// uuid5(uuid5(NAMESPACE_URL, "https://www.omg.org/spec/KerML/ScalarValues"), "ScalarValues::Real")
	if got := queried.Elements[0].Properties["elementId"]; got != "14c0aa22-5489-59b5-b438-ded26e83ba31" {
		t.Errorf("ScalarValues::Real: elementId %q, want its normative id", got)
	}
}

// A copy of a bundled library file parsed as a model is the library, as a
// conversion analyses it: its elements' elementIds are their normative ids.
func TestQueryElementIDOfALibraryCopyIsNormative(t *testing.T) {
	library, err := os.ReadFile(filepath.Join("..", "..", "workspace", "libs", "stdlib",
		"Kernel Libraries", "Kernel Data Type Library", "ScalarValues.kerml"))
	if err != nil {
		t.Fatal(err)
	}
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("ScalarValues.kerml", string(library)),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	queried, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{Scope: []string{"ScalarValues::Real"}, Select: []string{"elementId"}},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(queried.Elements) == 0 {
		t.Fatal("the query selected nothing")
	}
	if got := queried.Elements[0].Properties["elementId"]; got != "14c0aa22-5489-59b5-b438-ded26e83ba31" {
		t.Errorf("ScalarValues::Real in a library copy: elementId %q, want its normative id", got)
	}
}

// A model a conversion refuses has no elementIds: a query reading elementId
// fails with the refusal, and one that does not is answered as before.
func TestQueryElementIDOfAModelConvertRefuses(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments(
			"a.sysml", "package P { part def A; }\n",
			"b.sysml", "package P { part def B; }\n"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	converted, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}, ToFormat: "api-json",
	})
	if err != nil || converted.Error == "" {
		t.Fatalf("the model converted, so the case tests nothing: %v %q", err, converted.GetError())
	}
	_, err = srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{Select: []string{"@id", "elementId"}},
	})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("selecting elementId of a model Convert refuses: err = %v, want FAILED_PRECONDITION", err)
	}
	if _, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{Select: []string{"@id"}},
	}); err != nil {
		t.Errorf("a query not reading elementId failed: %v", err)
	}
	// An empty select reports every property but elementId, which only a query
	// naming it converts the model for.
	all, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: parsed.ModelHash, Query: &pb.Query{}})
	if err != nil {
		t.Fatalf("an empty select failed: %v", err)
	}
	for _, e := range all.Elements {
		if _, ok := e.Properties["elementId"]; ok {
			t.Errorf("%s: an empty select reported elementId", e.Id)
		}
	}
}

// A library element a model annotates with an id of its own is referenced by
// name in a conversion (no id is written for it), so it has no elementId.
func TestQueryElementIDOfAnAnnotatedLibraryElementIsAbsent(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	model := "package P {\n" +
		"    @IdentityMetadata::ProjectRef { projectId = \"p\"; branch = \"main\"; }\n" +
		"    metadata sid : IdentityMetadata::ElementId about ScalarValues::Boolean { id = \"custom-boolean\"; }\n" +
		"    attribute b : ScalarValues::Boolean;\n" +
		"}\n"
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments("p.sysml", model)})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	converted, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}, ToFormat: "api-json",
	})
	if err != nil || converted.Error != "" {
		t.Fatalf("Convert: %v %s", err, converted.GetError())
	}
	var elements []map[string]any
	if err := json.Unmarshal([]byte(converted.Content), &elements); err != nil {
		t.Fatal(err)
	}
	for _, e := range elements {
		if e["@type"] == "FeatureTyping" {
			if ref, _ := e["type"].(map[string]any); ref["@ref"] != "ScalarValues::Boolean" {
				t.Fatalf("the conversion no longer references Boolean by name (%v); revisit this case", e["type"])
			}
		}
	}
	queried, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{Scope: []string{"ScalarValues::Boolean"}, Select: []string{"elementId"}},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(queried.Elements) == 0 {
		t.Fatal("the query selected nothing")
	}
	if got, ok := queried.Elements[0].Properties["elementId"]; ok {
		t.Errorf("ScalarValues::Boolean: elementId %q, but the conversion writes no id for it", got)
	}
}
