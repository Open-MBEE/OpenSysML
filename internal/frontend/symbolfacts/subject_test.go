package symbolfacts

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// subjectModel declares a subject in each definition kind that takes one, a specialization
// and a usage of each; the constraint, which takes no subject, declares an `in` parameter.
const subjectModel = `
package P {
    private import ScalarValues::*;
    item def Thing;
    item def Special :> Thing;
    constraint def Cons { in x : Real; }
    requirement def Req { subject s : Thing; }
    use case def UC { subject s : Thing; }
    analysis def An { subject s : Thing; }
    verification def Ver { subject s : Thing; }
    requirement def Req2 :> Req { subject s : Special; }
    use case def UC2 :> UC { subject s : Special; }
    analysis def An2 :> An { subject s : Special; }
    verification def Ver2 :> Ver { subject s : Special; }
    requirement req : Req { subject s : Special; }
    use case uc : UC { subject s : Special; }
    analysis an : An { subject s : Special; }
    verification ver : Ver { subject s : Special; }
}
`

// Every subject is one `in` ReferenceUsage parameter typed as written (SysML v2 §8.2.2.19,
// §8.3.21.11; KerML §8.3.4.6) on the symbol, query and reflective surfaces, in either body.
func TestSubjectIsAnInReferenceUsageOnEverySurface(t *testing.T) {
	idx, sc := modelWithLibrary(t, subjectModel)
	owners := map[string]string{
		"P::Req": "P::Thing", "P::UC": "P::Thing", "P::An": "P::Thing", "P::Ver": "P::Thing",
		"P::Req2": "P::Special", "P::UC2": "P::Special", "P::An2": "P::Special", "P::Ver2": "P::Special",
		"P::req": "P::Special", "P::uc": "P::Special", "P::an": "P::Special", "P::ver": "P::Special",
	}
	for owner, typ := range owners {
		id := owner + "::s"
		syms := LookupNamed(idx, id)
		if len(syms) != 1 {
			t.Errorf("%s resolves to %d symbols", id, len(syms))
			continue
		}
		sym := syms[0]
		info := Of(sym, sc)
		if info.Kind != "referenceUsage" {
			t.Errorf("%s kind = %s, want referenceUsage", id, info.Kind)
		}
		if got := info.Metadata["direction"]; got != "in" {
			t.Errorf("%s direction = %q, want in", id, got)
		}
		if got := info.Metadata["type"]; got != typ[len("P::"):] {
			t.Errorf("%s type metadata = %q, want %s", id, got, typ[len("P::"):])
		}
		if info.TypeInfo == nil || info.TypeInfo.ResolvedId != typ {
			t.Errorf("%s type info = %+v, want resolved %s", id, info.TypeInfo, typ)
		}
		if got := query.MetamodelTypeNameOf(sym); got != "ReferenceUsage" {
			t.Errorf("%s @type = %s, want ReferenceUsage", id, got)
		}
		if mc := sc.Semantics.MetaclassOf(sym); mc == nil || mc.Name != "ReferenceUsage" {
			t.Errorf("%s reflective metaclass = %v, want ReferenceUsage", id, mc)
		}
		if dir, ok := semantics.ReflectiveDirection(sym); !ok || dir != ast.DirIn {
			t.Errorf("%s reflective direction = %v (%v), want in", id, dir, ok)
		}
		if got := sc.Semantics.MembershipMetaclassOf(LookupNamed(idx, owner)[0], sym); got != "SubjectMembership" {
			t.Errorf("%s is owned through a %s, want SubjectMembership", id, got)
		}
	}
	x := LookupNamed(idx, "P::Cons::x")
	if len(x) != 1 {
		t.Fatalf("P::Cons::x resolves to %d symbols", len(x))
	}
	if info := Of(x[0], sc); info.Kind != "referenceUsage" || info.Metadata["direction"] != "in" || info.Metadata["type"] != "Real" {
		t.Errorf("P::Cons::x = %s %v, want an in referenceUsage of Real", info.Kind, info.Metadata)
	}
}
