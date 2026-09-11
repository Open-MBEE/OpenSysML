package lower

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Footprints are the static projection of what one move may touch, from the names
// lowering resolves; every approximation errs towards dependence (see
// docs/internals/design/bounded-model-checking.md, "The independence relation").

// Place is one feature a move reads or writes: the declaration the name resolves to
// (nil when unresolved) and the name; Local marks a pin, held per performance.
type Place struct {
	Sym   *symbols.Symbol
	Name  string
	Local bool
}

// Conflicts reports whether the two places may name one value: same-named places
// do, except two distinct pins, which each performance holds its own of.
func (p Place) Conflicts(q Place) bool {
	if p.Name != q.Name {
		return false
	}
	if p.Local && q.Local && p.Sym != nil && q.Sym != nil && p.Sym != q.Sym {
		return false
	}
	return true
}

// String renders the place as the name it was written under.
func (p Place) String() string {
	if p.Local {
		return p.Name + " (pin)"
	}
	return p.Name
}

// Channel is one message a move sends or waits for: the signal type as written
// ("" for any) and the port it travels through ("" for none).
type Channel struct {
	Signal string
	Port   string
}

// String renders the channel as `Signal via Port`, dropping what is absent.
func (c Channel) String() string {
	text := c.Signal
	if text == "" {
		text = "*"
	}
	if c.Port != "" {
		text += " via " + c.Port
	}
	return text
}

// Footprint is what one atomic move (one token advancing one node) may touch. Control
// lists the joins and merges it arrives at; Dynamic marks an unresolved target.
type Footprint struct {
	Reads   []Place
	Writes  []Place
	Sends   []Channel
	Accepts []Channel
	Control []ast.Node
	Dynamic bool
}

// Dependent reports whether the two moves may not commute: a data race, both
// touching the bus (a send meeting an accept, two accepts competing for one message,
// two sends ordering it), convergence on one join or merge, or a dynamic target.
func (f Footprint) Dependent(g Footprint) bool {
	if f.Dynamic || g.Dynamic {
		return true
	}
	if placesMeet(f.Writes, g.Reads) || placesMeet(f.Writes, g.Writes) || placesMeet(g.Writes, f.Reads) {
		return true
	}
	if f.messages() && g.messages() {
		return true
	}
	for _, node := range f.Control {
		for _, other := range g.Control {
			if node == other {
				return true
			}
		}
	}
	return false
}

// messages reports whether the move sends or accepts a message.
func (f Footprint) messages() bool {
	return len(f.Sends) > 0 || len(f.Accepts) > 0
}

func placesMeet(as, bs []Place) bool {
	for _, a := range as {
		for _, b := range bs {
			if a.Conflicts(b) {
				return true
			}
		}
	}
	return false
}

// String renders the footprint one clause per line, names sorted, for tests and
// the checker's report.
func (f Footprint) String() string {
	var b strings.Builder
	writePlaces := func(label string, places []Place) {
		if len(places) == 0 {
			return
		}
		names := make([]string, 0, len(places))
		for _, p := range places {
			names = append(names, p.String())
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "%s: %s\n", label, strings.Join(names, ", "))
	}
	writeChannels := func(label string, channels []Channel) {
		if len(channels) == 0 {
			return
		}
		names := make([]string, 0, len(channels))
		for _, c := range channels {
			names = append(names, c.String())
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "%s: %s\n", label, strings.Join(names, ", "))
	}
	writePlaces("reads", f.Reads)
	writePlaces("writes", f.Writes)
	writeChannels("sends", f.Sends)
	writeChannels("accepts", f.Accepts)
	if len(f.Control) > 0 {
		names := make([]string, 0, len(f.Control))
		for _, node := range f.Control {
			names = append(names, controlNodeName(node))
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "control: %s\n", strings.Join(names, ", "))
	}
	if f.Dynamic {
		b.WriteString("dynamic\n")
	}
	return b.String()
}

func controlNodeName(node ast.Node) string {
	if name := getNodeName(node); name != "" {
		return name
	}
	return fmt.Sprintf("%T", node)
}

// lowerFootprints computes the footprint of every node of the graph once its
// nodes, edges, bodies, flows and bindings are lowered.
func lowerFootprints(graph *ActionGraph) {
	graph.Footprints = make(map[ast.Node]Footprint, len(graph.Nodes))
	for _, node := range graph.Nodes {
		graph.Footprints[node] = footprintOf(graph, node)
	}
}

// footprintOf projects the footprint of one node from the graph's side tables.
func footprintOf(graph *ActionGraph, node ast.Node) Footprint {
	b := &footprintBuilder{graph: graph, node: node, scope: nodeScopeOf(graph, node)}
	if nodePerformsAction(node) {
		// The performed action runs in an executor of its own; what it touches is its.
		b.footprint.Dynamic = true
	}
	b.statements(graph.Bodies[node])
	b.features()
	b.bindings()
	b.flows()
	b.accept()
	b.successions()
	b.control()
	return b.footprint
}

// nodePerformsAction reports whether a node performs an action declared elsewhere,
// which runs in an executor of its own.
func nodePerformsAction(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.PerformActionNode, *ast.ActionExecutionNode:
		return true
	case *ast.Usage:
		return performsAction(n)
	}
	return false
}

// nodeScopeOf returns the scope a node's own expressions resolve in: the
// namespace it owns, else the graph's.
func nodeScopeOf(graph *ActionGraph, node ast.Node) *symbols.Scope {
	if scope := graph.Scopes[node]; scope != nil {
		return scope
	}
	return graph.Scope
}

type footprintBuilder struct {
	graph     *ActionGraph
	node      ast.Node
	scope     *symbols.Scope
	footprint Footprint
}

func (b *footprintBuilder) read(p Place) {
	b.footprint.Reads = appendPlace(b.footprint.Reads, p)
}

func (b *footprintBuilder) write(p Place) {
	b.footprint.Writes = appendPlace(b.footprint.Writes, p)
}

func appendPlace(places []Place, p Place) []Place {
	if p.Name == "" {
		return places
	}
	for _, have := range places {
		if have == p {
			return places
		}
	}
	return append(places, p)
}

// place resolves a written name to its place, and to the names its declaration
// redefines, so a redefining feature and the redefined one meet under either name.
func (b *footprintBuilder) place(scope *symbols.Scope, segments []string, each func(Place)) {
	if len(segments) == 0 {
		return
	}
	name := segments[len(segments)-1]
	sym, _ := resolve.FeatureSymbolInScope(scope, segments)
	local := sym != nil && b.declaredByNode(sym)
	each(Place{Sym: sym, Name: name, Local: local})
	if sym == nil {
		return
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		for _, redefined := range redefinedNames(usage) {
			if redefined != name {
				each(Place{Sym: sym, Name: redefined, Local: local})
			}
		}
	}
}

// declaredByNode reports whether sym is a feature some node of the graph declares.
func (b *footprintBuilder) declaredByNode(sym *symbols.Symbol) bool {
	for _, features := range b.graph.Features {
		for _, f := range features {
			if f.Node == sym.Decl {
				return true
			}
		}
	}
	return false
}

// reads adds every feature an expression reads. A chain from a computed base and
// an invocation of a behavior the model declares are dynamic.
func (b *footprintBuilder) reads(scope *symbols.Scope, expr ast.Node) {
	switch e := expr.(type) {
	case nil:
	case *ast.LiteralBool, *ast.LiteralString, *ast.LiteralInteger, *ast.LiteralReal,
		*ast.LiteralInfinity, *ast.NullExpr, *ast.MetadataAccessExpr, *ast.CastExpr:
	case *ast.FeatureReference:
		b.readName(scope, e.Name)
	case *ast.QualifiedName:
		b.readName(scope, e)
	case *ast.FeatureChainExpr:
		base, segments := flattenChain(e)
		if ref, ok := base.(*ast.FeatureReference); ok && ref.Name != nil && len(ref.Name.Parts) == 1 && len(segments) > 0 {
			b.place(scope, append([]string{ref.Name.Parts[0].Text}, segments...), b.read)
			b.readName(scope, ref.Name)
			return
		}
		b.footprint.Dynamic = true
		b.reads(scope, base)
	case *ast.OperatorExpr:
		for _, operand := range e.Operands {
			b.reads(scope, operand)
		}
	case *ast.IndexExpr:
		b.reads(scope, e.Operand)
		b.reads(scope, e.Index)
	case *ast.SequenceExpr:
		for _, element := range e.Elements {
			b.reads(scope, element)
		}
	case *ast.CollectExpr:
		b.reads(scope, e.Operand)
		b.reads(scope, e.Body)
	case *ast.SelectExpr:
		b.reads(scope, e.Operand)
		b.reads(scope, e.Body)
	case *ast.ConstructorExpr:
		for _, arg := range e.Args {
			b.reads(scope, arg)
		}
		for _, arg := range e.NamedArgs {
			b.reads(scope, arg.Value)
		}
	case *ast.InvocationExpr:
		b.reads(scope, e.Operand)
		for _, arg := range e.Args {
			b.reads(scope, arg)
		}
		for _, arg := range e.NamedArgs {
			b.reads(scope, arg.Value)
		}
		if b.invokesDeclaredBehavior(scope, e.Type) {
			b.footprint.Dynamic = true
		}
	case *ast.BodyExpr:
		for _, param := range e.Params {
			b.reads(scope, param.Value)
		}
		for _, member := range e.Members {
			if u, ok := unwrapMembership(member).(*ast.Usage); ok {
				b.reads(scope, u.Value)
			}
		}
		b.reads(scope, e.Result)
	default:
		b.footprint.Dynamic = true
	}
}

// readName adds the read of a written name: a plain name as the place it resolves
// to, a qualified name by its last segment.
func (b *footprintBuilder) readName(scope *symbols.Scope, qn *ast.QualifiedName) {
	if qn == nil || len(qn.Parts) == 0 {
		return
	}
	if len(qn.Parts) == 1 {
		b.place(scope, []string{qn.Parts[0].Text}, b.read)
		return
	}
	b.read(Place{Name: qn.Parts[len(qn.Parts)-1].Text})
}

// invokesDeclaredBehavior reports whether an invocation names a calc or action the
// model declares, whose body reads beyond its arguments.
func (b *footprintBuilder) invokesDeclaredBehavior(scope *symbols.Scope, qn *ast.QualifiedName) bool {
	decl, _, ok := resolve.TypeDeclInScope(scope, qn)
	if !ok {
		return false
	}
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefCalc || d.Kind == ast.DefAction
	case *ast.Usage:
		return d.Kind == ast.UsageCalc || d.Kind == ast.UsageAction
	}
	return false
}

// statements adds what a body's statements touch, blocks and their own flows included.
func (b *footprintBuilder) statements(stmts []Statement) {
	for _, stmt := range stmts {
		b.statement(stmt)
	}
}

func (b *footprintBuilder) statement(stmt Statement) {
	switch s := stmt.(type) {
	case Assign:
		b.reads(s.Scope, s.Value)
		if s.Chain == nil {
			b.place(s.Scope, []string{s.Target}, b.write)
			return
		}
		ref, ok := s.Chain.Base.(*ast.FeatureReference)
		if !ok || ref.Name == nil || len(ref.Name.Parts) != 1 {
			b.footprint.Dynamic = true
			b.reads(s.Scope, s.Chain.Base)
			return
		}
		segments := append([]string{ref.Name.Parts[0].Text}, s.Chain.Steps...)
		b.readName(s.Scope, ref.Name)
		b.place(s.Scope, append(segments, s.Target), b.write)
	case Send:
		b.reads(s.Scope, s.Message)
		if s.IsVia {
			// A `via` send is routed by connections at run time.
			b.footprint.Dynamic = true
		}
		b.footprint.Sends = append(b.footprint.Sends, Channel{Port: viaPortOf(s)})
	case Declare:
		b.reads(s.Scope, s.Value)
		b.write(Place{Name: s.Name})
	case DeclareUsage:
		b.footprint.Dynamic = true
	case Block:
		b.block(s)
	case Loop:
		b.reads(s.Body.Scope, s.Condition)
		b.reads(s.Body.Scope, s.Until)
		b.reads(s.Scope, s.Collection)
		if s.Variable != "" {
			b.write(Place{Name: s.Variable})
		}
		b.block(s.Body)
	case If:
		b.reads(s.Scope, s.Condition)
		b.block(s.Then)
		if s.Else != nil {
			b.block(*s.Else)
		}
	case Return:
		b.reads(s.Scope, s.Value)
		for _, f := range b.graph.Features[b.node] {
			if f.IsResult || f.Direction == ast.DirOut {
				b.write(Place{Sym: featureSymbol(b.scope, f), Name: f.Name, Local: true})
			}
		}
	case Effect:
		// Performing, accepting or terminating inside a body reaches beyond it.
		b.footprint.Dynamic = true
	case Unsupported:
		// Reaching it fails the run; it reads nothing first.
	}
}

// block adds a block's steps: its statements, or the bodies of its own flow's
// nodes, which run within the enclosing node's atomic step.
func (b *footprintBuilder) block(block Block) {
	if block.Graph == nil {
		b.statements(block.Statements)
		return
	}
	for _, node := range block.Graph.Nodes {
		nested, ok := block.Graph.Footprints[node]
		if !ok {
			nested = footprintOf(block.Graph, node)
		}
		if !block.Graph.StatementRuns[node] {
			// A nested action or a perform inside a block runs a behavior of its own.
			nested.Dynamic = true
		}
		b.merge(nested)
	}
}

func (b *footprintBuilder) merge(other Footprint) {
	for _, p := range other.Reads {
		b.read(p)
	}
	for _, p := range other.Writes {
		b.write(p)
	}
	b.footprint.Sends = append(b.footprint.Sends, other.Sends...)
	b.footprint.Accepts = append(b.footprint.Accepts, other.Accepts...)
	b.footprint.Control = append(b.footprint.Control, other.Control...)
	b.footprint.Dynamic = b.footprint.Dynamic || other.Dynamic
}

func viaPortOf(s Send) string {
	if s.IsVia {
		return s.Target
	}
	return ""
}

// features adds the node's own pins, written as the performance starts; a node
// performing an action also reads unconnected inputs from, and returns outputs
// to, the enclosing feature of the pin's name.
func (b *footprintBuilder) features() {
	performs := nodePerformsAction(b.node)
	for _, f := range b.graph.Features[b.node] {
		b.reads(f.Scope, f.Value)
		b.write(Place{Sym: featureSymbol(b.scope, f), Name: f.Name, Local: true})
		if !performs {
			continue
		}
		in := f.Direction == ast.DirIn || f.Direction == ast.DirInOut
		out := f.Direction == ast.DirOut || f.Direction == ast.DirInOut || f.IsResult
		if in && f.Value == nil && !b.pinConnected(f.Name) {
			b.place(b.graph.Scope, []string{f.Name}, b.read)
		}
		if out {
			b.place(b.graph.Scope, []string{f.Name}, b.write)
		}
	}
}

// featureSymbol returns the symbol of a node's own feature, nil when the scope
// tree does not hold it.
func featureSymbol(scope *symbols.Scope, f Feature) *symbols.Symbol {
	if scope == nil {
		return nil
	}
	if sym, ok := scope.LookupLocal(f.Name); ok && sym.Decl == f.Node {
		return sym
	}
	return nil
}

// pinConnected reports whether a binding or a flow supplies the named pin of the node.
func (b *footprintBuilder) pinConnected(pin string) bool {
	for _, binding := range b.graph.Bindings {
		if binding.Node == b.node && len(binding.Path) == 0 && binding.Pin == pin {
			return true
		}
		if binding.OtherNode == b.node && len(binding.OtherPath) == 0 && binding.OtherPin == pin {
			return true
		}
	}
	for _, flows := range b.graph.DataFlows {
		for _, flow := range flows {
			if flow.Target == b.node && flow.TargetPin == pin {
				return true
			}
		}
	}
	return false
}

// bindings adds the other end of every binding at one of the node's pins: a value
// flows both ways through a binding, so the end is both read and written.
func (b *footprintBuilder) bindings() {
	for _, binding := range b.graph.Bindings {
		switch {
		case binding.Node == b.node:
			b.bindingEnd(binding, binding.OtherNode, binding.OtherPath, binding.OtherPin, binding.Other, binding.OtherChain, binding.OtherFeature)
		case binding.OtherNode == b.node:
			b.bindingEnd(binding, binding.Node, binding.Path, binding.Pin, nil, nil, "")
		}
	}
}

func (b *footprintBuilder) bindingEnd(binding PinBinding, node ast.Node, path []ast.Node, pin string, other ast.Node, chain *AssignTarget, feature string) {
	if node != nil {
		if len(path) > 0 {
			// A pin under a nested flow belongs to a performance only that flow reaches.
			b.footprint.Dynamic = true
			return
		}
		place := Place{Name: pin, Local: true}
		if scope := b.graph.Scopes[node]; scope != nil {
			if sym, ok := scope.LookupLocal(pin); ok {
				place.Sym = sym
			}
		}
		b.read(place)
		b.write(place)
		return
	}
	if chain != nil {
		ref, ok := chain.Base.(*ast.FeatureReference)
		if !ok || ref.Name == nil || len(ref.Name.Parts) != 1 {
			b.footprint.Dynamic = true
			return
		}
		segments := append([]string{ref.Name.Parts[0].Text}, chain.Steps...)
		b.readName(binding.Scope, ref.Name)
		b.place(binding.Scope, append(segments, feature), b.read)
		b.place(binding.Scope, append(segments, feature), b.write)
		return
	}
	segments := endSegments(other)
	if len(segments) == 0 {
		b.reads(binding.Scope, other)
		return
	}
	b.place(binding.Scope, segments, b.read)
	b.place(binding.Scope, segments, b.write)
}

// flows adds the pins the node's outputs are delivered to when it completes.
func (b *footprintBuilder) flows() {
	for _, flow := range b.graph.DataFlows[b.node] {
		place := Place{Name: flow.TargetPin, Local: true}
		if scope := b.graph.Scopes[flow.Target]; scope != nil {
			if sym, ok := scope.LookupLocal(flow.TargetPin); ok {
				place.Sym = sym
			}
		}
		b.write(place)
	}
}

// accept adds the message the node waits for and the features its trigger
// condition reads, which answer the wait of a parked token.
func (b *footprintBuilder) accept() {
	accept, ok := b.graph.Accepts[b.node]
	if !ok {
		return
	}
	channel := Channel{Port: accept.ViaPort}
	if accept.SignalType != nil {
		channel.Signal = qualifiedNameText(accept.SignalType)
	}
	if accept.ParamName != "" {
		b.write(Place{Sym: acceptParamSymbol(b.scope, accept.ParamName), Name: accept.ParamName, Local: true})
	}
	switch t := accept.Trigger.(type) {
	case nil:
		b.footprint.Accepts = append(b.footprint.Accepts, channel)
		b.reads(accept.Scope, accept.SubsetsEvent)
	case *ast.ChangeEvent:
		b.reads(accept.Scope, t.Condition)
	case *ast.TimeEvent:
		b.reads(accept.Scope, t.Duration)
	default:
		b.footprint.Dynamic = true
	}
}

func acceptParamSymbol(scope *symbols.Scope, name string) *symbols.Symbol {
	if scope == nil {
		return nil
	}
	if sym, ok := scope.LookupLocal(name); ok {
		return sym
	}
	return nil
}

func qualifiedNameText(qn *ast.QualifiedName) string {
	parts := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "::")
}

// successions adds the features the guards of the node's outgoing successions
// read: advancing the token evaluates them to pick the successor.
func (b *footprintBuilder) successions() {
	for _, edge := range b.graph.Edges[b.node] {
		b.reads(b.graph.Scope, edge.Guard)
	}
}

// control adds the join and merge nodes the move touches: the node itself when it
// is one, and every one its successions arrive at.
func (b *footprintBuilder) control() {
	if isConvergence(b.node) {
		b.footprint.Control = append(b.footprint.Control, b.node)
	}
	for _, edge := range b.graph.Edges[b.node] {
		if isConvergence(edge.Target) {
			b.footprint.Control = append(b.footprint.Control, edge.Target)
		}
	}
}

func isConvergence(node ast.Node) bool {
	switch node.(type) {
	case *ast.JoinNode, *ast.MergeNode:
		return true
	}
	return false
}
