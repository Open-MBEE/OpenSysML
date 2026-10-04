package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A condition may be stated in a requirement definition as well as in a usage,
// as the constraint it names or as a nested constraint, and after ordinary
// declarations.
func TestRequirementConditionForms(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"def body require", "package P { requirement def R { attribute a = 1.0; attribute b = 2.0; require constraint { a <= b } } }"},
		{"def body require first", "package P { requirement def R { require constraint { a <= b } attribute a = 1.0; attribute b = 2.0; } }"},
		{"usage body require", "package P { requirement r { attribute a = 1.0; attribute b = 2.0; require constraint { a <= b } } }"},
		{"def body require reference", "package P { constraint def C; requirement def R { require P::C; } }"},
		{"usage body require reference", "package P { constraint def C; requirement r { require P::C; } }"},
		{"def body assume", "package P { requirement def R { attribute a = 1.0; assume constraint { a <= 2.0 } require constraint { a <= 3.0 } } }"},
		{"def body assume constraint", "package P { requirement def R { attribute a = 1.0; assume constraint { a <= 2.0 } } }"},
		{"concern def body require", "package P { concern def C { attribute a = 1.0; require constraint { a <= 2.0 } } }"},
		{"viewpoint def body require", "package P { viewpoint def V { attribute a = 1.0; require constraint { a <= 2.0 } } }"},
		{"satisfy body require", "package P { requirement q; part x; part c { satisfy requirement q by x { require constraint { 1.0 <= 2.0 } } } }"},
		{"subject redefines inherited", "package P { requirement def R { subject subj : Thing[1] :>> Other::subj; require constraint { 1.0 <= 2.0 } } }"},
		{"bare subject", "package P { viewpoint def V { subject; require constraint { 1.0 <= 2.0 } } }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(c.src)))
			p.ParseFile()
			for _, d := range p.Diagnostics {
				t.Errorf("unexpected diagnostic: %s", d.Message)
			}
		})
	}
}

// The braced spelling keeps every condition of the nested constraint it states.
func TestRequireMemberRetainsConditions(t *testing.T) {
	src := `package P {
		requirement def R {
			attribute a = 1.0;
			require constraint {
				a <= 2.0
				a <= 3.0
			}
			require constraint { a <= 4.0 }
		}
	}`
	p := New(source.New("t.sysml", []byte(src)))
	root := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", d.Message)
	}
	dump := ast.Dump(root)
	if got := strings.Count(dump, "(ConstraintMember"); got != 3 {
		t.Errorf("nested conditions kept = %d, want 3\n%s", got, dump)
	}
	if got := strings.Count(dump, "(RequireMember"); got != 2 {
		t.Errorf("require members = %d, want 2\n%s", got, dump)
	}
}

// A named nested constraint states its conditions in a body, and `constraint`
// remains usable as a feature name in a condition.
func TestConstraintMemberNestedBody(t *testing.T) {
	src := `package P {
		constraint def C {
			in pressure;
			assert constraint safetyLimit { pressure < 100 }
			assert constraint { pressure > 0 }
		}
	}`
	p := New(source.New("t.sysml", []byte(src)))
	root := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", d.Message)
	}
	dump := ast.Dump(root)
	if !strings.Contains(dump, `name="safetyLimit"`) {
		t.Errorf("nested constraint name not kept\n%s", dump)
	}
	if got := strings.Count(dump, "(ConstraintMember"); got != 4 {
		t.Errorf("constraint members = %d, want 4 (2 nested, 2 conditions)\n%s", got, dump)
	}
}
