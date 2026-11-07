package symbolfacts

import (
	"strings"
	"testing"
)

// derivedTypeModel types features through subsetting, reference subsetting
// and redefinition rather than a written `: T` (KerML 8.3.3.3).
const derivedTypeModel = `
package P {
    part def T;
    part def V;
    part def Owner {
        part q : T;
        part pSub :> q;
        ref part pRef ::> q;
        part pBoth : V :> q;
        attribute a : ScalarValues::Real;
    }
    part def SubOwner :> Owner {
        part :>> q;
    }
    part tuned : Owner {
        attribute :>> a = 3.5;
    }
}
`

func derivedTypeFacts(t *testing.T, id string) *Info {
	t.Helper()
	idx, sc := modelWithLibrary(t, derivedTypeModel)
	syms := LookupNamed(idx, id)
	if len(syms) != 1 {
		t.Fatalf("%s resolves to %d symbols", id, len(syms))
	}
	return Of(syms[0], sc)
}

func typeRefIDs(refs []*TypeRef) string {
	var ids []string
	for _, ref := range refs {
		ids = append(ids, ref.Id+"/"+ref.Kind)
	}
	return strings.Join(ids, ",")
}

func TestTypeInfoTypesDeriveThroughSubsettingAndRedefinition(t *testing.T) {
	for _, tc := range []struct{ id, declared, resolved, types string }{
		{"P::Owner::q", "T", "P::T", "P::T/partDef"},
		{"P::Owner::pSub", "", "", "P::T/partDef"},
		{"P::Owner::pRef", "", "", "P::T/partDef"},
		{"P::Owner::pBoth", "V", "P::V", "P::V/partDef,P::T/partDef"},
		{"P::SubOwner::q", "", "", "P::T/partDef"},
		{"P::tuned::a", "", "", "ScalarValues::Real/attributeDef"},
	} {
		info := derivedTypeFacts(t, tc.id).TypeInfo
		if info == nil {
			t.Errorf("%s: no type info", tc.id)
			continue
		}
		if info.Declared != tc.declared || info.ResolvedId != tc.resolved {
			t.Errorf("%s: written typing %q/%q, want %q/%q", tc.id, info.Declared, info.ResolvedId, tc.declared, tc.resolved)
		}
		if got := typeRefIDs(info.Types); got != tc.types {
			t.Errorf("%s: types %q, want %q", tc.id, got, tc.types)
		}
	}
}

func TestTypeInfoTypesAbsentOnADefinition(t *testing.T) {
	if info := derivedTypeFacts(t, "P::Owner").TypeInfo; info == nil || len(info.Types) != 0 {
		t.Errorf("definition type info %+v, want no derived types", info)
	}
}

func TestSpecializationsReportReferenceSubsetting(t *testing.T) {
	specs := derivedTypeFacts(t, "P::Owner::pRef").Specializations
	if len(specs) != 1 || specs[0].Kind != "references" || specs[0].Declared != "q" ||
		specs[0].TargetId != "P::Owner::q" || specs[0].TargetKind != "partUsage" {
		t.Fatalf("specializations %+v, want one references edge to P::Owner::q", specs)
	}
}
