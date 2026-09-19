package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestF99BodyDeclarationsAreLocalMembers(t *testing.T) {
	root := build(t, `
		package P {
			calc def C {
				in xs : Integer[*];
				xs->collect {
					in i : Integer;
					private attribute doubled : Integer = i * 2;
					doubled
				}
			}
		}
	`)

	bodies := findBodyScopes(root)
	if len(bodies) != 1 {
		t.Fatalf("body-expression scopes = %d, want 1", len(bodies))
	}
	body := bodies[0]
	sym, ok := body.LookupLocal("doubled")
	if !ok {
		t.Fatal("body declaration doubled is not a scope member")
	}
	if sym.Visibility != ast.VisibilityPrivate {
		t.Errorf("doubled visibility = %v, want private", sym.Visibility)
	}
	if sym.Kind != SymbolAttributeUsage {
		t.Errorf("doubled kind = %v, want attributeUsage", sym.Kind)
	}
	if sym.OwnerScope != body {
		t.Error("doubled owner scope is not the expression body scope")
	}
	if got, ok := LookupBodyLocal(body, "doubled"); !ok || got != sym {
		t.Errorf("LookupBodyLocal(doubled) = %v, %v; want %v, true", got, ok, sym)
	}
}

func TestF99BodyDeclarationsResolveForwardAndNested(t *testing.T) {
	root := build(t, `
		package P {
			calc def C {
				in xs : Integer[*];
				xs->collect {
					in i : Integer;
					private attribute computed : Integer = laterValue + 1;
					private attribute laterValue : Integer = i;
					xs->collect {
						in j : Integer;
						computed + j
					}
				}
			}
		}
	`)

	bodies := findBodyScopes(root)
	if len(bodies) != 2 {
		t.Fatalf("body-expression scopes = %d, want 2", len(bodies))
	}
	outer, inner := bodies[0], bodies[1]
	if inner.Parent() != outer {
		t.Fatalf("inner body parent = %v, want outer body", inner.Parent())
	}
	for _, name := range []string{"computed", "laterValue"} {
		if _, ok := outer.LookupLocal(name); !ok {
			t.Fatalf("outer body does not contain forward declaration %q", name)
		}
		if got, ok := LookupBodyLocal(outer, name); !ok || got.OwnerScope != outer {
			t.Errorf("outer LookupBodyLocal(%q) = %v, %v", name, got, ok)
		}
		if got, ok := LookupBodyLocal(inner, name); !ok || got.OwnerScope != outer {
			t.Errorf("nested LookupBodyLocal(%q) = %v, %v; want enclosing declaration", name, got, ok)
		}
	}
}

func TestF99BodyDeclarationsDoNotEscape(t *testing.T) {
	root := build(t, `
		package P {
			calc def C {
				in xs : Integer[*];
				xs->collect {
					in i : Integer;
					private attribute innerOnly : Integer = i;
					innerOnly
				}
			}
			attribute outside : Integer = innerOnly;
		}
	`)

	bodies := findBodyScopes(root)
	if len(bodies) != 1 {
		t.Fatalf("body-expression scopes = %d, want 1", len(bodies))
	}
	if _, ok := bodies[0].LookupLocal("innerOnly"); !ok {
		t.Fatal("body declaration innerOnly missing from body scope")
	}
	pkg, ok := root.LookupLocal("P")
	if !ok {
		t.Fatal("package P not found")
	}
	if _, ok := pkg.Scope.LookupLocal("innerOnly"); ok {
		t.Fatal("body declaration escaped into the enclosing package scope")
	}
	if _, ok := LookupBodyLocal(pkg.Scope, "innerOnly"); ok {
		t.Fatal("LookupBodyLocal found a body declaration outside its body")
	}
}
