package lower

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Footprints are the static projection of what one move may touch, from the names
// lowering resolves; every approximation errs towards dependence (see
// docs/internals/design/bounded-model-checking.md, "The independence relation").

// Place is one feature a move reads or writes: the declaration the name resolves to
// (nil when unresolved) and the name; Local marks a pin, held per performance.
// State marks the activity of a state, which a transition reads and writes.
// Direct marks a name written alone, not reached through a chain or qualified.
type Place struct {
	Sym    *symbols.Symbol
	Name   string
	Local  bool
	State  *ast.StateNode
	Direct bool
	// Node is the node whose pin the place is, each performance of a node holding
	// its own; nil for a feature or a declaration's own value.
	Node ast.Node
}

// Conflicts reports whether the two places may name one value: places resolving
// to one declaration do under any of its names (a short name, a redefined name),
// as do same-named places, except two distinct pins, which each performance
// holds its own of; the activity of a state meets only itself.
func (p Place) Conflicts(q Place) bool {
	if p.State != nil || q.State != nil {
		return p.State == q.State
	}
	if p.Node != nil && q.Node != nil && p.Node != q.Node {
		return false
	}
	if p.Sym != nil && q.Sym != nil {
		if p.Sym == q.Sym {
			return true
		}
		if p.Local || q.Local {
			return false
		}
	}
	return p.Name == q.Name
}

// String renders the place as the name it was written under.
func (p Place) String() string {
	if p.State != nil {
		return "active " + p.Name
	}
	if p.Local {
		return p.Name + " (pin)"
	}
	return p.Name
}

// Channel is one message a move sends or waits for: the signal type as written
// ("" for any) and the port it travels through ("" for none). Type is the accept's
// type reference and Message the send's message, each resolving in Scope; Via marks
// a send routed by connections, Drops a machine's dispatch, which may discard a
// message sent to its object. Own marks a send naming no target, for the sending object.
type Channel struct {
	Signal  string
	Port    string
	Type    *ast.QualifiedName
	Message ast.Node
	Scope   *symbols.Scope
	Via     bool
	Drops   bool
	Own     bool
}

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

// Footprint is what one atomic move (one token advancing one node, one transition
// firing) may touch. Control lists the joins and merges it arrives at; Completion
// marks a move that may queue a completion event; Dynamic an unresolved target.
// Creates marks a move that may create an object starting no behavior; Routes one
// sending through connections, which reads whatever the objects' structure holds.
type Footprint struct {
	Reads      []Place
	Writes     []Place
	Sends      []Channel
	Accepts    []Channel
	Control    []ast.Node
	Completion bool
	Dynamic    bool
	Creates    bool
	Routes     bool
}

// Dependent reports whether the two moves may not commute: a data race, both
// touching the bus (a send meeting an accept, two accepts competing for one message,
// two sends ordering it), both queuing a completion, convergence on one join or
// merge, creation meeting creation or an extent, routing meeting a structural
// write, or a dynamic target.
func (f Footprint) Dependent(g Footprint) bool {
	return f.DependentBy(g, Relation{})
}

// ChannelsMeet reports whether a message operation of one move may not commute
// with one of another: each channel with whether it sends.
type ChannelsMeet func(a Channel, aSends bool, b Channel, bSends bool) bool

// Relation is what is known of the executors two moves belong to. Channels decides
// which message operations meet, nil having every two meet. Apart marks two
// executors, each converging on control nodes of its own; Objects two run for two
// objects, each holding its own pins and state activity, and Held, where set, reports
// whether a direct place of the first move's and one of the second's are slots of different objects.
type Relation struct {
	Channels ChannelsMeet
	Apart    bool
	Objects  bool
	Held     func(p, q Place) bool
}

// DependentBy is Dependent under what rel knows of the moves' executors.
func (f Footprint) DependentBy(g Footprint, rel Relation) bool {
	if f.Dynamic || g.Dynamic {
		return true
	}
	if rel.placesMeet(f.Writes, g.Reads) || rel.placesMeet(f.Writes, g.Writes) || rel.placesMeet(f.Reads, g.Writes) {
		return true
	}
	if f.messagesMeet(g, rel.Channels) {
		return true
	}
	if f.Completion && g.Completion {
		return true
	}
	if f.Creates && (g.Creates || readsExtent(g.Reads)) || g.Creates && readsExtent(f.Reads) {
		return true
	}
	if f.Routes && rel.writesStructure(g.Writes) || g.Routes && rel.writesStructure(f.Writes) {
		return true
	}
	if rel.Apart {
		return false
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

// placesMeet reports whether a place of the first move's, among as, may name the
// value of one of the second's, among bs.
func (rel Relation) placesMeet(as, bs []Place) bool {
	for _, a := range as {
		for _, b := range bs {
			if a.Conflicts(b) && !rel.held(a, b) {
				return true
			}
		}
	}
	return false
}

// PlaceMeets reports whether p, a place of the first move's, may name the value
// of one of the second's places among qs.
func (rel Relation) PlaceMeets(p Place, qs []Place) bool {
	return rel.placesMeet([]Place{p}, qs)
}

// held reports whether two conflicting places, the first move's and the
// second's, are nonetheless two values.
func (rel Relation) held(p, q Place) bool {
	if !rel.Objects {
		return false
	}
	if p.Local || q.Local || p.State != nil || q.State != nil {
		return true
	}
	return p.Direct && q.Direct && rel.Held != nil && rel.Held(p, q)
}

// readsExtent reports whether a read may be of what objects exist: of anything
// but a state's activity, a pin or a declared feature.
func readsExtent(reads []Place) bool {
	for _, p := range reads {
		if p.State == nil && !p.Local && (p.Sym == nil || !isFeatureSymbol(p.Sym)) {
			return true
		}
	}
	return false
}

// writesStructure reports whether a write may change what a connection reaches:
// any but of an attribute or a state's activity, or of another executor's pin.
func (rel Relation) writesStructure(writes []Place) bool {
	for _, p := range writes {
		if p.State != nil || p.Local && rel.Objects {
			continue
		}
		if p.Sym == nil || p.Sym.Kind != symbols.SymbolAttributeUsage {
			return true
		}
	}
	return false
}

// isFeatureSymbol reports whether the symbol is a usage, which holds values rather than classifying them.
func isFeatureSymbol(sym *symbols.Symbol) bool {
	_, usage := sym.Decl.(*ast.Usage)
	return usage
}

func (f Footprint) messagesMeet(g Footprint, meet ChannelsMeet) bool {
	if !f.messages() || !g.messages() {
		return false
	}
	if meet == nil {
		return true
	}
	return channelsMeet(f.Sends, true, g.Sends, true, meet) ||
		channelsMeet(f.Sends, true, g.Accepts, false, meet) ||
		channelsMeet(f.Accepts, false, g.Sends, true, meet) ||
		channelsMeet(f.Accepts, false, g.Accepts, false, meet)
}

func channelsMeet(as []Channel, aSends bool, bs []Channel, bSends bool, meet ChannelsMeet) bool {
	for _, a := range as {
		for _, b := range bs {
			if meet(a, aSends, b, bSends) {
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
	if f.Completion {
		b.WriteString("completion\n")
	}
	if f.Creates {
		b.WriteString("creates\n")
	}
	if f.Routes {
		b.WriteString("routes\n")
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

// Footprints is node → what advancing a token through the node may read, write,
// send, accept and converge on, for the model checker's independence relation.
// It is computed from the lowered graph on first use and shared by every caller.
func (g *ActionGraph) Footprints() map[ast.Node]Footprint {
	g.footprintsOnce.Do(func() {
		g.footprints = make(map[ast.Node]Footprint, len(g.Nodes))
		declared := declaredFeatures(g)
		for _, node := range g.Nodes {
			g.footprints[node] = footprintOf(g, node, declared)
		}
	})
	return g.footprints
}

// declaredFeatures is the declarations the graph's nodes hold as their own
// features, each to the node holding it.
func declaredFeatures(graph *ActionGraph) map[ast.Node]ast.Node {
	declared := make(map[ast.Node]ast.Node)
	for node, features := range graph.Features {
		for _, f := range features {
			declared[f.Node] = node
		}
	}
	return declared
}

// footprintOf projects the footprint of one node from the graph's side tables;
// declared is declaredFeatures(graph).
func footprintOf(graph *ActionGraph, node ast.Node, declared map[ast.Node]ast.Node) Footprint {
	b := &footprintBuilder{resolver: graph.resolver, graph: graph, node: node, scope: nodeScopeOf(graph, node), declared: declared}
	if nodePerformsAction(node) {
		// The performed action runs in an executor of its own; what it touches is its.
		b.footprint.Dynamic = true
	}
	b.statements(graph.Bodies[node])
	b.features()
	b.bindings()
	b.flows()
	b.streams()
	b.accept()
	b.successions()
	b.control()
	return b.footprint
}

// BindingFootprints is node → what binding the node's own features, pins and
// accepted payload touches, apart from the statements its body runs: what a
// performance's inputs are bound from and where its outputs go. Computed on
// first use, as Footprints is.
func (g *ActionGraph) BindingFootprints() map[ast.Node]Footprint {
	g.bindingFootprintsOnce.Do(func() {
		g.bindingFootprints = make(map[ast.Node]Footprint, len(g.Nodes))
		declared := declaredFeatures(g)
		for _, node := range g.Nodes {
			b := &footprintBuilder{resolver: g.resolver, graph: g, node: node, scope: nodeScopeOf(g, node), declared: declared}
			b.features()
			b.bindings()
			b.flows()
			b.streams()
			b.accept()
			g.bindingFootprints[node] = b.footprint
		}
	})
	return g.bindingFootprints
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
	// resolver reads a name the scope tree alone does not reach, nil for none.
	resolver *resolve.Resolver
	graph    *ActionGraph
	node     ast.Node
	scope    *symbols.Scope
	// declared is each node-held feature declaration to its node, and a body's
	// own declaration to nil: the places the footprint marks local.
	declared          map[ast.Node]ast.Node
	localDeclarations bool
	footprint         Footprint
	// expanding holds the calc usages whose bindings are being read, so a cyclic binding stops.
	expanding map[*ast.Usage]bool
}

// ConstraintBodyFootprint projects the reads and writes of one constraint
// performance, marking declarations in its body-local scopes as local.
func ConstraintBodyFootprint(scope *symbols.Scope, stmts []Statement) Footprint {
	declared := make(map[ast.Node]ast.Node)
	var collect func(*symbols.Scope)
	collect = func(current *symbols.Scope) {
		if current == nil || !current.BodyLocal() {
			return
		}
		for _, member := range current.AllMembers() {
			if member != nil && member.Decl != nil {
				declared[member.Decl] = nil
			}
		}
		for _, child := range current.Children() {
			collect(child)
		}
	}
	collect(scope)
	builder := &footprintBuilder{scope: scope, declared: declared, localDeclarations: true}
	builder.statements(stmts)
	return builder.footprint
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
// It returns the feature named, nil where the scope does not resolve it.
func (b *footprintBuilder) place(scope *symbols.Scope, segments []string, each func(Place)) *symbols.Symbol {
	if len(segments) == 0 {
		return nil
	}
	name := segments[len(segments)-1]
	sym, _ := resolve.FeatureSymbolInScope(scope, segments)
	local := sym != nil && b.declaredByNode(sym)
	direct := len(segments) == 1
	each(Place{Sym: sym, Name: name, Local: local, Direct: direct})
	if sym == nil {
		return nil
	}
	if b.graph != nil && sym.Decl != nil && slices.Contains(b.graph.Nodes, sym.Decl) {
		// A node named as a value is read by its outputs (`assign total := diff`).
		for _, p := range b.outputs(sym.Decl) {
			each(p)
		}
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		for _, redefined := range redefinedNames(usage) {
			if redefined != name {
				each(Place{Sym: sym, Name: redefined, Local: local, Direct: direct})
			}
		}
		b.calcUsage(sym, usage)
	}
	return sym
}

// readPlace adds a read of the feature segments name, and of what its value is
// computed from where an expression values it.
func (b *footprintBuilder) readPlace(scope *symbols.Scope, segments []string) {
	b.boundValue(b.place(scope, segments, b.read))
}

// boundValue adds what reading a feature valued by an expression (`attribute y = x + 1`)
// evaluates: the places its expression reads, through features so valued in turn,
// since the value is computed from them as it is read (KerML 1.1 §7.3.4.2, a
// FeatureValue that is not initial binds the feature to its expression's result).
// Only a value-holding usage is so read: a behavior usage's value is what it
// performs, read by its outputs, a calc usage's members by calcUsage; an initial
// or default value is written once, not computed on read, and adds nothing.
func (b *footprintBuilder) boundValue(sym *symbols.Symbol) {
	if sym == nil {
		return
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !valueIsBinding(usage.Value, usage.ValueIsInitial, usage.ValueIsDefault) {
		return
	}
	switch usage.Kind {
	case ast.UsagePart, ast.UsageAttribute, ast.UsageItem, ast.UsageOccurrence,
		ast.UsageIndividual, ast.UsageEnumeration:
	default:
		return
	}
	if b.expanding[usage] {
		return
	}
	if b.expanding == nil {
		b.expanding = make(map[*ast.Usage]bool)
	}
	b.expanding[usage] = true
	defer delete(b.expanding, usage)
	scope := sym.Scope
	if scope == nil {
		scope = sym.OwnerScope
	}
	if scope == nil {
		scope = b.scope
	}
	b.reads(scope, usage.Value)
}

// calcUsage adds what reading a calc usage evaluates: the values its members
// bind, and its body, which reads beyond them.
func (b *footprintBuilder) calcUsage(sym *symbols.Symbol, usage *ast.Usage) {
	switch usage.Kind {
	case ast.UsageCalc, ast.UsageAnalysisCase, ast.UsageVerificationCase:
	default:
		return
	}
	b.footprint.Dynamic = true
	if b.expanding[usage] {
		return
	}
	if b.expanding == nil {
		b.expanding = make(map[*ast.Usage]bool)
	}
	b.expanding[usage] = true
	defer delete(b.expanding, usage)
	scope := sym.Scope
	if scope == nil {
		scope = b.scope
	}
	for _, member := range usage.Members {
		if u, ok := unwrapMembership(member).(*ast.Usage); ok {
			b.reads(scope, u.Value)
		}
	}
}

// path adds a feature reached through a chain: each names the feature at its end,
// and every prefix is read, since a write to one changes what the chain reaches.
// It returns the feature at the chain's end, nil where the scope does not resolve it.
func (b *footprintBuilder) path(scope *symbols.Scope, segments []string, each func(Place)) *symbols.Symbol {
	for n := 1; n < len(segments); n++ {
		b.readPlace(scope, segments[:n])
	}
	return b.place(scope, segments, each)
}

// address adds what routing a send reads: every feature its target path leads
// through and the port or receiver it ends at, chained or qualified.
func (b *footprintBuilder) address(scope *symbols.Scope, target string, chained bool) {
	if target == "" {
		return
	}
	separator := "::"
	if chained {
		separator = "."
	}
	b.boundValue(b.path(scope, strings.Split(target, separator), b.read))
}

// declaredByNode reports whether sym is a feature some node of the graph declares.
func (b *footprintBuilder) declaredByNode(sym *symbols.Symbol) bool {
	_, declared := b.declared[sym.Decl]
	return declared
}

// reads adds every feature an expression reads. A chain from a computed base, an
// invocation of a behavior the model declares and a constructor are dynamic.
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
			b.boundValue(b.path(scope, append([]string{ref.Name.Parts[0].Text}, segments...), b.read))
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
		// Constructing an object extends its type's extent and may start classifier behaviors.
		if plainType(scope, e.Type) {
			b.footprint.Creates = true
		} else {
			b.footprint.Dynamic = true
		}
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
		if sym, found := resolve.FeatureSymbolInScope(scope, []string{qn.Parts[0].Text}); found || b.resolver == nil {
			b.readPlace(scope, []string{qn.Parts[0].Text})
			if found && b.fallsThrough(sym) {
				b.readPlace(b.graph.Scope, []string{qn.Parts[0].Text})
			}
			return
		}
	}
	name := qn.Parts[len(qn.Parts)-1].Text
	if b.resolver != nil {
		if sym, ok := b.resolver.ReadQualified(scope, qn).Symbol(); ok && sym != nil {
			b.read(Place{Sym: sym, Name: name, Local: b.declaredByNode(sym), Direct: len(qn.Parts) == 1})
			b.boundValue(sym)
			return
		}
	}
	b.read(Place{Name: name})
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
		return d.Kind == ast.DefCalc || d.Kind.IsAction()
	case *ast.Usage:
		return d.Kind == ast.UsageCalc || d.Kind.IsAction()
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
		b.path(s.Scope, append(segments, s.Target), b.write)
	case Send:
		b.reads(s.Scope, s.Message)
		b.address(s.Scope, s.Target, s.TargetPath)
		b.address(s.Scope, s.Receiver, s.ReceiverPath)
		if s.Target == "" && s.TargetExpr != nil {
			b.reads(s.Scope, s.TargetExpr)
			b.footprint.Dynamic = true
		}
		if s.Receiver == "" && s.ReceiverExpr != nil {
			b.reads(s.Scope, s.ReceiverExpr)
			b.footprint.Dynamic = true
		}
		if s.IsVia {
			// A `via` send is routed by connections at run time, materializing the ports it reaches.
			b.footprint.Routes, b.footprint.Creates = true, true
		}
		own := s.Target == "" && s.TargetExpr == nil && s.Receiver == "" && s.ReceiverExpr == nil && !s.IsVia
		b.footprint.Sends = append(b.footprint.Sends, Channel{Port: viaPortOf(s), Message: s.Message, Scope: s.Scope, Via: s.IsVia, Own: own})
	case Declare:
		b.reads(s.Scope, s.Value)
		if b.localDeclarations {
			if sym, _ := resolve.FeatureSymbolInScope(s.Scope, []string{s.Name}); sym != nil && sym.Decl != nil {
				b.declared[sym.Decl] = nil
			}
			b.place(s.Scope, []string{s.Name}, b.write)
			return
		}
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
		wrote := false
		if b.graph != nil {
			for _, f := range b.graph.Features[b.node] {
				if f.IsResult || f.Direction == ast.DirOut {
					b.write(Place{Sym: featureSymbol(b.scope, f), Name: f.Name, Local: true, Node: b.node})
					wrote = true
				}
			}
		}
		if !wrote {
			b.write(Place{Name: "result"})
		}
	case Assert:
		b.assertion(s)
	case Effect:
		// Performing, accepting or terminating inside a body reaches beyond it.
		b.footprint.Dynamic = true
	case Unsupported:
		// Reaching it fails the run; it reads nothing first.
	}
}

// assertion adds what checking an assertion reads: the conditions its own body
// states; one whose conditions come from elsewhere reads what lowering cannot see.
func (b *footprintBuilder) assertion(s Assert) {
	if s.Sym == nil {
		return
	}
	if len(s.Node.Relationships) > 0 || s.Node.Value != nil || s.Sym.Scope == nil {
		b.footprint.Dynamic = true
		return
	}
	b.conditions(s.Sym.Scope, s.Node.Members)
}

// conditions adds the reads of the condition members of a constraint body;
// any other member makes the move dynamic.
func (b *footprintBuilder) conditions(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		switch m := unwrapMembership(member).(type) {
		case *ast.ConstraintMember:
			b.reads(scope, m.Expression)
			if len(m.Body) > 0 {
				b.conditions(symbols.ConstraintBodyScope(scope, m), m.Body)
			}
		default:
			b.footprint.Dynamic = true
		}
	}
}

// block adds a block's steps: its statements, or the bodies of its own flow's
// nodes, which run within the enclosing node's atomic step.
func (b *footprintBuilder) block(block Block) {
	if block.Graph == nil {
		b.statements(block.Statements)
		return
	}
	b.merge(FlowFootprint(block.Graph))
}

// FlowFootprint is what running the flow may touch: every node's footprint and
// those of the flows its nodes own; a node performing an action is dynamic.
func FlowFootprint(graph *ActionGraph) Footprint {
	var b footprintBuilder
	b.flow(graph, make(map[*ActionGraph]bool))
	return b.footprint
}

func (b *footprintBuilder) flow(graph *ActionGraph, seen map[*ActionGraph]bool) {
	if graph == nil || seen[graph] {
		return
	}
	seen[graph] = true
	for _, node := range graph.Nodes {
		b.merge(graph.Footprints()[node])
		if sub := graph.Subflows[node]; sub != nil {
			if sub.Graph == nil {
				b.footprint.Dynamic = true
			}
			b.flow(sub.Graph, seen)
		}
	}
}

// InstantDelay is the time event of an accept or transition trigger that may end
// the instant: a relative delay written as a number, with or without a unit.
func InstantDelay(trigger ast.Node) (*ast.TimeEvent, bool) {
	t, ok := trigger.(*ast.TimeEvent)
	if !ok || t.Absolute {
		return nil, false
	}
	duration := t.Duration
	if ix, quantity := duration.(*ast.IndexExpr); quantity && ix.Bracket {
		duration = ix.Operand
	}
	switch duration.(type) {
	case *ast.LiteralInteger, *ast.LiteralReal:
		return t, true
	}
	return nil, false
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
	b.footprint.Completion = b.footprint.Completion || other.Completion
	b.footprint.Dynamic = b.footprint.Dynamic || other.Dynamic
	b.footprint.Creates = b.footprint.Creates || other.Creates
	b.footprint.Routes = b.footprint.Routes || other.Routes
}

// plainType reports whether constructing the named type creates an object that
// starts no behavior and evaluates nothing: a non-abstract item, attribute, part or
// port definition declaring no member and specializing nothing explicitly.
func plainType(scope *symbols.Scope, qn *ast.QualifiedName) bool {
	if scope == nil || qn == nil {
		return false
	}
	decl, _, ok := resolve.TypeDeclInScope(scope, qn)
	def, isDef := decl.(*ast.Definition)
	if !ok || !isDef || def.IsAbstract || len(def.Members) > 0 || len(def.Relationships) > 0 || len(def.Prefixes) > 0 {
		return false
	}
	switch def.Kind {
	case ast.DefItem, ast.DefAttribute, ast.DefPart, ast.DefPort:
		return true
	}
	return false
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
	params, body := b.performedParameters(b.node)
	arguments := b.arguments(params)
	own := make(map[string]bool)
	for _, f := range b.graph.Features[b.node] {
		own[f.Name] = true
		b.reads(f.Scope, f.Value)
		b.pin(Place{Sym: featureSymbol(b.scope, f), Name: f.Name, Local: true, Node: b.node}, f, performs, arguments[f.Name])
	}
	// The parameters of the action a node performs are pins of the node as well,
	// bound from the arguments the node writes, else by name where the node
	// declares none of its own.
	for _, f := range params {
		if !own[f.Name] {
			b.reads(f.Scope, f.Value)
			b.pin(Place{Sym: featureSymbol(body, f), Name: f.Name, Local: true, Node: b.node}, f, true, arguments[f.Name])
		}
	}
}

// performedParameters is the parameters of the action a node performs by
// reference, typing or invocation, with the scope declaring them; none for a
// node performing no action a name states.
func (b *footprintBuilder) performedParameters(node ast.Node) ([]Feature, *symbols.Scope) {
	if !nodePerformsAction(node) {
		return nil, nil
	}
	target := performedTarget(node)
	if target == nil {
		return nil, nil
	}
	decl, body, ok := resolve.TypeDeclInScope(nodeScopeOf(b.graph, node), target)
	if !ok {
		return nil, nil
	}
	return lowerParameters(ast.DeclMembers(decl), body), body
}

// outputs is the pins a node's value is read from: its own outputs and those of
// the action it performs, each the node's own place.
func (b *footprintBuilder) outputs(node ast.Node) []Place {
	var out []Place
	each := func(f Feature, scope *symbols.Scope) {
		if f.Direction == ast.DirOut || f.Direction == ast.DirInOut || f.IsResult {
			out = append(out, Place{Sym: featureSymbol(scope, f), Name: f.Name, Local: true, Node: node})
		}
	}
	for _, f := range b.graph.Features[node] {
		each(f, nodeScopeOf(b.graph, node))
	}
	params, body := b.performedParameters(node)
	for _, f := range params {
		each(f, body)
	}
	return out
}

// arguments is the expression an invocation the node writes (`Add(x = 5)`,
// `Add(5)`) supplies each input parameter with: named by name, positional in
// the order the inputs are declared.
func (b *footprintBuilder) arguments(params []Feature) map[string]ast.Node {
	usage, ok := b.node.(*ast.Usage)
	if !ok || usage.PerformedInvocation() == nil {
		return nil
	}
	inv := usage.PerformedInvocation()
	supplied := make(map[string]ast.Node, len(inv.Args)+len(inv.NamedArgs))
	for _, arg := range inv.NamedArgs {
		if arg.Name != nil && len(arg.Name.Parts) > 0 {
			supplied[arg.Name.Parts[len(arg.Name.Parts)-1].Text] = arg.Value
		}
	}
	i := 0
	for _, f := range params {
		if i >= len(inv.Args) {
			break
		}
		if f.Direction == ast.DirIn || f.Direction == ast.DirInOut {
			supplied[f.Name] = inv.Args[i]
			i++
		}
	}
	return supplied
}

// pin adds the write of a node's own pin and, where the node performs an action,
// what binds it: the argument supplying an input, else the enclosing feature of
// its name where nothing else supplies it, and the enclosing feature an output
// is written to.
func (b *footprintBuilder) pin(place Place, f Feature, performs bool, argument ast.Node) {
	b.write(place)
	in := f.Direction == ast.DirIn || f.Direction == ast.DirInOut
	out := f.Direction == ast.DirOut || f.Direction == ast.DirInOut || f.IsResult
	switch {
	case !in:
	case argument != nil:
		b.reads(b.scope, argument)
	case f.Value == nil && !b.pinConnected(f.Name) && b.enclosingDeclares(f.Name):
		b.readPlace(b.graph.Scope, []string{f.Name})
	}
	if out && performs {
		b.place(b.graph.Scope, []string{f.Name}, b.write)
	}
}

// fallsThrough reports whether a read of the symbol reads the enclosing feature
// of its name: an input pin of a node of the flow that nothing values, binds or
// supplies is read from the flow's own feature of that name.
func (b *footprintBuilder) fallsThrough(sym *symbols.Symbol) bool {
	if sym == nil || b.graph == nil {
		return false
	}
	node, declared := b.declared[sym.Decl]
	usage, ok := sym.Decl.(*ast.Usage)
	if !declared || node == nil || !ok || usage.Direction != ast.DirIn || usage.Value != nil {
		return false
	}
	return !(&footprintBuilder{graph: b.graph, node: node}).pinConnected(sym.Name) && b.enclosingDeclares(sym.Name)
}

// enclosingDeclares reports whether the flow the node is in declares a feature of
// the name, which an input nothing else supplies is read from.
func (b *footprintBuilder) enclosingDeclares(name string) bool {
	_, ok := resolve.FeatureSymbolInScope(b.graph.Scope, []string{name})
	return ok
}

// performedTarget is the name of the action a node performs by reference, typing
// or invocation; nil when it performs none a name states.
func performedTarget(node ast.Node) *ast.QualifiedName {
	switch n := node.(type) {
	case *ast.PerformActionNode:
		if qn, ok := n.ActionRef.(*ast.QualifiedName); ok {
			return qn
		}
	case *ast.Usage:
		if inv := n.PerformedInvocation(); inv != nil {
			return inv.Type
		}
		for _, rel := range n.Relationships {
			if rel == nil || rel.Kind != ast.RelReferences && rel.Kind != ast.RelTyping {
				continue
			}
			if qn, ok := rel.Target.(*ast.QualifiedName); ok {
				return qn
			}
		}
	}
	return nil
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
		place := Place{Name: pin, Local: true, Node: node}
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

// flows adds the target pins of the node's flows: written at completion and,
// for a streaming flow, at each write to the source pin.
func (b *footprintBuilder) flows() {
	seen := make(map[pinKey]bool)
	for _, flow := range b.graph.DataFlows[b.node] {
		b.flowTarget(b.graph, flow, seen)
	}
}

// pinKey is one pin of one node of a graph.
type pinKey struct {
	node ast.Node
	pin  string
}

// flowTarget adds the write to the flow's target pin and, when the flow
// streams, the writes a value reaching that pin streams on to.
func (b *footprintBuilder) flowTarget(graph *ActionGraph, flow ObjectFlow, seen map[pinKey]bool) {
	place := Place{Name: flow.TargetPin, Local: true, Node: flow.Target}
	if scope := graph.Scopes[flow.Target]; scope != nil {
		if sym, ok := scope.LookupLocal(flow.TargetPin); ok {
			place.Sym = sym
		}
	}
	b.write(place)
	if flow.Kind != FlowStreaming {
		return
	}
	b.streamsFrom(graph, flow.Target, flow.TargetPin, seen)
}

// streamsFrom adds the writes a value written to node's pin streams on to, along
// the streaming flows out of node in graph; a pin and the names it redefines are one.
func (b *footprintBuilder) streamsFrom(graph *ActionGraph, node ast.Node, pin string, seen map[pinKey]bool) {
	aliases := pinAliases(graph.Features[node])
	pin = pinName(aliases, pin)
	key := pinKey{node: node, pin: pin}
	if seen[key] {
		return
	}
	seen[key] = true
	for _, flow := range graph.DataFlows[node] {
		if flow.Kind == FlowStreaming && pinName(aliases, flow.SourcePin) == pin {
			b.flowTarget(graph, flow, seen)
		}
	}
}

// pinAliases maps each name a node's feature redefines to the feature's own name.
func pinAliases(features []Feature) map[string]string {
	var aliases map[string]string
	for _, f := range features {
		u, ok := f.Node.(*ast.Usage)
		if !ok {
			continue
		}
		for _, redefined := range redefinedNames(u) {
			if redefined == f.Name {
				continue
			}
			if aliases == nil {
				aliases = make(map[string]string)
			}
			aliases[redefined] = f.Name
		}
	}
	return aliases
}

// pinName is the name a node's pin is held under, following what it redefines.
func pinName(aliases map[string]string, pin string) string {
	if name, ok := aliases[pin]; ok {
		return name
	}
	return pin
}

// streams adds the writes the node's own writes stream on to: a write to a pin
// of an enclosing node is carried at once along the streaming flows out of it.
func (b *footprintBuilder) streams() {
	seen := make(map[pinKey]bool)
	for _, place := range slices.Clone(b.footprint.Writes) {
		if place.Sym == nil {
			continue
		}
		for g := b.graph; g.Enclosing != nil; g = g.Enclosing {
			for _, f := range g.Enclosing.Features[g.EnclosingNode] {
				if featureSymbol(g.Enclosing.Scopes[g.EnclosingNode], f) == place.Sym {
					b.streamsFrom(g.Enclosing, g.EnclosingNode, f.Name, seen)
				}
			}
		}
	}
}

// accept adds the message the node waits for and the features its trigger
// condition reads, which answer the wait of a parked token.
func (b *footprintBuilder) accept() {
	accept, ok := b.graph.Accepts[b.node]
	if !ok {
		return
	}
	channel := Channel{Port: accept.ViaPort, Type: accept.SignalType, Scope: accept.Scope}
	if accept.SignalType != nil {
		channel.Signal = accept.SignalType.Text()
	}
	if accept.ParamName != "" {
		b.write(Place{Sym: acceptParamSymbol(b.scope, accept.ParamName), Name: accept.ParamName, Local: true, Node: b.node})
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

// ExpressionFootprint is what evaluating expr in scope reads: the places a guard
// or condition depends on, dynamic where the declarations do not decide them.
func ExpressionFootprint(scope *symbols.Scope, expr ast.Node) Footprint {
	b := &footprintBuilder{scope: scope}
	b.reads(scope, expr)
	return b.footprint
}
