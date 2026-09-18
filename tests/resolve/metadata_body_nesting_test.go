package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// nestedMetadataBodies annotates C four ways, each body restating a's feature
// b and, under c, U's feature d; `part x` is an ordinary nesting beside them.
var nestedMetadataBodies = map[string]string{
	"nested.kerml": `package P {
	class U { feature d; }
	class T { feature b; feature c : U; }
	metaclass M { feature a : T; }
	class C {
		@M { a { b = 1; c { d = 2; } } }
		feature x : T { feature y; feature z : U { feature w; } }
	}
	@M about C { a { b = 1; c { d = 2; } } }
	@M about C { :>> a { b = 1; c { d = 2; } } }
	metadata m : M about C { a { b = 1; c { d = 2; } } }
}`,
	"nested.sysml": `package P {
	attribute def U { attribute d; }
	attribute def T { attribute b; attribute c : U; }
	metadata def M { attribute a : T; }
	part def C {
		@M { a { b = 1; c { d = 2; } } }
		part x : T { attribute y; attribute z : U { attribute w; } }
	}
	@M about C { a { b = 1; c { d = 2; } } }
	@M about C { :>> a { b = 1; c { d = 2; } } }
	metadata m : M about C { a { b = 1; c { d = 2; } } }
}`,
}

// scopesOwnedAt returns the scopes whose owner is declared at one of the source
// offsets where marker occurs, in source order.
func scopesOwnedAt(root *symbols.Scope, src, marker string) []*symbols.Scope {
	offsets := map[int]bool{}
	for from := 0; ; {
		i := strings.Index(src[from:], marker)
		if i < 0 {
			break
		}
		offsets[from+i] = true
		from += i + 1
	}
	var out []*symbols.Scope
	var visit func(*symbols.Scope)
	visit = func(s *symbols.Scope) {
		if owner := s.Owner(); owner != nil && owner.Decl != nil && offsets[owner.Decl.Span().Offset] {
			out = append(out, s)
		}
		for _, c := range s.Children() {
			visit(c)
		}
	}
	visit(root)
	return out
}

func TestNestedMetadataBodiesAreOwnedByTheRedefinedFeature(t *testing.T) {
	for name, src := range nestedMetadataBodies {
		r, _, rootScope := resolvedDocNamed(t, name, src)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%s: %v", name, r.Diagnostics)
		}
		for _, tc := range []struct {
			marker string
			bodies int
			want   string
		}{
			{"a { b = 1;", 3, "P::M::a"},
			{":>> a { b =", 1, "P::M::a"},
			{"c { d = 2;", 4, "P::T::c"},
		} {
			scopes := scopesOwnedAt(rootScope, src, tc.marker)
			if len(scopes) != tc.bodies {
				t.Fatalf("%s: %d body scopes at %q, want %d", name, len(scopes), tc.marker, tc.bodies)
			}
			for _, scope := range scopes {
				if got := symbols.FQNOf(r.MetadataBodyOwner(scope)); got != tc.want {
					t.Errorf("%s: MetadataBodyOwner of the %q body = %q, want %s", name, tc.marker, got, tc.want)
				}
			}
		}
		for _, marker := range []string{"x : T {", "z : U {"} {
			for _, scope := range scopesOwnedAt(rootScope, src, marker) {
				if owner := r.MetadataBodyOwner(scope); owner != nil {
					t.Errorf("%s: MetadataBodyOwner of the ordinary %q body = %v, want none", name, marker, owner)
				}
			}
		}
	}
}
