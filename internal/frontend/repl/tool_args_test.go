package repl

import "testing"

// %tool on an action binds the invocation's arguments as the real run would:
// positionally, by name, and reports too many.
func TestToolBindsAnActionsArguments(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)

	wants(t, run(t, s, "%tool Tools::Heating(30)"), "mass = 30")
	wants(t, run(t, s, "%tool Tools::Heating(mass=30)"), "mass = 30")
	wants(t, run(t, s, "%tool Tools::Heating(1,2)"), "argument count mismatch", "takes 1 argument(s), got 2")
}
