package symbolfacts

import (
	"strings"
	"testing"
)

const inheritedParameterModel = `
package P {
    private import ScalarValues::*;
    item def Thing;
    requirement def Req { subject s : Thing; }
    use case def UC { subject s : Thing; }
    analysis def An { subject s : Thing; return r : Real; }
    verification def Ver { subject s : Thing; }
    verification def Ver2 :> Ver { subject s : Thing; }
}
`

func memberNames(members []*Member) string {
	var names []string
	for _, m := range members {
		names = append(names, m.Name)
	}
	return strings.Join(names, ",")
}

// A library-inherited return parameter (`result`, `verdict`) is part of the signature,
// so it is listed like a declared `return r`; a declared `return` masks the one it redefines.
func TestInheritedReturnParametersAreListed(t *testing.T) {
	idx, sc := modelWithLibrary(t, inheritedParameterModel)
	for id, want := range map[string]struct{ members, from string }{
		"P::Req":  {"s,result", "Requirements::RequirementConstraintCheck"},
		"P::UC":   {"s,result", "Cases::Case"},
		"P::An":   {"s,r", ""},
		"P::Ver":  {"s,verdict", "VerificationCases::VerificationCase"},
		"P::Ver2": {"s,verdict", "VerificationCases::VerificationCase"},
	} {
		sym := LookupNamed(idx, id)[0]
		info := Of(sym, sc)
		if got := memberNames(info.Members); got != want.members {
			t.Errorf("%s members = %s, want %s", id, got, want.members)
		}
		if info.WithheldLibraryMembers == 0 {
			t.Errorf("%s withholds no library member", id)
		}
		last := info.Members[len(info.Members)-1]
		if want.from == "" {
			if last.Inherited || last.Library {
				t.Errorf("%s %s is reported as inherited from %q", id, last.Name, last.InheritedFrom)
			}
			continue
		}
		if !last.Inherited || !last.Library || last.InheritedFrom != want.from {
			t.Errorf("%s %s inherited from %q (library %v), want %s", id, last.Name, last.InheritedFrom, last.Library, want.from)
		}
		full := OfWith(sym, sc, Options{IncludeLibraryMembers: true})
		if n := strings.Count(","+memberNames(full.Members)+",", ","+last.Name+","); n != 1 {
			t.Errorf("%s lists %s %d times with library members included", id, last.Name, n)
		}
	}
}
