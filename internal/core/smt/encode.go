package smt

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
	// Inputs are the features of the initial state, free in their domains or
	// pinned at the model's values, in the order the performance declares them.
	Inputs []Input
	// Assumptions names the conditions Assume asserted over the initial state, in order.
	Assumptions []string
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
	// results are the features an inline expression node writes its value to.
	results map[ast.Node]*solve.Var
	// held are the values the performance holds at its start, ahead of its defaults;
	// unbound indexes the inputs it holds none for.
	held    runtime.Held
	unbound map[string]bool
	// releases names the bound features the question leaves free; released indexes them.
	releases []string
	released map[string]bool
	fresh    int
	// pair is the two-copy relation, built on first use.
	pair *Pair
}

// pin is one feature a node's performance holds, and its declared default.
type pin struct {
	feature lower.Feature
	v       *solve.Var
}

// Encode builds the transition relation of action's flow graph for k moves,
// unrolling each body loop unroll times. The action's features resolve as the
// interpreter resolves them, through ctx; held is what the performance holds at
// its start: its features, own and inherited, and their values ahead of the
// defaults it declares. A feature held bound by nothing is free in state 0, in
// the domain its declared type gives it; releases names bound ones to free too.
func Encode(ctx *runtime.Context, action *symbols.Symbol, graph *lower.ActionGraph, held runtime.Held, releases []string, k, unroll int) (*Encoding, error) {
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
	released := make(map[string]bool, len(releases))
	for _, release := range releases {
		released[release] = true
	}
	unbound := make(map[string]bool)
	for _, attr := range held.Unbound() {
		unbound[attr.Name] = true
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
		results:    make(map[ast.Node]*solve.Var),
		held:       held,
		unbound:    unbound,
		releases:   releases,
		released:   released,
		Query:      &solve.Query{Kind: "action", Element: name},
	}
	if err := e.checkReleases(name); err != nil {
		return nil, err
	}
	if err := e.collectFeatures(); err != nil {
		return nil, err
	}
	e.Query.Sorts = append(e.Query.Sorts, e.Sorts.Node, e.Sorts.Edge, e.Sorts.Choice)
	e.Query.Sorts = append(e.Query.Sorts, translator.Sorts()...)
	for i := 0; i <= k; i++ {
		e.States = append(e.States, newState(e.Sorts, f, i))
		e.declare(e.stateVars(e.States[i])...)
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

// stateVars lists every variable of the state in the named vector's order.
func (e *Encoding) stateVars(s *State) []*solve.Var {
	return s.vars(e.Features, e.flagged)
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
// encode: a feature of the performing object, a free input of a type with no domain.
func (e *Encoding) collectFeatures() error {
	graph := e.Flow.Graph
	declared := make(map[string]bool)
	for _, attr := range e.held.Features() {
		scope := attr.Scope
		if scope == nil {
			scope = graph.Scope
		}
		free, released := e.frees(attr)
		if free {
			if _, err := e.translator.Variable(attr.Name, scope, "attribute "+attr.Name); err != nil {
				return noDomain(attr, err)
			}
		}
		v, err := e.variable(attr.Name, scope, "attribute "+attr.Name, nodeLabel(graph.Initial))
		if err != nil {
			return err
		}
		declared[v.Name] = true
		in := Input{Name: attr.Name, Type: attr.Type, Var: v, Free: free, Released: released, def: attr.Value}
		switch {
		case free && v.Sort.Kind == solve.SortString:
			return noDomain(attr, errors.New("a string ranges over no domain the solver decides"))
		case free && attr.Optional:
			e.flagged[v.Name] = true
			in.Optional = true
		case free:
		case attr.Value == nil:
			e.flagged[v.Name] = true
			in.Value, _ = e.held.Value(attr.Name)
		default:
			in.Value, _ = e.held.Value(attr.Name)
			if _, err := e.expression(attr.Value, scope, "default of "+attr.Name, nodeLabel(graph.Initial)); err != nil {
				return err
			}
		}
		e.Inputs = append(e.Inputs, in)
	}
	for _, node := range e.Flow.Nodes {
		if err := e.collectPins(node, declared); err != nil {
			return err
		}
	}
	for _, node := range e.Flow.Nodes {
		label := e.Flow.label(node)
		frame := e.Flow.graphOf(node)
		if err := e.collectFlows(node); err != nil {
			return err
		}
		if err := e.collectBody(frame.Bodies[node], label, declared); err != nil {
			return err
		}
		if action, ok := node.(*ast.ActionExecutionNode); ok && action.Expression != nil {
			if _, err := e.expression(action.Expression, frame.Scope, "expression of "+label, label); err != nil {
				return err
			}
			result := resultPin(frame, node)
			v, err := e.variable(result, frame.Scope, "result "+result+" of "+label, label)
			if err != nil {
				return err
			}
			e.results[node] = v
		}
		for _, i := range e.Flow.Outgoing[node] {
			edge := e.Flow.Edges[i]
			if edge.Guard == nil {
				continue
			}
			if _, err := e.boolean(edge.Guard, frame.Scope, "guard of "+edgeLabel(e.Flow, i), label); err != nil {
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
	for i := range e.Inputs {
		e.Inputs[i].Domain = e.domainText(e.Inputs[i].Var)
	}
	e.Features = make([]*solve.Var, 0, len(e.features))
	for _, v := range e.features {
		e.Features = append(e.Features, v)
	}
	sort.Slice(e.Features, func(i, j int) bool { return e.Features[i].Name < e.Features[j].Name })
	return nil
}

// resultPin names the feature an inline expression node writes its value to: the
// source pin of its first object flow, else `result`, as the interpreter picks it.
func resultPin(graph *lower.ActionGraph, node ast.Node) string {
	if flows := graph.DataFlows[node]; len(flows) > 0 && flows[0].SourcePin != "" {
		return flows[0].SourcePin
	}
	return "result"
}

// collectPins registers the features a node's performance holds, each starting
// without a value unless a delivery or its own default gives it one.
func (e *Encoding) collectPins(node ast.Node, declared map[string]bool) error {
	label := e.Flow.label(node)
	graph := e.Flow.graphOf(node)
	for _, feature := range graph.Features[node] {
		scope := feature.Scope
		if scope == nil {
			scope = graph.Scopes[node]
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
	graph := e.Flow.graphOf(node)
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
		return e.variable(name, e.Flow.graphOf(node).Scope, within, label)
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
			construct := "declaration of " + s.Name
			v, err := e.variable(s.Name, s.Scope, construct, label)
			if err != nil {
				return err
			}
			if declared[v.Name] {
				return &UnsupportedError{Node: label, Construct: construct,
					Reason: "a body declaring a name the action declares is not encoded"}
			}
			if s.Value == nil {
				e.flagged[v.Name] = true
			} else if _, err := e.expression(s.Value, s.Scope, construct, label); err != nil {
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
					arrived = append(arrived, and(
						eq(solve.VarTerm(other.At), nodeValue(e.Sorts, f, n)),
						eq(solve.VarTerm(other.Via), edgeValue(e.Sorts, f, i))))
				}
				complete = append(complete, or(arrived...))
			}
			var earliest []*solve.Term
			for u, other := range s.Slots {
				if u == t {
					continue
				}
				earliest = append(earliest, implies(
					and(eq(solve.VarTerm(other.At), nodeValue(e.Sorts, f, n)), not(e.noEdge(other.Via))),
					gt(solve.VarTerm(other.ID), solve.VarTerm(slot.ID))))
			}
			cases = append(cases, and(here, or(
				e.noEdge(slot.Via),
				and(and(complete...), and(earliest...)))))
		}
		terms[t] = or(cases...)
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
	return and(terms...)
}

func (e *Encoding) absent(at *solve.Var) *solve.Term {
	return eq(solve.VarTerm(at), solve.ValueTerm(e.Sorts.Node, Absent))
}

func (e *Encoding) noEdge(via *solve.Var) *solve.Term {
	return eq(solve.VarTerm(via), solve.ValueTerm(e.Sorts.Edge, NoEdge))
}

// initial constrains state 0: one token at the initial node, the attributes at the
// values held for them, else their defaults, in declaration order as the interpreter
// initializes them.
func (e *Encoding) initial() error {
	const initialToken, initialState = "initial token", "initial state"
	s := e.States[0]
	f := e.Flow
	for t, slot := range s.Slots {
		if t == 0 {
			e.assert(eq(solve.VarTerm(slot.At), nodeValue(e.Sorts, f, f.Index[f.Graph.Initial])), initialToken)
			e.assert(eq(solve.VarTerm(slot.ID), solve.IntTerm(1)), initialToken)
		} else {
			e.assert(e.absent(slot.At), initialToken)
			e.assert(eq(solve.VarTerm(slot.ID), solve.IntTerm(0)), initialToken)
		}
		e.assert(e.noEdge(slot.Via), initialToken)
	}
	e.assert(eq(solve.VarTerm(s.NextID), solve.IntTerm(2)), initialToken)
	for _, flag := range s.Loop {
		e.assert(not(solve.VarTerm(flag)), initialState)
	}
	if s.Overflow != nil {
		e.assert(not(solve.VarTerm(s.Overflow)), initialState)
	}
	env := e.environment(s)
	open := make(map[string]bool, len(e.Inputs))
	for _, in := range e.Inputs {
		if in.Optional {
			open[in.Var.Name] = true
		}
	}
	for _, base := range e.Features {
		// A flagged feature starts absent, but an optional free input's presence is
		// the solver's to choose: its state-0 flag stays open.
		if e.flagged[base.Name] && !open[base.Name] {
			env.has[base.Name] = solve.BoolTerm(false)
		}
	}
	failed, err := e.initialInputs(env)
	if err != nil {
		return err
	}
	for _, base := range e.Features {
		if value := env.values[base.Name]; value.Op != solve.OpVar || value.Var != s.value(base) {
			e.assert(eq(solve.VarTerm(s.value(base)), value), "initial value of "+base.Name)
		}
		if !e.flagged[base.Name] {
			continue
		}
		if has := env.has[base.Name]; has.Op != solve.OpVar || has.Var != s.has(base) {
			e.assert(eq(solve.VarTerm(s.has(base)), has), "initial value of "+base.Name)
		}
	}
	e.assert(eq(solve.VarTerm(s.Failed), or(failed...)), initialState)
	return nil
}

// initialInputs writes each held or defaulted input into env and bounds the free
// ones; the terms returned are the ways a default or domain fails in state 0.
func (e *Encoding) initialInputs(env *env) ([]*solve.Term, error) {
	f := e.Flow
	var failed []*solve.Term
	for _, in := range e.Inputs {
		v := in.Var
		if in.Free {
			if v.Sort.Kind == solve.SortInt {
				// The interpreter's Integer is int64; beyond it there is no value to replay.
				e.assert(solve.Int64(env.values[v.Name]), "range of "+in.Name)
			}
			if domain := e.domain(v.Name, env.values[v.Name]); domain != nil {
				e.assert(domain, "domain of "+in.Name)
			}
			continue
		}
		var value *solve.Term
		if in.Value.Kind != runtime.ValInvalid {
			term, err := e.translator.Literal(v, in.Value)
			if err != nil {
				return nil, refusal(f.label(f.Graph.Initial), "value held by "+in.Name, err)
			}
			value = term
		} else if expr, ok := e.exprs[in.def]; ok {
			var defined *solve.Term
			value, defined = env.evaluate(expr)
			failed = append(failed, not(defined))
		} else {
			continue
		}
		env.write(v.Name, value)
		if domain := e.domain(v.Name, value); domain != nil {
			failed = append(failed, not(domain))
		}
	}
	return failed, nil
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
	anyChoosable := or(e.Choosable[i-1]...)
	e.assert(eq(stutter, not(anyChoosable)), fmt.Sprintf("move %d stutters only when no token may act", i))
	for t := range prev.Slots {
		e.assert(implies(eq(choice, solve.ValueTerm(e.Sorts.Choice, slotLabel(t))), e.Choosable[i-1][t]),
			fmt.Sprintf("move %d acts on a token that may act", i))
	}

	acts := e.actsAt(prev, choice)
	effects, guards, err := e.performances(i, prev, m, acts)
	if err != nil {
		return err
	}
	e.featureValues(i, prev, next, effects, acts)
	failed, overflow := e.carried(prev, effects, acts)
	e.loopsPastBound(i, prev, next, effects, acts)

	// The tokens after the move.
	travel := solve.VarTerm(m.Travel)
	var frame []*solve.Term
	for t := range prev.Slots {
		frame = append(frame, e.unchanged(prev.Slots[t], next.Slots[t]))
	}
	e.assert(implies(stutter, and(
		and(frame...),
		eq(solve.VarTerm(next.NextID), solve.VarTerm(prev.NextID)),
		eq(travel, solve.ValueTerm(e.Sorts.Edge, NoEdge)))), fmt.Sprintf("move %d stutters", i))

	for t := range prev.Slots {
		for n, node := range f.Nodes {
			pick := and(eq(choice, solve.ValueTerm(e.Sorts.Choice, slotLabel(t))),
				eq(solve.VarTerm(prev.Slots[t].At), nodeValue(e.Sorts, f, n)))
			step, fails, full := e.tokens(t, n, node, prev, next, m, guards[n])
			e.assert(implies(pick, step), fmt.Sprintf("move %d: slot %d acts at %s", i, t, f.Labels[n]))
			if fails != nil {
				failed = append(failed, and(pick, fails))
			}
			if full != nil {
				overflow = append(overflow, and(pick, full))
			}
		}
	}
	e.assert(eq(solve.VarTerm(next.Failed), or(failed...)), fmt.Sprintf("move %d fails", i))
	if next.Overflow != nil {
		e.assert(eq(solve.VarTerm(next.Overflow), or(overflow...)), fmt.Sprintf("move %d overflows", i))
	} else if len(overflow) > 0 {
		return &FlowError{Node: nodeLabel(f.Graph.Initial), Reason: "a move may overflow a state declaring no overflow"}
	}
	return nil
}

// actsAt is, per node, whether the token choice picks is at that node in state prev.
func (e *Encoding) actsAt(prev *State, choice *solve.Term) []*solve.Term {
	f := e.Flow
	acts := make([]*solve.Term, len(f.Nodes))
	for n := range f.Nodes {
		var cases []*solve.Term
		for t, slot := range prev.Slots {
			cases = append(cases, and(
				eq(choice, solve.ValueTerm(e.Sorts.Choice, slotLabel(t))),
				eq(solve.VarTerm(slot.At), nodeValue(e.Sorts, f, n))))
		}
		acts[n] = or(cases...)
	}
	return acts
}

// outgoingGuards are what a node's successions' guards evaluate to after its
// performance, in succession order, and whether each evaluation is defined.
type outgoingGuards struct {
	holds, defined []*solve.Term
}

// performances are the effect of each node's performance in move i on the features,
// and the guards it then reads, over the values before the move.
func (e *Encoding) performances(i int, prev *State, m *Move, acts []*solve.Term) ([]*nodeEffect, []outgoingGuards, error) {
	f := e.Flow
	effects := make([]*nodeEffect, len(f.Nodes))
	guards := make([]outgoingGuards, len(f.Nodes))
	for n, node := range f.Nodes {
		effect, err := e.perform(i, n, node, prev)
		if err != nil {
			return nil, nil, err
		}
		effects[n] = effect
		g := outgoingGuards{
			holds:   make([]*solve.Term, len(f.Outgoing[node])),
			defined: make([]*solve.Term, len(f.Outgoing[node])),
		}
		for p, edge := range f.Outgoing[node] {
			guard := f.Edges[edge].Guard
			if guard == nil {
				g.holds[p], g.defined[p] = solve.BoolTerm(true), solve.BoolTerm(true)
				continue
			}
			g.holds[p], g.defined[p] = effect.guards.evaluate(e.exprs[guard])
			if held := m.Held[edge]; held != nil {
				e.assert(eq(solve.VarTerm(held), and(acts[n], g.defined[p], g.holds[p])),
					fmt.Sprintf("move %d: the guard of %s is read and holds", i, edgeLabel(f, edge)))
			}
		}
		guards[n] = g
	}
	return effects, guards, nil
}

// featureValues ties the features after move i to what the acting node's body left,
// else to their values before it.
func (e *Encoding) featureValues(i int, prev, next *State, effects []*nodeEffect, acts []*solve.Term) {
	for _, base := range e.Features {
		value := solve.VarTerm(prev.value(base))
		var has *solve.Term
		if e.flagged[base.Name] {
			has = solve.VarTerm(prev.has(base))
		}
		for n := len(e.Flow.Nodes) - 1; n >= 0; n-- {
			after := effects[n].env
			if after.values[base.Name] != solve.VarTerm(prev.value(base)) && !sameVar(after.values[base.Name], prev.value(base)) {
				value = ite(acts[n], after.values[base.Name], value)
			}
			if has != nil && !sameVar(after.has[base.Name], prev.has(base)) {
				has = ite(acts[n], after.has[base.Name], has)
			}
		}
		e.assert(eq(solve.VarTerm(next.value(base)), value), fmt.Sprintf("move %d: %s", i, base.Name))
		if has != nil {
			e.assert(eq(solve.VarTerm(next.has(base)), has), fmt.Sprintf("move %d: %s", i, base.Name))
		}
	}
}

// carried is the errors and overflows a move carries: those already met, which
// stay, and those the acting node's performance meets.
func (e *Encoding) carried(prev *State, effects []*nodeEffect, acts []*solve.Term) (failed, overflow []*solve.Term) {
	failed = []*solve.Term{solve.VarTerm(prev.Failed)}
	if prev.Overflow != nil {
		overflow = append(overflow, solve.VarTerm(prev.Overflow))
	}
	for n := range e.Flow.Nodes {
		if len(effects[n].failed) > 0 {
			failed = append(failed, and(acts[n], or(effects[n].failed...)))
		}
		if len(effects[n].overflow) > 0 {
			overflow = append(overflow, and(acts[n], or(effects[n].overflow...)))
		}
	}
	return failed, overflow
}

// loopsPastBound ties each body loop's past-the-bound flag after move i to the flag
// before it or the acting node's body passing the bound.
func (e *Encoding) loopsPastBound(i int, prev, next *State, effects []*nodeEffect, acts []*solve.Term) {
	for l := range e.Flow.Loops {
		loop := []*solve.Term{solve.VarTerm(prev.Loop[l])}
		for n := range e.Flow.Nodes {
			if term, ok := effects[n].loops[l]; ok {
				loop = append(loop, and(acts[n], term))
			}
		}
		e.assert(eq(solve.VarTerm(next.Loop[l]), or(loop...)), fmt.Sprintf("move %d: loop %d past the bound", i, l))
	}
}

// sameVar reports whether term reads exactly the variable v.
func sameVar(term *solve.Term, v *solve.Var) bool {
	return term != nil && term.Op == solve.OpVar && term.Var == v
}

// unchanged says slot after holds what slot before did.
func (e *Encoding) unchanged(before, after Slot) *solve.Term {
	return and(
		eq(solve.VarTerm(after.At), solve.VarTerm(before.At)),
		eq(solve.VarTerm(after.Via), solve.VarTerm(before.Via)),
		eq(solve.VarTerm(after.ID), solve.VarTerm(before.ID)))
}

// tokenStep is the token in slot t at node n acting in a move: the slots it consumes
// and frees, the ones a fork places tokens in, and the terms the step asserts.
type tokenStep struct {
	e          *Encoding
	t, n       int
	node       ast.Node
	out        []int
	prev, next *State
	guards     outgoingGuards
	travel     *solve.Term
	noEdge     *solve.Term
	absent     *solve.Term
	// actorID is the acting token's identifier, fresh after a synchronization;
	// base is the next identifier to give out after it.
	actorID, base *solve.Term
	consumed      []*solve.Term
	free          []*solve.Term
	placed        []*solve.Term
	terms         []*solve.Term
	nextID        *solve.Term
	fails, full   *solve.Term
}

// tokens moves the token in slot t at node n (synchronization, then succession); the
// other results are when the succession errors and when a fork finds no free slot, nil if never.
func (e *Encoding) tokens(t, n int, node ast.Node, prev, next *State, m *Move, guards outgoingGuards) (step, fails, full *solve.Term) {
	s := &tokenStep{
		e: e, t: t, n: n, node: node, out: e.Flow.Outgoing[node], prev: prev, next: next, guards: guards,
		travel: solve.VarTerm(m.Travel),
		noEdge: solve.ValueTerm(e.Sorts.Edge, NoEdge),
		absent: solve.ValueTerm(e.Sorts.Node, Absent),
	}
	s.synchronize()
	s.nextID = s.base
	switch node.(type) {
	case *ast.FinalNode:
		s.terms = append(s.terms, s.retire())
	case *ast.ForkNode:
		s.fork()
	case *ast.DecisionNode:
		s.decide()
	default:
		s.succeed()
	}
	s.others()
	s.terms = append(s.terms, eq(solve.VarTerm(next.NextID), s.nextID))
	return and(s.terms...), s.fails, s.full
}

// synchronize is the synchronization at a join: the earliest arrival over each
// succession collapses into a fresh token and the slots consumed are freed.
func (s *tokenStep) synchronize() {
	e, f, prev := s.e, s.e.Flow, s.prev
	slot := prev.Slots[s.t]
	synced := solve.BoolTerm(false)
	if e.synchronizes(s.node) {
		synced = not(e.noEdge(slot.Via))
	}
	s.consumed = make([]*solve.Term, len(prev.Slots))
	for u, other := range prev.Slots {
		if u == s.t {
			s.consumed[u] = solve.BoolTerm(false)
			continue
		}
		var earliest []*solve.Term
		for w, third := range prev.Slots {
			if w == u {
				continue
			}
			earliest = append(earliest, implies(
				and(eq(solve.VarTerm(third.At), nodeValue(e.Sorts, f, s.n)), eq(solve.VarTerm(third.Via), solve.VarTerm(other.Via))),
				gt(solve.VarTerm(third.ID), solve.VarTerm(other.ID))))
		}
		s.consumed[u] = and(synced,
			eq(solve.VarTerm(other.At), nodeValue(e.Sorts, f, s.n)),
			not(e.noEdge(other.Via)),
			and(earliest...))
	}
	s.actorID = ite(synced, solve.VarTerm(prev.NextID), solve.VarTerm(slot.ID))
	s.base = ite(synced, add(solve.VarTerm(prev.NextID), solve.IntTerm(1)), solve.VarTerm(prev.NextID))

	// free[u] says slot u (other than t) is free once consumption is done.
	s.free = make([]*solve.Term, len(prev.Slots))
	s.placed = make([]*solve.Term, len(prev.Slots))
	for u, other := range prev.Slots {
		s.placed[u] = solve.BoolTerm(false)
		if u == s.t {
			s.free[u] = solve.BoolTerm(false)
			continue
		}
		s.free[u] = or(e.absent(other.At), s.consumed[u])
	}
}

// retire says the acting token leaves the flow.
func (s *tokenStep) retire() *solve.Term {
	after := s.next.Slots[s.t]
	return and(
		eq(solve.VarTerm(after.At), s.absent),
		eq(solve.VarTerm(after.Via), s.noEdge),
		eq(solve.VarTerm(after.ID), s.actorID),
		eq(s.travel, s.noEdge))
}

// take says the acting token travels its p-th succession.
func (s *tokenStep) take(p int) *solve.Term {
	e, f := s.e, s.e.Flow
	edge := f.Edges[s.out[p]]
	after := s.next.Slots[s.t]
	return and(
		eq(solve.VarTerm(after.At), nodeValue(e.Sorts, f, f.Index[edge.Target])),
		eq(solve.VarTerm(after.Via), edgeValue(e.Sorts, f, s.out[p])),
		eq(solve.VarTerm(after.ID), s.actorID),
		eq(s.travel, edgeValue(e.Sorts, f, s.out[p])))
}

// stay says the acting token stays where it is.
func (s *tokenStep) stay() *solve.Term {
	before, after := s.prev.Slots[s.t], s.next.Slots[s.t]
	return and(
		eq(solve.VarTerm(after.At), solve.VarTerm(before.At)),
		eq(solve.VarTerm(after.Via), solve.VarTerm(before.Via)),
		eq(solve.VarTerm(after.ID), s.actorID),
		eq(s.travel, s.noEdge))
}

// fork gives each enabled succession a fresh token, in order: the first in the
// actor's slot, the rest in the free slots in order.
func (s *tokenStep) fork() {
	e, f, out, prev, next := s.e, s.e.Flow, s.out, s.prev, s.next
	guards := s.guards.holds
	rank := make([]*solve.Term, len(out))
	count := solve.IntTerm(0)
	for p := range out {
		rank[p] = count
		count = add(count, ite(guards[p], solve.IntTerm(1), solve.IntTerm(0)))
	}
	none := eq(count, solve.IntTerm(0))
	var actor []*solve.Term
	for p := range out {
		edge := f.Edges[out[p]]
		isFirst := and(guards[p], eq(rank[p], solve.IntTerm(0)))
		actor = append(actor, implies(isFirst, and(
			eq(solve.VarTerm(next.Slots[s.t].At), nodeValue(e.Sorts, f, f.Index[edge.Target])),
			eq(solve.VarTerm(next.Slots[s.t].Via), edgeValue(e.Sorts, f, out[p])),
			eq(solve.VarTerm(next.Slots[s.t].ID), s.base),
			eq(s.travel, edgeValue(e.Sorts, f, out[p])))))
	}
	s.terms = append(s.terms, implies(none, s.retire()), implies(not(none), and(actor...)))
	freeRank := make([]*solve.Term, len(prev.Slots))
	running := solve.IntTerm(0)
	for u := range prev.Slots {
		freeRank[u] = running
		if u != s.t {
			running = add(running, ite(s.free[u], solve.IntTerm(1), solve.IntTerm(0)))
		}
	}
	for u := range prev.Slots {
		if u == s.t {
			continue
		}
		var here []*solve.Term
		for p := range out {
			edge := f.Edges[out[p]]
			takes := and(guards[p], ge(rank[p], solve.IntTerm(1)),
				eq(freeRank[u], sub(rank[p], solve.IntTerm(1))))
			here = append(here, takes)
			s.terms = append(s.terms, implies(and(s.free[u], takes), and(
				eq(solve.VarTerm(next.Slots[u].At), nodeValue(e.Sorts, f, f.Index[edge.Target])),
				eq(solve.VarTerm(next.Slots[u].Via), edgeValue(e.Sorts, f, out[p])),
				eq(solve.VarTerm(next.Slots[u].ID), add(s.base, rank[p])))))
		}
		s.placed[u] = and(s.free[u], or(here...))
	}
	s.nextID = add(s.base, count)
	s.fails = or(undefinedGuards(s.guards.defined)...)
	if f.Cyclic {
		s.full = gt(count, add(running, solve.IntTerm(1)))
	}
}

// decide takes the branch whose guard holds; none holding takes the unguarded
// one, and without one the run fails.
func (s *tokenStep) decide() {
	e, f, out := s.e, s.e.Flow, s.out
	guards, defined := s.guards.holds, s.guards.defined
	var holds []*solve.Term
	unguarded := -1
	var anyHolds *solve.Term = solve.BoolTerm(false)
	var undefined []*solve.Term
	for p := range out {
		if f.Edges[out[p]].Guard == nil {
			unguarded = p
			continue
		}
		held := and(defined[p], guards[p])
		undefined = append(undefined, and(not(anyHolds), not(defined[p])))
		holds = append(holds, held)
		anyHolds = or(anyHolds, held)
	}
	var branches []*solve.Term
	q := 0
	for p := range out {
		if f.Edges[out[p]].Guard == nil {
			continue
		}
		branches = append(branches, and(eq(s.travel, edgeValue(e.Sorts, f, out[p])), holds[q]))
		s.terms = append(s.terms, implies(and(anyHolds, eq(s.travel, edgeValue(e.Sorts, f, out[p]))), s.take(p)))
		q++
	}
	if len(branches) > 0 {
		s.terms = append(s.terms, implies(anyHolds, or(branches...)))
	}
	if unguarded >= 0 {
		s.terms = append(s.terms, implies(not(anyHolds), s.take(unguarded)))
	} else {
		s.terms = append(s.terms, implies(not(anyHolds), s.stay()))
		undefined = append(undefined, not(anyHolds))
	}
	s.fails = or(undefined...)
}

// succeed takes the one enabled succession; none retires the token; several
// out of a node other than the initial one is an error.
func (s *tokenStep) succeed() {
	guards := s.guards.holds
	_, initial := s.node.(*ast.InitialNode)
	count := solve.IntTerm(0)
	taken := solve.BoolTerm(false)
	for p := range s.out {
		isFirst := and(guards[p], not(taken))
		s.terms = append(s.terms, implies(isFirst, s.take(p)))
		taken = or(taken, guards[p])
		count = add(count, ite(guards[p], solve.IntTerm(1), solve.IntTerm(0)))
	}
	s.terms = append(s.terms, implies(not(taken), s.retire()))
	failures := undefinedGuards(s.guards.defined)
	if !initial && len(s.out) > 1 {
		failures = append(failures, gt(count, solve.IntTerm(1)))
	}
	if len(failures) > 0 {
		s.fails = or(failures...)
	}
}

// others ties the other slots: consumed ones are freed, the rest are as they
// were unless a fork placed a token in them.
func (s *tokenStep) others() {
	prev, next := s.prev, s.next
	for u := range prev.Slots {
		if u == s.t {
			continue
		}
		freed := and(
			eq(solve.VarTerm(next.Slots[u].At), s.absent),
			eq(solve.VarTerm(next.Slots[u].Via), s.noEdge),
			eq(solve.VarTerm(next.Slots[u].ID), solve.VarTerm(prev.Slots[u].ID)))
		s.terms = append(s.terms,
			implies(and(s.consumed[u], not(s.placed[u])), freed),
			implies(and(not(s.consumed[u]), not(s.placed[u])), s.e.unchanged(prev.Slots[u], next.Slots[u])))
	}
}

// undefinedGuards are the conditions under which evaluating the guards, in
// order, meets an error before a decision is reached.
func undefinedGuards(defined []*solve.Term) []*solve.Term {
	var terms []*solve.Term
	for p := range defined {
		if defined[p].Op == solve.OpBool && defined[p].Bool {
			continue
		}
		terms = append(terms, not(defined[p]))
	}
	return terms
}

// perform is one performance of node n in move i, in the interpreter's order: pins take their
// deliveries or defaults, the body runs over prev, the guards are read, the object flows carry.
func (e *Encoding) perform(i, n int, node ast.Node, prev *State) (*nodeEffect, error) {
	effect := &nodeEffect{env: e.environment(prev), loops: make(map[int]*solve.Term)}
	where := fmt.Sprintf("%d.%s", i, e.Flow.Labels[n])
	e.begin(effect, node, where)
	body := e.Flow.graphOf(node).Bodies[node]
	if err := e.statements(effect, body, solve.BoolTerm(true), where, node); err != nil {
		return nil, err
	}
	if action, ok := node.(*ast.ActionExecutionNode); ok && action.Expression != nil {
		value, defined := effect.env.evaluate(e.exprs[action.Expression])
		effect.fail(solve.BoolTerm(true), defined)
		e.write(effect, solve.BoolTerm(true), e.results[node], value, where)
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
	always := solve.BoolTerm(true)
	for _, p := range e.pins[node] {
		name := p.v.Name
		value, has := x.env.values[name], solve.BoolTerm(false)
		seeded := always
		pending, queued := e.pending[name]
		if queued {
			seeded = not(x.env.has[pending.Name])
		}
		if p.feature.Value != nil {
			declared, defined := x.env.evaluate(e.exprs[p.feature.Value])
			x.fail(seeded, defined)
			if domain := e.domain(name, declared); domain != nil {
				x.fail(seeded, domain)
			}
			value, has = declared, always
		}
		if queued {
			value = ite(seeded, value, x.env.values[pending.Name])
			has = ite(seeded, has, always)
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
	always := solve.BoolTerm(true)
	label := e.Flow.label(node)
	for _, flow := range e.Flow.graphOf(node).DataFlows[node] {
		source, err := e.flowEnd(node, flow.SourcePin, flowLabel(flow), label)
		if err != nil {
			return err
		}
		target, err := e.flowEnd(flow.Target, flow.TargetPin, flowLabel(flow), label)
		if err != nil {
			return err
		}
		if has, flagged := x.env.has[source.Name]; flagged {
			x.fail(always, has)
		}
		value := x.env.values[source.Name]
		if domain := e.domain(target.Name, value); domain != nil {
			x.fail(always, domain)
		}
		pending, queued := e.pending[target.Name]
		if !queued {
			e.write(x, always, target, value, where)
			continue
		}
		x.overflow = append(x.overflow, x.env.has[pending.Name])
		e.write(x, always, pending, value, where)
	}
	return nil
}

// fail records that the interpreter reports an error where path holds and
// defined does not.
func (x *nodeEffect) fail(path, defined *solve.Term) {
	if defined.Op == solve.OpBool && defined.Bool {
		return
	}
	x.failed = append(x.failed, and(path, not(defined)))
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
			if err := e.statements(then, s.Then.Statements, and(path, cond), where, node); err != nil {
				return err
			}
			otherwise := &nodeEffect{env: x.env.clone(), loops: make(map[int]*solve.Term)}
			if s.Else != nil {
				if err := e.statements(otherwise, s.Else.Statements, and(path, not(cond)), where, node); err != nil {
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
			x.loops[l] = or(held, term)
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
		e.assert(eq(solve.VarTerm(v), ite(cond, a, b)), "merge of "+name)
		merged.values[name] = solve.VarTerm(v)
	}
	for name, a := range then.has {
		b := otherwise.has[name]
		if a == b {
			merged.has[name] = a
			continue
		}
		merged.has[name] = ite(cond, a, b)
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
			alive = and(alive, cond)
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
			alive = and(alive, not(cond))
		}
	}
	past := alive
	if pre != nil {
		cond, defined := x.env.evaluate(e.exprs[pre])
		x.fail(alive, defined)
		past = and(alive, cond)
	}
	if held, ok := x.loops[index]; ok {
		x.loops[index] = or(held, past)
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
	defined = and(conditions...)
	if len(conditions) > 0 {
		// Where the interpreter would not compute it, the value is any of its
		// sort, so no term the relation states is ever evaluated undefined.
		value = ite(defined, value, zeroOf(value.Sort))
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
