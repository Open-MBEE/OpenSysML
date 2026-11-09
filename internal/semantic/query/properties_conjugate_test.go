package query

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The ConjugatedPortDefinition a port definition implies (SysML v2 8.3.12.3) declares nothing,
// yet it is a definition: its isAbstract and isIndividual read false, as the conversion states them.
func TestTypeAttributesReadTheImpliedConjugatedPortDefinition(t *testing.T) {
	file := source.New("ports.sysml", []byte(`package Demo {
		port def DataPort { attribute d; }
	}`))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	index := symbols.NewIndex()
	index.AddDocument(file.Name(), root)
	conjugates := index.LookupQualified("Demo::DataPort::~DataPort")
	if len(conjugates) != 1 || !conjugates[0].IsConjugatedPortDefinition() {
		t.Fatalf("index has no Demo::DataPort::~DataPort conjugate: %v", conjugates)
	}
	reader := NewPropertyReader(index, nil, nil)
	for _, property := range []string{PropertyIsAbstract, PropertyIsIndividual} {
		values, ok := reader.Values(conjugates[0], property)
		if !ok || len(values) != 1 || values[0] != "false" {
			t.Errorf("~DataPort %s = %v, %v; want [false], true", property, values, ok)
		}
	}
}

// A conjugated typing types its feature by the ConjugatedPortDefinition the written
// definition implies (SysML v2 8.3.12.3), so `type` names `P::'~P'`, not P.
func TestTypeOfAConjugatedTypingIsTheConjugate(t *testing.T) {
	file := source.New("ports.sysml", []byte(`package Demo {
		port def CmdPort;
		part def Sender { port cmdOut : CmdPort; }
		part def Receiver { port cmdIn : ~CmdPort; }
	}`))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	index := symbols.NewIndex()
	index.AddDocument(file.Name(), root)
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	reader := NewPropertyReader(index, resolver, model)
	lookup := func(fqn string) *symbols.Symbol {
		t.Helper()
		matches := index.LookupQualified(fqn)
		if len(matches) != 1 {
			t.Fatalf("lookup %s: %d symbols", fqn, len(matches))
		}
		return matches[0]
	}
	for _, tc := range []struct{ port, want string }{
		{"Demo::Sender::cmdOut", "Demo::CmdPort"},
		{"Demo::Receiver::cmdIn", "Demo::CmdPort::~CmdPort"},
	} {
		values, ok := reader.Values(lookup(tc.port), PropertyElementType)
		if !ok || len(values) != 1 || values[0] != tc.want {
			t.Errorf("%s type = %v, %t; want [%s]", tc.port, values, ok, tc.want)
		}
	}
}
