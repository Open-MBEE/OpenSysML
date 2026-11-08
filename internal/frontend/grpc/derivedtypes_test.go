package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// derivedTypesModel types features through subsetting, reference subsetting
// and redefinition rather than a written `: T`.
const derivedTypesModel = `
package P {
    part def T;
    part def V;
    part def Owner {
        part q : T;
        part pSub :> q;
        ref part pRef ::> q;
        part pBoth : V :> q;
        attribute a : ScalarValues::Real;
        part inner { ref part q ::> q; }
    }
    part def SubOwner :> Owner {
        part :>> q;
    }
    part tuned : Owner {
        attribute :>> a = 3.5;
    }
}
`

func typeIDs(info *pb.TypeInfo) string {
	var ids []string
	for _, ref := range info.GetTypes() {
		ids = append(ids, ref.GetId()+"/"+ref.GetKind())
	}
	return strings.Join(ids, ",")
}

// TestTypeInfoDerivedTypes verifies types reports Feature::type while declared
// and resolved_id keep the written typing.
func TestTypeInfoDerivedTypes(t *testing.T) {
	get := parseForFacts(t, derivedTypesModel)
	cases := []struct{ fqn, declared, resolved, types string }{
		{"P::Owner::q", "T", "P::T", "P::T/partDef"},
		{"P::Owner::pSub", "", "", "P::T/partDef"},
		{"P::Owner::pRef", "", "", "P::T/partDef"},
		{"P::Owner::pBoth", "V", "P::V", "P::V/partDef,P::T/partDef"},
		{"P::SubOwner::q", "", "", "P::T/partDef"},
		{"P::tuned::a", "", "", "ScalarValues::Real/attributeDef"},
	}
	for _, tc := range cases {
		info := get(tc.fqn).TypeInfo
		if info == nil {
			t.Errorf("%s: no type info", tc.fqn)
			continue
		}
		if info.Declared != tc.declared || info.ResolvedId != tc.resolved {
			t.Errorf("%s: written typing %q/%q, want %q/%q", tc.fqn, info.Declared, info.ResolvedId, tc.declared, tc.resolved)
		}
		if got := typeIDs(info); got != tc.types {
			t.Errorf("%s: types %q, want %q", tc.fqn, got, tc.types)
		}
	}
}

// TestTypeInfoDerivedTypesAgreeWithElementProperties verifies the Symbol
// surface answers Feature::type as GetElementProperties does. The element form
// keeps a written typing as stated (pBoth: V), so there the derived list only
// has to contain it.
func TestTypeInfoDerivedTypesAgreeWithElementProperties(t *testing.T) {
	r := parseForElementProperties(t, derivedTypesModel)
	for _, tc := range []struct {
		fqn   string
		exact bool
	}{
		{"P::Owner::pSub", true}, {"P::Owner::pRef", true}, {"P::SubOwner::q", true}, {"P::tuned::a", true},
		{"P::Owner::pBoth", false},
	} {
		fromProperties := elementNames(r.property(tc.fqn, "type").Values)
		resp, err := r.srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: r.hash, SymbolId: tc.fqn})
		if err != nil || resp.Symbol == nil {
			t.Fatalf("GetSymbol(%s): %v %s", tc.fqn, err, resp.GetError())
		}
		var fromSymbol []string
		for _, ref := range resp.Symbol.GetTypeInfo().GetTypes() {
			fromSymbol = append(fromSymbol, ref.GetId())
		}
		if tc.exact && strings.Join(fromSymbol, ",") != strings.Join(fromProperties, ",") {
			t.Errorf("%s: type_info.types %v, element_properties type %v", tc.fqn, fromSymbol, fromProperties)
		}
		if len(fromProperties) == 0 || !containsAll(fromSymbol, fromProperties...) {
			t.Errorf("%s: type_info.types %v does not contain element_properties type %v", tc.fqn, fromSymbol, fromProperties)
		}
	}
}

// TestSpecializationsReferences verifies reference subsetting is reported as a
// generalization edge of kind "references".
func TestSpecializationsReferences(t *testing.T) {
	get := parseForFacts(t, derivedTypesModel)
	specs := get("P::Owner::pRef").Specializations
	if len(specs) != 1 {
		t.Fatalf("got %d specializations, want 1", len(specs))
	}
	spec := specs[0]
	if spec.Kind != "references" || spec.Declared != "q" || spec.TargetId != "P::Owner::q" || spec.TargetKind != "partUsage" {
		t.Errorf("got %s %q -> %q (%s), want references \"q\" -> \"P::Owner::q\" (partUsage)", spec.Kind, spec.Declared, spec.TargetId, spec.TargetKind)
	}
	// A referencing feature named like its target resolves outside itself.
	inner := get("P::Owner::inner::q").Specializations
	if len(inner) != 1 || inner[0].TargetId != "P::Owner::q" {
		t.Errorf("inner::q specializations %v, want one references edge to P::Owner::q", inner)
	}
}
