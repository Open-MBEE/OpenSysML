package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestSatisfyByRejectsExpressionOperand: a `by` operand wider than a feature chain
// (SysML.xtext SatisfactionReferenceExpression) is one diagnostic; the usage closes at ';'.
func TestSatisfyByRejectsExpressionOperand(t *testing.T) {
	cases := []struct {
		name, operand, want string
		subject             string
	}{
		{"index", "sub#(1)", "not an expression", "sub"},
		{"tuple", "(a, b)", "expected a feature reference after 'by'", ""},
		{"arrow", "sub->first()", "not an expression", "sub"},
		{"chained_index", "self.items#(2)", "not an expression", "items"},
		{"arithmetic", "1 + 2", "expected a feature reference after 'by'", ""},
		{"constructor", "new Sub()", "expected a feature reference after 'by'", ""},
		{"succession_words", "a to b", "not an expression", "a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `package Test {
	requirement def R1 { subject s[1]; }
	part def Sub;
	part context {
		part sub : Sub[2];
		part a : Sub;
		part b : Sub;
		attribute items;
		satisfy requirement sr : R1 by ` + tc.operand + `;
		part after : Sub;
	}
}`
			p := New(source.New("test.sysml", []byte(src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 1 {
				for _, d := range p.Diagnostics {
					t.Logf("  %s", d.Message)
				}
				t.Fatalf("expected exactly one diagnostic, got %d", len(p.Diagnostics))
			}
			if got := p.Diagnostics[0].Message; !strings.Contains(got, tc.want) {
				t.Errorf("diagnostic %q does not mention %q", got, tc.want)
			}
			u := findSatisfyUsage(root)
			if u == nil {
				t.Fatal("no satisfy usage found")
			}
			if u.Ident.Name != "sr" {
				t.Errorf("satisfy usage name = %q, want sr", u.Ident.Name)
			}
			var subjects []*ast.Relationship
			for _, rel := range u.Relationships {
				if rel.Kind == ast.RelSubject {
					subjects = append(subjects, rel)
				}
			}
			switch {
			case tc.subject == "" && len(subjects) != 0:
				t.Errorf("expected no subject relationship, got %d", len(subjects))
			case tc.subject != "" && (len(subjects) != 1 || subjectName(subjects[0]) != tc.subject):
				t.Errorf("expected one subject relationship to %s, got %v", tc.subject, subjects)
			}
			if !hasPart(root, "after") {
				t.Error("the member after the satisfy usage was lost to recovery")
			}
		})
	}
}

// subjectName is the last name of a subject relationship's target, a qualified
// name or a feature chain.
func subjectName(rel *ast.Relationship) string {
	switch target := rel.Target.(type) {
	case *ast.QualifiedName:
		return target.Parts[len(target.Parts)-1].Text
	case *ast.FeatureChainExpr:
		return target.Member.Parts[len(target.Member.Parts)-1].Text
	}
	return ""
}

// hasPart reports whether a part usage named name is declared anywhere under node.
func hasPart(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if u, ok := n.(*ast.Usage); ok && u.Kind == ast.UsagePart && u.Ident.Name == name {
			found = true
		}
		return !found
	})
	return found
}

// TestSatisfyByMissingTerminator: a `by` subject the next line's member follows
// lacks its ';', and is reported as that, not as an expression operand.
func TestSatisfyByMissingTerminator(t *testing.T) {
	p := New(source.New("test.sysml", []byte(`package Test {
	requirement def R1;
	part def Sub;
	part context {
		part a : Sub;
		satisfy requirement sr : R1 by a
		part after : Sub;
	}
}`)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 1 {
		for _, d := range p.Diagnostics {
			t.Logf("  %s", d.Message)
		}
		t.Fatalf("expected exactly one diagnostic, got %d", len(p.Diagnostics))
	}
	if got := p.Diagnostics[0].Message; !strings.Contains(got, "missing ';'") {
		t.Errorf("diagnostic %q does not report the missing ';'", got)
	}
	if !hasPart(root, "after") {
		t.Error("the member after the satisfy usage was lost to recovery")
	}
}

// TestSatisfyByOperandContinuesOnNextLine: an operator opening the next line extends
// the operand into an expression, reported once; it is not a member lacking its ';'.
func TestSatisfyByOperandContinuesOnNextLine(t *testing.T) {
	p := New(source.New("test.sysml", []byte(`package Test {
	requirement def R1;
	part def Sub;
	part context {
		part a : Sub;
		part b : Sub;
		satisfy requirement sr : R1 by a
			+ b;
		part after : Sub;
	}
}`)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 1 {
		for _, d := range p.Diagnostics {
			t.Logf("  %s", d.Message)
		}
		t.Fatalf("expected exactly one diagnostic, got %d", len(p.Diagnostics))
	}
	if got := p.Diagnostics[0].Message; !strings.Contains(got, "not an expression") {
		t.Errorf("diagnostic %q does not report the expression operand", got)
	}
	if !hasPart(root, "after") {
		t.Error("the member after the satisfy usage was lost to recovery")
	}
}
