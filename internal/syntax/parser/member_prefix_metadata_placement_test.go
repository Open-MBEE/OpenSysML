package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Each case writes prefix metadata ahead of a member keyword that takes it
// after itself (SysML.xtext SubjectUsage, ActorUsage, StakeholderUsage,
// ObjectiveRequirementUsage, VariantUsageElement, RequirementConstraintUsage).
// The pilot parser rejects every one of them at the `#`.
var misplacedMemberPrefixCases = []struct {
	name    string
	invalid string // the rejected spelling
	valid   string // the accepted spelling the diagnostic points at
	message string
}{
	{"subject", "requirement def R { #M subject s : T; }", "requirement def R { subject #M s : T; }",
		"prefix metadata follows 'subject': write `subject #M s`"},
	{"subject_in_case", "use case def U { #M subject s : T; }", "use case def U { subject #M s : T; }",
		"prefix metadata follows 'subject': write `subject #M s`"},
	{"actor", "requirement def R { #M actor a : A; }", "requirement def R { actor #M a : A; }",
		"prefix metadata follows 'actor': write `actor #M a`"},
	{"stakeholder", "requirement def R { #M stakeholder k; }", "requirement def R { stakeholder #M k; }",
		"prefix metadata follows 'stakeholder': write `stakeholder #M k`"},
	{"objective", "use case def U { #M objective o : R; }", "use case def U { objective #M o : R; }",
		"prefix metadata follows 'objective': write `objective #M o`"},
	{"variant", "variation part def V { #M variant part v; }", "variation part def V { variant #M part v; }",
		"prefix metadata follows 'variant': write `variant #M part`"},
	{"assume_constraint", "requirement def R { #M assume constraint a : C; }", "requirement def R { assume #M constraint a : C; }",
		"prefix metadata follows 'assume': write `assume #M constraint`"},
	{"require_constraint", "requirement def R { #M require constraint r : C; }", "requirement def R { require #M constraint r : C; }",
		"prefix metadata follows 'require': write `require #M constraint`"},
	{"require_short_name", "requirement def R { #M require <r> x : C; }", "requirement def R { require #M <r> x : C; }",
		"prefix metadata follows 'require': write `require #M`"},
	{"two_prefixes", "requirement def R { #M #B subject s : T; }", "requirement def R { subject #M #B s : T; }",
		"prefix metadata follows 'subject': write `subject #M #B s`"},
}

// Prefix metadata ahead of the keyword is one syntax error at the `#` run, and
// the member is still read as the accepted spelling reads it, so later
// diagnostics and editor features keep working.
func TestPrefixMetadataBeforeMemberKeywordIsReported(t *testing.T) {
	for _, c := range misplacedMemberPrefixCases {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("neg.sysml", []byte(c.invalid)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %v, want one", p.Diagnostics)
			}
			d := p.Diagnostics[0]
			if d.Message != c.message {
				t.Errorf("message = %q, want %q", d.Message, c.message)
			}
			if d.Span.Offset != strings.Index(c.invalid, "#") {
				t.Errorf("span starts at %d, want the '#' at %d", d.Span.Offset, strings.Index(c.invalid, "#"))
			}
			wantRun := strings.TrimSpace(c.invalid[strings.Index(c.invalid, "#") : strings.LastIndex(c.invalid, "#")+2])
			if got := spanText(c.invalid, d.Span); got != wantRun {
				t.Errorf("span covers %q, want %q", got, wantRun)
			}
			if len(d.Fixes) != 1 {
				t.Fatalf("fixes = %+v, want one", d.Fixes)
			}
			if got := applyEdits(c.invalid, d.Fixes[0].Edits); got != c.valid {
				t.Errorf("fixed source = %q, want %q", got, c.valid)
			}

			valid := New(source.New("pos.sysml", []byte(c.valid)))
			want := valid.ParseFile()
			if len(valid.Diagnostics) != 0 {
				t.Fatalf("accepted spelling has diagnostics: %v", valid.Diagnostics)
			}
			if got, want := ast.Dump(root), ast.Dump(want); got != want {
				t.Errorf("recovered AST differs from the accepted spelling's:\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// A comment between the misplaced run and the keyword stays where it is: the
// fix moves only the annotations, and the diagnostic spans only them.
func TestPrefixMetadataBeforeMemberKeywordFixLeavesCommentsInPlace(t *testing.T) {
	cases := []struct{ name, invalid, valid, run string }{
		{"line_comment", "requirement def R {\n\t#M // note\n\tsubject s : T;\n}", "requirement def R {\n\t// note\n\tsubject #M s : T;\n}", "#M"},
		{"block_comment", "requirement def R { #M /* note */ assume constraint a : C; }", "requirement def R { /* note */ assume #M constraint a : C; }", "#M"},
		{"comment_between_prefixes", "requirement def R { #M /* one */ #Q::N // two\n require constraint r : C; }", "requirement def R { /* one */ // two\n require #M #Q::N constraint r : C; }", "#M /* one */ #Q::N"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("neg.sysml", []byte(c.invalid)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 1 || len(p.Diagnostics[0].Fixes) != 1 {
				t.Fatalf("diagnostics = %v, want one with one fix", p.Diagnostics)
			}
			d := p.Diagnostics[0]
			if got := spanText(c.invalid, d.Span); got != c.run {
				t.Errorf("span covers %q, want %q", got, c.run)
			}
			if got := applyEdits(c.invalid, d.Fixes[0].Edits); got != c.valid {
				t.Errorf("fixed source = %q, want %q", got, c.valid)
			}
			valid := New(source.New("pos.sysml", []byte(c.valid)))
			want := valid.ParseFile()
			if len(valid.Diagnostics) != 0 {
				t.Fatalf("fixed source has diagnostics: %v", valid.Diagnostics)
			}
			if got, want := ast.Dump(root), ast.Dump(want); got != want {
				t.Errorf("recovered AST differs from the fixed source's:\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// The accepted placements parse clean, and prefix metadata ahead of an
// ordinary usage or definition, a modifier or `assert` stays as it is.
func TestPrefixMetadataPlacementsThatStayClean(t *testing.T) {
	cases := []struct{ name, src string }{
		{"subject", "requirement def R { subject #M s : T; }"},
		{"subject_short", "requirement def R { subject #M <s> x : T; }"},
		{"actor", "requirement def R { actor #M a : A; }"},
		{"stakeholder", "requirement def R { stakeholder #M k; }"},
		{"objective", "use case def U { objective #M o : R; }"},
		{"variant", "variation part def V { variant #M part v; }"},
		{"assume", "requirement def R { assume #M constraint a : C; }"},
		{"assume_short", "requirement def R { assume #M constraint <a> ac : C; }"},
		{"require", "requirement def R { require #M constraint r : C; }"},
		{"require_short", "requirement def R { require #M <r> rc : C; }"},
		{"assert", "part def D { #B assert not constraint c; }"},
		{"usage", "part def D { #M part p; }"},
		{"definition", "#M part def D;"},
		{"abstract_usage", "part def D { abstract #M part p; }"},
		{"snapshot_usage", "part def D { snapshot #M part s; }"},
		{"keywordless_usage", "part def D { #M p; }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("pos.sysml", []byte(c.src)))
			p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Errorf("diagnostics = %v, want none", p.Diagnostics)
			}
		})
	}
}
