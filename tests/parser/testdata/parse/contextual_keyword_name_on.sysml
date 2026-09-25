// `on` is a name, not a keyword: the pilot's grammars use the word nowhere, and
// the OMG training corpus declares a state `on` and names it as a transition
// target (`24. States/State Actions.sysml`, `25. Transitions/Transition Actions.sysml`).
package P {
	attribute def OnSignal;
	attribute def OffSignal;

	state def Machine {
		first start then off;

		state off;
		accept OnSignal
			then on;

		state on {
			entry action light;
			exit action dark;
		}
		accept OffSignal
			then off;

		// A source named `on` and a trigger read one after the other.
		transition first on accept OffSignal then off;
	}

	part def Lamp {
		attribute on : Boolean;
		attribute lit = on and not on;
		port on : OnPort;
	}
	port def OnPort;
}
