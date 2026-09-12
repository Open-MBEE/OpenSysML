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

// Condition is the property that sym's requirement or constraint (inherited conditions included)
// holds at every state; one reading a feature the action does not carry is refused.
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

// exact holds when state i is one the interpreter reaches as encoded: no move
// so far failed, overflowed the token slots or ran a body loop past its unrolling.
func (e *Encoding) exact(i int) *solve.Term {
	return solve.And(solve.Not(solve.VarTerm(e.States[i].Failed)), solve.Not(e.cut(i)))
}

// cut holds when a bound has cut state i short of the interpreter's run: a
// fork found no free slot, or a body loop ran past its unrolling.
func (e *Encoding) cut(i int) *solve.Term {
	s := e.States[i]
	terms := make([]*solve.Term, 0, len(s.Loop)+1)
	if s.Overflow != nil {
		terms = append(terms, solve.VarTerm(s.Overflow))
	}
	for _, flag := range s.Loop {
		terms = append(terms, solve.VarTerm(flag))
	}
	return solve.Or(terms...)
}

// MarkVar is the integer variable a violation or failure query adds: the state
// its model points at, so a witness knows how far to replay.
const MarkVar = "mark"

// Violation is satisfiable exactly when some run reaches, without an error, a state violating p;
// its model decodes as a witness through MarkVar.
func (e *Encoding) Violation(p *Property) *solve.Query {
	cases := make([]*solve.Term, 0, e.Moves+1)
	for i := 0; i <= e.Moves; i++ {
		cases = append(cases, solve.And(e.exact(i), p.Violated[i]))
	}
	return e.marked(cases, "violation of "+p.Name)
}

// Failure is satisfiable exactly when some run within the bounds meets what the interpreter
// reports as an error: a body or guard that fails, or p undecidable.
func (e *Encoding) Failure(p *Property) *solve.Query {
	cases := make([]*solve.Term, 0, e.Moves+1)
	for i := 0; i <= e.Moves; i++ {
		failed := solve.VarTerm(e.States[i].Failed)
		if p != nil {
			failed = solve.Or(failed, p.Undefined[i])
		}
		cases = append(cases, solve.And(solve.Not(e.cut(i)), failed))
	}
	return e.marked(cases, "failure")
}

// Completion is satisfiable exactly when some run completes within the bounds, MarkVar naming
// the state; its models, told apart by Outputs, are the exploration's outcomes.
func (e *Encoding) Completion() *solve.Query {
	cases := make([]*solve.Term, 0, e.Moves+1)
	for i := 0; i <= e.Moves; i++ {
		cases = append(cases, solve.And(e.exact(i), e.Completed[i]))
	}
	return e.marked(cases, "completion")
}

// Output is one attribute of the action as a completed run leaves it: the copy
// of its value the final state holds, and whether it holds one.
type Output struct {
	Name string
	Var  *solve.Var
	// Has is nil for an attribute that holds a value in every state.
	Has *solve.Var
}

// Outputs are the action's attributes as the final state leaves them, in
// declaration order: what the interpreter reports as a run's results.
func (e *Encoding) Outputs() []Output {
	final := e.States[e.Moves]
	outputs := make([]Output, 0, len(e.Flow.Graph.Attributes))
	for _, attr := range e.Flow.Graph.Attributes {
		scope := attr.Scope
		if scope == nil {
			scope = e.Flow.Graph.Scope
		}
		base, ok := e.features[e.translatedName(attr.Name, scope)]
		if !ok {
			continue
		}
		out := Output{Name: attr.Name, Var: final.value(base)}
		if e.flagged[base.Name] {
			out.Has = final.has(base)
		}
		outputs = append(outputs, out)
	}
	return outputs
}

// marked is the query satisfiable when some state i meets cases[i], with
// MarkVar naming the first such state of the run in every model.
func (e *Encoding) marked(cases []*solve.Term, role string) *solve.Query {
	mark := intVar(MarkVar)
	terms := make([]*solve.Term, len(cases))
	for i, c := range cases {
		first := make([]*solve.Term, 0, i+2)
		first = append(first, eq(solve.VarTerm(mark), solve.IntTerm(int64(i))), c)
		for _, earlier := range cases[:i] {
			first = append(first, solve.Not(earlier))
		}
		terms[i] = solve.And(first...)
	}
	q := e.query(solve.Or(terms...), role)
	q.Vars = append(slices.Clone(e.Query.Vars), mark)
	return q
}

// Uncertainty is satisfiable exactly when some non-failing run is cut short by a bound (a token
// live after move k, a loop past its unrolling, no free slot); unsat makes an unsat property proved.
func (e *Encoding) Uncertainty() *solve.Query {
	k := e.Moves
	live := solve.Or(e.Choosable[k]...)
	return e.query(solve.And(solve.Not(solve.VarTerm(e.States[k].Failed)), solve.Or(live, e.cut(k))), "cut by the bound")
}

// Cut is which bounds a model of Uncertainty shows reached.
type Cut struct {
	// Moves: a token may still act after move k.
	Moves bool
	// Unroll: a body loop ran past its unrolling.
	Unroll bool
	// Slots: a fork found no free token slot.
	Slots bool
}

// Cuts reads a model of Uncertainty back as the bounds it reached.
func (e *Encoding) Cuts(result *solve.Result) (Cut, error) {
	if result == nil || result.Status != solve.StatusSat || len(result.Model) == 0 {
		return Cut{}, ErrNoWitness
	}
	m, err := readModel(result.Model)
	if err != nil {
		return Cut{}, err
	}
	var c Cut
	s := e.States[e.Moves]
	for _, slot := range s.Slots {
		able, err := m.boolean(slot.Able.Name)
		if err != nil {
			return Cut{}, err
		}
		c.Moves = c.Moves || able
	}
	for _, flag := range s.Loop {
		ran, err := m.boolean(flag.Name)
		if err != nil {
			return Cut{}, err
		}
		c.Unroll = c.Unroll || ran
	}
	if s.Overflow != nil {
		if c.Slots, err = m.boolean(s.Overflow.Name); err != nil {
			return Cut{}, err
		}
	}
	return c, nil
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
