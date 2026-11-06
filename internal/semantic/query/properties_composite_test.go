package query

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// compositionModel declares one usage of each kind the derivation tells apart:
// composite by default, `ref`, an end, a directed parameter, and a KerML feature.
const compositionModel = `package Demo {
	part def Engine;
	part def Person;
	part def Car {
		part engine : Engine;
		ref part driver : Person;
		attribute mass : ScalarValues::Real;
		ref attribute label : ScalarValues::String;
		item cargo;
		port p { port inner; }
		action drive { in speed : ScalarValues::Real; out odometer : ScalarValues::Real; }
		connection link connect engine to driver;
	}
}`

const compositionKerML = `package Kernel {
	classifier Thing;
	classifier Whole {
		composite feature piece : Thing;
		feature known : Thing;
	}
}`

func compositionReader(t *testing.T, name, text string) (*PropertyReader, func(string) *symbols.Symbol) {
	t.Helper()
	file := source.New(name, []byte(text))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	// Over the standard library, as every frontend reads a model: a port's
	// featuring type is known to be a Part only through it.
	base, _ := libs.SharedLibrary()
	index := symbols.NewOverlay(base)
	index.AddDocument(file.Name(), root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	lookup := func(fqn string) *symbols.Symbol {
		t.Helper()
		matches := index.LookupQualified(fqn)
		if len(matches) != 1 {
			t.Fatalf("lookup %s: %d symbols", fqn, len(matches))
		}
		return matches[0]
	}
	return NewPropertyReader(index, resolver, model), lookup
}

// Feature::isComposite and Usage::isReference read what the model derives
// (deriveUsageIsReference: isReference = not isComposite): a usage without
// `ref` is composite; `ref`, a directed parameter, an attribute and a port owned
// by a part (validatePortUsageIsReference) are references.
func TestIsCompositeAndIsReferenceReadUsages(t *testing.T) {
	reader, lookup := compositionReader(t, "composition.sysml", compositionModel)
	cases := []struct {
		fqn       string
		composite string
	}{
		{"Demo::Car::engine", "true"},
		{"Demo::Car::driver", "false"},
		{"Demo::Car::mass", "false"},
		{"Demo::Car::label", "false"},
		{"Demo::Car::cargo", "true"},
		{"Demo::Car::p", "false"},
		{"Demo::Car::p::inner", "true"},
		{"Demo::Car::drive", "true"},
		{"Demo::Car::drive::speed", "false"},
		{"Demo::Car::drive::odometer", "false"},
		{"Demo::Car::link", "true"},
	}
	for _, tc := range cases {
		sym := lookup(tc.fqn)
		if got, ok := reader.Values(sym, PropertyIsComposite); !ok || len(got) != 1 || got[0] != tc.composite {
			t.Errorf("%s isComposite = %v, %v; want [%s], true", tc.fqn, got, ok, tc.composite)
		}
		wantReference := "true"
		if tc.composite == "true" {
			wantReference = "false"
		}
		if got, ok := reader.Values(sym, PropertyIsReference); !ok || len(got) != 1 || got[0] != wantReference {
			t.Errorf("%s isReference = %v, %v; want [%s], true", tc.fqn, got, ok, wantReference)
		}
	}
}

// Neither property is read for a definition: it is not a feature.
func TestIsCompositeIsAbsentForADefinition(t *testing.T) {
	reader, lookup := compositionReader(t, "composition.sysml", compositionModel)
	car := lookup("Demo::Car")
	for _, property := range []string{PropertyIsComposite, PropertyIsReference} {
		if got, ok := reader.Values(car, property); ok {
			t.Errorf("Demo::Car %s = %v, true; want absent", property, got)
		}
	}
}

// A connector's ends are reference usages, so they read false/true.
func TestIsCompositeReadsAConnectorEnd(t *testing.T) {
	reader, lookup := compositionReader(t, "composition.sysml", compositionModel)
	link := lookup("Demo::Car::link")
	ends := 0
	link.Scope.ForEachMember(func(end *symbols.Symbol) bool {
		if end.Kind != symbols.SymbolConnectorEnd {
			return true
		}
		ends++
		if got, ok := reader.Values(end, PropertyIsComposite); !ok || len(got) != 1 || got[0] != "false" {
			t.Errorf("end %d: isComposite = %v, %v; want [false], true", ends, got, ok)
		}
		if got, ok := reader.Values(end, PropertyIsReference); !ok || len(got) != 1 || got[0] != "true" {
			t.Errorf("end %d: isReference = %v, %v; want [true], true", ends, got, ok)
		}
		return true
	})
	if ends != 2 {
		t.Errorf("%d connector ends, want 2", ends)
	}
}

// A KerML feature has Feature::isComposite but no Usage::isReference.
func TestIsCompositeReadsAKerMLFeatureAndIsReferenceDoesNot(t *testing.T) {
	reader, lookup := compositionReader(t, "composition.kerml", compositionKerML)
	for fqn, want := range map[string]string{"Kernel::Whole::piece": "true", "Kernel::Whole::known": "false"} {
		sym := lookup(fqn)
		if got, ok := reader.Values(sym, PropertyIsComposite); !ok || len(got) != 1 || got[0] != want {
			t.Errorf("%s isComposite = %v, %v; want [%s], true", fqn, got, ok, want)
		}
		if got, ok := reader.Values(sym, PropertyIsReference); ok {
			t.Errorf("%s isReference = %v, true; want absent", fqn, got)
		}
	}
}

// The OSLC name table and the closed property list both carry the two names.
func TestIsCompositeAndIsReferenceAreQueryableNames(t *testing.T) {
	for _, name := range []string{PropertyIsComposite, PropertyIsReference} {
		if !IsProperty(name) {
			t.Errorf("IsProperty(%q) = false", name)
		}
		if got, ok := oslcPropertyMappings[sysmlNS+name]; !ok || got != name {
			t.Errorf("oslc sysml:%s = %q, %v; want %q", name, got, ok, name)
		}
	}
}
