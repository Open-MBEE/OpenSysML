package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A model may name a package exactly as a standard-library package is named.
// The ID then matches two symbols, and the client asked about its own.
func TestASymbolIDNamingBothAModelAndALibraryElementDenotesTheModelsOwn(t *testing.T) {
	get := parseForFacts(t, `
package Occurrences {
    part def Widget {
        attribute mass : ScalarValues::Real = 1.0;
    }
}
`)
	pkg := get("Occurrences")
	if len(pkg.ChildIds) != 1 || pkg.ChildIds[0] != "Occurrences::Widget" {
		t.Errorf("children of the model's own Occurrences = %v, want [Occurrences::Widget]", pkg.ChildIds)
	}
	if kind := get("Occurrences::Widget").Kind; kind != "partDef" {
		t.Errorf("Occurrences::Widget kind = %q, want partDef", kind)
	}
}

// A metadata annotation written as a member of a type once collapsed that
// type's members to its own: the annotation's type was resolved while the
// type's supertypes were being derived, and the answer computed under that
// guard was memoized (internal/semantic/semantics/reference.go).
func TestAMetadataAnnotationMemberDoesNotHideInheritedAttributes(t *testing.T) {
	srv := mustNewService(t, 10)
	both := make(map[string][]string)
	for _, tc := range []struct {
		name, src string
	}{
		{"annotated", `
package Demo {
    metadata def Safety;
    part def Car {
        @Safety;
        attribute mass : ScalarValues::Real = 1.0;
    }
}
`},
		{"plain", `
package Demo {
    metadata def Safety;
    part def Car {
        attribute mass : ScalarValues::Real = 1.0;
    }
}
`},
	} {
		parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{Content: tc.src},
		})
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", tc.name, err)
		}
		resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
			ModelHash: parsed.ModelHash, SymbolId: "Demo::Car",
		})
		if err != nil {
			t.Fatalf("GetSymbol(%s): %v", tc.name, err)
		}
		car := resp.Symbol
		if car == nil {
			t.Fatalf("GetSymbol(%s): %s", tc.name, resp.Error)
		}
		var names []string
		for _, a := range car.Attributes {
			names = append(names, a.Name)
		}
		if car.WithheldLibraryAttributes == 0 {
			t.Errorf("%s: no library-inherited attributes reported withheld", tc.name)
		}
		both[tc.name] = names
	}
	if strings.Join(both["annotated"], ",") != strings.Join(both["plain"], ",") {
		t.Errorf("annotated attributes = %v, want the unannotated %v", both["annotated"], both["plain"])
	}
}
