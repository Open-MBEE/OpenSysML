package grpc

import (
	"slices"
	"testing"
)

// answeredElements reads one property of the element id and names each value by qualified name,
// or by id and metaclass where the element is unnamed.
func answeredElements(t *testing.T, r *elementPropertiesReader, id, property string) []string {
	t.Helper()
	resp := r.read(id, property)
	if resp.Error != "" {
		t.Fatalf("%s.%s: %s", id, property, resp.Error)
	}
	if len(resp.Properties) != 1 || resp.Properties[0].Unsupported != "" {
		t.Fatalf("%s.%s = %v", id, property, resp.Properties)
	}
	var out []string
	for _, value := range resp.Properties[0].Values {
		el := value.GetElement()
		if el == nil {
			t.Fatalf("%s.%s holds a value that is no element: %v", id, property, value)
		}
		if el.QualifiedName != "" {
			out = append(out, el.QualifiedName)
		} else {
			out = append(out, el.Id+":"+el.Metaclass)
		}
	}
	return out
}

// Type::input is every in or inout feature of the type (KerML 8.3.3.1.10): a send action's payload and
// sender parameters, then the receiver it inherits, not the argument expressions valuing them.
func TestGetElementPropertiesListsASendActionsInputParameters(t *testing.T) {
	// Sig declares no feature for the argument, which the reader answers over all the same.
	r := parseForElementPropertiesWithErrors(t, `package P {
		attribute def Sig;
		port def SP;
		part def T {
			port sp : SP;
			attribute gain;
			action sendRef send new Sig(gain) via sp;
		}
	}`)
	parameters := answeredElements(t, r, "P__T__sendRef", "parameter")
	// Each is named by the SendAction parameter it redefines (SysML 8.4.12.4), and keeps its positional id.
	if want := []string{"P::T::sendRef::payload", "P::T::sendRef::sender"}; !slices.Equal(parameters, want) {
		t.Fatalf("parameter = %v, want %v", parameters, want)
	}
	for _, parameter := range []string{"P__T__sendRef___400", "P__T__sendRef___401"} {
		resp := r.read(parameter, "direction")
		if len(resp.Properties) != 1 || len(resp.Properties[0].Values) != 1 || resp.Properties[0].Values[0].GetStringValue() != "in" {
			t.Fatalf("%s.direction = %v, want in", parameter, resp.Properties)
		}
	}
	input := answeredElements(t, r, "P__T__sendRef", "input")
	if want := append(parameters, "Transfers::SendPerformance::receiver"); !slices.Equal(input, want) {
		t.Errorf("input = %v, want %v", input, want)
	}
}
