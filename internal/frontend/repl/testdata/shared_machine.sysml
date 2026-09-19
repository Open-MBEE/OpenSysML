// A part exhibiting one state definition as two usages, so the definition's
// name alone names no one running machine; and a part whose value does not
// materialize, so a path through it fails for a reason other than absence.
package Shared {
	state def Blink {
		entry; then dark;
		state dark;
		accept after 2 [SI::s] then lit;
		state lit;
	}

	part def Bulb;

	part def Lamp {
		exhibit state front : Blink;
		exhibit state rear : Blink;
		part spare : Bulb = null;
	}

	part lamp : Lamp;
}
