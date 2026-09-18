package ast

import "testing"

func TestQualifiedNameText(t *testing.T) {
	var nilName *QualifiedName
	if got := nilName.Text(); got != "" {
		t.Fatalf("nil name: got %q, want empty", got)
	}
	if got := (&QualifiedName{}).Text(); got != "" {
		t.Fatalf("empty name: got %q, want empty", got)
	}
	name := &QualifiedName{Parts: []NameSegment{{Text: "A"}, {Text: "'b c'"}, {Text: "D"}}}
	if got, want := name.Text(), "A::'b c'::D"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDeclMembers(t *testing.T) {
	member := &Membership{}
	def := &Definition{Members: []Node{member}}
	if got := DeclMembers(def); len(got) != 1 || got[0] != member {
		t.Fatalf("definition members: got %v", got)
	}
	usage := &Usage{Members: []Node{member}}
	if got := DeclMembers(usage); len(got) != 1 || got[0] != member {
		t.Fatalf("usage members: got %v", got)
	}
	if got := DeclMembers(&Membership{}); got != nil {
		t.Fatalf("non-declaration: got %v, want nil", got)
	}
	if got := DeclMembers(nil); got != nil {
		t.Fatalf("nil node: got %v, want nil", got)
	}
}
