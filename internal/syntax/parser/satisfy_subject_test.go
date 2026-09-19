package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseSatisfyUsage parses src and returns the single satisfy usage nested in `part context`.
func parseSatisfyUsage(t *testing.T, src string) *ast.Usage {
	t.Helper()

	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if root == nil {
		t.Fatal("ParseFile returned nil")
	}
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("  %s", d.Message)
		}
		t.Fatalf("expected no diagnostics, got %d", len(p.Diagnostics))
	}

	found := findSatisfyUsage(root)
	if found == nil {
		t.Fatal("no satisfy usage found")
	}
	return found
}

// findSatisfyUsage returns the first satisfy usage in the tree rooted at node.
func findSatisfyUsage(node ast.Node) *ast.Usage {
	var members []ast.Node
	switch v := node.(type) {
	case *ast.Membership:
		return findSatisfyUsage(v.Member)
	case *ast.RootNamespace:
		members = v.Members
	case *ast.Package:
		members = v.Members
	case *ast.Definition:
		members = v.Members
	case *ast.Usage:
		if v.Kind == ast.UsageSatisfy {
			return v
		}
		members = v.Members
	default:
		return nil
	}
	for _, member := range members {
		if found := findSatisfyUsage(member); found != nil {
			return found
		}
	}
	return nil
}

func relTargetName(t *testing.T, rel *ast.Relationship) string {
	t.Helper()
	qn, ok := rel.Target.(*ast.QualifiedName)
	if !ok || len(qn.Parts) == 0 {
		t.Fatalf("relationship target is not a qualified name: %T", rel.Target)
	}
	return qn.Parts[len(qn.Parts)-1].Text
}

// TestSatisfyByDoesNotNameUsage locks in that the `by` operand of a
// SatisfyRequirementUsage names the subject, never the usage itself. Naming the
// usage after the subject declared a shadowing member that broke sibling
// references to the real subject.
func TestSatisfyByDoesNotNameUsage(t *testing.T) {
	u := parseSatisfyUsage(t, `
	package Test {
		requirement touchdown;
		part lander01;

		part context {
			assert satisfy touchdown by lander01;
		}
	}
	`)

	if u.Ident.Name != "" {
		t.Errorf("satisfy usage should stay anonymous, got name %q", u.Ident.Name)
	}

	var subsets, subjects []*ast.Relationship
	for _, rel := range u.Relationships {
		switch rel.Kind {
		case ast.RelSubsets:
			subsets = append(subsets, rel)
		case ast.RelSubject:
			subjects = append(subjects, rel)
		}
	}
	if len(subsets) != 1 || relTargetName(t, subsets[0]) != "touchdown" {
		t.Errorf("expected one subsets relationship to touchdown, got %v", subsets)
	}
	if len(subjects) != 1 || relTargetName(t, subjects[0]) != "lander01" {
		t.Errorf("expected one subject relationship to lander01, got %v", subjects)
	}
}

// TestSatisfyRequirementKeywordStillDeclaresName keeps the declaring form
// `satisfy requirement <newName> by <subject>` naming <newName>.
func TestSatisfyRequirementKeywordStillDeclaresName(t *testing.T) {
	u := parseSatisfyUsage(t, `
	package Test {
		part lander01;

		part context {
			satisfy requirement touchdownConformance by lander01;
		}
	}
	`)

	if u.Ident.Name != "touchdownConformance" {
		t.Errorf("expected usage named touchdownConformance, got %q", u.Ident.Name)
	}
	if len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelSubject ||
		relTargetName(t, u.Relationships[0]) != "lander01" {
		t.Errorf("expected a single subject relationship to lander01, got %v", u.Relationships)
	}
}
