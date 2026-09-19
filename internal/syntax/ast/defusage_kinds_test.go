package ast

import "testing"

func qn(name string) *QualifiedName {
	return &QualifiedName{Parts: []NameSegment{{Text: name}}}
}

func TestDumpConnectorEnds(t *testing.T) {
	u := &Usage{
		Kind:  UsageConnection,
		Ident: Identification{Name: "c"},
		ConnectorEnds: []*ConnectorEnd{
			{Target: qn("a")},
			{Target: qn("b")},
		},
	}
	got := Dump(u)
	want := "(Usage kind=\"connection\" name=\"c\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false\n" +
		"  (ConnectorEnd target=\"a\"\n" +
		"    (*ast.QualifiedName))\n" +
		"  (ConnectorEnd target=\"b\"\n" +
		"    (*ast.QualifiedName)))"
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpFlowEnds(t *testing.T) {
	u := &Usage{
		Kind:  UsageFlow,
		Ident: Identification{Name: "f"},
		FlowEnds: &FlowEnds{
			From:    qn("a"),
			To:      qn("b"),
			Payload: qn("Fuel"),
		},
	}
	got := Dump(u)
	want := "(Usage kind=\"flow\" name=\"f\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false\n" +
		"  (FlowEnds from=\"a\" to=\"b\" payload=\"Fuel\" declared=false))"
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpConjugated(t *testing.T) {
	u := &Usage{
		Kind:  UsagePort,
		Ident: Identification{Name: "p"},
		Relationships: []*Relationship{{
			Kind:       RelTyping,
			Target:     qn("P"),
			Conjugated: true,
		}},
	}
	if _, ok := u.ConjugatedTyping(); !ok {
		t.Fatal("ConjugatedTyping() did not report the `~` typing")
	}
	got := Dump(u)
	want := "(Usage kind=\"port\" name=\"p\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false\n" +
		"  (Relationship kind=\"typing\" target=P conjugated=true\n" +
		"    (*ast.QualifiedName)))"
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpFlowEndsDeclaredPayload(t *testing.T) {
	payload := &Usage{Kind: UsageAttribute, Ident: Identification{Name: "pay"}}
	u := &Usage{
		Kind:  UsageFlow,
		Ident: Identification{Name: "f"},
		FlowEnds: &FlowEnds{
			From:        qn("a"),
			To:          qn("b"),
			Payload:     qn("pay"),
			PayloadDecl: payload,
		},
		Members: []Node{payload},
	}
	got := Dump(u)
	want := "(Usage kind=\"flow\" name=\"f\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false\n" +
		"  (FlowEnds from=\"a\" to=\"b\" payload=\"pay\" declared=true)\n" +
		"  (Usage kind=\"attribute\" name=\"pay\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false))"
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
