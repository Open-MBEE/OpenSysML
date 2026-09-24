package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A subject, assume or require member takes an identification like any usage
// (SysML.xtext SubjectUsage, RequirementConstraintUsage → UsageDeclaration →
// Identification), so `<short>` is read with or without a name, and each span
// points at the identifier it names.
func TestRequirementMemberShortNames(t *testing.T) {
	src := `package P {
	part def T;
	constraint def C;
	requirement def R {
		subject <s> x : T;
		assume constraint <a> ac : C;
		require constraint <r> rc : C;
		subject <t> : T;
		assume constraint <b> : C;
		require constraint <q> : C;
	}
}`
	sf := source.New("short.sysml", []byte(src))
	p := New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[2].(*ast.Membership).Member.(*ast.Definition)
	if len(def.Members) != 6 {
		t.Fatalf("members = %d, want 6", len(def.Members))
	}

	idents := []ast.Identification{
		def.Members[0].(*ast.SubjectMember).Ident,
		def.Members[1].(*ast.AssumeMember).Ident,
		def.Members[2].(*ast.RequireMember).Ident,
		def.Members[3].(*ast.SubjectMember).Ident,
		def.Members[4].(*ast.AssumeMember).Ident,
		def.Members[5].(*ast.RequireMember).Ident,
	}
	want := []struct{ short, name string }{
		{"s", "x"}, {"a", "ac"}, {"r", "rc"}, {"t", ""}, {"b", ""}, {"q", ""},
	}
	for i, id := range idents {
		if id.ShortName != want[i].short || id.Name != want[i].name {
			t.Errorf("member %d: ident = <%s> %q, want <%s> %q", i, id.ShortName, id.Name, want[i].short, want[i].name)
		}
		if got := spanText(src, id.ShortNameSpan); got != want[i].short {
			t.Errorf("member %d: ShortNameSpan covers %q, want %q", i, got, want[i].short)
		}
		if want[i].name == "" {
			if id.NameSpan != (source.Span{}) {
				t.Errorf("member %d: NameSpan = %v, want none", i, id.NameSpan)
			}
			continue
		}
		if got := spanText(src, id.NameSpan); got != want[i].name {
			t.Errorf("member %d: NameSpan covers %q, want %q", i, got, want[i].name)
		}
	}

	// The owned constraint an assume/require member declares carries the same
	// identification, which is what the symbol for it is registered under.
	for _, i := range []int{1, 2, 4, 5} {
		oc, ok := ast.OwnedConstraintOf(def.Members[i])
		if !ok {
			t.Fatalf("member %d: not an owned constraint", i)
		}
		if oc.Ident != idents[i] {
			t.Errorf("member %d: OwnedConstraint.Ident = %+v, want %+v", i, oc.Ident, idents[i])
		}
	}
}

// A malformed short name on one of the three members is reported where the
// identification parser reports it for any usage, and parsing goes on.
func TestRequirementMemberShortNameDiagnostics(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"subject_empty", "requirement def R { subject <> x; }", msgExpectedShortName},
		{"subject_unclosed", "requirement def R { subject <s x; }", msgExpectedCloseAngle},
		{"assume_empty", "requirement def R { assume constraint <> c; }", msgExpectedShortName},
		{"assume_unclosed", "requirement def R { assume constraint <c x; }", msgExpectedCloseAngle},
		{"require_empty", "requirement def R { require constraint <> c; }", msgExpectedShortName},
		{"require_unclosed", "requirement def R { require constraint <c x; }", msgExpectedCloseAngle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(source.New("neg.sysml", []byte(tc.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			var msgs []string
			for _, d := range p.Diagnostics {
				msgs = append(msgs, d.Message)
			}
			if !containsString(msgs, tc.want) {
				t.Errorf("diagnostics = %v, want one saying %q", msgs, tc.want)
			}
		})
	}
}

func spanText(src string, sp source.Span) string {
	if sp.Offset+sp.Len > len(src) {
		return ""
	}
	return src[sp.Offset : sp.Offset+sp.Len]
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}
