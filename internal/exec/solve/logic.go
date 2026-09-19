package solve

import "strings"

// NonStandardLogic is the logic a script sets when no SMT-LIB logic covers what
// it uses. The SMT-LIB 2.6 logic list names no logic with algebraic datatypes or
// with strings, so a query using either sets this, which z3 and cvc5 accept.
const NonStandardLogic = "ALL"

// Features is what a query uses of SMT-LIB: the sorts its variables and terms
// range over, and the arithmetic its terms apply.
type Features struct {
	// Bool, Int, Real, Strings and Datatypes are set for each sort used, by a
	// declared variable or by a term.
	Bool      bool
	Int       bool
	Real      bool
	Strings   bool
	Datatypes bool

	// Nonlinear is set when a product or a quotient of two non-literal terms is
	// asserted, which no linear logic admits.
	Nonlinear bool

	// IntegerDivision is set when `div` or `mod` from the Ints theory is used.
	// By a literal divisor this stays linear; by a computed one it is nonlinear.
	IntegerDivision bool
}

// Features inventories what the query uses, reading its terms rather than trusting
// the flags a translation recorded, so a hand-built query inventories the same.
func (q *Query) Features() Features {
	var f Features
	if q == nil {
		return f
	}
	f.Nonlinear, f.IntegerDivision = q.Nonlinear, q.IntegerDivision
	note := func(s Sort) {
		switch s.Kind {
		case SortBool:
			f.Bool = true
		case SortInt:
			f.Int = true
		case SortReal:
			f.Real = true
		case SortString:
			f.Strings = true
		case SortDatatype:
			f.Datatypes = true
		}
	}
	for _, v := range q.Vars {
		note(v.Sort)
	}
	for _, s := range q.Sorts {
		note(s)
	}
	for _, a := range q.Assertions {
		a.Term.walk(func(t *Term) {
			note(t.Sort)
			f.noteTerm(t)
		})
	}
	for _, o := range q.Objectives {
		o.Term.walk(func(t *Term) {
			note(t.Sort)
			f.noteTerm(t)
		})
	}
	return f
}

// noteTerm records the arithmetic one term applies: integer division, and a
// product or quotient outside the linear fragment.
func (f *Features) noteTerm(t *Term) {
	switch t.Op {
	case OpIntDiv:
		f.IntegerDivision = true
		if !t.Args[1].Literal() {
			f.Nonlinear = true
		}
	case OpDiv:
		if !t.Args[1].Literal() {
			f.Nonlinear = true
		}
	case OpMul:
		if !t.Args[0].Literal() && !t.Args[1].Literal() {
			f.Nonlinear = true
		}
	}
}

// LogicChoice is the logic a script sets: its name, whether SMT-LIB defines it,
// and what the query uses that forced it.
type LogicChoice struct {
	// Name is the logic as `set-logic` names it.
	Name string

	// Standard reports that the SMT-LIB 2.6 logic list defines Name. It is false
	// only for NonStandardLogic.
	Standard bool

	// Why names the features that forced this logic, for the script's comment
	// and for a message about a solver that refuses it.
	Why string
}

// LogicChoice names the narrowest SMT-LIB logic that covers what the query uses,
// falling back to NonStandardLogic only for features the logic list has no logic
// for. Names are the SMT-LIB 2.6 list's own; nothing is widened to avoid a hard
// case, and a wider logic than the terms need is never set.
func (q *Query) LogicChoice() LogicChoice {
	f := q.Features()
	switch {
	case f.Datatypes || f.Strings:
		return LogicChoice{Name: NonStandardLogic, Why: unstandardisedWhy(f)}
	case f.Int && f.Real:
		// The only SMT-LIB logics over Reals_Ints are the quantified AUFLIRA and
		// AUFNIRA: there is no quantifier-free mixed logic to narrow to.
		if f.Nonlinear {
			return LogicChoice{Name: "AUFNIRA", Standard: true, Why: "nonlinear mixed integer and real arithmetic"}
		}
		return LogicChoice{Name: "AUFLIRA", Standard: true, Why: "linear mixed integer and real arithmetic"}
	case f.Real:
		if f.Nonlinear {
			return LogicChoice{Name: "QF_NRA", Standard: true, Why: "nonlinear real arithmetic"}
		}
		return LogicChoice{Name: "QF_LRA", Standard: true, Why: "linear real arithmetic"}
	case f.Int:
		if f.Nonlinear {
			return LogicChoice{Name: "QF_NIA", Standard: true, Why: "nonlinear integer arithmetic"}
		}
		return LogicChoice{Name: "QF_LIA", Standard: true, Why: "linear integer arithmetic"}
	}
	return LogicChoice{Name: "QF_UF", Standard: true, Why: "propositional logic over declared constants"}
}

// Logic is the name the script's `set-logic` sets.
func (q *Query) Logic() string { return q.LogicChoice().Name }

// unstandardisedWhy names the features that no SMT-LIB logic covers.
func unstandardisedWhy(f Features) string {
	var uses []string
	if f.Datatypes {
		uses = append(uses, "algebraic datatypes (declare-datatypes)")
	}
	if f.Strings {
		uses = append(uses, "the strings theory")
	}
	return strings.Join(uses, " and ")
}

// Requires are the capabilities a backend must have to answer this query: what
// its sorts and operators use, and the non-standard logic when it needs one.
// Capabilities an operation adds — a model, an unsat core — are the operation's.
func (q *Query) Requires() []Capability {
	if q == nil {
		return nil
	}
	f := q.Features()
	var out []Capability
	if f.Datatypes {
		out = append(out, CapDatatypes)
	}
	if f.Strings {
		out = append(out, CapStrings)
	}
	if f.IntegerDivision {
		out = append(out, CapIntegerDivision)
	}
	if f.Nonlinear {
		out = append(out, CapNonlinearArith)
	}
	if f.Int && f.Real {
		out = append(out, CapMixedArith)
	}
	if !q.LogicChoice().Standard {
		out = append(out, CapNonStandardLogic)
	}
	return out
}
