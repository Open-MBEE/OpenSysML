package ast

import "testing"

func TestDumpDefinition(t *testing.T) {
	def := &Definition{
		Kind:       DefPart,
		IsAbstract: true,
		Ident:      Identification{Name: "Vehicle"},
		Relationships: []*Relationship{
			{Kind: RelSpecializes, Target: &QualifiedName{Parts: []NameSegment{{Text: "Base"}}}},
		},
		Members: []Node{
			&Usage{Kind: UsagePart, Ident: Identification{Name: "engine"}},
		},
		HasBody: true,
	}
	got := Dump(def)
	want := "(Definition kind=\"part\" abstract=true variation=false name=\"Vehicle\"\n" +
		"  (Relationship kind=\"specializes\" target=Base\n" +
		"    (*ast.QualifiedName))\n" +
		"  (Usage kind=\"part\" name=\"engine\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false))"
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpUsageWithMultiplicityAndValue(t *testing.T) {
	u := &Usage{
		Kind:  UsageAttribute,
		Ident: Identification{Name: "mass"},
		Relationships: []*Relationship{
			{Kind: RelTyping, Target: &QualifiedName{Parts: []NameSegment{{Text: "Real"}}}},
		},
		Multiplicity: &Multiplicity{Upper: &LiteralInteger{Value: "4"}},
		Value:        &LiteralInteger{Value: "42"},
	}
	got := Dump(u)
	want := "(Usage kind=\"attribute\" name=\"mass\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false\n" +
		"  (Relationship kind=\"typing\" target=Real\n" +
		"    (*ast.QualifiedName))\n" +
		"  (Multiplicity range=false\n" +
		"    (LiteralInteger value=\"4\"))\n" +
		"  (LiteralInteger value=\"42\"))"
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
