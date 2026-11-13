package grpc_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// redefinedParameterModel declares a calc whose return redefines Evaluation::result by
// position, a constraint inheriting BooleanEvaluation's unnamed return, and a
// verification inheriting VerificationCase::verdict, which redefines result.
const redefinedParameterModel = `package P {
	private import ScalarValues::*;
	calc def Calc { in x : Real; return r : Real; }
	constraint def Cons { in x : Real; x > 0 }
	verification def Ver { subject s; }
}`

// A library result a declared or inherited parameter redefines is no inherited member
// (KerML 8.3.3.1), whether library members are listed or withheld, and no directed,
// inherited or plain feature of the metamodel; an unnamed library return takes the
// name of the result its position redefines (KerML 8.3.3.3) and is addressable by it.
func TestGRPCRedefinedLibraryParametersAgreeAcrossSurfaces(t *testing.T) {
	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: redefinedParameterModel}, Language: "sysml"})
	if err != nil || parsed.GetError() != "" {
		t.Fatalf("ParseFile: %v %s", err, parsed.GetError())
	}
	const evaluationResult = "Performances::Evaluation::result"
	const booleanResult = "Performances::BooleanEvaluation::result"
	cases := []struct {
		id         string
		parameters []string // the directed features, in order
	}{
		{"P::Calc", []string{"P::Calc::x", "P::Calc::r"}},
		{"P::Cons", []string{"P::Cons::x", booleanResult}},
		{"P::Ver", []string{"P::Ver::s", "VerificationCases::VerificationCase::verdict"}},
	}
	for _, tc := range cases {
		for _, library := range []bool{false, true} {
			resp, err := srv.GetSymbol(ctx, &pb.GetSymbolRequest{ModelHash: parsed.ModelHash, SymbolId: tc.id, IncludeLibraryMembers: library})
			if err != nil || resp.GetError() != "" {
				t.Fatalf("GetSymbol(%s): %v %s", tc.id, err, resp.GetError())
			}
			var ids, named []string
			for _, member := range resp.GetSymbol().GetMembers() {
				ids = append(ids, member.GetId())
				if member.GetName() == "result" {
					named = append(named, member.GetId())
				}
			}
			if slices.Contains(ids, evaluationResult) {
				t.Errorf("%s members (library=%v) = %v, list the redefined %s", tc.id, library, ids, evaluationResult)
			}
			for _, want := range tc.parameters {
				if !slices.Contains(ids, want) {
					t.Errorf("%s members (library=%v) = %v, want %s among them", tc.id, library, ids, want)
				}
			}
			if tc.id == "P::Cons" && !slices.Equal(named, []string{booleanResult}) {
				t.Errorf("%s members named result (library=%v) = %v, want [%s]", tc.id, library, named, booleanResult)
			}
		}
		for _, property := range []string{"directedFeature", "feature", "inheritedFeature"} {
			ids := elementPropertyIDs(t, srv, parsed.ModelHash, tc.id, property)
			if slices.Contains(ids, evaluationResult) {
				t.Errorf("%s.%s = %v, lists the redefined %s", tc.id, property, ids, evaluationResult)
			}
			if property == "directedFeature" && !slices.Equal(ids, tc.parameters) {
				t.Errorf("%s.directedFeature = %v, want %v", tc.id, ids, tc.parameters)
			}
			for _, id := range tc.parameters {
				if property == "inheritedFeature" && strings.HasPrefix(id, tc.id+"::") {
					continue
				}
				if !slices.Contains(ids, id) {
					t.Errorf("%s.%s = %v, want %s among them", tc.id, property, ids, id)
				}
			}
		}
	}
	resp, err := srv.GetSymbol(ctx, &pb.GetSymbolRequest{ModelHash: parsed.ModelHash, SymbolId: booleanResult})
	if err != nil || resp.GetError() != "" {
		t.Fatalf("GetSymbol(%s): %v %s", booleanResult, err, resp.GetError())
	}
	if got := resp.GetSymbol(); got.GetId() != booleanResult || got.GetName() != "result" {
		t.Errorf("GetSymbol(%s) = %s named %q, want it named result", booleanResult, got.GetId(), got.GetName())
	}
}

// elementPropertyIDs is the qualified names GetElementProperties answers for one
// element-valued metamodel property, in order.
func elementPropertyIDs(t *testing.T, srv *grpc.Service, modelHash, elementID, property string) []string {
	t.Helper()
	resp, err := srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{ModelHash: modelHash, ElementId: elementID, Properties: []string{property}})
	if err != nil || resp.GetError() != "" {
		t.Fatalf("GetElementProperties(%s, %s): %v %s", elementID, property, err, resp.GetError())
	}
	var ids []string
	for _, prop := range resp.GetProperties() {
		if prop.GetUnsupported() != "" {
			t.Fatalf("GetElementProperties(%s, %s): unsupported: %s", elementID, property, prop.GetUnsupported())
		}
		for _, value := range prop.GetValues() {
			if element, ok := value.GetValue().(*pb.ElementPropertyValue_Element); ok {
				ids = append(ids, element.Element.GetQualifiedName())
			}
		}
	}
	return ids
}
