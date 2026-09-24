package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// F71: `snapshot` and `timeslice` are PortionKind literals of SysML.xtext (:864)
// and of neither KerML grammar, so in a `.kerml` file they name features. Read
// as keywords they were dropped from the declaration, which then carried no
// name and built no symbol.
func TestF71PortionWordsNameKerMLFeatures(t *testing.T) {
	src := `package P {
		class Interval;
		class Timeslice { feature interval : Interval; }
		class Occ {
			expr while {
				in timeslice : Timeslice;
				in snapshot : Timeslice;
				return result : Interval = timeslice.interval;
			}
		}
	}`
	sf := source.New("f71.kerml", []byte(src))
	p := New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	occ := pkg.Members[2].(*ast.Membership).Member.(*ast.Usage)
	expr := occ.Members[0].(*ast.Membership).Member.(*ast.Usage)
	for i, want := range []string{"timeslice", "snapshot"} {
		member := expr.Members[i]
		if m, ok := member.(*ast.Membership); ok {
			member = m.Member
		}
		u, ok := member.(*ast.Usage)
		if !ok {
			t.Fatalf("parameter %d = %T, want *ast.Usage", i, member)
		}
		if u.Ident.Name != want {
			t.Errorf("parameter %d name = %q, want %q", i, u.Ident.Name, want)
		}
	}
}

// The same words stay keywords in a `.sysml` file, where the grammar reserves
// them: `snapshot s;` declares a snapshot named `s`.
func TestF71PortionWordsStayKeywordsInSysML(t *testing.T) {
	sf := source.New("f71.sysml", []byte("package P { occurrence def O { snapshot s; } }"))
	p := New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
	member := def.Members[0]
	if m, ok := member.(*ast.Membership); ok {
		member = m.Member
	}
	u, ok := member.(*ast.Usage)
	if !ok {
		t.Fatalf("member = %T, want *ast.Usage", member)
	}
	if u.Ident.Name != "s" || u.Keyword != "snapshot" {
		t.Errorf("got name=%q keyword=%q, want s declared by the snapshot keyword", u.Ident.Name, u.Keyword)
	}
}

// A malformed declaration whose name is one of these words must produce
// diagnostics, never a panic.
func TestF71PortionWordsNegative(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		input string
	}{
		{"kerml_no_type", "f71_neg.kerml", "class C { feature timeslice : ; }"},
		{"kerml_no_terminator", "f71_neg.kerml", "class C { feature timeslice : T }"},
		{"kerml_at_eof", "f71_neg.kerml", "class C { in timeslice"},
		{"kerml_chain_no_member", "f71_neg.kerml", "class C { expr e { in timeslice : T; return : T = timeslice.; } }"},
		{"sysml_snapshot_no_name", "f71_neg.sysml", "occurrence def O { snapshot : ; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New(tt.file, []byte(tt.input))
			p := New(sf)
			if p.ParseFile() == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Errorf("expected diagnostics for %q, got none", tt.input)
			}
		})
	}
}
