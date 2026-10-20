package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A mis-spelt keyword in an alias or type featuring, or a parenthesised
// parameter list on a declaration, is reported once and the member it was
// meant as is still built, so the rest of the body is read as written.
func TestErratumSpellingsRecoverTheMember(t *testing.T) {
	const code = `package P {
	package SC { step s1; }
	class Vehicle;
	feature engine;
	behavior ST {
		step trns {
			alias s1 of SC::s1;
			in feature x;
		}
	}
	featuring engine_by_Vehicle of engine featured by Vehicle;
	featuring engine featured by Vehicle {
		doc /* kept */
	}
	behavior setX(c : Vehicle, newX : ScalarValues::Real) {
		language "alf"
			/* c.x = newX; */
	}
	feature after;
}`

	p := New(source.New("test.kerml", []byte(code)))
	file := p.ParseFile()

	want := []string{
		"expected 'for' in alias: write `alias s1 for <qualified name>`",
		"expected 'by' after the featured feature of a featuring relationship, not 'featured by'",
		"expected 'by' after the featured feature of a featuring relationship, not 'featured by'",
		"a declaration takes no parenthesised parameter list",
	}
	if len(p.Diagnostics) != len(want) {
		for _, d := range p.Diagnostics {
			t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
		}
		t.Fatalf("got %d diagnostics, want %d", len(p.Diagnostics), len(want))
	}
	for i, d := range p.Diagnostics {
		if !strings.HasPrefix(d.Message, want[i]) {
			t.Errorf("diagnostic %d = %q, want prefix %q", i, d.Message, want[i])
		}
	}

	var aliases []*ast.Alias
	var featurings []*ast.RelationshipMember
	var reps []*ast.TextualRepresentation
	var usages []string
	var walk func(nodes []ast.Node)
	walk = func(nodes []ast.Node) {
		for _, n := range nodes {
			switch v := n.(type) {
			case *ast.Membership:
				walk([]ast.Node{v.Member})
			case *ast.Package:
				walk(v.Members)
			case *ast.Usage:
				usages = append(usages, v.Ident.Name)
				walk(v.Members)
			case *ast.Alias:
				aliases = append(aliases, v)
			case *ast.RelationshipMember:
				featurings = append(featurings, v)
				walk(v.Members)
			case *ast.TextualRepresentation:
				reps = append(reps, v)
			case *ast.ErrorNode:
				t.Errorf("error node: %s", v.Message)
			}
		}
	}
	walk(file.Members)

	if len(aliases) != 1 || aliases[0].Ident.Name != "s1" || ast.QualifiedText(aliases[0].For) != "SC::s1" {
		t.Errorf("aliases = %+v, want one `alias s1 for SC::s1`", aliases)
	}
	if len(featurings) != 2 {
		t.Fatalf("got %d featuring members, want 2", len(featurings))
	}
	for i, f := range featurings {
		if f.Kind != ast.RelFeaturedBy || ast.QualifiedText(f.Source) != "engine" || ast.QualifiedText(f.Target) != "Vehicle" {
			t.Errorf("featuring %d = %s %s by %s, want engine by Vehicle", i, f.Kind, ast.QualifiedText(f.Source), ast.QualifiedText(f.Target))
		}
	}
	if !featurings[1].HasBody || len(featurings[1].Members) != 1 {
		t.Errorf("the second featuring lost its relationship body: %+v", featurings[1].Members)
	}
	if len(reps) != 1 || reps[0].Language != `"alf"` {
		t.Errorf("textual representations = %+v, want the one of setX", reps)
	}
	joined := strings.Join(usages, " ")
	for _, name := range []string{"s1", "Vehicle", "engine", "ST", "trns", "x", "setX", "after"} {
		if !strings.Contains(" "+joined+" ", " "+name+" ") {
			t.Errorf("usage %q was not read; usages read: %s", name, joined)
		}
	}
}

// The same parameter-list recovery applies to a bodiless declaration and in SysML.
func TestParameterListRecoveryKeepsTheNextMember(t *testing.T) {
	for _, tc := range []struct{ name, code string }{
		{"bodiless.kerml", "package P { behavior B(x : Real); feature after; }"},
		{"sysml.sysml", "package P { action def A(x : Real) { in y : Real; } part def After; }"},
		{"nested.sysml", "package P { calc def C(f(x), y) { in y : Real; } part def After; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := New(source.New(tc.name, []byte(tc.code)))
			file := p.ParseFile()
			if len(p.Diagnostics) != 1 || !strings.HasPrefix(p.Diagnostics[0].Message, "a declaration takes no parenthesised parameter list") {
				for _, d := range p.Diagnostics {
					t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
				}
				t.Fatalf("got %d diagnostics, want the parameter-list one", len(p.Diagnostics))
			}
			pkg := file.Members[0].(*ast.Membership).Member.(*ast.Package)
			if len(pkg.Members) != 2 {
				t.Fatalf("package has %d members, want the declaration and the one after it", len(pkg.Members))
			}
			if _, ok := pkg.Members[1].(*ast.Membership); !ok {
				t.Errorf("second member is %T, want a Membership", pkg.Members[1])
			}
		})
	}
}
