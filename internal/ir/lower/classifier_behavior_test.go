package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A note beside a behavior a type binds describes the binding, so the binding
// still names the element holding the body rather than stating one itself.
func TestClassifierBehaviorAnnotationIsNotABody(t *testing.T) {
	behaviors := classifierBehaviorsIn(t, `
		package test {
			state def Modes;
			action def Report;

			part def Monitor {
				exhibit state modes : Modes {
					doc /* the operating modes */
					comment /* watched closely */
				}
				perform action report : Report {
					doc /* the report */
				}
			}
		}
	`)

	if len(behaviors) != 2 {
		t.Fatalf("expected the type to bind 2 behaviors, got %d", len(behaviors))
	}
	for _, behavior := range behaviors {
		if behavior.StatesBody {
			t.Errorf("%s %q states a body, but its members only annotate the binding",
				behavior.Kind, behavior.Name)
		}
	}
}

// A binding that both annotates itself and states a body still states one.
func TestClassifierBehaviorAnnotatedBodyIsABody(t *testing.T) {
	behaviors := classifierBehaviorsIn(t, `
		package test {
			part def Monitor {
				exhibit state modes {
					doc /* the operating modes */
					entry; then start;
					state start;
					state idle;
					succession first start then idle;
				}
			}
		}
	`)

	if len(behaviors) != 1 {
		t.Fatalf("expected the type to bind 1 behavior, got %d", len(behaviors))
	}
	if !behaviors[0].StatesBody {
		t.Error("an annotated machine body was read as a binding that states none")
	}
}

// A feature redefinition configures the performance occurrence without
// replacing the behavior body supplied by the machine's type.
func TestClassifierBehaviorAttributeRedefinitionIsNotABody(t *testing.T) {
	behaviors := classifierBehaviorsIn(t, `
		package test {
			state def Modes { attribute count = 0; }
			part def Monitor {
				exhibit state modes : Modes {
					attribute redefines count = 5;
				}
			}
		}
	`)

	if len(behaviors) != 1 {
		t.Fatalf("expected the type to bind 1 behavior, got %d", len(behaviors))
	}
	if behaviors[0].StatesBody {
		t.Error("an attribute redefinition was read as a replacement behavior body")
	}
}

// An attribute declared by an inline exhibited state belongs to that machine's
// body rather than configuring a separately named definition.
func TestClassifierBehaviorDeclaredAttributeIsABody(t *testing.T) {
	behaviors := classifierBehaviorsIn(t, `
		package test {
			part def Monitor {
				exhibit state modes {
					attribute count = 0;
				}
			}
		}
	`)

	if len(behaviors) != 1 {
		t.Fatalf("expected the type to bind 1 behavior, got %d", len(behaviors))
	}
	if !behaviors[0].StatesBody {
		t.Error("a declared attribute was not read as part of the inline behavior body")
	}
}

// The `in` and `inout` members of a binding are its arguments; an `out` member
// with a value declares what the behavior answers, so it is not one.
func TestClassifierBehaviorOutMemberIsNotAnArgument(t *testing.T) {
	behaviors := classifierBehaviorsIn(t, `
		package test {
			action def Report { in n : Integer; inout m : Integer; out total : Integer; }
			part def Monitor {
				attribute level : Integer = 1;
				perform action report : Report { in n = level; inout m = level; out total = 7; }
			}
		}
	`)

	if len(behaviors) != 1 {
		t.Fatalf("expected the type to bind 1 behavior, got %d", len(behaviors))
	}
	var names []string
	for _, arg := range behaviors[0].Arguments {
		names = append(names, arg.Name)
	}
	if len(names) != 2 || names[0] != "n" || names[1] != "m" {
		t.Errorf("arguments = %v, want [n m]", names)
	}
	if behaviors[0].StatesBody {
		t.Error("parameter members were read as a replacement behavior body")
	}
}

// A binding names the element holding the body by the reference form
// (`perform a;`, `exhibit m;`), a `references`/`::>` clause, or a typing; one
// naming none is the body itself (SysML v2 §8.3.16, eventOccurrence).
func TestClassifierBehaviorNamesBehavior(t *testing.T) {
	behaviors := classifierBehaviorsIn(t, `
		package test {
			action def A;
			action b;
			state m;

			part def P {
				perform action a { in x = 1; }
				perform action typed : A;
				perform action ref ::> b;
				perform b;
				exhibit m;
			}
		}
	`)

	if len(behaviors) != 5 {
		t.Fatalf("expected the type to bind 5 behaviors, got %d", len(behaviors))
	}
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"a", false},
		{"typed", true},
		{"ref", true},
		{"b", true},
		{"m", true},
	} {
		var got *ClassifierBehavior
		for i := range behaviors {
			if behaviors[i].Name == tc.name {
				got = &behaviors[i]
			}
		}
		if got == nil {
			t.Fatalf("no behavior named %q", tc.name)
		}
		if got.NamesBehavior != tc.want {
			t.Errorf("NamesBehavior of %q = %v, want %v", tc.name, got.NamesBehavior, tc.want)
		}
	}
}

// classifierBehaviorsIn parses src and reports the behaviors the first part
// definition in it binds to its objects.
func classifierBehaviorsIn(t *testing.T, src string) []ClassifierBehavior {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	var found *ast.Definition
	var walk func(members []ast.Node)
	walk = func(members []ast.Node) {
		for _, member := range members {
			if found != nil {
				return
			}
			switch node := unwrapMembership(member).(type) {
			case *ast.Package:
				walk(node.Members)
			case *ast.Definition:
				if node.Kind == ast.DefPart {
					found = node
				}
			}
		}
	}
	walk(root.Members)
	if found == nil {
		t.Fatal("no part definition in the source")
	}
	return ClassifierBehaviorsOf(found.Members)
}
