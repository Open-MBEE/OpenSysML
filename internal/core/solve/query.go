package solve

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Role says why a query asserts a term.
type Role int

const (
	// RoleRequired is a condition the element requires to hold (`require`,
	// `assert`).
	RoleRequired Role = iota
	// RoleAssumed is a condition the element assumes (`assume`), trusted rather
	// than required.
	RoleAssumed
	// RoleDenied is the assertion a negated element makes: that its required
	// conditions do not all hold.
	RoleDenied
	// RoleDomain is a bound a declaration puts on a variable's values rather
	// than a condition the model wrote, such as a Natural being non-negative.
	RoleDomain
	// RoleDefined is a side condition a condition needs for the solver to mean by
	// it what the evaluator means, such as a divisor being non-zero.
	RoleDefined
	// RolePinned is a value the model already fixes — held by an object, declared
	// by the model, or chosen by the caller — asserted so the solver synthesises
	// only what is still free.
	RolePinned
	// RoleExcluded is an assignment already reported, denied so enumerating asks
	// for a different one.
	RoleExcluded
	// RoleTransition is one step of a behavior's execution as the interpreter
	// takes it, asserted by a model-checking query over a run.
	RoleTransition
)

var roleNames = map[Role]string{
	RoleRequired:   "required condition",
	RoleAssumed:    "assumed condition",
	RoleDenied:     "denied conditions",
	RoleDomain:     "declared domain",
	RoleDefined:    "well-definedness",
	RolePinned:     "fixed value",
	RoleExcluded:   "excluded assignment",
	RoleTransition: "transition",
}

// String names the role as an assertion's comment reads it.
func (r Role) String() string {
	if name, ok := roleNames[r]; ok {
		return name
	}
	return "condition"
}

// Var is one variable a query declares: the value a feature may take. It is
// unconstrained except by its sort and the domain assertions the query makes
// about it — values a model declares are not asserted here.
type Var struct {
	// Name is the variable's name, the qualified name of the feature it stands
	// for, with a feature chain's further steps appended with '.'.
	Name string

	// Sort is the sort of its values.
	Sort Sort

	// Symbol is the feature declaration it stands for.
	Symbol *symbols.Symbol

	// Dimension is the quantity dimension its magnitude is expressed in, over
	// base units, empty for a value that has none.
	Dimension string

	// Unit names the base units its magnitude is expressed in ("g", "m·s^-1"),
	// empty when no quantity of that dimension was written to name them.
	Unit string

	// File and Span are where the feature was declared.
	File string
	Span source.Span

	// Location renders File and Span as `file:line:col`.
	Location string
}

// Provenance records what an assertion came from, so a later step can map an
// answer about the script back to what the user wrote.
type Provenance struct {
	// Kind is the kind of element the assertion came from: "constraint",
	// "requirement", "satisfaction", or "declaration" for a domain assertion.
	Kind string

	// Element names that element as a verdict about it would.
	Element string

	// Condition is the condition as written, negation and grouping included.
	Condition string

	// Role says why the query asserts the term.
	Role Role

	// Declared is the element that declared the condition, which is the
	// supertype it was inherited from for an inherited one.
	Declared *symbols.Symbol

	// File and Span are where the condition was written.
	File string
	Span source.Span

	// Location renders File and Span as `file:line:col`.
	Location string
}

// Direction is the way an objective's value is to be improved.
type Direction int

const (
	// Minimize asks for the least value the conditions permit.
	Minimize Direction = iota
	// Maximize asks for the greatest value the conditions permit.
	Maximize
)

// String names the direction as the emitted objective form reads.
func (d Direction) String() string {
	if d == Maximize {
		return "maximize"
	}
	return "minimize"
}

// Objective is one objective a query optimizes: which way its value is to be
// improved, the term stating that value, and where the objective was written.
type Objective struct {
	// Direction is the way its value is to be improved.
	Direction Direction

	// Term is the translated objective expression, of an arithmetic sort.
	Term *Term

	// Name is the objective's name as the model wrote it, empty for an anonymous
	// one.
	Name string

	// Symbol is the objective usage it came from.
	Symbol *symbols.Symbol

	// Expression is the objective expression as written.
	Expression string

	// Dimension is the quantity dimension its value is expressed in over base
	// units, empty for a value that has none.
	Dimension string

	// Unit names the base units its magnitude is expressed in ("g", "m·s^-1"),
	// empty when the value is no quantity or no quantity named them.
	Unit string

	// File and Span are where the objective was written.
	File string
	Span source.Span

	// Location renders File and Span as `file:line:col`.
	Location string
}

// Assertion is one term the query asserts, with where it came from.
type Assertion struct {
	// Term is the boolean term asserted.
	Term *Term

	// From records the condition the term encodes.
	From Provenance
}

// Query is a translated element: the variables its conditions read, the finite
// sorts they range over, and the terms it asserts. It is what a solver-facing
// step consumes, and what the SMT-LIB2 writer writes.
type Query struct {
	// Kind is the kind of element translated: "constraint", "requirement" or
	// "satisfaction".
	Kind string

	// Element names the translated element as a verdict about it would.
	Element string

	// Negated is set for an element asserting that its conditions do not all
	// hold (`assert not …`), which the query asserts as one denial.
	Negated bool

	// Sorts are the datatype sorts the query declares, ordered by name.
	Sorts []Sort

	// Vars are the variables the query declares, ordered by name.
	Vars []*Var

	// Assertions are the terms asserted: declared domains first, then the
	// well-definedness side conditions, then the conditions in the order the
	// evaluator checks them.
	Assertions []Assertion

	// Nonlinear is set when a product or a quotient of two non-literal terms was
	// asserted, which is what a nonlinear logic is set for.
	Nonlinear bool

	// IntegerDivision is set when integer division or remainder was encoded,
	// which needs `div` and `mod` from the Ints theory of a backend.
	IntegerDivision bool

	// Pinned are the values the query fixes rather than leaves free, each naming
	// the assertion that fixes it; nil for a query that fixes none.
	Pinned []PinnedValue

	// Unread are the values that were to be fixed but that no variable of the
	// query reads, reported rather than dropped.
	Unread []Unread

	// Objectives are the objectives to optimize, in the order the analysis case
	// declares them, which is the order they are optimized in; nil for a query
	// that only asks about satisfiability.
	Objectives []Objective
}

// Optimizes reports whether the query asks for an optimum rather than only for
// satisfiability.
func (q *Query) Optimizes() bool { return len(q.Objectives) > 0 }

// Fixes reports whether the query fixes any value, which is what makes an unsat
// verdict about it a verdict about those values too.
func (q *Query) Fixes() bool { return len(q.Pinned) > 0 }

// Free are the variables the query leaves for the solver to choose, in the order
// they are declared.
func (q *Query) Free() []*Var {
	fixed := make(map[*Var]bool, len(q.Pinned))
	for _, p := range q.Pinned {
		fixed[p.Var] = true
	}
	out := make([]*Var, 0, len(q.Vars))
	for _, v := range q.Vars {
		if !fixed[v] {
			out = append(out, v)
		}
	}
	return out
}

// sortVars orders variables by name, which is what makes a script deterministic.
func sortVars(vars []*Var) {
	sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
}

// sortSorts orders datatype sorts by name.
func sortSorts(sorts []Sort) {
	sort.Slice(sorts, func(i, j int) bool { return sorts[i].Name < sorts[j].Name })
}

// sortAssertions orders assertions by what they came from, which is what makes
// declared domains, collected as variables appear, deterministic.
func sortAssertions(asserts []Assertion) {
	sort.SliceStable(asserts, func(i, j int) bool {
		a, b := asserts[i].From, asserts[j].From
		if a.Element != b.Element {
			return a.Element < b.Element
		}
		return a.Condition < b.Condition
	})
}
