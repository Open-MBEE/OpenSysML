package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSubjectWithBody(t *testing.T) {
	input := `
	package Test {
		requirement def R {
			subject s : Thing[1] {
				doc /* doc inside subject body */
			}
		}
	}
	`

	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	root := p.ParseFile()

	if root == nil {
		t.Fatal("ParseFile returned nil")
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

// A subject declared with an unrestricted (quoted) name stores the unquoted
// text, as every other declaration does, so references to it resolve.
func TestSubjectUnrestrictedName(t *testing.T) {
	input := `package Test { requirement def R { subject 'my subject' : Thing; } }`
	sf := source.New("subject_unrestricted.sysml", []byte(input))
	p := New(sf)
	root := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
	subj := def.Members[0].(*ast.SubjectMember)
	if subj.Ident.Name != "my subject" {
		t.Errorf("Name = %q, want %q", subj.Ident.Name, "my subject")
	}
}
