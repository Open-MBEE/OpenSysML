package grpc_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

func runConvertLibraryReferencesCase(t *testing.T, srv *grpc.Service, ctx context.Context, hash string, tc conformanceCase) {
	t.Helper()
	for _, compact := range []bool{false, true} {
		for _, legacy := range []bool{false, true} {
			runConvertOwnedLibraryReferences(t, srv, ctx, hash, tc, compact, legacy)
			response, err := srv.Convert(ctx, &pb.ConvertRequest{
				Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, ToFormat: "api-json",
				Compact: compact, IncludeLibraryStubs: legacy, InheritedMembers: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if response.Error != "" {
				t.Fatal(response.Error)
			}
			if len(response.LibraryReferences) != len(tc.LibraryReferences) {
				t.Fatalf("table = %v", response.LibraryReferences)
			}
			for i, want := range tc.LibraryReferences {
				got := response.LibraryReferences[i]
				if got.Id != want.Id || got.QualifiedName != want.QualifiedName || got.Type != want.Type || got.LibraryPath != want.LibraryPath {
					t.Fatalf("table[%d] = %v, want %v", i, got, want)
				}
			}
			var elements []map[string]any
			if compact {
				var doc struct {
					Elements []map[string]any    `json:"elements"`
					Table    []map[string]string `json:"libraryReferences"`
				}
				if err := json.Unmarshal([]byte(response.Content), &doc); err != nil {
					t.Fatal(err)
				}
				elements = doc.Elements
				want := make([]map[string]string, len(tc.LibraryReferences))
				for i, ref := range tc.LibraryReferences {
					want[i] = map[string]string{"@id": ref.Id, "@type": ref.Type, "qualifiedName": ref.QualifiedName}
					if ref.LibraryPath != "" {
						// An unnamed member is named by its positional path; its qualifiedName stays, empty.
						want[i]["libraryPath"] = ref.LibraryPath
					}
				}
				if !reflect.DeepEqual(doc.Table, want) {
					t.Fatalf("literal compact table = %v, want %v", doc.Table, want)
				}
			} else if err := json.Unmarshal([]byte(response.Content), &elements); err != nil {
				t.Fatal(err)
			}
			stubs := 0
			for _, element := range elements {
				if element["isLibraryElement"] == true {
					stubs++
				}
			}
			wantStubs := 0
			if legacy {
				wantStubs = len(tc.LibraryReferences)
			}
			if stubs != wantStubs {
				t.Fatalf("legacy=%v compact=%v: %d stubs, want %d", legacy, compact, stubs, wantStubs)
			}
		}
	}
}

// The owned structure names a strict part of the case's table, the same with and without the
// legacy stubs, and writes a stub for exactly those.
func runConvertOwnedLibraryReferences(t *testing.T, srv *grpc.Service, ctx context.Context, hash string, tc conformanceCase, compact, legacy bool) {
	t.Helper()
	response, err := srv.Convert(ctx, &pb.ConvertRequest{
		Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, ToFormat: "api-json", Compact: compact, IncludeLibraryStubs: legacy,
	})
	if err != nil || response.Error != "" {
		t.Fatalf("owned convert: %v %v", response.GetError(), err)
	}
	complete := make(map[string]bool, len(tc.LibraryReferences))
	for _, ref := range tc.LibraryReferences {
		complete[ref.Id] = true
	}
	owned := make(map[string]bool, len(response.LibraryReferences))
	for _, ref := range response.LibraryReferences {
		if !complete[ref.Id] {
			t.Fatalf("the owned table names %v, which the complete one does not", ref)
		}
		owned[ref.Id] = true
	}
	if len(owned) == 0 || len(owned) >= len(complete) {
		t.Fatalf("the owned table has %d entries of the complete %d, want a strict non-empty part", len(owned), len(complete))
	}
	content := []byte(response.Content)
	if compact {
		var doc struct {
			Elements json.RawMessage `json:"elements"`
		}
		if err := json.Unmarshal(content, &doc); err != nil {
			t.Fatal(err)
		}
		content = doc.Elements
	}
	var elements []map[string]any
	if err := json.Unmarshal(content, &elements); err != nil {
		t.Fatal(err)
	}
	stubs := 0
	for _, element := range elements {
		if element["isLibraryElement"] != true {
			continue
		}
		stubs++
		// The compact form leaves a stub's id to its table entry.
		if id, _ := element["@id"].(string); !compact && !owned[id] {
			t.Fatalf("legacy=%v: stub %v is not in the owned table", legacy, element)
		}
	}
	if want := map[bool]int{true: len(owned)}[legacy]; stubs != want {
		t.Fatalf("legacy=%v compact=%v: %d stubs, want %d", legacy, compact, stubs, want)
	}
}
