// A feature valued by a call through a feature chain holds the arguments the calc
// applied passes on: `lead`, returned by `picker.pickChosen`, is a Tallied and
// carries its features; `trail`, which the calc never returns, is not.
package test {
	private import ScalarValues::*;

	item def Tallied { attribute tally : Integer = 7; }
	item def Picker {
		calc pickChosen { in chosen; in other; return : Anything = chosen; }
	}
	item def Rack {
		item picker : Picker;
		item lead [1];
		item trail [1];
		item tallied : Tallied [1] = picker.pickChosen(lead, trail);
	}
	item rack : Rack;
}
