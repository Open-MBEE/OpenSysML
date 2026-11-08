package parser

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// commentMembers returns the body of every Comment element of a parse, in
// member order, and the parse errors.
func commentMembers(t *testing.T, input string) (bodies []string, errs []string) {
	t.Helper()
	sf := source.New("a.sysml", []byte(input))
	p := New(sf)
	root := p.ParseFile()
	ast.Inspect(root, func(n ast.Node) bool {
		if c, ok := n.(*ast.Comment); ok {
			bodies = append(bodies, source.CommentBody(sf.Text(c.BodySpan)))
		}
		return true
	})
	for _, d := range p.Diagnostics {
		errs = append(errs, d.Message)
	}
	return bodies, errs
}

// A regular comment written where a member may start, or where a body closes,
// is a Comment element with neither name nor `about` (KerML.xtext Comment,
// SysML.xtext AnnotatingMember), owned by the namespace it is written in; one
// written anywhere else, or as a comment notation's body, is not one of its own.
func TestRegularCommentsAreCommentMembers(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  []string
	}{
		{"member_position", `package P { /* c */ part a; part b; /* d */ }`, []string{"c ", "d "}},
		{"file_level", `/* c */ part a; /* d */`, []string{"c ", "d "}},
		{"notation_bodies_once", `package P { comment /* c */ comment n /* d */ doc /* e */ }`, []string{"c ", "d "}},
		{"after_notation_body", `package P { doc /* d */ /* c */ }`, []string{"c "}},
		{"after_comment_body", `package P { comment /* d */ /* c */ }`, []string{"d ", "c "}},
		{"definition_body", `part def P { /* c */ attribute x; /* d */ }`, []string{"c ", "d "}},
		{"usage_body", `part p { /* c */ attribute x; /* d */ }`, []string{"c ", "d "}},
		{"action_body", `action def A { /* c */ action a; /* d */ then action b; /* e */ }`, []string{"c ", "d ", "e "}},
		{"state_body", `state def S { /* c */ state A; /* d */ transition first A then A; /* e */ }`, []string{"c ", "d ", "e "}},
		{"calc_body", `calc def F { /* c */ attribute x = 1; /* d */ x }`, []string{"c ", "d "}},
		{"constraint_body", `constraint def C { in x : Real; /* c */ x > 0 }`, []string{"c "}},
		{"requirement_body", `requirement def R { /* c */ subject s; /* d */ require constraint { true } /* e */ }`, []string{"c ", "d ", "e "}},
		{"enum_body", `enum def E { a; /* c */ b; /* d */ }`, []string{"c ", "d "}},
		{"body_expression", `part def P { attribute y = xs.?{ /* c */ in x; /* d */ x }; }`, []string{"c ", "d "}},
		{"metadata_body", `package P { metadata def M; @M { /* c */ } }`, []string{"c "}},
		{"inside_declaration", `part def P { attribute x /* c */ : Real; }`, nil},
		{"inside_expression", `part def P { attribute x = 1 /* c */ + 2; }`, nil},
		{"after_result", `calc def F { 1 /* c */ }`, nil},
		{"after_body_expression_result", `part def P { attribute y = xs.?{ in x; x /* c */ }; }`, nil},
		{"between_transition_clauses", `state def S { state A; transition first A accept Go /* c */ then A; }`, nil},
		{"before_attached_then", `action def A { first start; /* c */ then done; }`, []string{"c "}},
		{"line_notes_are_not_elements", "package P { // c\n part a; // d\n }", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, errs := commentMembers(t, tc.input)
			if len(errs) != 0 {
				t.Fatalf("parse errors %v, want none", errs)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("comment bodies = %q, want %q", got, tc.want)
			}
		})
	}
}

// A comment is a member of the body it is written in, in its place among the
// members, and stays trivia of the member after it.
func TestRegularCommentMemberPlacement(t *testing.T) {
	sf := source.New("a.sysml", []byte("package P { part a; /* c */ part def B { /* d */ } /* e */ }"))
	root := New(sf).ParseFile()
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	kinds := make([]string, 0, len(pkg.Members))
	for _, m := range pkg.Members {
		switch n := m.(*ast.Membership).Member.(type) {
		case *ast.Comment:
			kinds = append(kinds, source.CommentBody(sf.Text(n.BodySpan)))
		case *ast.Usage:
			kinds = append(kinds, "part "+n.Ident.Name)
		case *ast.Definition:
			kinds = append(kinds, "part def "+n.Ident.Name)
			if len(n.Members) != 1 {
				t.Fatalf("B has %d members, want the comment", len(n.Members))
			}
		}
	}
	if want := []string{"part a", "c ", "part def B", "e "}; !slices.Equal(kinds, want) {
		t.Errorf("members = %q, want %q", kinds, want)
	}
	if triv := pkg.Members[2].(*ast.Membership).LeadingTrivia(); len(triv) != 1 || triv[0].Kind != ast.TriviaComment {
		t.Errorf("part def B has leading trivia %v, want the comment", triv)
	}
	c := pkg.Members[1].(*ast.Membership).Member.(*ast.Comment)
	if got := sf.Text(c.Span()); got != "/* c */" {
		t.Errorf("comment span covers %q, want the comment", got)
	}
}

// Prefix metadata read as if it followed its keyword leaves the comments
// written among it before the keyword, so they are Comment members ahead of
// the member, as in the spelling the fix produces.
func TestRecoveredPrefixRunCommentsAreMembers(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"requirement_member", `requirement def R { #M /* c */ assume constraint a : C; }`},
		{"usage_member", `requirement def R { #M /* c */ actor a : A; }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sf := source.New("a.sysml", []byte(tc.input))
			p := New(sf)
			root := p.ParseFile()
			def := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
			if len(def.Members) != 2 {
				t.Fatalf("members = %d, want the comment and the member:\n%s", len(def.Members), ast.Dump(root))
			}
			if _, ok := def.Members[0].(*ast.Membership).Member.(*ast.Comment); !ok {
				t.Errorf("first member = %T, want *ast.Comment", def.Members[0].(*ast.Membership).Member)
			}
			for _, w := range p.Warnings {
				if w.Code == codeNonstandardNotation {
					t.Errorf("warned about the comment: %s", w.Message)
				}
			}
		})
	}
}
