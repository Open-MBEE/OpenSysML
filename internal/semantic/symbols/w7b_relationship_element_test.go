package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// kermlScope builds the scope tree of a KerML document, whose notation the
// keyword-first relationship members belong to.
func kermlScope(t *testing.T, src string) *Scope {
	t.Helper()
	root := parser.New(source.New("w7b.kerml", []byte(src))).ParseFile()
	return Build(root)
}

const relationshipModel = `package P {
	classifier A;
	classifier B;
	feature f : A;
	feature g : B;
	specialization Gen subtype A specializes B;
	specialization Sub subset f subsets g;
	redefinition f redefines g;
}
`

// TestF86RelationshipElementIsAMember pins the first-class relationship element:
// a keyword-first relationship names an element of its namespace, classified as
// a relationship rather than as an anonymous attribute usage.
func TestF86RelationshipElementIsAMember(t *testing.T) {
	root := kermlScope(t, relationshipModel)
	pkg := childNamed(t, root, "P")
	for _, name := range []string{"Gen", "Sub"} {
		sym, ok := pkg.LookupLocal(name)
		if !ok {
			t.Fatalf("%s is not a member of P", name)
		}
		if sym.Kind != SymbolRelationship {
			t.Errorf("%s has kind %v, want %v", name, sym.Kind, SymbolRelationship)
		}
		if _, ok := sym.Decl.(*ast.RelationshipMember); !ok {
			t.Errorf("%s declares a %T, want an *ast.RelationshipMember", name, sym.Decl)
		}
	}
}

// TestF86RelationshipEndsAreOrdered is the representation defect F86–F91 named:
// the two ends were indistinguishable when both were relationships of one usage.
func TestF86RelationshipEndsAreOrdered(t *testing.T) {
	root := kermlScope(t, relationshipModel)
	pkg := childNamed(t, root, "P")
	sym, ok := pkg.LookupLocal("Gen")
	if !ok {
		t.Fatal("Gen is not a member of P")
	}
	rel := sym.Decl.(*ast.RelationshipMember)
	if got := endName(rel.Source); got != "A" {
		t.Errorf("source end %q, want A", got)
	}
	if got := endName(rel.Target); got != "B" {
		t.Errorf("target end %q, want B", got)
	}
	if rel.Kind != ast.RelSpecializes || rel.Keyword != "subtype" {
		t.Errorf("Gen is a %v written %q, want a specializes written subtype", rel.Kind, rel.Keyword)
	}
}

// TestF86UnnamedRelationshipDeclaresNoName checks a relationship the notation
// leaves unnamed adds no member, so `redefinition f redefines g;` does not
// shadow the feature it names.
func TestF86UnnamedRelationshipDeclaresNoName(t *testing.T) {
	root := kermlScope(t, relationshipModel)
	pkg := childNamed(t, root, "P")
	sym, ok := pkg.LookupLocal("f")
	if !ok {
		t.Fatal("f is not a member of P")
	}
	if sym.Kind == SymbolRelationship {
		t.Errorf("f resolves to the redefinition relationship, want the feature")
	}
}

// childNamed returns the scope of the named member of scope.
func childNamed(t *testing.T, scope *Scope, name string) *Scope {
	t.Helper()
	sym, ok := scope.LookupLocal(name)
	if !ok || sym.Scope == nil {
		t.Fatalf("%s declares no scope", name)
	}
	return sym.Scope
}

// endName is the written name of a relationship end.
func endName(end ast.Node) string {
	qn, ok := end.(*ast.QualifiedName)
	if !ok || len(qn.Parts) == 0 {
		return ""
	}
	return qn.Parts[len(qn.Parts)-1].Text
}
