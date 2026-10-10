package main

import "testing"

const hiddenImportModel = `package Left { part def Engine; }
package Right { part def Engine; }
package Use { private import Left::*; private import Right::*; part e : Engine; }`

// Two imports bringing distinct elements of one name hide both from the
// importing namespace, so the unqualified name is an ordinary unresolved
// reference, in default and strict mode alike; the import warning stays.
func TestValidateHiddenImportedName(t *testing.T) {
	binary := buildCLI(t)
	for _, args := range [][]string{{"-validate"}, {"-strict", "-validate"}} {
		wantReport(t, check(t, binary, hiddenImportModel, args...), 2,
			"warning: Duplicate of imported member name 'Engine': Left::Engine (import Left::*), Right::Engine (import Right::*)",
			"error: unresolved reference: Engine",
			"The name is hidden here: import Left::* and import Right::* bring distinct elements named 'Engine', so none is a member; qualify the one meant: Left::Engine or Right::Engine.")
	}
}

const hiddenReexportModel = `package Left { part def Engine; }
package Right { part def Engine; }
package Both { public import Left::*; public import Right::*; }
package Use { private import Both::*; part e : Engine; }`

// A re-export namespace has no member under a name it hides, so an import of
// it brings none; the use-site hint names the imports that hide it.
func TestValidateHiddenImportedNameThroughReexport(t *testing.T) {
	binary := buildCLI(t)
	wantReport(t, check(t, binary, hiddenReexportModel, "-validate"), 2,
		"error: unresolved reference: Engine",
		"The name is hidden here: import Left::* and import Right::* bring distinct elements named 'Engine', so none is a member; qualify the one meant: Left::Engine or Right::Engine.")
}
