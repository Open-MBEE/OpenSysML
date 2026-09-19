package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// F84–F95: the KerML declaration forms. Each case is taken from the file in
// examples/pilot-corpora/kerml-examples that the reference validates clean.
func parseKerML(t *testing.T, name, src string) (*ast.RootNamespace, []Diagnostic) {
	t.Helper()
	p := New(source.New(name, []byte(src)))
	root := p.ParseFile()
	return root, p.Diagnostics
}

func TestF84F95KerMLDeclarations(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		// F84: a feature declared with no kind keyword.
		{"f84_namespace_typing", "package P { a : Integer; }", `(Usage kind="attribute" name="a"`},
		{"f84_namespace_bare", "package P { x; }", `name="x"`},
		{"f84_specialization_only", "package P { class V { feature m; } class C { composite e1 redefines V::m; } }", `kind="redefines"`},
		{"f84_multiplicity_before_typing", "package P { p5[1] : Real; }", "(Multiplicity"},
		{"f84_var_prefix", "package P { class C { var identifier[0..1]; } }", "variable=true"},
		{"f84_const_prefix", "package P { class C { const identifier : Real; } }", `name="identifier"`},

		// F85: `type`, whose specialization is mandatory.
		{"f85_type_specializes", "package P { classifier A; type T specializes A; }", `kind="class" name="T"`},
		{"f85_type_all", "package P { classifier A; type all x specializes A, Base::things; }", `kind="class"`},
		{"f85_type_conjugates", "package P { classifier A; type C conjugates A; }", `conjugated=true`},

		// F86: relationships written keyword-first (KerML.xtext NonFeatureElement).
		{"f86_specialization_subtype", "package P { classifier A; classifier B; specialization Gen subtype A specializes B; }", `kind="specializes"`},
		{"f86_subtype", "package P { classifier A; classifier B; subtype A :> B; }", `kind="specializes"`},
		{"f86_subclassifier", "package P { classifier A; classifier B; specialization subclassifier B specializes A; }", `kind="specializes"`},
		{"f86_typing", "package P { classifier B; feature f; specialization t1 typing f typed by B; }", `kind="typing"`},
		{"f86_subset", "package P { feature parent; feature person; specialization Sub subset parent subsets person; }", `kind="subsets"`},
		{"f86_redefinition", "package P { feature parent; feature vin; redefinition vin redefines parent; }", `kind="redefines"`},
		{"f86_conjugation", "package P { classifier Original; classifier C1; conjugation c1 conjugate C1 conjugates Original; }", `conjugated=true`},
		{"f86_inverse", "package P { feature a; feature b; inverting inv1 inverse a of b; }", `kind="inverse"`},
		{"f86_featuring", "package P { classifier C; feature y; featuring F of y by C; }", `kind="featured by"`},

		// F87: `typed by` as the long spelling of ':'.
		{"f87_typed_by", "package P { classifier A; classifier B; feature x typed by A, B; }", `kind="typing"`},
		{"f87_typed_as_name", "package P { feature typed; }", `name="typed"`},

		// F88: a feature chain as a connector's first end.
		{"f88_chain_first_end", "package P { class A { feature a : A; connector f.a to a.g; } }", "(Usage kind=\"connector\""},

		// F89: binding and succession with everything optional.
		{"f89_binding_body_ends", "package P { class A { feature a : A; feature b : A; binding { end feature references a; end feature references b; } } }", `kind="binding"`},
		{"f89_binding_typed_of", "package P { class A { feature a : A; feature b : A; binding ab1 : AS of a = b; } }", `name="ab1"`},
		{"f89_succession_body_ends", "package P { class A { feature a : A; succession { end feature references a; } } }", `kind="succession"`},

		// F90: conjugation inside a declaration.
		{"f90_classifier_conjugates", "package P { classifier A; class B conjugates A; }", `conjugated=true`},
		{"f90_feature_tilde", "package P { class B { feature f; } feature g ~ B::f; }", `conjugated=true`},

		// F91: `const` before `end`.
		{"f91_const_end", "package P { assoc struct AS { const end [1] feature a; const end feature b; } }", "end=true"},

		// F92: an anonymous comment with only a locale; `doc` with a short name.
		{"f92_anonymous_locale", "package P { comment locale \"en\" /* text */ }", `(Comment`},
		{"f92_doc_short_name", "package P { doc <a> /* text */ feature q; }", `name="q"`},
		{"f92_anonymous_comment_keeps_next_name", "package P { comment /* text */ x : Integer; }", `name="x"`},

		// F93: a second filter bracket on a filter-package import.
		{"f93_two_filters", "package P { private import DesignModel::**[@Structure][x]; }", `operator="and"`},

		// F94: prefix metadata standing in for the `feature` keyword.
		{"f94_prefix_metadata_feature", "package P { metadata def Classified; abstract #Classified z2; }", `name="z2"`},

		// F95: a named `expr` whose body is an expression with no ';'.
		{"f95_named_expr_body", "package P { behavior w { inout v : Integer; expr whileTest {v > 3} } }", `kind="expr"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, diags := parseKerML(t, tt.name+".kerml", tt.src)
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(diags) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if dump := ast.Dump(root); !strings.Contains(dump, tt.want) {
				t.Fatalf("AST dump does not contain %q:\n%s", tt.want, dump)
			}
		})
	}
}

// TestF84F95DeclarationFields locks the declaration state the AST dump omits:
// the `const` modifier and an annotation's short name.
func TestF84F95DeclarationFields(t *testing.T) {
	member := func(t *testing.T, root *ast.RootNamespace, path ...int) ast.Node {
		t.Helper()
		members := root.Members
		var node ast.Node
		for _, i := range path {
			node = members[i].(*ast.Membership).Member
			switch n := node.(type) {
			case *ast.Package:
				members = n.Members
			case *ast.Usage:
				members = n.Members
			}
		}
		return node
	}

	t.Run("f84_const_prefix", func(t *testing.T) {
		root, diags := parseKerML(t, "f84_const.kerml", "package P { class C { const identifier : Real; } }")
		if len(diags) != 0 {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		u := member(t, root, 0, 0, 0).(*ast.Usage)
		if !u.IsConstant {
			t.Error("IsConstant = false, want true for `const identifier`")
		}
	})

	t.Run("f91_const_end", func(t *testing.T) {
		root, diags := parseKerML(t, "f91_const_end.kerml", "package P { assoc struct AS { const end [1] feature a; } }")
		if len(diags) != 0 {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		u := member(t, root, 0, 0, 0).(*ast.Usage)
		if !u.IsConstant || !u.IsEnd {
			t.Errorf("IsConstant=%v IsEnd=%v, want both true", u.IsConstant, u.IsEnd)
		}
	})

	t.Run("f92_doc_short_name", func(t *testing.T) {
		root, diags := parseKerML(t, "f92_doc.kerml", "package P { doc <a> /* text */ feature q; }")
		if len(diags) != 0 {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		d := member(t, root, 0, 0).(*ast.Documentation)
		if d.Ident.ShortName != "a" || d.Ident.Name != "" {
			t.Errorf("identification = %+v, want short name a only", d.Ident)
		}
	})

	// An anonymous comment must not read the following member's name as its own.
	t.Run("f92_anonymous_comment_keeps_next_name", func(t *testing.T) {
		root, diags := parseKerML(t, "f92_anon_comment.kerml", "package P { comment /* text */ x : Integer; }")
		if len(diags) != 0 {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		c := member(t, root, 0, 0).(*ast.Comment)
		if c.Ident.Name != "" || c.Ident.ShortName != "" {
			t.Errorf("comment identification = %+v, want empty", c.Ident)
		}
		if u := member(t, root, 0, 1).(*ast.Usage); u.Ident.Name != "x" {
			t.Errorf("following usage name = %q, want x", u.Ident.Name)
		}
	})
}

func TestNegativeF84F95KerMLDeclarations(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		// F85: TypeDeclaration makes the specialization mandatory, so the
		// reference rejects `type A;` as well.
		{"f85_type_without_specialization", "package P { type A; }"},
		{"f86_subtype_without_separator", "package P { subtype A B; }"},
		{"f86_typing_without_type", "package P { specialization typing f typed by ; }"},
		{"f86_featuring_without_by", "package P { featuring F of y C; }"},
		{"f87_typed_without_by", "package P { feature x typed A; }"},
		{"f88_connector_chain_without_second_end", "package P { connector f.a to ; }"},
		{"f89_binding_of_without_end", "package P { binding ab of ; }"},
		{"f90_conjugates_without_target", "package P { class B conjugates ; }"},
		{"f91_const_end_without_feature", "package P { assoc struct AS { const end : ; } }"},
		{"f92_doc_short_name_unterminated", "package P { doc <a> }"},
		{"f93_unterminated_second_filter", "package P { private import D::**[@A][x; }"},
		{"f95_expr_body_unterminated", "package P { expr e {v > 3 }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			root, diags := parseKerML(t, tt.name+".kerml", tt.src)
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(diags) == 0 {
				t.Fatal("expected a diagnostic")
			}
		})
	}
}

func TestNoPanicF84F95KerMLDeclarationsTruncated(t *testing.T) {
	sources := []string{
		"package P { a : Integer; x; p5[1] : Real; }",
		"package P { classifier A; type all x specializes A, Base::things; type C conjugates A; }",
		"package P { specialization t1 typing f typed by B; featuring F of y by C; inverting i inverse a of b; }",
		"package P { class A { connector f.a to a.g; binding { end feature references a; } succession { end a; } } }",
		"package P { assoc struct AS { const end [1] feature a; } }",
		"package P { comment locale \"en\" /* text */ doc <a> /* text */ }",
		"package P { private import D::**[@A][x]; abstract #C z2; expr e {v > 3} }",
	}
	for _, src := range sources {
		for i := 0; i <= len(src); i++ {
			t.Run("", func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic at prefix %d: %v", i, r)
					}
				}()
				if root, _ := parseKerML(t, "truncated.kerml", src[:i]); root == nil {
					t.Fatal("ParseFile returned nil")
				}
			})
		}
	}
}
