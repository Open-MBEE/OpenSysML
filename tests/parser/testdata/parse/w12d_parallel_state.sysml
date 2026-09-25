// A state body may be marked parallel (SysML.xtext StateDefBody,
// StateUsageBody).
package W12DParallelState {
	state def S1 parallel {
		state s11;
		state s12;
	}

	part p {
		state s parallel {
			state a;
			state b;
		}
	}
}
