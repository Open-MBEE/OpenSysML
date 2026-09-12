package smt

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Encoding is the transition relation of one action's flow over k moves: the
// states after each move, the choice each move makes, and the assertions that
// tie state i to state i-1 as one step of the interpreter would.
type Encoding struct {
	Flow   *Flow
	Sorts  Sorts
	Moves  int
	Unroll int
	// States[i] is the state after move i; States[0] is the initial state.
	States []*State
	// Choices[i-1] is the choice made in move i.
	Choices []*Move
	// Features are the variables the bodies and guards read or write, one per
	// feature, ordered by name; each state holds a copy of every one.
	Features []*solve.Var
	// Choosable[i][t] holds when the token in slot t may act from state i.
	Choosable [][]*solve.Term
	// Completed[i] holds when no token is left in state i.
	Completed []*solve.Term
	// Sorts, variables and assertions of the relation, for a query to build on.
	Query *solve.Query

	translator *solve.Translator
	features   map[string]*solve.Var
	// canonical maps a feature's symbol to its variable, so a chain naming a
	// node's pin (`p.v`) reads the variable the pin's own name declares.
	canonical map[*symbols.Symbol]*solve.Var
	aliases   map[string]*solve.Var
	flagged   map[string]bool
	domains   map[string]*solve.Term
	exprs     map[ast.Node]*solve.Expression
	// pins are the features each node's performance starts afresh; pending
	// holds the delivery an object flow left at a pin, keyed by the pin.
	pins    map[ast.Node][]pin
	pending map[string]*solve.Var
	fresh   int
}

// pin is one feature a node's performance holds, and its declared default.
type pin struct {
	feature lower.Feature
	v       *solve.Var
}

// Encode builds the transition relation of action's flow graph for k moves,
// unrolling each body loop unroll times. The action's features resolve as the
// interpreter resolves them, through ctx.
func Encode(ctx *runtime.Context, action *symbols.Symbol, graph *lower.ActionGraph, k, unroll int) (*Encoding, error) {
	f, err := Analyze(graph, k)
	if err != nil {
		return nil, err
	}
	if unroll < 0 {
		return nil, fmt.Errorf("smt: an unroll bound of %d iterations", unroll)
	}
	name := "action"
	if action != nil {
		name = action.Name
	}
	translator, err := solve.NewTranslator(ctx, solve.Subject{Kind: "action", Name: name, Symbol: action})
	if err != nil {
		return nil, err
	}
	e := &Encoding{
		Flow:       f,
		Sorts:      newSorts(name, f),
		Moves:      k,
		Unroll:     unroll,
		translator: translator,
		features:   make(map[string]*solve.Var),
		canonical:  make(map[*symbols.Symbol]*solve.Var),
		aliases:    make(map[string]*solve.Var),
		flagged:    make(map[string]bool),
		domains:    make(map[string]*solve.Term),
		exprs:      make(map[ast.Node]*solve.Expression),
		pins:       make(map[ast.Node][]pin),
		pending:    make(map[string]*solve.Var),
		Query:      &solve.Query{Kind: "action", Element: name},
	}
	if err := e.collectFeatures(); err != nil {
		return nil, err
	}
	e.Query.Sorts = append(e.Query.Sorts, e.Sorts.Node, e.Sorts.Edge, e.Sorts.Choice)
	e.Query.Sorts = append(e.Query.Sorts, translator.Sorts()...)
	for i := 0; i <= k; i++ {
		e.States = append(e.States, newState(e.Sorts, f, i))
		e.declare(e.States[i].vars(e.Features)...)
		e.declareFlags(e.States[i])
	}
	for i := 1; i <= k; i++ {
		m := newMove(e.Sorts, f, i)
		e.Choices = append(e.Choices, m)
		e.declare(m.vars()...)
	}
	e.Choosable = make([][]*solve.Term, k+1)
	e.Completed = make([]*solve.Term, k+1)
	for i := 0; i <= k; i++ {
		s := e.States[i]
		e.Choosable[i] = make([]*solve.Term, len(s.Slots))
		for t, term := range e.choosable(s) {
			e.assert(eq(solve.VarTerm(s.Slots[t].Able), term), fmt.Sprintf("state %d: slot %d may act", i, t))
			e.Choosable[i][t] = solve.VarTerm(s.Slots[t].Able)
		}
		e.Completed[i] = e.completed(s)
	}
	if err := e.initial(); err != nil {
		return nil, err
	}
	for i := 1; i <= k; i++ {
		if err := e.move(i); err != nil {
			return nil, err
		}
	}
	e.Query.Nonlinear = nonlinear(e.Query.Assertions)
	e.Query.IntegerDivision = integerDivision(e.Query.Assertions)
	return e, nil
}

// declare adds variables to the query.
func (e *Encoding) declare(vars ...*solve.Var) {
	e.Query.Vars = append(e.Query.Vars, vars...)
}

// declareFlags declares the state's copies of the flags saying which features
// hold a value.
func (e *Encoding) declareFlags(s *State) {
	for _, base := range e.Features {
		if e.flagged[base.Name] {
			e.declare(s.has(base))
		}
	}
}

// assert adds an assertion about role of the relation.
func (e *Encoding) assert(term *solve.Term, role string) {
	if term.Op == solve.OpBool && term.Bool {
		return
	}
	e.Query.Assertions = append(e.Query.Assertions, solve.Assertion{
		Term: term,
		From: solve.Provenance{Kind: "action", Element: e.Query.Element, Condition: role, Role: solve.RoleTransition},
	})
}

// collectFeatures translates every expression the flow evaluates and gathers
// the features they read and the bodies write, refusing one the stage does not
// encode: a feature of the performing object, a free input.
func (e *Encoding) collectFeatures() error {
	graph := e.Flow.Graph
	declared := make(map[string]bool)
	for _, attr := range graph.Attributes {
		scope := attr.Scope
		if scope == nil {
			scope = graph.Scope
		}
		v, err := e.variable(attr.Name, scope, "attribute "+attr.Name, nodeLabel(graph.Initial))
		if err != nil {
			return err
		}
		declared[v.Name] = true
		if attr.Value == nil {
			e.flagged[v.Name] = true
			continue
		}
		if _, err := e.expression(attr.Value, scope, "default of "+attr.Name, nodeLabel(graph.Initial)); err != nil {
			return err
		}
	}
	for _, node := range e.Flow.Nodes {
		if err := e.collectPins(node, declared); err != nil {
			return err
		}
	}
	for _, node := range e.Flow.Nodes {
		label := e.Flow.label(node)
		if err := e.collectFlows(node); err != nil {
			return err
		}
		if err := e.collectBody(graph.Bodies[node], label, declared); err != nil {
			return err
		}
		if action, ok := node.(*ast.ActionExecutionNode); ok && action.Expression != nil {
			if _, err := e.expression(action.Expression, graph.Scope, "expression of "+label, label); err != nil {
				return err
			}
		}
		for _, i := range e.Flow.Outgoing[node] {
			edge := e.Flow.Edges[i]
			if edge.Guard == nil {
				continue
			}
			if _, err := e.boolean(edge.Guard, graph.Scope, "guard of "+edgeLabel(e.Flow, i), label); err != nil {
				return err
			}
		}
	}
	for _, v := range e.translator.Vars() {
		if _, ok := e.features[v.Name]; ok {
			continue
		}
		if _, ok := e.aliases[v.Name]; ok {
			continue
		}
		return &UnsupportedError{Node: nodeLabel(graph.Initial), Construct: "feature " + v.Name,
			Reason: "a feature the action does not declare is read; objects and their features are encoded by a later stage"}
	}
	for _, d := range e.translator.Domains() {
		var read *solve.Var
		term := solve.Substitute(d.Term, func(v *solve.Var) *solve.Term {
			read = e.resolve(v)
			return solve.VarTerm(read)
		})
		if read != nil {
			e.domains[read.Name] = term
		}
	}
	e.Features = make([]*solve.Var, 0, len(e.features))
	for _, v := range e.features {
		e.Features = append(e.Features, v)
	}
	sort.Slice(e.Features, func(i, j int) bool { return e.Features[i].Name < e.Features[j].Name })
	return nil
}

// collectPins registers the features a node's performance holds, each starting
// without a value unless a delivery or its own default gives it one.
func (e *Encoding) collectPins(node ast.Node, declared map[string]bool) error {
	label := e.Flow.label(node)
	for _, feature := range e.Flow.Graph.Features[node] {
		scope := feature.Scope
		if scope == nil {
			scope = e.Flow.Graph.Scopes[node]
		}
		v, err := e.variable(feature.Name, scope, "pin "+feature.Name, label)
		if err != nil {
			return err
		}
		if declared[v.Name] {
			return &UnsupportedError{Node: label, Construct: "pin " + feature.Name,
				Reason: "a pin sharing its variable with a feature the action declares is not encoded"}
		}
		declared[v.Name] = true
		e.flagged[v.Name] = true
		e.pins[node] = append(e.pins[node], pin{feature: feature, v: v})
		if feature.Value != nil {
			if _, err := e.expression(feature.Value, scope, "default of pin "+feature.Name, label); err != nil {
				return err
			}
		}
	}
	return nil
}

// collectFlows registers the pins the object flows out of node read and write,
// and a pending slot per pin a flow delivers to a node performing in a frame of its own.
func (e *Encoding) collectFlows(node ast.Node) error {
	graph := e.Flow.Graph
	label := e.Flow.label(node)
	for _, flow := range graph.DataFlows[node] {
		within := flowLabel(flow)
		if _, err := e.flowEnd(node, flow.SourcePin, within, label); err != nil {
			return err
		}
		target, err := e.flowEnd(flow.Target, flow.TargetPin, within, label)
		if err != nil {
			return err
		}
		if _, performs := flow.Target.(*ast.Usage); !performs {
			continue
		}
		if _, ok := e.pending[target.Name]; ok {
			continue
		}
		p := &solve.Var{Name: "pending(" + target.Name + ")", Sort: target.Sort, Symbol: target.Symbol,
			Dimension: target.Dimension, Unit: target.Unit}
		e.pending[target.Name] = p
		e.features[p.Name] = p
		e.flagged[p.Name] = true
	}
	return nil
}

// flowEnd is the variable an object flow reads or writes at node: the pin a
// node performing in a frame of its own declares, else the action's feature.
func (e *Encoding) flowEnd(node ast.Node, name, within, label string) (*solve.Var, error) {
	if _, performs := node.(*ast.Usage); !performs {
		return e.variable(name, e.Flow.Graph.Scope, within, label)
	}
	for _, p := range e.pins[node] {
		if p.feature.Name == name {
			return p.v, nil
		}
	}
	return nil, &UnsupportedError{Node: label, Construct: within,
		Reason: fmt.Sprintf("%s declares no pin %s", e.Flow.label(node), name)}
}

// flowLabel names an object flow as a diagnostic does.
func flowLabel(flow lower.ObjectFlow) string {
	if flow.Name != "" {
		return "flow " + flow.Name
	}
	return fmt.Sprintf("flow from %s to %s", flow.SourcePin, flow.TargetPin)
}

// collectBody translates a body's expressions and registers what it declares.
func (e *Encoding) collectBody(body []lower.Statement, label string, declared map[string]bool) error {
	for _, stmt := range body {
		switch s := stmt.(type) {
		case lower.Assign:
			if _, err := e.expression(s.Value, s.Scope, "assignment to "+s.Target, label); err != nil {
				return err
			}
			if _, err := e.variable(s.Target, s.Scope, "assignment to "+s.Target, label); err != nil {
				return err
			}
		case lower.Declare:
			v, err := e.variable(s.Name, s.Scope, "declaration of "+s.Name, label)
			if err != nil {
				return err
			}
			if declared[v.Name] {
				return &UnsupportedError{Node: label, Construct: "declaration of " + s.Name,
					Reason: "a body declaring a name the action declares is not encoded"}
			}
			if s.Value == nil {
				e.flagged[v.Name] = true
			} else if _, err := e.expression(s.Value, s.Scope, "declaration of "+s.Name, label); err != nil {
				return err
			}
		case lower.Block:
			if err := e.collectBody(s.Statements, label, declared); err != nil {
				return err
			}
		case lower.If:
			if _, err := e.boolean(s.Condition, s.Scope, "condition of 'if'", label); err != nil {
				return err
			}
			if err := e.collectBody(s.Then.Statements, label, declared); err != nil {
				return err
			}
			if s.Else != nil {
				if err := e.collectBody(s.Else.Statements, label, declared); err != nil {
					return err
				}
			}
		case lower.Loop:
			if s.Condition != nil {
				if _, err := e.boolean(s.Condition, s.Body.Scope, "condition of '"+s.Kind.String()+"'", label); err != nil {
					return err
				}
			}
			if s.Until != nil {
				if _, err := e.boolean(s.Until, s.Body.Scope, "condition of 'until'", label); err != nil {
					return err
				}
			}
			if err := e.collectBody(s.Body.Statements, label, declared); err != nil {
				return err
			}
		}
	}
	return nil
}

// variable is the feature name resolves to in scope, registered as one the
// states carry.
func (e *Encoding) variable(name string, scope *symbols.Scope, within, label string) (*solve.Var, error) {
	v, err := e.translator.Variable(name, scope, within)
	if err != nil {
		return nil, refusal(label, within, err)
	}
	if held, ok := e.features[v.Name]; ok {
		return held, nil
	}
	e.features[v.Name] = v
	if v.Symbol != nil {
		e.canonical[v.Symbol] = v
	}
	return v, nil
}

// resolve is the variable standing for the feature v reads: the one its symbol
// declares when a chain named it otherwise, else v itself.
func (e *Encoding) resolve(v *solve.Var) *solve.Var {
	if held, ok := e.features[v.Name]; ok {
		return held
	}
	if v.Symbol != nil {
		if held, ok := e.canonical[v.Symbol]; ok {
			e.aliases[v.Name] = held
			return held
		}
	}
	return v
}

// expression translates node once, in scope, registering the features it reads.
func (e *Encoding) expression(node ast.Node, scope *symbols.Scope, within, label string) (*solve.Expression, error) {
	if expr, ok := e.exprs[node]; ok {
		return expr, nil
	}
	expr, err := e.translator.Expression(node, scope, within)
	if err != nil {
		return nil, refusal(label, within, err)
	}
	e.register(expr)
	e.exprs[node] = expr
	return expr, nil
}

// boolean translates a condition once, in scope, refusing one that is not Boolean.
func (e *Encoding) boolean(node ast.Node, scope *symbols.Scope, within, label string) (*solve.Expression, error) {
	if expr, ok := e.exprs[node]; ok {
		return expr, nil
	}
	expr, err := e.translator.Boolean(node, scope, within)
	if err != nil {
		return nil, refusal(label, within, err)
	}
	e.register(expr)
	e.exprs[node] = expr
	return expr, nil
}

// register records the features an expression reads, rewriting each read
// to the variable the feature's own name declares.
func (e *Encoding) register(expr *solve.Expression) {
	visit := func(v *solve.Var) *solve.Term {
		held := e.resolve(v)
		if _, ok := e.features[held.Name]; !ok {
			e.features[held.Name] = held
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
}

// refusal wraps a translation refusal as the stage's typed refusal naming the node.
func refusal(label, within string, err error) error {
	return &UnsupportedError{Node: label, Construct: within, Reason: err.Error()}
}

// choosable is, per slot, whether its token may act from state s: a token at
// a node, unless a join it arrived at waits for an arrival or would let an
// earlier arrival act for it.
func (e *Encoding) choosable(s *State) []*solve.Term {
	f := e.Flow
	terms := make([]*solve.Term, len(s.Slots))
	for t, slot := range s.Slots {
		at := solve.VarTerm(slot.At)
		var cases []*solve.Term
		for n, node := range f.Nodes {
			here := eq(at, nodeValue(e.Sorts, f, n))
			if !e.synchronizes(node) {
				cases = append(cases, here)
				continue
			}
			var complete []*solve.Term
			for _, i := range f.Incoming[node] {
				var arrived []*solve.Term
				for _, other := range s.Slots {
					arrived = append(arrived, solve.And(
						eq(solve.VarTerm(other.At), nodeValue(e.Sorts, f, n)),
						eq(solve.VarTerm(other.Via), edgeValue(e.Sorts, f, i))))
				}
				complete = append(complete, solve.Or(arrived...))
			}
			var earliest []*solve.Term
			for u, other := range s.Slots {
				if u == t {
					continue
				}
				earliest = append(earliest, implies(
					solve.And(eq(solve.VarTerm(other.At), nodeValue(e.Sorts, f, n)), solve.Not(e.noEdge(other.Via))),
					solve.Binary(solve.OpGt, solve.Bool, solve.VarTerm(other.ID), solve.VarTerm(slot.ID))))
			}
			cases = append(cases, solve.And(here, solve.Or(
				e.noEdge(slot.Via),
				solve.And(solve.And(complete...), solve.And(earliest...)))))
		}
		terms[t] = solve.Or(cases...)
	}
	return terms
}

// synchronizes reports whether node waits for an arrival over each of its
// successions before a token acts: a join does; a merge never does.
func (e *Encoding) synchronizes(node ast.Node) bool {
	_, join := node.(*ast.JoinNode)
	return join
}

// completed holds when no slot of s holds a token.
func (e *Encoding) completed(s *State) *solve.Term {
	terms := make([]*solve.Term, len(s.Slots))
	for t, slot := range s.Slots {
		terms[t] = e.absent(slot.At)
	}
	return solve.And(terms...)
}

func (e *Encoding) absent(at *solve.Var) *solve.Term {
	return eq(solve.VarTerm(at), solve.ValueTerm(e.Sorts.Node, Absent))
}

func (e *Encoding) noEdge(via *solve.Var) *solve.Term {
	return eq(solve.VarTerm(via), solve.ValueTerm(e.Sorts.Edge, NoEdge))
}

// initial constrains state 0: one token at the initial node, the attributes at
// their defaults, evaluated in declaration order as the interpreter evaluates them.
func (e *Encoding) initial() error {
	s := e.States[0]
	f := e.Flow
	for t, slot := range s.Slots {
		if t == 0 {
			e.assert(eq(solve.VarTerm(slot.At), nodeValue(e.Sorts, f, f.Index[f.Graph.Initial])), "initial token")
			e.assert(eq(solve.VarTerm(slot.ID), solve.IntTerm(1)), "initial token")
		} else {
			e.assert(e.absent(slot.At), "initial token")
			e.assert(eq(solve.VarTerm(slot.ID), solve.IntTerm(0)), "initial token")
		}
		e.assert(e.noEdge(slot.Via), "initial token")
	}
	e.assert(eq(solve.VarTerm(s.NextID), solve.IntTerm(2)), "initial token")
	for _, flag := range s.Loop {
		e.assert(solve.Not(solve.VarTerm(flag)), "initial state")
	}
	if s.Overflow != nil {
		e.assert(solve.Not(solve.VarTerm(s.Overflow)), "initial state")
	}
	env := e.environment(s)
	for _, base := range e.Features {
		if e.flagged[base.Name] {
			env.has[base.Name] = solve.BoolTerm(false)
		}
	}
	var failed []*solve.Term
	for _, attr := range f.Graph.Attributes {
		scope := attr.Scope
		if scope == nil {
			scope = f.Graph.Scope
		}
		v := e.features[e.translatedName(attr.Name, scope)]
		if attr.Value == nil || v == nil {
			continue
		}
		expr := e.exprs[attr.Value]
		value, defined := env.evaluate(expr)
		failed = append(failed, solve.Not(defined))
		env.write(v.Name, value)
		if domain := e.domain(v.Name, value); domain != nil {
			failed = append(failed, solve.Not(domain))
		}
	}
	for _, base := range e.Features {
		e.assert(eq(solve.VarTerm(s.value(base)), env.values[base.Name]), "initial value of "+base.Name)
		if e.flagged[base.Name] {
			e.assert(eq(solve.VarTerm(s.has(base)), env.has[base.Name]), "initial value of "+base.Name)
		}
	}
	e.assert(eq(solve.VarTerm(s.Failed), solve.Or(failed...)), "initial state")
	return nil
}

// translatedName is the name the translator gives the feature name resolves to in scope.
func (e *Encoding) translatedName(name string, scope *symbols.Scope) string {
	v, err := e.translator.Variable(name, scope, name)
	if err != nil {
		return ""
	}
	return v.Name
}

// domain is the declared domain of feature name over value, nil when none.
func (e *Encoding) domain(name string, value *solve.Term) *solve.Term {
	d, ok := e.domains[name]
	if !ok {
		return nil
	}
	return solve.Substitute(d, func(*solve.Var) *solve.Term { return value })
}

// environment is the symbolic values of the features as state s holds them.
func (e *Encoding) environment(s *State) *env {
	env := &env{enc: e, values: make(map[string]*solve.Term), has: make(map[string]*solve.Term)}
	for _, base := range e.Features {
		env.values[base.Name] = solve.VarTerm(s.value(base))
		if e.flagged[base.Name] {
			env.has[base.Name] = solve.VarTerm(s.has(base))
		}
	}
	return env
}

// nodeEffect is one performance's effect on the features, when it errors and when a body loop
// passes the unroll bound; guards is the environment its guards read (after the body).
type nodeEffect struct {
	env      *env
	guards   *env
	failed   []*solve.Term
	overflow []*solve.Term
	loops    map[int]*solve.Term
}

// move ties state i to state i-1 by the choice of move i.
func (e *Encoding) move(i int) error {
	prev, next := e.States[i-1], e.States[i]
	m := e.Choices[i-1]
	f := e.Flow
	choice := solve.VarTerm(m.Choice)
	stutter := eq(choice, solve.ValueTerm(e.Sorts.Choice, Stutter))

	// The choice is a choosable token, or a stutter when none is.
	anyChoosable := solve.Or(e.Choosable[i-1]...)
	e.assert(eq(stutter, solve.Not(anyChoosable)), fmt.Sprintf("move %d stutters only when no token may act", i))
	for t := range prev.Slots {
		e.assert(implies(eq(choice, solve.ValueTerm(e.Sorts.Choice, slotLabel(t))), e.Choosable[i-1][t]),
			fmt.Sprintf("move %d acts on a token that may act", i))
	}

	// acts[n] holds when the acting token is at node n.
	acts := make([]*solve.Term, len(f.Nodes))
	for n := range f.Nodes {
		var cases []*solve.Term
		for t, slot := range prev.Slots {
			cases = append(cases, solve.And(
				eq(choice, solve.ValueTerm(e.Sorts.Choice, slotLabel(t))),
				eq(solve.VarTerm(slot.At), nodeValue(e.Sorts, f, n))))
		}
		acts[n] = solve.Or(cases...)
	}

	// The effect of each node's performance on the features, and the guards it
	// then reads, over the values before the move.
	effects := make([]*nodeEffect, len(f.Nodes))
	guards := make([][]*solve.Term, len(f.Nodes))
	guardsDefined := make([][]*solve.Term, len(f.Nodes))
	for n, node := range f.Nodes {
		effect, err := e.perform(i, n, node, prev)
		if err != nil {
			return err
		}
		effects[n] = effect
		guards[n] = make([]*solve.Term, len(f.Outgoing[node]))
		guardsDefined[n] = make([]*solve.Term, len(f.Outgoing[node]))
		for p, edge := range f.Outgoing[node] {
			guard := f.Edges[edge].Guard
			if guard == nil {
				guards[n][p], guardsDefined[n][p] = solve.BoolTerm(true), solve.BoolTerm(true)
				continue
			}
			guards[n][p], guardsDefined[n][p] = effect.guards.evaluate(e.exprs[guard])
			if held := m.Held[edge]; held != nil {
				e.assert(eq(solve.VarTerm(held), solve.And(acts[n], guardsDefined[n][p], guards[n][p])),
					fmt.Sprintf("move %d: the guard of %s is read and holds", i, edgeLabel(f, edge)))
			}
		}
	}

	// Feature values after the move: what the acting node's body left, else unchanged.
	for _, base := range e.Features {
		value := solve.VarTerm(prev.value(base))
		var has *solve.Term
		if e.flagged[base.Name] {
			has = solve.VarTerm(prev.has(base))
		}
		for n := len(f.Nodes) - 1; n >= 0; n-- {
			after := effects[n].env
			if after.values[base.Name] != solve.VarTerm(prev.value(base)) && !sameVar(after.values[base.Name], prev.value(base)) {
				value = solve.Ite(acts[n], after.values[base.Name], value)
			}
			if has != nil && !sameVar(after.has[base.Name], prev.has(base)) {
				has = solve.Ite(acts[n], after.has[base.Name], has)
			}
		}
		e.assert(eq(solve.VarTerm(next.value(base)), value), fmt.Sprintf("move %d: %s", i, base.Name))
		if has != nil {
			e.assert(eq(solve.VarTerm(next.has(base)), has), fmt.Sprintf("move %d: %s", i, base.Name))
		}
	}

	// Errors, overflows and loops past the bound, once met, stay.
	failed := []*solve.Term{solve.VarTerm(prev.Failed)}
	var overflow []*solve.Term
	if prev.Overflow != nil {
		overflow = append(overflow, solve.VarTerm(prev.Overflow))
	}
	for n := range f.Nodes {
		if len(effects[n].failed) > 0 {
			failed = append(failed, solve.And(acts[n], solve.Or(effects[n].failed...)))
		}
		if len(effects[n].overflow) > 0 {
			overflow = append(overflow, solve.And(acts[n], solve.Or(effects[n].overflow...)))
		}
	}
	for l := range f.Loops {
		loop := []*solve.Term{solve.VarTerm(prev.Loop[l])}
		for n := range f.Nodes {
			if term, ok := effects[n].loops[l]; ok {
				loop = append(loop, solve.And(acts[n], term))
			}
		}
		e.assert(eq(solve.VarTerm(next.Loop[l]), solve.Or(loop...)), fmt.Sprintf("move %d: loop %d past the bound", i, l))
	}

	// The tokens after the move.
	travel := solve.VarTerm(m.Travel)
	var frame []*solve.Term
	for t := range prev.Slots {
		frame = append(frame, e.unchanged(prev.Slots[t], next.Slots[t]))
	}
	e.assert(implies(stutter, solve.And(
		solve.And(frame...),
		eq(solve.VarTerm(next.NextID), solve.VarTerm(prev.NextID)),
		eq(travel, solve.ValueTerm(e.Sorts.Edge, NoEdge)))), fmt.Sprintf("move %d stutters", i))

	for t := range prev.Slots {
		for n, node := range f.Nodes {
			pick := solve.And(eq(choice, solve.ValueTerm(e.Sorts.Choice, slotLabel(t))),
				eq(solve.VarTerm(prev.Slots[t].At), nodeValue(e.Sorts, f, n)))
			step, fails, full := e.tokens(i, t, n, node, prev, next, m, guards[n], guardsDefined[n])
			e.assert(implies(pick, step), fmt.Sprintf("move %d: slot %d acts at %s", i, t, f.Labels[n]))
			if fails != nil {
				failed = append(failed, solve.And(pick, fails))
			}
			if full != nil {
				overflow = append(overflow, solve.And(pick, full))
			}
		}
	}
	e.assert(eq(solve.VarTerm(next.Failed), solve.Or(failed...)), fmt.Sprintf("move %d fails", i))
	if next.Overflow != nil {
		e.assert(eq(solve.VarTerm(next.Overflow), solve.Or(overflow...)), fmt.Sprintf("move %d overflows", i))
	} else if len(overflow) > 0 {
		return &FlowError{Node: nodeLabel(f.Graph.Initial), Reason: "a move may overflow a state declaring no overflow"}
	}
	return nil
}

// sameVar reports whether term reads exactly the variable v.
func sameVar(term *solve.Term, v *solve.Var) bool {
	return term != nil && term.Op == solve.OpVar && term.Var == v
}

// unchanged says slot after holds what slot before did.
func (e *Encoding) unchanged(before, after Slot) *solve.Term {
	return solve.And(
		eq(solve.VarTerm(after.At), solve.VarTerm(before.At)),
		eq(solve.VarTerm(after.Via), solve.VarTerm(before.Via)),
		eq(solve.VarTerm(after.ID), solve.VarTerm(before.ID)))
}

// tokens moves the token in slot t at node n in move i (synchronization, then succession); the
// other results are when the succession errors and when a fork finds no free slot, nil if never.
func (e *Encoding) tokens(i, t, n int, node ast.Node, prev, next *State, m *Move, guards, defined []*solve.Term) (step, fails, full *solve.Term) {
	f := e.Flow
	slot := prev.Slots[t]
	out := f.Outgoing[node]
	travel := solve.VarTerm(m.Travel)
	noEdge := solve.ValueTerm(e.Sorts.Edge, NoEdge)
	absent := solve.ValueTerm(e.Sorts.Node, Absent)

	// Synchronization at a join collapses the earliest arrival over each
	// succession into a fresh token; the slots consumed are freed.
	synced := solve.BoolTerm(false)
	if e.synchronizes(node) {
		synced = solve.Not(e.noEdge(slot.Via))
	}
	consumed := make([]*solve.Term, len(prev.Slots))
	for u, other := range prev.Slots {
		if u == t {
			consumed[u] = solve.BoolTerm(false)
			continue
		}
		var earliest []*solve.Term
		for w, third := range prev.Slots {
			if w == u {
				continue
			}
			earliest = append(earliest, implies(
				solve.And(eq(solve.VarTerm(third.At), nodeValue(e.Sorts, f, n)), eq(solve.VarTerm(third.Via), solve.VarTerm(other.Via))),
				solve.Binary(solve.OpGt, solve.Bool, solve.VarTerm(third.ID), solve.VarTerm(other.ID))))
		}
		consumed[u] = solve.And(synced,
			eq(solve.VarTerm(other.At), nodeValue(e.Sorts, f, n)),
			solve.Not(e.noEdge(other.Via)),
			solve.And(earliest...))
	}
	actorID := solve.Ite(synced, solve.VarTerm(prev.NextID), solve.VarTerm(slot.ID))
	base := solve.Ite(synced, solve.Binary(solve.OpAdd, solve.Int, solve.VarTerm(prev.NextID), solve.IntTerm(1)), solve.VarTerm(prev.NextID))

	// free[u] says slot u (other than t) is free once consumption is done.
	free := make([]*solve.Term, len(prev.Slots))
	for u, other := range prev.Slots {
		if u == t {
			free[u] = solve.BoolTerm(false)
			continue
		}
		free[u] = solve.Or(e.absent(other.At), consumed[u])
	}

	// The others' slots: consumed ones are freed, the rest are as they were
	// unless a fork places a token in them.
	placed := make([]*solve.Term, len(prev.Slots))
	for u := range prev.Slots {
		placed[u] = solve.BoolTerm(false)
	}
	var terms []*solve.Term
	retire := solve.And(
		eq(solve.VarTerm(next.Slots[t].At), absent),
		eq(solve.VarTerm(next.Slots[t].Via), noEdge),
		eq(solve.VarTerm(next.Slots[t].ID), actorID),
		eq(travel, noEdge))
	go_ := func(p int) *solve.Term {
		edge := f.Edges[out[p]]
		return solve.And(
			eq(solve.VarTerm(next.Slots[t].At), nodeValue(e.Sorts, f, f.Index[edge.Target])),
			eq(solve.VarTerm(next.Slots[t].Via), edgeValue(e.Sorts, f, out[p])),
			eq(solve.VarTerm(next.Slots[t].ID), actorID),
			eq(travel, edgeValue(e.Sorts, f, out[p])))
	}
	stay := solve.And(
		eq(solve.VarTerm(next.Slots[t].At), solve.VarTerm(slot.At)),
		eq(solve.VarTerm(next.Slots[t].Via), solve.VarTerm(slot.Via)),
		eq(solve.VarTerm(next.Slots[t].ID), actorID),
		eq(travel, noEdge))
	nextID := base

	switch node.(type) {
	case *ast.FinalNode:
		terms = append(terms, retire)
	case *ast.ForkNode:
		// Enabled successions each get a fresh token, in order: the first in the
		// actor's slot, the rest in the free slots in order.
		rank := make([]*solve.Term, len(out))
		count := solve.IntTerm(0)
		for p := range out {
			rank[p] = count
			count = solve.Binary(solve.OpAdd, solve.Int, count, solve.Ite(guards[p], solve.IntTerm(1), solve.IntTerm(0)))
		}
		none := eq(count, solve.IntTerm(0))
		var actor []*solve.Term
		for p := range out {
			edge := f.Edges[out[p]]
			isFirst := solve.And(guards[p], eq(rank[p], solve.IntTerm(0)))
			actor = append(actor, implies(isFirst, solve.And(
				eq(solve.VarTerm(next.Slots[t].At), nodeValue(e.Sorts, f, f.Index[edge.Target])),
				eq(solve.VarTerm(next.Slots[t].Via), edgeValue(e.Sorts, f, out[p])),
				eq(solve.VarTerm(next.Slots[t].ID), base),
				eq(travel, edgeValue(e.Sorts, f, out[p])))))
		}
		terms = append(terms, implies(none, retire), implies(solve.Not(none), solve.And(actor...)))
		freeRank := make([]*solve.Term, len(prev.Slots))
		running := solve.IntTerm(0)
		for u := range prev.Slots {
			freeRank[u] = running
			if u != t {
				running = solve.Binary(solve.OpAdd, solve.Int, running, solve.Ite(free[u], solve.IntTerm(1), solve.IntTerm(0)))
			}
		}
		for u := range prev.Slots {
			if u == t {
				continue
			}
			var here []*solve.Term
			for p := range out {
				edge := f.Edges[out[p]]
				takes := solve.And(guards[p], solve.Binary(solve.OpGe, solve.Bool, rank[p], solve.IntTerm(1)),
					eq(freeRank[u], solve.Binary(solve.OpSub, solve.Int, rank[p], solve.IntTerm(1))))
				here = append(here, takes)
				terms = append(terms, implies(solve.And(free[u], takes), solve.And(
					eq(solve.VarTerm(next.Slots[u].At), nodeValue(e.Sorts, f, f.Index[edge.Target])),
					eq(solve.VarTerm(next.Slots[u].Via), edgeValue(e.Sorts, f, out[p])),
					eq(solve.VarTerm(next.Slots[u].ID), solve.Binary(solve.OpAdd, solve.Int, base, rank[p])))))
			}
			placed[u] = solve.And(free[u], solve.Or(here...))
		}
		nextID = solve.Binary(solve.OpAdd, solve.Int, base, count)
		fails = solve.Or(undefinedGuards(guards, defined)...)
		if f.Cyclic {
			full = solve.Binary(solve.OpGt, solve.Bool, count, solve.Binary(solve.OpAdd, solve.Int, running, solve.IntTerm(1)))
		}
	case *ast.DecisionNode:
		// The guarded successions that hold are the branches; none holding
		// takes the unguarded one, and without one the run fails.
		var holds []*solve.Term
		unguarded := -1
		var anyHolds *solve.Term = solve.BoolTerm(false)
		var undefined []*solve.Term
		for p := range out {
			if f.Edges[out[p]].Guard == nil {
				unguarded = p
				continue
			}
			held := solve.And(defined[p], guards[p])
			undefined = append(undefined, solve.And(solve.Not(anyHolds), solve.Not(defined[p])))
			holds = append(holds, held)
			anyHolds = solve.Or(anyHolds, held)
		}
		var branches []*solve.Term
		q := 0
		for p := range out {
			if f.Edges[out[p]].Guard == nil {
				continue
			}
			branches = append(branches, solve.And(eq(travel, edgeValue(e.Sorts, f, out[p])), holds[q]))
			terms = append(terms, implies(solve.And(anyHolds, eq(travel, edgeValue(e.Sorts, f, out[p]))), go_(p)))
			q++
		}
		if len(branches) > 0 {
			terms = append(terms, implies(anyHolds, solve.Or(branches...)))
		}
		if unguarded >= 0 {
			terms = append(terms, implies(solve.Not(anyHolds), go_(unguarded)))
		} else {
			terms = append(terms, implies(solve.Not(anyHolds), stay))
			undefined = append(undefined, solve.Not(anyHolds))
		}
		fails = solve.Or(undefined...)
	default:
		// One enabled succession is taken; none retires the token; several
		// out of a node other than the initial one is an error.
		_, initial := node.(*ast.InitialNode)
		count := solve.IntTerm(0)
		taken := solve.BoolTerm(false)
		for p := range out {
			isFirst := solve.And(guards[p], solve.Not(taken))
			terms = append(terms, implies(isFirst, go_(p)))
			taken = solve.Or(taken, guards[p])
			count = solve.Binary(solve.OpAdd, solve.Int, count, solve.Ite(guards[p], solve.IntTerm(1), solve.IntTerm(0)))
		}
		terms = append(terms, implies(solve.Not(taken), retire))
		failures := undefinedGuards(guards, defined)
		if !initial && len(out) > 1 {
			failures = append(failures, solve.Binary(solve.OpGt, solve.Bool, count, solve.IntTerm(1)))
		}
		if len(failures) > 0 {
			fails = solve.Or(failures...)
		}
	}

	for u := range prev.Slots {
		if u == t {
			continue
		}
		freed := solve.And(
			eq(solve.VarTerm(next.Slots[u].At), absent),
			eq(solve.VarTerm(next.Slots[u].Via), noEdge),
			eq(solve.VarTerm(next.Slots[u].ID), solve.VarTerm(prev.Slots[u].ID)))
		terms = append(terms,
			implies(solve.And(consumed[u], solve.Not(placed[u])), freed),
			implies(solve.And(solve.Not(consumed[u]), solve.Not(placed[u])), e.unchanged(prev.Slots[u], next.Slots[u])))
	}
	terms = append(terms, eq(solve.VarTerm(next.NextID), nextID))
	return solve.And(terms...), fails, full
}

// undefinedGuards are the conditions under which evaluating the guards, in
// order, meets an error before a decision is reached.
func undefinedGuards(guards, defined []*solve.Term) []*solve.Term {
	var terms []*solve.Term
	for p := range guards {
		if defined[p].Op == solve.OpBool && defined[p].Bool {
			continue
		}
		terms = append(terms, solve.Not(defined[p]))
	}
	_ = guards
	return terms
}

// perform is one performance of node n in move i, in the interpreter's order: pins take their
// deliveries or defaults, the body runs over prev, the guards are read, the object flows carry.
func (e *Encoding) perform(i, n int, node ast.Node, prev *State) (*nodeEffect, error) {
	effect := &nodeEffect{env: e.environment(prev), loops: make(map[int]*solve.Term)}
	where := fmt.Sprintf("%d.%s", i, e.Flow.Labels[n])
	e.begin(effect, node, where)
	body := e.Flow.Graph.Bodies[node]
	if err := e.statements(effect, body, solve.BoolTerm(true), where, node); err != nil {
		return nil, err
	}
	if action, ok := node.(*ast.ActionExecutionNode); ok && action.Expression != nil {
		_, defined := effect.env.evaluate(e.exprs[action.Expression])
		effect.fail(solve.BoolTerm(true), defined)
	}
	effect.guards = effect.env.clone()
	if err := e.flows(effect, node, where); err != nil {
		return nil, err
	}
	return effect, nil
}

// begin starts a performance of node: each pin holds the delivery queued for
// it, else the value its own declaration gives it, else none.
func (e *Encoding) begin(x *nodeEffect, node ast.Node, where string) {
	true_ := solve.BoolTerm(true)
	for _, p := range e.pins[node] {
		name := p.v.Name
		value, has := x.env.values[name], solve.BoolTerm(false)
		seeded := true_
		pending, queued := e.pending[name]
		if queued {
			seeded = solve.Not(x.env.has[pending.Name])
		}
		if p.feature.Value != nil {
			default_, defined := x.env.evaluate(e.exprs[p.feature.Value])
			x.fail(seeded, defined)
			if domain := e.domain(name, default_); domain != nil {
				x.fail(seeded, domain)
			}
			value, has = default_, true_
		}
		if queued {
			value = solve.Ite(seeded, value, x.env.values[pending.Name])
			has = solve.Ite(seeded, has, true_)
			x.env.has[pending.Name] = solve.BoolTerm(false)
		}
		e.fresh++
		v := &solve.Var{Name: fmt.Sprintf("%s@%s#%d", name, where, e.fresh), Sort: p.v.Sort,
			Symbol: p.v.Symbol, Dimension: p.v.Dimension, Unit: p.v.Unit}
		e.declare(v)
		e.assert(eq(solve.VarTerm(v), value), "start of "+name)
		x.env.values[name] = solve.VarTerm(v)
		x.env.has[name] = has
	}
}

// flows carries what a completed node produced over its object flows: to a pin queue of a node in
// a frame of its own, else to the action's feature. An empty source pin is the interpreter's error.
func (e *Encoding) flows(x *nodeEffect, node ast.Node, where string) error {
	true_ := solve.BoolTerm(true)
	label := e.Flow.label(node)
	for _, flow := range e.Flow.Graph.DataFlows[node] {
		source, err := e.flowEnd(node, flow.SourcePin, flowLabel(flow), label)
		if err != nil {
			return err
		}
		target, err := e.flowEnd(flow.Target, flow.TargetPin, flowLabel(flow), label)
		if err != nil {
			return err
		}
		if has, flagged := x.env.has[source.Name]; flagged {
			x.fail(true_, has)
		}
		value := x.env.values[source.Name]
		if domain := e.domain(target.Name, value); domain != nil {
			x.fail(true_, domain)
		}
		pending, queued := e.pending[target.Name]
		if !queued {
			e.write(x, true_, target, value, where)
			continue
		}
		x.overflow = append(x.overflow, x.env.has[pending.Name])
		e.write(x, true_, pending, value, where)
	}
	return nil
}

// fail records that the interpreter reports an error where path holds and
// defined does not.
func (x *nodeEffect) fail(path, defined *solve.Term) {
	if defined.Op == solve.OpBool && defined.Bool {
		return
	}
	x.failed = append(x.failed, solve.And(path, solve.Not(defined)))
}

// statements runs a body's statements symbolically under path.
func (e *Encoding) statements(x *nodeEffect, body []lower.Statement, path *solve.Term, where string, node ast.Node) error {
	for _, stmt := range body {
		switch s := stmt.(type) {
		case lower.Assign:
			target := e.features[e.translatedName(s.Target, s.Scope)]
			value, defined := x.env.evaluate(e.exprs[s.Value])
			x.fail(path, defined)
			e.write(x, path, target, value, where)
		case lower.Declare:
			target := e.features[e.translatedName(s.Name, s.Scope)]
			if s.Value == nil {
				x.env.has[target.Name] = solve.BoolTerm(false)
				continue
			}
			value, defined := x.env.evaluate(e.exprs[s.Value])
			x.fail(path, defined)
			e.write(x, path, target, value, where)
		case lower.Block:
			if err := e.statements(x, s.Statements, path, where, node); err != nil {
				return err
			}
		case lower.If:
			cond, defined := x.env.evaluate(e.exprs[s.Condition])
			x.fail(path, defined)
			then := &nodeEffect{env: x.env.clone(), loops: make(map[int]*solve.Term)}
			if err := e.statements(then, s.Then.Statements, solve.And(path, cond), where, node); err != nil {
				return err
			}
			otherwise := &nodeEffect{env: x.env.clone(), loops: make(map[int]*solve.Term)}
			if s.Else != nil {
				if err := e.statements(otherwise, s.Else.Statements, solve.And(path, solve.Not(cond)), where, node); err != nil {
					return err
				}
			}
			x.env = e.merge(cond, then.env, otherwise.env, where)
			x.failed = append(x.failed, then.failed...)
			x.failed = append(x.failed, otherwise.failed...)
			x.absorb(then)
			x.absorb(otherwise)
		case lower.Loop:
			if err := e.loop(x, s, path, where, node); err != nil {
				return err
			}
		default:
			return &UnsupportedError{Node: e.Flow.label(node), Construct: fmt.Sprintf("%T", stmt), Reason: "the interpreter runs no such statement"}
		}
	}
	return nil
}

// absorb takes the loop conditions of a branch's effect into x.
func (x *nodeEffect) absorb(branch *nodeEffect) {
	for l, term := range branch.loops {
		if held, ok := x.loops[l]; ok {
			x.loops[l] = solve.Or(held, term)
		} else {
			x.loops[l] = term
		}
	}
}

// write records a body's write of value to target in x's environment through a
// fresh variable, so the terms stay linear in the body's size; a value the
// feature's declared type does not hold is the interpreter's error.
func (e *Encoding) write(x *nodeEffect, path *solve.Term, target *solve.Var, value *solve.Term, where string) {
	if domain := e.domain(target.Name, value); domain != nil {
		x.fail(path, domain)
	}
	e.fresh++
	v := &solve.Var{Name: fmt.Sprintf("%s@%s#%d", target.Name, where, e.fresh), Sort: target.Sort,
		Symbol: target.Symbol, Dimension: target.Dimension, Unit: target.Unit}
	e.declare(v)
	e.assert(eq(solve.VarTerm(v), value), "write to "+target.Name)
	x.env.write(target.Name, solve.VarTerm(v))
}

// merge joins the environments of two branches by cond, through fresh
// variables for the features they left different.
func (e *Encoding) merge(cond *solve.Term, then, otherwise *env, where string) *env {
	merged := &env{enc: e, values: make(map[string]*solve.Term), has: make(map[string]*solve.Term)}
	for name, a := range then.values {
		b := otherwise.values[name]
		if a == b {
			merged.values[name] = a
			continue
		}
		e.fresh++
		v := &solve.Var{Name: fmt.Sprintf("%s@%s#%d", name, where, e.fresh), Sort: a.Sort,
			Symbol: e.features[name].Symbol, Dimension: e.features[name].Dimension, Unit: e.features[name].Unit}
		e.declare(v)
		e.assert(eq(solve.VarTerm(v), solve.Ite(cond, a, b)), "merge of "+name)
		merged.values[name] = solve.VarTerm(v)
	}
	for name, a := range then.has {
		b := otherwise.has[name]
		if a == b {
			merged.has[name] = a
			continue
		}
		merged.has[name] = solve.Ite(cond, a, b)
	}
	return merged
}

// loop unrolls a conditional loop e.Unroll times; the condition still holding
// after the last iteration is the loop running past the bound.
func (e *Encoding) loop(x *nodeEffect, s lower.Loop, path *solve.Term, where string, node ast.Node) error {
	index := -1
	for l, bl := range e.Flow.Loops {
		if bl.Node == node && bl.Loop.Node == s.Node {
			index = l
			break
		}
	}
	if index < 0 {
		return &FlowError{Node: e.Flow.label(node), Reason: "a body loop the flow's analysis did not record"}
	}
	pre, post := s.Condition, s.Until
	if s.Kind == ast.LoopUntil {
		pre, post = nil, s.Condition
	}
	alive := path
	for j := 0; j < e.Unroll; j++ {
		if pre != nil {
			cond, defined := x.env.evaluate(e.exprs[pre])
			x.fail(alive, defined)
			alive = solve.And(alive, cond)
		}
		body := &nodeEffect{env: x.env.clone(), loops: make(map[int]*solve.Term)}
		if err := e.statements(body, s.Body.Statements, alive, where, node); err != nil {
			return err
		}
		x.env = e.merge(alive, body.env, x.env, where)
		x.failed = append(x.failed, body.failed...)
		x.absorb(body)
		if post != nil {
			cond, defined := x.env.evaluate(e.exprs[post])
			x.fail(alive, defined)
			alive = solve.And(alive, solve.Not(cond))
		}
	}
	past := alive
	if pre != nil {
		cond, defined := x.env.evaluate(e.exprs[pre])
		x.fail(alive, defined)
		past = solve.And(alive, cond)
	}
	if held, ok := x.loops[index]; ok {
		x.loops[index] = solve.Or(held, past)
	} else {
		x.loops[index] = past
	}
	return nil
}

// env is the symbolic value of every feature at one point of a body, and for a
// feature that may hold no value, whether it holds one.
type env struct {
	enc    *Encoding
	values map[string]*solve.Term
	has    map[string]*solve.Term
}

func (v *env) clone() *env {
	c := &env{enc: v.enc, values: make(map[string]*solve.Term, len(v.values)), has: make(map[string]*solve.Term, len(v.has))}
	for k, t := range v.values {
		c.values[k] = t
	}
	for k, t := range v.has {
		c.has[k] = t
	}
	return c
}

// write sets a feature's value; it then holds one.
func (v *env) write(name string, value *solve.Term) {
	v.values[name] = value
	if _, flagged := v.has[name]; flagged {
		v.has[name] = solve.BoolTerm(true)
	}
}

// evaluate is expr's value over the environment, and the condition under which
// the interpreter computes it: its own side conditions, and every feature it
// reads holding a value.
func (v *env) evaluate(expr *solve.Expression) (value, defined *solve.Term) {
	var reads []*solve.Term
	replace := func(base *solve.Var) *solve.Term {
		if has, flagged := v.has[base.Name]; flagged {
			reads = append(reads, has)
		}
		if held, ok := v.values[base.Name]; ok {
			return held
		}
		return nil
	}
	value = solve.Substitute(expr.Term, replace)
	conditions := make([]*solve.Term, 0, len(expr.Defined)+len(reads))
	for _, d := range expr.Defined {
		conditions = append(conditions, solve.Substitute(d, replace))
	}
	conditions = append(conditions, reads...)
	defined = solve.And(conditions...)
	if len(conditions) > 0 {
		// Where the interpreter would not compute it, the value is any of its
		// sort, so no term the relation states is ever evaluated undefined.
		value = solve.Ite(defined, value, zeroOf(value.Sort))
	}
	return value, defined
}

// zeroOf is a literal of the sort, the value an undefined computation stands in as.
func zeroOf(sort solve.Sort) *solve.Term {
	switch sort.Kind {
	case solve.SortBool:
		return solve.BoolTerm(false)
	case solve.SortInt:
		return solve.IntTerm(0)
	case solve.SortReal:
		return solve.RealTerm(new(big.Rat))
	case solve.SortString:
		return solve.StringTerm("")
	}
	return solve.ValueTerm(sort, sort.Values[0])
}

// has is state s's copy of the flag saying whether base holds a value.
func (s *State) has(base *solve.Var) *solve.Var {
	name := "has " + base.Name
	if v, ok := s.Values[name]; ok {
		return v
	}
	v := &solve.Var{Name: fmt.Sprintf("has(%s)@%d", base.Name, s.Move), Sort: solve.Bool, Symbol: base.Symbol}
	s.Values[name] = v
	return v
}

// nonlinear reports whether an assertion multiplies or divides two non-literal terms.
func nonlinear(assertions []solve.Assertion) bool {
	found := false
	for _, a := range assertions {
		walk(a.Term, func(t *solve.Term) {
			if (t.Op == solve.OpMul || t.Op == solve.OpDiv) && !t.Args[0].Literal() && !t.Args[1].Literal() {
				found = true
			}
		})
	}
	return found
}

// integerDivision reports whether an assertion divides integers.
func integerDivision(assertions []solve.Assertion) bool {
	found := false
	for _, a := range assertions {
		walk(a.Term, func(t *solve.Term) {
			if t.Op == solve.OpIntDiv {
				found = true
			}
		})
	}
	return found
}

func walk(t *solve.Term, visit func(*solve.Term)) {
	if t == nil {
		return
	}
	visit(t)
	for _, arg := range t.Args {
		walk(arg, visit)
	}
}
