package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestParseKeywordAsName(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		part def Item {
			part done: Item;
		}
	`))
	p := New(src)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("%s", d.Message)
		}
		t.FailNow()
	}

	// root -> membership -> definition -> membership -> usage
	mem1 := root.Members[0].(*ast.Membership)
	def := mem1.Member.(*ast.Definition)
	mem2 := def.Members[0].(*ast.Membership)
	usage := mem2.Member.(*ast.Usage)

	if usage.Ident.Name != "done" {
		t.Errorf("Expected usage name 'done', got %q", usage.Ident.Name)
	}
}

// A keyword that names a declaration whose kind is also a keyword keeps its
// name: `action flow { ... }` is an action named flow, and dropping the name
// would leave the declaration anonymous and unreferenceable.
func TestParseKeywordAsNameAfterKindKeyword(t *testing.T) {
	tests := []struct {
		src  string
		kind ast.UsageKind
		name string
	}{
		{"package P { action flow { assign x := 1; } }", ast.UsageAction, "flow"},
		{"package P { action flow; }", ast.UsageAction, "flow"},
		{"package P { attribute item : Integer = 1; }", ast.UsageAttribute, "item"},
		{"package P { part state; }", ast.UsagePart, "state"},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.kind.String(), func(t *testing.T) {
			p := New(source.New("test.sysml", []byte(tt.src)))
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("parse errors: %v", p.Diagnostics)
			}
			pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
			usage, ok := pkg.Members[0].(*ast.Membership).Member.(*ast.Usage)
			if !ok {
				t.Fatalf("%s parsed to %T, want a usage", tt.src, pkg.Members[0].(*ast.Membership).Member)
			}
			if usage.Ident.Name != tt.name {
				t.Errorf("%s declared %q, want %q", tt.src, usage.Ident.Name, tt.name)
			}
			if usage.Kind != tt.kind {
				t.Errorf("%s has kind %v, want %v", tt.src, usage.Kind, tt.kind)
			}
		})
	}
}

// A kind keyword ending at `;` declares an anonymous usage of that kind, as
// `exit action;` performs an anonymous action (SysML.xtext PerformActionUsage).
func TestParseBareKindKeywordIsAnonymousUsage(t *testing.T) {
	src := "package P { state def S { exit action; } action def A { action; } }"
	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	exit := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition).Members[0].(*ast.ExitMember)
	usage := exit.Actions[0].(*ast.Membership).Member.(*ast.Usage)
	if usage.Kind != ast.UsageAction || usage.Ident.Name != "" || usage.PrefixKeyword != "exit" {
		t.Errorf("exit action; parsed as %v %q prefix %q", usage.Kind, usage.Ident.Name, usage.PrefixKeyword)
	}
	nested := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition).Members[0].(*ast.Usage)
	if nested.Kind != ast.UsageAction || nested.Ident.Name != "" {
		t.Errorf("action; parsed as %v %q", nested.Kind, nested.Ident.Name)
	}
}

// A prefix keyword followed by a kind keyword and a name declares that kind:
// `variant attribute diameterSmall` is an attribute variant, not a variant
// named `attribute`. With no name after it, the kind keyword is the name.
func TestParseVariantWithKindKeyword(t *testing.T) {
	tests := []struct {
		src  string
		kind ast.UsageKind
		name string
	}{
		{"package P { variation attribute def C { variant attribute diameterSmall = 70; } }", ast.UsageAttribute, "diameterSmall"},
		{"package P { variation part def C { variant part smallEngine; } }", ast.UsagePart, "smallEngine"},
		{"package P { variation part def C { variant attribute; } }", ast.UsagePart, "attribute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New("test.sysml", []byte(tt.src)))
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("parse errors: %v", p.Diagnostics)
			}
			pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
			def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
			usage, ok := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
			if !ok {
				t.Fatalf("%s parsed to %T, want a usage", tt.src, def.Members[0].(*ast.Membership).Member)
			}
			if usage.Ident.Name != tt.name {
				t.Errorf("%s declared %q, want %q", tt.src, usage.Ident.Name, tt.name)
			}
			if usage.Kind != tt.kind {
				t.Errorf("%s has kind %v, want %v", tt.src, usage.Kind, tt.kind)
			}
		})
	}
}

// A keyword in name position is reported: SysML reserves keywords there, and
// only an unrestricted name may spell one. It is a warning rather than an error
// because the normative OMG library relies on unquoted keyword names
// (`step entry[1];`, `part done : Part;`) and must keep parsing clean.
func TestParseKeywordAsNameIsReported(t *testing.T) {
	p := New(source.New("test.sysml", []byte("package P { action flow { assign x := 1; } }")))
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Errorf("errors = %v, want none: a keyword name still parses", p.Diagnostics)
	}
	if len(p.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one", p.Warnings)
	}
	want := `"flow" is a reserved keyword; write 'flow' to use it as a name`
	if p.Warnings[0].Message != want {
		t.Errorf("warning = %q, want %q", p.Warnings[0].Message, want)
	}

	// The quoted spelling the warning suggests is well-formed and warning-free.
	q := New(source.New("test.sysml", []byte("package P { action 'flow' { assign x := 1; } }")))
	q.ParseFile()
	if len(q.Diagnostics) != 0 || len(q.Warnings) != 0 {
		t.Errorf("quoted name reported %v / %v, want neither", q.Diagnostics, q.Warnings)
	}

	// Keywords with their own meaning in this position are not names at all, so
	// they must not be reported as one.
	for _, src := range []string{
		"package P { part def S { part x; connect x to x; } }",
		"package P { action a { first start; then done; } }",
		"package P { part def S { attribute x : Integer default 1; } }",
	} {
		r := New(source.New("test.sysml", []byte(src)))
		r.ParseFile()
		if len(r.Warnings) != 0 {
			t.Errorf("%s warned %v, want none", src, r.Warnings)
		}
	}
}

// The converse: a keyword before a kind keyword qualifies the kind, so the name
// that follows the kind is the declaration's, and a declaration with no name
// stays anonymous rather than being named after its own kind.
func TestParseKeywordBeforeKindKeywordIsNotAName(t *testing.T) {
	tests := []struct {
		src  string
		name string
	}{
		{"package P { individual item def Alice; }", "Alice"},
		{"package P { part def S { var feature x : Integer; } }", "x"},
		{"package P { part def S { assert constraint { 1 > 0 } } }", ""},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			p := New(source.New("test.sysml", []byte(tt.src)))
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("parse errors: %v", p.Diagnostics)
			}
			if got := lastDeclaredName(root); got != tt.name {
				t.Errorf("%s declared %q, want %q", tt.src, got, tt.name)
			}
		})
	}
}

// lastDeclaredName returns the name of the innermost declaration in the tree.
func lastDeclaredName(node ast.Node) string {
	name := ""
	var members []ast.Node
	switch n := node.(type) {
	case *ast.RootNamespace:
		members = n.Members
	case *ast.Membership:
		return lastDeclaredName(n.Member)
	case *ast.Package:
		members = n.Members
	case *ast.Definition:
		name, members = n.Ident.Name, n.Members
	case *ast.Usage:
		name, members = n.Ident.Name, n.Members
	}
	for _, member := range members {
		switch member.(type) {
		case *ast.Membership, *ast.Definition, *ast.Usage:
			return lastDeclaredName(member)
		}
	}
	return name
}
