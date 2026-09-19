package ast

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestQualifiedNameParts(t *testing.T) {
	qn := &QualifiedName{
		Global: false,
		Parts:  []NameSegment{{Text: "A"}, {Text: "B"}, {Text: "C"}},
	}
	if len(qn.Parts) != 3 || qn.Parts[2].Text != "C" {
		t.Fatalf("parts = %+v", qn.Parts)
	}
}

func TestPackageIsNamespaceMember(t *testing.T) {
	var _ Node = &Package{}
	var _ Node = &Namespace{}
	var _ Node = &Import{}
	var _ Node = &Alias{}
	var _ Node = &Dependency{}
	var _ Node = &Comment{}
	var _ Node = &Documentation{}
	var _ Node = &TextualRepresentation{}
	var _ Node = &RootNamespace{}
	var _ Node = &Membership{}
	p := &Package{Ident: Identification{Name: "P"}}
	if p.Ident.Name != "P" {
		t.Fatalf("name = %q", p.Ident.Name)
	}
}

func TestMembershipVisibility(t *testing.T) {
	m := &Membership{Visibility: VisibilityPrivate}
	if m.Visibility != VisibilityPrivate {
		t.Fatalf("vis = %v", m.Visibility)
	}
	_ = source.Span{}
}

// A usage, subject or owned constraint declared under a short name alone is
// known by that name; only a nameless declaration takes its naming feature's.
func TestEffectiveNameShortNameOnly(t *testing.T) {
	target := &QualifiedName{Parts: []NameSegment{{Text: "x", Span: source.Span{Offset: 9, Len: 1}}}}
	redefines := &Relationship{Kind: RelRedefines, Target: target}
	references := &Relationship{Kind: RelReferences, Target: target}
	short := Identification{ShortName: "f", ShortNameSpan: source.Span{Offset: 1, Len: 1}}

	if name, span := short.DeclaredName(); name != "f" || span.Offset != 1 {
		t.Fatalf("DeclaredName() = %q %v, want f at 1", name, span)
	}
	cases := []struct {
		what string
		got  func(Identification) (string, source.Span)
	}{
		{"usage", func(id Identification) (string, source.Span) {
			return EffectiveName(&Usage{Ident: id, Relationships: []*Relationship{redefines}})
		}},
		{"subject", func(id Identification) (string, source.Span) {
			return (&SubjectMember{Ident: id, Relationships: []*Relationship{redefines}}).EffectiveName()
		}},
		{"constraint", func(id Identification) (string, source.Span) {
			return OwnedConstraint{Ident: id, Relationships: []*Relationship{references}}.EffectiveName()
		}},
	}
	for _, tc := range cases {
		if name, span := tc.got(short); name != "f" || span.Offset != 1 {
			t.Errorf("%s <f> naming x: effective name = %q %v, want f at 1", tc.what, name, span)
		}
		if name, span := tc.got(Identification{}); name != "x" || span.Offset != 9 {
			t.Errorf("%s naming x unnamed: effective name = %q %v, want x at 9", tc.what, name, span)
		}
	}
}
