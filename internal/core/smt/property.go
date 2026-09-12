package smt

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// PropertyKind is what a property asks of every state of a run.
type PropertyKind int

const (
	// PropertyDeadlock asks that no run stops with a token left that can never act.
	PropertyDeadlock PropertyKind = iota
	// PropertyRequirement asks that a requirement's conditions hold at every move boundary.
	PropertyRequirement
	// PropertyConstraint asks that a constraint's conditions hold at every move boundary.
	PropertyConstraint
)

func (k PropertyKind) String() string {
	switch k {
	case PropertyDeadlock:
		return "deadlock freedom"
	case PropertyRequirement:
		return "requirement"
	case PropertyConstraint:
		return "constraint"
	}
	return fmt.Sprintf("property(%d)", int(k))
}

// Property is a property of the runs of an encoding, decided at every state the
// interpreter reaches: after move 0 (the initial state) through move k.
type Property struct {
	Kind PropertyKind
	// Condition is the requirement or constraint asked about; nil for deadlock freedom.
	Condition *symbols.Symbol
	// Name is the property as a verdict names it.
	Name string
	// Violated[i] holds when state i violates the property: a required
	// condition false there, or move i stuttered with tokens left.
	Violated []*solve.Term
	// Undefined[i] holds when the interpreter could not evaluate the property
	// at state i: a feature it reads holds no value, or a side condition fails.
	Undefined []*solve.Term
}

// Deadlock is the property that no run stops short: a move stutters only once
// the flow is complete.
func (e *Encoding) Deadlock() *Property {
	p := &Property{Kind: PropertyDeadlock, Name: "deadlock freedom",
		Violated: make([]*solve.Term, e.Moves+1), Undefined: make([]*solve.Term, e.Moves+1)}
	p.Violated[0] = solve.Not(solve.Or(solve.Or(e.Choosable[0]...), e.Completed[0]))
	p.Undefined[0] = solve.BoolTerm(false)
	for i := 1; i <= e.Moves; i++ {
		stutter := eq(solve.VarTerm(e.Choices[i-1].Choice), solve.ValueTerm(e.Sorts.Choice, Stutter))
		p.Violated[i] = solve.And(stutter, solve.Not(e.Completed[i-1]))
		p.Undefined[i] = solve.BoolTerm(false)
	}
	return p
}

// Condition is the property that the requirement or constraint sym states,
// its inherited conditions included, holds at every state, judged as the
// interpreter judges it against the performance's values. A condition reading
// a feature the action does not carry is refused: the stage encodes no object.
func (e *Encoding) Condition(ctx *runtime.Context, sym *symbols.Symbol, scope *symbols.Scope) (*Property, error) {
	if sym == nil {
		return nil, fmt.Errorf("%w: no condition named", runtime.ErrNoConditions)
	}
	kind := PropertyConstraint
	if err := runtime.RequireConstraint(sym); err != nil {
		if runtime.RequireRequirement(sym) != nil {
			return nil, err
		}
		kind = PropertyRequirement
	}
	subject := solve.Subject{Kind: kind.String(), Name: sym.Name, Symbol: sym, Negated: runtime.NegatedDecl(sym)}
	translator, err := solve.NewTranslator(ctx, subject)
	if err != nil {
		return nil, err
	}
	expr, err := translator.Conditions(ctx.ConditionsOf(sym, scope))
	if err != nil {
		return nil, err
	}
	if err := e.adopt(translator, expr, kind.String()+" "+sym.Name); err != nil {
		return nil, err
	}
	p := &Property{Kind: kind, Condition: sym, Name: kind.String() + " " + sym.Name,
		Violated: make([]*solve.Term, e.Moves+1), Undefined: make([]*solve.Term, e.Moves+1)}
	for i := 0; i <= e.Moves; i++ {
		value, defined := e.environment(e.States[i]).evaluate(expr)
		p.Violated[i] = solve.And(defined, solve.Not(value))
		p.Undefined[i] = solve.Not(defined)
	}
	return p, nil
}

// adopt rewrites a property's reads to the encoding's features and declares the
// sorts it ranges over, refusing a read of a feature no state carries.
func (e *Encoding) adopt(translator *solve.Translator, expr *solve.Expression, within string) error {
	label := nodeLabel(e.Flow.Graph.Initial)
	var refused error
	visit := func(v *solve.Var) *solve.Term {
		held := e.resolve(v)
		if _, ok := e.features[held.Name]; !ok {
			if refused == nil {
				refused = &UnsupportedError{Node: label, Construct: within,
					Reason: "it reads " + v.Name + ", a feature the action does not carry; objects and their features are encoded by a later stage"}
			}
			return nil
		}
		if held == v {
			return nil
		}
		return solve.VarTerm(held)
	}
	expr.Term = solve.Substitute(expr.Term, visit)
	for i, d := range expr.Defined {
		expr.Defined[i] = solve.Substitute(d, visit)
	}
	if refused != nil {
		return refused
	}
	declared := make(map[string]bool, len(e.Query.Sorts))
	for _, s := range e.Query.Sorts {
		declared[s.Name] = true
	}
	for _, s := range translator.Sorts() {
		if !declared[s.Name] {
			e.Query.Sorts = append(e.Query.Sorts, s)
		}
	}
	e.Query.Nonlinear = e.Query.Nonlinear || nonlinear([]solve.Assertion{{Term: expr.Term}})
	e.Query.IntegerDivision = e.Query.IntegerDivision || integerDivision([]solve.Assertion{{Term: expr.Term}})
	return nil
}

// sound holds when state i is one the interpreter reaches without an error:
// no move so far failed or overflowed the token slots.
func (e *Encoding) sound(i int) *solve.Term {
	s := e.States[i]
	terms := []*solve.Term{solve.Not(solve.VarTerm(s.Failed))}
	if s.Overflow != nil {
		terms = append(terms, solve.Not(solve.VarTerm(s.Overflow)))
	}
	return solve.And(terms...)
}

// MarkVar is the integer variable a violation or failure query adds: the state
// its model points at, so a witness knows how far to replay.
const MarkVar = "mark"

// Violation is the query satisfiable exactly when some run of at most k moves
// reaches, without an error, a state violating p. Its Vars are the relation's
// and MarkVar, so a model decodes as a witness.
func (e *Encoding) Violation(p *Property) *solve.Query {
	cases := make([]*solve.Term, 0, e.Moves+1)
	for i := 0; i <= e.Moves; i++ {
		cases = append(cases, solve.And(e.sound(i), p.Violated[i]))
	}
	return e.marked(cases, "violation of "+p.Name)
}

// Failure is the query satisfiable exactly when some run of at most k moves
// meets what the interpreter reports as an error rather than an outcome: a
// body or guard that fails, a token the slots cannot hold, or p undecidable.
func (e *Encoding) Failure(p *Property) *solve.Query {
	cases := make([]*solve.Term, 0, e.Moves+1)
	for i := 0; i <= e.Moves; i++ {
		failed := solve.Not(e.sound(i))
		if p != nil {
			failed = solve.Or(failed, p.Undefined[i])
		}
		cases = append(cases, failed)
	}
	return e.marked(cases, "failure")
}

// marked is the query satisfiable when some state i meets cases[i], with
// MarkVar naming one such state in every model.
func (e *Encoding) marked(cases []*solve.Term, role string) *solve.Query {
	mark := intVar(MarkVar)
	terms := make([]*solve.Term, len(cases))
	for i, c := range cases {
		terms[i] = solve.And(eq(solve.VarTerm(mark), solve.IntTerm(int64(i))), c)
	}
	q := e.query(solve.Or(terms...), role)
	q.Vars = append(slices.Clone(e.Query.Vars), mark)
	return q
}

// Uncertainty is the query satisfiable exactly when some run of k moves is
// cut short by the bound: a token may still act after move k, or a body loop
// ran past its unrolling. Unsatisfiable, every run ends within the bounds and
// a property no run violates is proved rather than bounded.
func (e *Encoding) Uncertainty() *solve.Query {
	k := e.Moves
	live := solve.Or(e.Choosable[k]...)
	loops := make([]*solve.Term, 0, len(e.States[k].Loop))
	for _, flag := range e.States[k].Loop {
		loops = append(loops, solve.VarTerm(flag))
	}
	return e.query(solve.And(e.sound(k), solve.Or(live, solve.Or(loops...))), "cut by the bound")
}

// query is the relation with one more assertion, sharing the relation's own.
func (e *Encoding) query(term *solve.Term, role string) *solve.Query {
	q := *e.Query
	q.Assertions = make([]solve.Assertion, 0, len(e.Query.Assertions)+1)
	q.Assertions = append(q.Assertions, e.Query.Assertions...)
	q.Assertions = append(q.Assertions, solve.Assertion{
		Term: term,
		From: solve.Provenance{Kind: "action", Element: e.Query.Element, Condition: role, Role: solve.RoleRequired},
	})
	return &q
}
