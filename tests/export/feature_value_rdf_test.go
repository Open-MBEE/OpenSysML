package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// The operator a feature value is written with decides whether a redefinition
// may override it, so `default` and `:=` must survive the trip through RDF.
func TestFeatureValueOperatorsSurviveRDF(t *testing.T) {
	turtle := roundTripsExactly(t, `package P {
	private import ScalarValues::Integer;
	part def V {
		attribute bound : Integer = 1;
		attribute overridable : Integer default = 2;
		attribute initial : Integer := 3;
		attribute both : Integer default := 4;
	}
	part def W;
	part w0 : W;
	requirement req {
		subject w : W default = w0;
	}
}
`)
	flagsOf := func(element string) (isDefault, isInitial bool) {
		_, rest, found := strings.Cut(string(turtle), "\n"+element+"\n")
		if !found {
			t.Fatalf("graph lacks %s:\n%s", element, turtle)
		}
		block, _, _ := strings.Cut(rest, " .\n")
		return strings.Contains(block, `sysml:isDefault "true"^^xsd:boolean`),
			strings.Contains(block, `sysml:isInitial "true"^^xsd:boolean`)
	}
	for element, want := range map[string][2]bool{
		"elmt:P__V__bound":       {false, false},
		"elmt:P__V__overridable": {true, false},
		"elmt:P__V__initial":     {false, true},
		"elmt:P__V__both":        {true, true},
		"elmt:P__req__w":         {true, false},
	} {
		isDefault, isInitial := flagsOf(element)
		if isDefault != want[0] || isInitial != want[1] {
			t.Errorf("%s: isDefault=%v isInitial=%v, want %v %v", element, isDefault, isInitial, want[0], want[1])
		}
	}
}

// A requirement's `assume`/`require constraint` declares a constraint usage of
// its own, whose name, specializations, multiplicity and value are as much a
// part of the model as those of any other usage.
func TestOwnedConstraintDeclarationsSurviveRDF(t *testing.T) {
	roundTripsExactly(t, `package P {
	constraint def C;
	constraint c0 : C;
	requirement def R {
		assume constraint c1 : C[1] = c0;
		require constraint c2 : C default = c0;
		require constraint c3 subsets c0 := c0;
		assume constraint c4 : C subsets c0[1] default := c0 {
			true;
		}
		require constraint c5;
		assume constraint c6 {
		}
		require c0[1] subsets c0;
	}
}
`)
}

// The flags spell a feature value's operator, so a graph stating one on a
// feature with no value, or stating a flag twice, is refused rather than
// written with the flag dropped or one of its values chosen.
func TestFeatureValueFlagsWithoutAValueOrStatedTwiceAreReported(t *testing.T) {
	src := "package P {\n\tprivate import ScalarValues::Integer;\n\tpart def V {\n\t\tattribute a : Integer default = 1;\n\t\tattribute b : Integer;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	const flag = `sysml:isDefault "true"^^xsd:boolean ;`
	if !strings.Contains(structural, flag) || !strings.Contains(structural, `sysml:declaredName "b" ;`) {
		t.Fatalf("the flagged and the unvalued attribute were not found in the graph:\n%s", structural)
	}
	refused := func(name, graph string, wants ...string) {
		t.Helper()
		_, err := export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("%s: expected an unsupported error, got %v", name, err)
		}
		for _, want := range wants {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: expected %q in error:\n%s", name, want, err.Error())
			}
		}
	}
	for _, property := range []string{"sysml:isDefault", "sysml:isInitial"} {
		for _, value := range []string{"true", "false"} {
			orphan := strings.Replace(structural, `sysml:declaredName "b" ;`, `sysml:declaredName "b" ;`+"\n    "+property+` "`+value+`"^^xsd:boolean ;`, 1)
			refused(property+" "+value+" without a value", orphan, "the subject <urn:sysmlv2:element:P__V__b>", "states "+property+" without a sysml:value")
		}
		for _, values := range []string{`"true"^^xsd:boolean, "false"^^xsd:boolean`, `"false"^^xsd:boolean, "true"^^xsd:boolean`} {
			twice := strings.Replace(structural, flag, property+" "+values+" ;", 1)
			refused(property+" as "+values, twice, "the subject <urn:sysmlv2:element:P__V__a>", "states "+property+" twice", "one of them would be dropped")
		}
	}
}
