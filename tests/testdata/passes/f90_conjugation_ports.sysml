// In SysML `~` occurs only in a ConjugatedPortTyping (SysML.xtext:974
// ConjugatedQualifiedName), whose target must be a port definition: scoping the
// KerML case must not retire that rule, and this notation must stay clean.
// The non-port target is a negative case, kept inline in f90_conjugation_scope_test.go.
package F90ConjugationPorts {
	port def P {
		in item command;
	}

	part def X {
		port p : ~P;
	}
}
