package export

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ActionForm is one lowered ActionGraph. Vertices are numbered by their position
// in Nodes; every other field refers to them by that number.
type ActionForm struct {
	// Name is the qualified name of the action; Kind is its symbol kind.
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Error is the lowering's refusal of a performed action, with no graph beside it.
	Error string `json:"error,omitempty"`
	// Scope is the scope the action's body resolves names in.
	Scope string `json:"scope,omitempty"`
	// Parameters is the signature a caller writes and reads back, in invocation
	// order: the action's own, then the inherited ones none redefines.
	Parameters []ParameterForm `json:"parameters,omitempty"`
	Attributes []AttributeForm `json:"attributes,omitempty"`
	Nodes      []NodeForm      `json:"nodes"`
	// Initial is the vertex the flow starts at, absent for a graph without one.
	Initial *int  `json:"initial,omitempty"`
	Finals  []int `json:"finals,omitempty"`
	// Edges are the control flows, by source vertex then declaration order.
	Edges []EdgeForm `json:"edges,omitempty"`
	// Flows are the object flows between pins, by source vertex then declaration order.
	Flows       []ObjectFlowForm `json:"flows,omitempty"`
	Bindings    []PinBindingForm `json:"bindings,omitempty"`
	Connections []ConnectionForm `json:"connections,omitempty"`
}

// NodeForm is one vertex of an action graph with everything the lowering
// attached to it.
type NodeForm struct {
	ID   int      `json:"id"`
	Kind string   `json:"kind"`
	Name string   `json:"name,omitempty"`
	Span SpanForm `json:"span"`
	// Scope is the scope the node's own members resolve in, when it owns one.
	Scope    string        `json:"scope,omitempty"`
	Features []FeatureForm `json:"features,omitempty"`
	// Body is the node's statements; StatementRun marks a body the node runs as
	// statements rather than as a flow of its own.
	Body         []StatementForm `json:"body,omitempty"`
	StatementRun bool            `json:"statementRun,omitempty"`
	// Block lists the vertices a block's body sequences, for a node that is one.
	Block []int `json:"block,omitempty"`
	// Accept is the message an accept node waits for.
	Accept *AcceptForm `json:"accept,omitempty"`
	// Subflow is the flow the node's own members state, for a node that has one.
	Subflow *SubflowForm `json:"subflow,omitempty"`
	// Performs are the qualified names of the behaviors the node performs or is
	// typed by, as they resolve; each has a graph of its own in the form.
	Performs []string `json:"performs,omitempty"`
}

// EdgeForm is a control flow, guarded when Guard is present; Else marks the
// branch of a decision written as `else`, taken when no guarded branch is.
type EdgeForm struct {
	Source int       `json:"source"`
	Target int       `json:"target"`
	Guard  *ExprForm `json:"guard,omitempty"`
	Else   bool      `json:"else,omitempty"`
	Decl   SpanForm  `json:"decl"`
}

// ObjectFlowForm is a data flow from a pin of Source to a pin of Target.
type ObjectFlowForm struct {
	Name      string   `json:"name,omitempty"`
	Source    int      `json:"source"`
	SourcePin string   `json:"sourcePin,omitempty"`
	Target    int      `json:"target"`
	TargetPin string   `json:"targetPin,omitempty"`
	Decl      SpanForm `json:"decl"`
}

// PinBindingForm is a binding connector with an end at a node's pin; see
// lower.PinBinding for the reading of each field.
type PinBindingForm struct {
	Node         *int       `json:"node,omitempty"`
	Path         []string   `json:"path,omitempty"`
	Pin          string     `json:"pin,omitempty"`
	Other        *ExprForm  `json:"other,omitempty"`
	OtherNode    *int       `json:"otherNode,omitempty"`
	OtherPath    []string   `json:"otherPath,omitempty"`
	OtherPin     string     `json:"otherPin,omitempty"`
	OtherChain   *ChainForm `json:"otherChain,omitempty"`
	OtherFeature string     `json:"otherFeature,omitempty"`
	Scope        string     `json:"scope,omitempty"`
	Decl         SpanForm   `json:"decl"`
	FromValue    bool       `json:"fromValue,omitempty"`
}

// ChainForm is a feature chain walked from a base expression.
type ChainForm struct {
	Base  *ExprForm `json:"base,omitempty"`
	Steps []string  `json:"steps,omitempty"`
	Text  string    `json:"text"`
}

// ConnectionForm is a connector the behavior routes a `send … via` through.
type ConnectionForm struct {
	Ends      []string `json:"ends"`
	Variation string   `json:"variation,omitempty"`
	Variant   string   `json:"variant,omitempty"`
	Owner     string   `json:"owner"`
	Scope     string   `json:"scope,omitempty"`
}

// AcceptForm is what an accept node or a transition trigger receives.
type AcceptForm struct {
	Param        string       `json:"param,omitempty"`
	SignalType   string       `json:"signalType,omitempty"`
	ViaPort      string       `json:"viaPort,omitempty"`
	SubsetsEvent *ExprForm    `json:"subsetsEvent,omitempty"`
	Trigger      *TriggerForm `json:"trigger,omitempty"`
	Scope        string       `json:"scope,omitempty"`
}

// TriggerForm is a trigger event as classified by the lowering: an accepted
// signal, a change of a condition, or a time.
type TriggerForm struct {
	Kind string   `json:"kind"`
	Text string   `json:"text,omitempty"`
	Span SpanForm `json:"span"`
	// accept
	SignalType string `json:"signalType,omitempty"`
	Subsets    string `json:"subsets,omitempty"`
	Payload    string `json:"payload,omitempty"`
	// call
	Operation  string   `json:"operation,omitempty"`
	Parameters []string `json:"parameters,omitempty"`
	// change
	Condition *ExprForm `json:"condition,omitempty"`
	// time
	Duration *ExprForm `json:"duration,omitempty"`
	Absolute bool      `json:"absolute,omitempty"`
}

// AttributeForm is an attribute the behavior declares, with the types and unit
// the semantic model derives for it.
type AttributeForm struct {
	Name  string    `json:"name"`
	Types []string  `json:"types,omitempty"`
	Unit  string    `json:"unit,omitempty"`
	Value *ExprForm `json:"value,omitempty"`
	Span  SpanForm  `json:"span"`
	Scope string    `json:"scope,omitempty"`
}

// FeatureForm is a parameter or attribute a node declares itself.
type FeatureForm struct {
	Name      string    `json:"name"`
	Direction string    `json:"direction,omitempty"`
	Result    bool      `json:"result,omitempty"`
	Types     []string  `json:"types,omitempty"`
	Unit      string    `json:"unit,omitempty"`
	Value     *ExprForm `json:"value,omitempty"`
	Span      SpanForm  `json:"span"`
	Scope     string    `json:"scope,omitempty"`
}

// ParameterForm is one parameter of the action's signature. Optional marks one a
// caller may leave out: it declares a default or its multiplicity admits no value.
type ParameterForm struct {
	Name      string    `json:"name"`
	Direction string    `json:"direction,omitempty"`
	Result    bool      `json:"result,omitempty"`
	Optional  bool      `json:"optional,omitempty"`
	Types     []string  `json:"types,omitempty"`
	Unit      string    `json:"unit,omitempty"`
	Value     *ExprForm `json:"value,omitempty"`
	Span      SpanForm  `json:"span"`
	Scope     string    `json:"scope,omitempty"`
}

// SubflowForm is the flow nested under a node: its graph, or the lowering's refusal.
type SubflowForm struct {
	Graph *ActionForm `json:"graph,omitempty"`
	Error string      `json:"error,omitempty"`
}

// StatementForm is one lowered statement; Kind selects which fields are set.
type StatementForm struct {
	Kind  string   `json:"kind"`
	Span  SpanForm `json:"span"`
	Scope string   `json:"scope,omitempty"`
	// send
	Message      *ExprForm `json:"message,omitempty"`
	Target       string    `json:"target,omitempty"`
	TargetSymbol string    `json:"targetSymbol,omitempty"`
	TargetPath   bool      `json:"targetPath,omitempty"`
	Via          bool      `json:"via,omitempty"`
	Receiver     string    `json:"receiver,omitempty"`
	ReceiverPath bool      `json:"receiverPath,omitempty"`
	// assign, declare, declare usage
	Name  string     `json:"name,omitempty"`
	Chain *ChainForm `json:"chain,omitempty"`
	Value *ExprForm  `json:"value,omitempty"`
	// loop
	Loop       string     `json:"loop,omitempty"`
	Condition  *ExprForm  `json:"condition,omitempty"`
	Until      *ExprForm  `json:"until,omitempty"`
	Variable   string     `json:"variable,omitempty"`
	Collection *ExprForm  `json:"collection,omitempty"`
	Body       *BlockForm `json:"body,omitempty"`
	// if
	Then *BlockForm `json:"then,omitempty"`
	Else *BlockForm `json:"else,omitempty"`
	// effect
	Effect   string   `json:"effect,omitempty"`
	Performs []string `json:"performs,omitempty"`
	// unsupported
	Description string `json:"description,omitempty"`
}

// BlockForm is a block of statements, with the flow graph of one that states
// a flow of its own.
type BlockForm struct {
	Statements []StatementForm `json:"statements"`
	Scope      string          `json:"scope,omitempty"`
	Own        bool            `json:"own,omitempty"`
	Stated     bool            `json:"stated,omitempty"`
	Graph      *ActionForm     `json:"graph,omitempty"`
}

// actionForm writes a lowered action graph.
func (x *graphsExporter) actionForm(sym *symbols.Symbol, graph *lower.ActionGraph) (*ActionForm, error) {
	form, err := x.actionGraph(graph)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", symbols.FQNOf(sym), err)
	}
	form.Name = symbols.FQNOf(sym)
	form.Kind = sym.Kind.String()
	form.Parameters = x.parameters(sym)
	return form, nil
}

// parameters writes the signature the runtime invokes the action with.
func (x *graphsExporter) parameters(sym *symbols.Symbol) []ParameterForm {
	var out []ParameterForm
	for _, p := range x.sem.BehaviorParametersOf(sym) {
		if p.Symbol == nil || p.Symbol.Name == "" {
			continue
		}
		scope := p.Symbol.OwnerScope
		var value ast.Node
		if usage, ok := p.Symbol.Decl.(*ast.Usage); ok {
			value = usage.Value
		}
		types, unit := x.typing(scope, p.Symbol.Name, value)
		direction := ""
		if p.Direction != 0 {
			direction = p.Direction.String()
		}
		out = append(out, ParameterForm{
			Name:      p.Symbol.Name,
			Direction: direction,
			Result:    p.IsResult,
			Optional:  x.sem.OptionalParameter(p.Symbol),
			Types:     types,
			Unit:      unit,
			Value:     x.expr(scope, value),
			Span:      x.span(scope, p.Symbol.Decl),
			Scope:     scopeName(scope),
		})
	}
	return out
}

// actionGraph writes a graph without naming it, as a nested flow is written.
func (x *graphsExporter) actionGraph(graph *lower.ActionGraph) (*ActionForm, error) {
	ids := newVertexIDs()
	for _, node := range graph.Nodes {
		ids.add(node)
	}
	ids.add(graph.Initial)
	for _, node := range graph.Finals {
		ids.add(node)
	}
	var rest []ast.Node
	for node := range graph.Edges {
		rest = append(rest, node)
	}
	for node := range graph.DataFlows {
		rest = append(rest, node)
	}
	for node := range graph.Bodies {
		rest = append(rest, node)
	}
	for node := range graph.Features {
		rest = append(rest, node)
	}
	for node := range graph.Accepts {
		rest = append(rest, node)
	}
	for node := range graph.Subflows {
		rest = append(rest, node)
	}
	for node := range graph.StatementRuns {
		rest = append(rest, node)
	}
	for node := range graph.BlockNodes {
		rest = append(rest, node)
	}
	if err := ids.addSorted(rest, func(ast.Node) string { return "" }); err != nil {
		return nil, err
	}
	// Vertices reached only as an edge's end are numbered after the declared
	// ones, in the order the declared ones reach them.
	for i := 0; i < len(ids.order); i++ {
		node := ids.order[i]
		for _, edge := range graph.Edges[node] {
			ids.add(edge.Source)
			ids.add(edge.Target)
		}
		for _, flow := range graph.DataFlows[node] {
			ids.add(flow.Target)
		}
		for _, member := range graph.BlockNodes[node] {
			ids.add(member)
		}
	}
	for _, b := range graph.Bindings {
		ids.add(b.Node)
		ids.add(b.OtherNode)
	}
	vertices := len(ids.order)

	form := &ActionForm{Scope: scopeName(graph.Scope)}
	for _, attr := range graph.Attributes {
		form.Attributes = append(form.Attributes, x.attribute(graph.Scope, attr))
	}
	form.Initial = ids.ref(graph.Initial)
	for _, node := range graph.Finals {
		form.Finals = append(form.Finals, ids.add(node))
	}
	for id, node := range ids.order {
		scope := graph.Scopes[node]
		if scope == nil {
			scope = graph.Scope
		}
		nf, err := x.node(graph, ids, id, node, scope)
		if err != nil {
			return nil, err
		}
		form.Nodes = append(form.Nodes, nf)
		for _, edge := range graph.Edges[node] {
			decl, _ := edge.Decl.(*ast.ControlFlowEdge)
			form.Edges = append(form.Edges, EdgeForm{
				Source: ids.add(edge.Source),
				Target: ids.add(edge.Target),
				Guard:  x.expr(scope, edge.Guard),
				Else:   decl != nil && decl.IsElse,
				Decl:   x.span(scope, edge.Decl),
			})
		}
		for _, flow := range graph.DataFlows[node] {
			form.Flows = append(form.Flows, ObjectFlowForm{
				Name:      flow.Name,
				Source:    id,
				SourcePin: flow.SourcePin,
				Target:    ids.add(flow.Target),
				TargetPin: flow.TargetPin,
				Decl:      x.span(scope, flow.Decl),
			})
		}
	}
	for _, b := range graph.Bindings {
		scope := b.Scope
		if scope == nil {
			scope = graph.Scope
		}
		bf := PinBindingForm{
			Node:         ids.ref(b.Node),
			Path:         vertexNames(b.Path),
			Pin:          b.Pin,
			Other:        x.expr(scope, b.Other),
			OtherNode:    ids.ref(b.OtherNode),
			OtherPath:    vertexNames(b.OtherPath),
			OtherPin:     b.OtherPin,
			OtherChain:   x.chain(scope, b.OtherChain),
			OtherFeature: b.OtherFeature,
			Scope:        scopeName(b.Scope),
			FromValue:    b.FromValue,
		}
		if b.Decl != nil {
			bf.Decl = x.span(scope, b.Decl)
		}
		form.Bindings = append(form.Bindings, bf)
	}
	if len(ids.order) != vertices {
		return nil, fmt.Errorf("%w: a vertex is reached only while it is written", ErrGraphsOrder)
	}
	form.Connections = connections(graph.Connections)
	return form, nil
}

// node writes one vertex with its features, body, accept and nested flow.
func (x *graphsExporter) node(graph *lower.ActionGraph, ids *vertexIDs, id int, node ast.Node, scope *symbols.Scope) (NodeForm, error) {
	nf := NodeForm{
		ID:           id,
		Kind:         vertexKind(node),
		Name:         vertexName(node),
		Span:         x.span(graph.Scope, node),
		Scope:        scopeName(graph.Scopes[node]),
		StatementRun: graph.StatementRuns[node],
		Performs:     x.perform(scope, node),
	}
	for _, f := range graph.Features[node] {
		nf.Features = append(nf.Features, x.feature(scope, f))
	}
	body, err := x.statements(scope, graph.Bodies[node])
	if err != nil {
		return nf, err
	}
	nf.Body = body
	for _, member := range graph.BlockNodes[node] {
		nf.Block = append(nf.Block, ids.add(member))
	}
	if accept, ok := graph.Accepts[node]; ok {
		nf.Accept = x.accept(scope, accept)
	}
	if sub, ok := graph.Subflows[node]; ok && sub != nil {
		sf := &SubflowForm{}
		if sub.Err != nil {
			sf.Error = sub.Err.Error()
		}
		if sub.Graph != nil {
			g, err := x.actionGraph(sub.Graph)
			if err != nil {
				return nf, err
			}
			sf.Graph = g
		}
		nf.Subflow = sf
	}
	return nf, nil
}

// statements writes a body in order.
func (x *graphsExporter) statements(scope *symbols.Scope, body []lower.Statement) ([]StatementForm, error) {
	var out []StatementForm
	for _, s := range body {
		sf, err := x.statement(scope, s)
		if err != nil {
			return nil, err
		}
		out = append(out, sf)
	}
	return out, nil
}

// statement writes one lowered statement by its kind.
func (x *graphsExporter) statement(enclosing *symbols.Scope, s lower.Statement) (StatementForm, error) {
	switch s := s.(type) {
	case lower.Send:
		scope := orScope(s.Scope, enclosing)
		return StatementForm{
			Kind:         "send",
			Span:         x.span(scope, s.Message),
			Scope:        scopeName(s.Scope),
			Message:      x.expr(scope, s.Message),
			Target:       s.Target,
			TargetSymbol: symbols.FQNOf(s.TargetSym),
			TargetPath:   s.TargetPath,
			Via:          s.IsVia,
			Receiver:     s.Receiver,
			ReceiverPath: s.ReceiverPath,
		}, nil
	case lower.Assign:
		scope := orScope(s.Scope, enclosing)
		return StatementForm{
			Kind:  "assign",
			Span:  x.span(scope, s.Node),
			Scope: scopeName(s.Scope),
			Name:  s.Target,
			Chain: x.chain(scope, s.Chain),
			Value: x.expr(scope, s.Value),
		}, nil
	case lower.Declare:
		scope := orScope(s.Scope, enclosing)
		return StatementForm{
			Kind:  "declare",
			Span:  x.span(scope, s.Node),
			Scope: scopeName(s.Scope),
			Name:  s.Name,
			Value: x.expr(scope, s.Value),
		}, nil
	case lower.DeclareUsage:
		scope := orScope(s.Scope, enclosing)
		var span SpanForm
		if s.Node != nil {
			span = x.span(scope, s.Node)
		}
		return StatementForm{Kind: "declare usage", Span: span, Scope: scopeName(s.Scope), Name: s.Name}, nil
	case lower.Block:
		scope := orScope(s.Scope, enclosing)
		body, err := x.block(scope, s)
		if err != nil {
			return StatementForm{}, err
		}
		return StatementForm{Kind: "block", Span: x.span(scope, s.Node), Scope: scopeName(s.Scope), Body: body}, nil
	case lower.Loop:
		scope := orScope(s.Scope, enclosing)
		body, err := x.block(scope, s.Body)
		if err != nil {
			return StatementForm{}, err
		}
		return StatementForm{
			Kind:       "loop",
			Span:       x.span(scope, s.Node),
			Scope:      scopeName(s.Scope),
			Loop:       s.Kind.String(),
			Condition:  x.expr(scope, s.Condition),
			Until:      x.expr(scope, s.Until),
			Variable:   s.Variable,
			Collection: x.expr(scope, s.Collection),
			Body:       body,
		}, nil
	case lower.If:
		scope := orScope(s.Scope, enclosing)
		then, err := x.block(scope, s.Then)
		if err != nil {
			return StatementForm{}, err
		}
		sf := StatementForm{
			Kind:      "if",
			Span:      x.span(scope, s.Node),
			Scope:     scopeName(s.Scope),
			Condition: x.expr(scope, s.Condition),
			Then:      then,
		}
		if s.Else != nil {
			sf.Else, err = x.block(scope, *s.Else)
			if err != nil {
				return StatementForm{}, err
			}
		}
		return sf, nil
	case lower.Return:
		scope := orScope(s.Scope, enclosing)
		return StatementForm{
			Kind:  "return",
			Span:  x.span(scope, s.Node),
			Scope: scopeName(s.Scope),
			Value: x.expr(scope, s.Value),
		}, nil
	case lower.Effect:
		scope := orScope(s.Scope, enclosing)
		return StatementForm{
			Kind:     "effect",
			Span:     x.span(scope, s.Node),
			Scope:    scopeName(s.Scope),
			Effect:   s.Kind.String(),
			Performs: x.perform(scope, s.Node),
		}, nil
	case lower.Unsupported:
		scope := orScope(s.Scope, enclosing)
		return StatementForm{
			Kind:        "unsupported",
			Span:        x.span(scope, s.Node),
			Scope:       scopeName(s.Scope),
			Description: s.Description,
		}, nil
	}
	return StatementForm{}, fmt.Errorf("graphs: statement %T is not exported", s)
}

// block writes a block and, when it states a flow of its own, that flow's graph.
func (x *graphsExporter) block(enclosing *symbols.Scope, b lower.Block) (*BlockForm, error) {
	scope := orScope(b.Scope, enclosing)
	statements, err := x.statements(scope, b.Statements)
	if err != nil {
		return nil, err
	}
	form := &BlockForm{Statements: statements, Scope: scopeName(b.Scope), Own: b.Own, Stated: b.Stated}
	if form.Statements == nil {
		form.Statements = []StatementForm{}
	}
	if b.Graph != nil {
		form.Graph, err = x.actionGraph(b.Graph)
		if err != nil {
			return nil, err
		}
	}
	return form, nil
}

// accept writes what an accept node waits for.
func (x *graphsExporter) accept(enclosing *symbols.Scope, a lower.Accept) *AcceptForm {
	scope := orScope(a.Scope, enclosing)
	return &AcceptForm{
		Param:        a.ParamName,
		SignalType:   qualifiedName(a.SignalType),
		ViaPort:      a.ViaPort,
		SubsetsEvent: x.expr(scope, a.SubsetsEvent),
		Trigger:      x.trigger(scope, a.Trigger),
		Scope:        scopeName(a.Scope),
	}
}

// trigger writes a trigger event by the class the lowering gave it.
func (x *graphsExporter) trigger(scope *symbols.Scope, node ast.Node) *TriggerForm {
	if nilNode(node) {
		return nil
	}
	e := x.expr(scope, node)
	form := &TriggerForm{Text: e.Text, Span: e.Span}
	switch t := node.(type) {
	case *ast.AcceptEvent:
		form.Kind = "accept"
		form.SignalType = qualifiedName(t.SignalType)
		form.Subsets = qualifiedName(t.Subsets)
		if t.Payload != nil {
			form.Payload, _ = ast.EffectiveName(t.Payload)
		}
	case *ast.CallEvent:
		form.Kind = "call"
		form.Operation = qualifiedName(t.Operation)
		for _, p := range t.Parameters {
			form.Parameters = append(form.Parameters, p.Text)
		}
	case *ast.ChangeEvent:
		form.Kind = "change"
		form.Condition = x.expr(scope, t.Condition)
	case *ast.TimeEvent:
		form.Kind = "time"
		form.Duration = x.expr(scope, t.Duration)
		form.Absolute = t.Absolute
	default:
		form.Kind = vertexKind(node)
	}
	return form
}

// attribute writes an attribute with the types and unit the semantic model derives.
func (x *graphsExporter) attribute(enclosing *symbols.Scope, a lower.Attribute) AttributeForm {
	scope := orScope(a.Scope, enclosing)
	types, unit := x.typing(scope, a.Name, a.Value)
	return AttributeForm{
		Name:  a.Name,
		Types: types,
		Unit:  unit,
		Value: x.expr(scope, a.Value),
		Span:  x.span(scope, a.Node),
		Scope: scopeName(a.Scope),
	}
}

// feature writes a node's own parameter or attribute.
func (x *graphsExporter) feature(enclosing *symbols.Scope, f lower.Feature) FeatureForm {
	scope := orScope(f.Scope, enclosing)
	types, unit := x.typing(scope, f.Name, f.Value)
	direction := ""
	if f.Direction != 0 {
		direction = f.Direction.String()
	}
	return FeatureForm{
		Name:      f.Name,
		Direction: direction,
		Result:    f.IsResult,
		Types:     types,
		Unit:      unit,
		Value:     x.expr(scope, f.Value),
		Span:      x.span(scope, f.Node),
		Scope:     scopeName(f.Scope),
	}
}

// typing is the declared types of the feature named in scope, by qualified name,
// and the unit its value carries; each empty where the semantic model derives none.
func (x *graphsExporter) typing(scope *symbols.Scope, name string, value ast.Node) ([]string, string) {
	var types []string
	if scope != nil && name != "" {
		if sym, ok := scope.LookupLocal(name); ok {
			for _, t := range x.sem.FeatureTypes(sym) {
				types = append(types, symbols.FQNOf(t))
			}
		}
	}
	unit := ""
	if value != nil {
		if u, err := x.sem.UnitOfExpr(scope, value); err == nil {
			unit = u.String()
		}
	}
	return types, unit
}

// chain writes a feature chain; nil for none.
func (x *graphsExporter) chain(scope *symbols.Scope, c *lower.AssignTarget) *ChainForm {
	if c == nil {
		return nil
	}
	return &ChainForm{Base: x.expr(scope, c.Base), Steps: c.Steps, Text: c.Text}
}

// span locates a node written in scope; zero for nil.
func (x *graphsExporter) span(scope *symbols.Scope, node ast.Node) SpanForm {
	if nilNode(node) {
		return SpanForm{}
	}
	s := node.Span()
	return SpanForm{Document: docOf(scope), Offset: s.Offset, Len: s.Len}
}

// connections writes the connectors a behavior routes through.
func connections(conns []lower.Connection) []ConnectionForm {
	var out []ConnectionForm
	for _, c := range conns {
		owner := "behavior"
		if c.Owner == lower.OwnerObject {
			owner = "object"
		}
		ends := c.Ends
		if ends == nil {
			ends = []string{}
		}
		out = append(out, ConnectionForm{Ends: ends, Variation: c.Variation, Variant: c.Variant, Owner: owner, Scope: scopeName(c.Scope)})
	}
	return out
}

// vertexNames names the nodes of a path under a vertex.
func vertexNames(path []ast.Node) []string {
	var names []string
	for _, node := range path {
		names = append(names, vertexName(node))
	}
	return names
}

// orScope is scope, or enclosing when the lowering recorded none.
func orScope(scope, enclosing *symbols.Scope) *symbols.Scope {
	if scope != nil {
		return scope
	}
	return enclosing
}
