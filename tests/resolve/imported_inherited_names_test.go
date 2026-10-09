package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
)

// A type's memberships are its owned, inherited and imported ones together
// (KerML §8.3.2.4.5), so a member a subtype imports is indistinguishable from
// one it inherits of the same name; the warning names the inherited member as
// such, beside the import that brought the other.
func TestImportedNameCollidingWithInheritedNameWarns(t *testing.T) {
	for name, tc := range map[string]struct{ src, want, at string }{
		"a wildcard import of a definition": {
			src: `package A { part def Base { part def X; } }
package Q { part def X; }
package C { part def Sub :> A::Base { private import Q::*; } }`,
			want: "Duplicate of imported member name 'X': Q::X (import Q::*), A::Base::X (inherited)",
			at:   "Q",
		},
		"an explicit import of a definition": {
			src: `package A { part def Base { part def X; } }
package Q { part def X; }
package C { part def Sub :> A::Base { private import Q::X; } }`,
			want: "Duplicate of imported member name 'X': Q::X (import Q::X), A::Base::X (inherited)",
			at:   "Q::X",
		},
		"a wildcard import of a usage": {
			src: `package A { part def Base { part x; } }
package Q { part x; }
package C { part def Sub :> A::Base { private import Q::*; } }`,
			want: "Duplicate of imported member name 'x': Q::x (import Q::*), A::Base::x (inherited)",
			at:   "Q",
		},
	} {
		t.Run(name, func(t *testing.T) {
			r, _, _ := resolvedDoc(t, tc.src)
			if len(r.Diagnostics) != 1 {
				t.Fatalf("got %d diagnostics, want 1: %v", len(r.Diagnostics), r.Diagnostics)
			}
			d := r.Diagnostics[0]
			if !d.Warning || d.Code != resolve.CodeNameConflict || d.Message != tc.want {
				t.Errorf("got warning=%v %s %q, want a warning %s %q", d.Warning, d.Code, d.Message, resolve.CodeNameConflict, tc.want)
			}
			if got := tc.src[d.Span.Offset:d.Span.End()]; got != tc.at {
				t.Errorf("diagnostic sits on %q, want %q", got, tc.at)
			}
		})
	}
}

// Importing what a supertype's member is a redefinition of is no collision:
// the redefined feature is no longer inherited (SysML v2 §7.6.1).
func TestImportedNameMatchingARedefinedInheritedNameIsDistinguishable(t *testing.T) {
	const src = `package A { part def Base { part x; } }
package Q { part x; }
package C { part def Sub :> A::Base { private import Q::*; part y :>> x; } }`
	r, _, _ := resolvedDoc(t, src)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("got diagnostics %v, want none", r.Diagnostics)
	}
}
