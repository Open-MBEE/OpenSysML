package queryexec

import (
	"strings"
	"testing"
)

// The occurrence attributes as the pilot exports them: a `state s;` of a state body and a
// connector's port end are OccurrenceUsages with an isIndividual, an enumeration definition is abstract.
const occurrenceAttributeModel = `
package Operations {
	port def OpticalPort;
	part def Subsystem { out port opticalOut : OpticalPort; in port opticalIn : OpticalPort; }
	part a : Subsystem; part b : Subsystem;
	state def TelescopeState;
	state telescopeState : TelescopeState {
		entry; then idle;
		state idle;
		state slewing;
		transition idle_to_slewing first idle then slewing;
	}
	enum def ObservingMode { imaging; spectroscopy; }
	interface def OpticalInterface { end source : OpticalPort; end target : OpticalPort; }
	interface opticalLink : OpticalInterface connect a.opticalOut to b.opticalIn;
}
calc def Individuals :> Query {
	in root : Element;
	in type : String;
	Project(source = WhereType(source = Descendants(source = root), type = type, exact = true), properties = ("name", "isIndividual"))
}
calc def Abstracts :> Query {
	in root : Element;
	in type : String;
	Project(source = WhereType(source = Descendants(source = root), type = type, exact = true), properties = ("name", "isAbstract"))
}
`

func TestOccurrenceAttributesAnswerAsThePilotExportsThem(t *testing.T) {
	fixture := loadExecutionFixture(t, occurrenceAttributeModel)
	context := Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}
	cases := []struct{ program, metaclass, want string }{
		{"Individuals", "StateUsage", "telescopeState false | idle false | slewing false"},
		{"Individuals", "TransitionUsage", "idle_to_slewing false"},
		{"Individuals", "PortUsage", "opticalOut false | opticalIn false | source false | target false | source false | target false"},
		{"Abstracts", "EnumerationDefinition", "ObservingMode true"},
	}
	for _, c := range cases {
		bindings := Bindings{
			"root": {ElementValue(fixture.symbol(t, "Operations"))},
			"type": {StringValue(c.metaclass)},
		}
		got, err := Execute(fixture.program(t, c.program), context, bindings, Options{})
		if err != nil {
			t.Fatalf("%s over %s: %v", c.program, c.metaclass, err)
		}
		if cells := strings.Join(metaclassPropertyCells(t, got), " | "); cells != c.want {
			t.Errorf("%s over %s = %q, want %q", c.program, c.metaclass, cells, c.want)
		}
	}
}
