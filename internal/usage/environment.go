package usage

// BudgetEnvironment describes the run bounds every binary reads, in one
// wording for all three pages. docs/reference/environment.md is the long form.
func BudgetEnvironment() []Item {
	return []Item{
		{"OPENSYSML_LIBRARY_PATH", "Directory to load the SysML/KerML standard library from, instead of the copy embedded in the binary."},
		{"OPENSYSML_MAX_STEPS", "Expression evaluations one run may spend before it is reported as a runaway. Default 10000000."},
		{"OPENSYSML_MAX_ACTION_STEPS", "Token-flow steps one action run may perform. Default 1000000."},
		{"OPENSYSML_MAX_EVENTS", "Events one state machine run may dispatch. Default 1000000."},
		{"OPENSYSML_MAX_DO_STEPS", "Do actions one state machine run may perform. Default 5000000."},
		{"OPENSYSML_MAX_ELEMENTS", "Collection elements one evaluation may hold, which bounds the memory a run holds rather than the work it does. Default 1000000."},
		{"OPENSYSML_MAX_CALC_DEPTH", "Nested calc invocations one run may hold on the stack, which is what a recursion spends. Default 10000, ceiling 25000."},
		{"OPENSYSML_MAX_SWEEP_RUNS", "Runs one parameter sweep or sample may make, each a whole analysis or calc run of its own with its own budgets. Default 1000."},
	}
}

// LegacyPrefixNote states how the superseded variable names are still read, and
// belongs with any list of them.
const LegacyPrefixNote = "Each variable above also answers to its legacy " +
	"SYSML_-prefixed name (SYSML_MAX_STEPS for OPENSYSML_MAX_STEPS, and so on), " +
	"which remains accepted. When both are set and the OPENSYSML_ value is " +
	"non-empty, the OPENSYSML_ value wins; setting only the legacy name prints a " +
	"one-time deprecation warning on standard error."

// A budget bounds one run rather than a session, which is the distinction a
// reader raising one needs.
const BudgetScopeNote = "A budget bounds one run — one evaluation, one " +
	"instantiation, one calc invocation, one action, one state machine — not a " +
	"whole session, so a long session of small operations never exhausts one. A " +
	"run started inside another shares the outer run's budget."
