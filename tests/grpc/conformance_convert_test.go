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
			response, err := srv.Convert(ctx, &pb.ConvertRequest{
				Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, ToFormat: "api-json",
				Compact: compact, IncludeLibraryStubs: legacy,
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
				if got.Id != want.Id || got.QualifiedName != want.QualifiedName || got.Type != want.Type {
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
