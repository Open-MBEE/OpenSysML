package runtime

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A nested action usage is a subperformance (`Actions::Action::subactions :> subperformances`):
// each token performs it anew in a frame of its own, with the enclosing performances in lexical reach.

// performances runs the nested performances of one behavior under root, its own,
// evaluated in ctx as self: an action's executor is one, a state behavior's body another.
type performances struct {
	ctx   *Context
	self  *Instance
	root  *actionFrame
	owner performanceOwner
}

// performanceOwner is the behavior whose nodes perform — an action executor or a state
// behavior (state_statements.go): what its root holds, what is around it, how a node's flow runs.
type performanceOwner interface {
	// setFeature writes a feature the root performance holds, checked and stored by its holder.
	setFeature(name string, value Value) error
	// assignAround writes a name no performance nor block around a node holds to
	// what is around the root, reporting whether something there holds it.
	assignAround(name string, value Value) (bool, error)
	// pauseAt pauses the run before node performs where a breakpoint is set on it.
	pauseAt(node ast.Node) error
	// runOwnFlow runs the flow perf's node states of its own to completion.
	runOwnFlow(perf *actionFrame) error
}

// actionFrame is one performance: the action's own (node nil) or a nested node's.
type actionFrame struct {
	node  ast.Node
	graph *lower.ActionGraph // the flow this performance runs, nil for a leaf node
	// flow is the graph node is a node of: parent's own, or the flow of a block
	// in a body parent runs (lower/block_graph.go). nil for the action's own.
	flow   *lower.ActionGraph
	scope  *symbols.Scope // the namespace the performance's features resolve in
	parent *actionFrame
	// locals are the block-local bindings entered around node in parent's body,
	// outermost first: a loop variable the node's declarations read.
	locals []map[string]Value
	// outer are the frames around a root performance its bodies read but no performance
	// holds, outermost first: a state machine's data and its states' attributes.
	outer []frame
	// live counts the tokens still running in this performance's flow, which a
	// fork inside it raises and a join or a retiring token lowers.
	live int
	// inBody marks a flow a body statement runs to completion (runSubflow) rather
	// than a token of the enclosing flow, so its last token retires instead of leaving.
	inBody bool
	// connections are the connectors a send in this flow may route through:
	// those of every flow around it, this one's own included.
	connections []lower.Connection
	// data holds the values the performance's own features hold.
	data map[string]Value
	// features are the parameters and attributes the performance holds, by name.
	features map[string]ast.FeatureDirection
	// aliases map each name a held feature redefines to the feature's own name.
	aliases map[string]string
	// result names the parameter a value read of the performance stands for,
	// "" when the action it performs states no result parameter.
	result string
	// began is the activation the performance began in, which orders performances.
	began int64
	// run is the identity of this performance among the context's runs (Context.newRun).
	run int64
	// performs is the flow of the action a typed or invoked node performed, whose
	// subactions the node's performance adopted as its own. nil otherwise.
	performs *lower.ActionGraph
	// outputs names the output parameters of that action, which return to the
	// same-named enclosing features once the performance ends.
	outputs []string
	// subactions holds the latest performance of each node of graph, which is
	// what a read of the node's pins by name sees.
	subactions map[ast.Node]*actionFrame
	// pending queues what flows and bindings delivered to a node's pins ahead of
	// its performances, each of which takes the oldest delivery at each pin.
	pending map[ast.Node]map[string][]Value
	// nested queues deliveries to pins of the nodes under a node (`leg.inner.v`) ahead
	// of its next performance, which forwards them to its own nodes.
	nested map[ast.Node][]nestedDelivery
	// ended marks a performance that has completed, so a delivery to a node under it
	// waits for the next performance rather than reaching one that is over.
	ended bool
	// nodes are the action nodes a state behavior's performance runs, which its body's
	// blocks declare; the frames of a state machine and its states hold none.
	nodes []ast.Node
	// label names a performance that is no action node's in a diagnostic: a state
	// machine's, a state's or a state behavior's (state_statements.go).
	label string
}

// nestedDelivery is a value bound for a pin of a node under another: path leads to the
// node from the one the delivery waits at.
type nestedDelivery struct {
	path  []ast.Node
	pin   string
	value Value
}

// boundEnd is a binding with an end at a performance's pin, and the performance of the
// node the binding was written at: perf's own for `p.v`, an enclosing one for `leg.inner.v`.
type boundEnd struct {
	lower.PinBinding
	at *actionFrame
}

// pinText renders the bound end as written: the node path and the pin.
func (b boundEnd) pinText() string {
	text := ActionNodeName(b.Node)
	for _, node := range b.Path {
		text += "." + ActionNodeName(node)
	}
	return text + "." + b.Pin
}

// newRootFrame is the performance of the action itself.
func (e *ActionExecutor) newRootFrame() *actionFrame {
	root := &actionFrame{
		graph:       e.graph,
		scope:       e.graph.Scope,
		connections: e.graph.Connections,
		data:        make(map[string]Value),
		features:    make(map[string]ast.FeatureDirection),
		subactions:  make(map[ast.Node]*actionFrame),
		run:         e.ctx.newRun(),
	}
	e.declareRootFeatures(root)
	return root
}

// declareRootFeatures gives root the attributes the graph declares and the
// features the action holds, aliasing what each redefines.
func (e *ActionExecutor) declareRootFeatures(root *actionFrame) {
	for _, attr := range e.graph.Attributes {
		root.features[attr.Name] = ast.DirNone
		scope := attr.Scope
		if scope == nil {
			scope = e.graph.Scope
		}
		e.ctx.aliasRedefinitions(&root.aliases, memberSymbol(scope, attr.Node), attr.Name)
	}
	e.addFeatureDirections(root.features, &root.aliases, e.action)
}

// addFeatureDirections adds the parameters and attributes an action holds, the
// inherited ones included, to features by name, aliasing what each redefines.
func (e *performances) addFeatureDirections(
	features map[string]ast.FeatureDirection, aliases *map[string]string, action *symbols.Symbol,
) {
	if action == nil {
		return
	}
	for _, member := range e.ctx.model.MembersOf(action) {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || !lower.DeclaresNodeFeature(usage) {
			continue
		}
		name, _ := ast.EffectiveName(usage)
		if name == "" {
			name = member.Name
		}
		if name == "" {
			continue
		}
		if _, held := features[name]; !held {
			features[name] = usage.Direction
		}
		e.ctx.aliasRedefinitions(aliases, member, name)
	}
}

// aliasRedefinitions maps every name sym redefines, by clause, to name: a body
// written against the redefined feature then reads the redefining one.
func (ctx *Context) aliasRedefinitions(aliases *map[string]string, sym *symbols.Symbol, name string) {
	if sym == nil || name == "" {
		return
	}
	seen := map[*symbols.Symbol]bool{sym: true}
	var visit func(*symbols.Symbol)
	visit = func(s *symbols.Symbol) {
		for _, redefined := range ctx.model.RedefinedFeatures(s) {
			if redefined == nil || seen[redefined] {
				continue
			}
			seen[redefined] = true
			*aliases = aliasRedefined(*aliases, redefined.Name, name)
			visit(redefined)
		}
	}
	visit(sym)
}

// key is the name the performance holds name under: its redefinition's, else its own.
func (f *actionFrame) key(name string) string {
	return canonical(f.aliases, name)
}

// beginPerformance starts a performance of node (of parent's flow, or a block's with locals bound),
// seeding its pins from deliveries, then the arguments it passes its callee, then input bindings,
// then its own declared defaults.
func (e *performances) beginPerformance(
	parent *actionFrame, flow *lower.ActionGraph, node ast.Node, locals []map[string]Value,
) (*actionFrame, error) {
	perf := &actionFrame{
		node:        node,
		flow:        flow,
		scope:       flow.Scopes[node],
		parent:      parent,
		locals:      locals,
		connections: parent.connections,
		data:        make(map[string]Value),
		features:    make(map[string]ast.FeatureDirection),
		run:         e.ctx.newRun(),
	}
	if perf.scope == nil {
		perf.scope = parent.scope
	}
	if sub, owns := e.subflowOf(flow, node); owns {
		perf.graph = sub.Graph
		perf.live = 1
		perf.connections = joinConnections(parent.connections, sub.Graph.Connections)
		perf.subactions = make(map[ast.Node]*actionFrame)
	}
	pins, err := e.nodePins(flow, node)
	if err != nil {
		return nil, err
	}
	perf.features = pins.directions
	perf.aliases = pins.aliases
	perf.result = pins.result
	if err := e.takeDeliveries(parent, node, perf); err != nil {
		return nil, err
	}
	if parent.subactions == nil {
		parent.subactions = make(map[ast.Node]*actionFrame)
	}
	parent.subactions[node] = perf

	// The arguments, bindings and defaults are one evaluation: a calc usage two of them
	// read answers once, and another performance evaluates it anew.
	activation, endStep := e.ctx.beginStep()
	defer endStep()
	perf.began = activation
	if err := e.bindArguments(perf, activation); err != nil {
		return nil, err
	}
	if err := e.bindInputPins(perf, activation); err != nil {
		return nil, err
	}
	if err := e.seedDeclaredValues(perf, flow.Features[node], activation); err != nil {
		return nil, err
	}
	return perf, nil
}

// bindArguments writes the arguments a node passes its callee (`F(a = 3)`) to its pins,
// evaluated in the caller's context: they stand over deliveries, bindings and defaults.
func (e *performances) bindArguments(perf *actionFrame, activation int64) error {
	usage, ok := perf.node.(*ast.Usage)
	if !ok {
		return nil
	}
	inv, performs := nestedInvocation(usage)
	if !performs || inv.expr == nil || lower.IsCaseNode(usage) {
		return nil
	}
	scope := nodeScope(perf.flow, perf.node)
	ec := e.evalContextAround(perf, scope)
	ec.inBehaviorBody = true
	ec.activation = activation
	arguments, err := invocationArguments(e.ctx, scope, inv, ec)
	if err != nil {
		return err
	}
	for name, value := range arguments {
		if err := e.setFrameFeature(perf, name, value); err != nil {
			return err
		}
	}
	return nil
}

// seedDeclaredValues evaluates the values a node's own declarations give its
// features (`in a = 3;`), where no delivery, argument or binding at the pin holds one yet.
func (e *performances) seedDeclaredValues(perf *actionFrame, features []lower.Feature, activation int64) error {
	for _, feature := range features {
		if feature.Value == nil {
			continue
		}
		if _, held := perf.data[perf.key(feature.Name)]; held {
			continue
		}
		ec := e.evalContextFor(perf, feature.Scope)
		ec.activation = activation
		value, err := ec.Eval(feature.Value)
		if err != nil {
			return fmt.Errorf("eval %s of %s: %w", feature.Name, nodeDescription(perf.node), err)
		}
		if err := e.ctx.checkNamedWrite(feature.Scope, perf.describe(), feature.Name, &value); err != nil {
			return err
		}
		perf.data[perf.key(feature.Name)] = value
	}
	return nil
}

// endPerformance completes a performance: the outputs of the action it performed
// return to same-named enclosing features, and the bindings at its output pins
// carry what it produced to their other ends.
func (e *performances) endPerformance(perf *actionFrame) error {
	perf.ended = true
	for _, name := range perf.outputs {
		value, ok := perf.data[perf.key(name)]
		if !ok {
			continue
		}
		if _, err := e.assignEnclosing(perf, name, value); err != nil {
			return err
		}
	}
	return e.bindOutputPins(perf)
}

// nodePins are the pins a performance of node holds (its own and its action's),
// and the pin read when the node is read as a value: `return`, else `out result`.
type nodePins struct {
	directions map[string]ast.FeatureDirection
	aliases    map[string]string
	result     string
}

// declares reports whether the pins hold name, under its own name or as a redefinition.
func (p nodePins) declares(name string) bool {
	_, ok := p.directions[canonical(p.aliases, name)]
	return ok
}

// nodePins returns the pins a performance of node holds.
func (e *performances) nodePins(graph *lower.ActionGraph, node ast.Node) (nodePins, error) {
	pins := nodePins{directions: make(map[string]ast.FeatureDirection)}
	usage, ok := node.(*ast.Usage)
	if !ok {
		return pins, nil
	}
	for _, feature := range graph.Features[node] {
		pins.directions[feature.Name] = feature.Direction
		e.ctx.aliasRedefinitions(&pins.aliases, memberSymbol(feature.Scope, feature.Node), feature.Name)
		if feature.IsResult {
			pins.result = feature.Name
		}
	}
	if inv, performs := nestedInvocation(usage); performs && !lower.IsCaseNode(usage) {
		sym, err := resolveActionSymbol(e.ctx, nodeScope(graph, node), inv)
		if err != nil {
			return nodePins{}, err
		}
		e.addFeatureDirections(pins.directions, &pins.aliases, e.ctx.actionBodySymbol(sym))
		for _, param := range e.ctx.actionParametersOf(sym) {
			if param.IsResult && pins.result == "" {
				pins.result = param.Name
			}
		}
	}
	if pins.result == "" {
		if dir, ok := pins.directions["result"]; ok && (dir == ast.DirOut || dir == ast.DirInOut) {
			pins.result = "result"
		}
	}
	return pins, nil
}

// nodeScope is the namespace a node's declaration resolves in: its own where the
// flow retains it (an inherited node keeps its declaring action's), else the flow's.
func nodeScope(graph *lower.ActionGraph, node ast.Node) *symbols.Scope {
	if scope := graph.Scopes[node]; scope != nil {
		return scope
	}
	return graph.Scope
}

// describe names the performance for a diagnostic.
func (f *actionFrame) describe() string {
	switch {
	case f.label != "":
		return f.label
	case f.node == nil:
		return "action"
	}
	return "action node " + ActionNodeName(f.node)
}

// path is the dotted path naming the performance from the action's own, "" for it.
func (f *actionFrame) path() string {
	if f.node == nil || f.parent == nil {
		return ""
	}
	name := ActionNodeName(f.node)
	if prefix := f.parent.path(); prefix != "" {
		return prefix + "." + name
	}
	return name
}

// declares reports whether the performance holds a feature of this name.
func (f *actionFrame) declares(name string) bool {
	_, ok := f.features[f.key(name)]
	return ok
}

// holds reports whether the performance holds name: a feature it declares, or a
// value its body stored.
func (f *actionFrame) holds(name string) bool {
	if f.declares(name) {
		return true
	}
	_, ok := f.data[f.key(name)]
	return ok
}

// lexicalFrames returns the frames a body of f reads, outermost first: each enclosing
// performance followed by the block-locals around its node, and last f's own.
func (f *actionFrame) lexicalFrames() []frame {
	var frames []frame
	if f.parent != nil {
		frames = f.parent.lexicalFrames()
	} else {
		frames = slices.Clone(f.outer)
	}
	for _, local := range f.locals {
		frames = append(frames, mapFrame(local))
	}
	return append(frames, performanceFrame(f))
}

// nodesNamed returns the nodes named name among the performance's subactions: its
// flow's nodes, those its bodies' blocks declare, and those of the action it performed.
func (f *actionFrame) nodesNamed(name string) []ast.Node {
	var named []ast.Node
	add := func(node ast.Node) {
		if _, isUsage := node.(*ast.Usage); isUsage && slices.Contains(ActionNodeNames(node), name) {
			named = append(named, node)
		}
	}
	inFlow := func(graph *lower.ActionGraph) {
		for _, node := range graph.Nodes {
			add(node)
		}
		for _, node := range graph.Nodes {
			if _, isUsage := node.(*ast.Usage); isUsage {
				continue
			}
			for _, declared := range graph.BlockNodes[node] {
				add(declared)
			}
		}
	}
	switch {
	case f.graph != nil:
		inFlow(f.graph)
	case f.flow != nil:
		for _, node := range f.flow.BlockNodes[f.node] {
			add(node)
		}
	default:
		for _, node := range f.nodes {
			add(node)
		}
	}
	if f.performs != nil {
		inFlow(f.performs)
	}
	return named
}

// subaction returns the latest performance of f's node named name; declared reports
// whether f has such a node, one not yet performed is ErrNodeNotPerformed. Among
// same-named nodes (one per branch of a conditional), decl names the reader's own,
// else the one performed latest answers.
func (f *actionFrame) subaction(name string, decl ast.Node) (perf *actionFrame, declared bool, err error) {
	named := f.nodesNamed(name)
	if len(named) == 0 {
		return nil, false, nil
	}
	node := named[0]
	if slices.Contains(named, decl) {
		node = decl
	} else {
		for _, candidate := range named {
			performed, ok := f.subactions[candidate]
			if ok && (f.subactions[node] == nil || f.subactions[node].began < performed.began) {
				node = candidate
			}
		}
	}
	perf, performed := f.subactions[node]
	if !performed {
		return nil, true, fmt.Errorf("%w: action node %s has not been performed yet",
			ErrNodeNotPerformed, ActionNodeName(node))
	}
	return perf, true, nil
}

// pin reads the value the performance's pin holds.
func (f *actionFrame) pin(name string) (Value, error) {
	value, ok := f.data[f.key(name)]
	if ok {
		return value, nil
	}
	if f.declares(name) {
		return Value{}, &NoValueError{Feature: f.path() + "." + name}
	}
	return Value{}, fmt.Errorf("%w: %s declares no %s", ErrNodePin, f.describe(), name)
}

// resultValue is what the performance stands for when read as a value: the
// result parameter of the action it performs.
func (f *actionFrame) resultValue() (Value, error) {
	if f.result == "" {
		return Value{}, fmt.Errorf("%w: %s declares no result to read it as a value by",
			ErrNodePin, f.describe())
	}
	if value, ok := f.data[f.key(f.result)]; ok {
		return value, nil
	}
	return Value{}, &NoValueError{Feature: f.path() + "." + f.result}
}

// deliver stores a value at a pin of node, a node of flow in f, ahead of its next
// performance, which is how a flow or binding reaches a node not yet running. A path
// leads on to a node under it: the value goes into node's running performance, else
// waits for its next one to forward it.
func (e *performances) deliver(f *actionFrame, flow *lower.ActionGraph, node ast.Node, path []ast.Node, pin string, value Value) error {
	if len(path) > 0 {
		if err := e.checkNestedDelivery(flow, node, path, pin, value); err != nil {
			return err
		}
		if sub, performed := f.subactions[node]; performed && !sub.ended {
			return e.deliver(sub, lower.NestedFlow(flow, node, path[0]), path[0], path[1:], pin, value)
		}
		if f.nested == nil {
			f.nested = make(map[ast.Node][]nestedDelivery)
		}
		f.nested[node] = append(f.nested[node], nestedDelivery{path: path, pin: pin, value: value})
		return nil
	}
	pins, err := e.nodePins(flow, node)
	if err != nil {
		return err
	}
	if !pins.declares(pin) {
		return fmt.Errorf("%w: %s declares no %s", ErrNodePin, nodeDescription(node), pin)
	}
	if err := e.ctx.checkNamedWrite(flow.Scopes[node], nodeDescription(node), pin, &value); err != nil {
		return err
	}
	pin = canonical(pins.aliases, pin)
	if f.pending == nil {
		f.pending = make(map[ast.Node]map[string][]Value)
	}
	if f.pending[node] == nil {
		f.pending[node] = make(map[string][]Value)
	}
	f.pending[node][pin] = append(f.pending[node][pin], value)
	return nil
}

// checkNestedDelivery checks that path leads from node through the flows under it to a
// node declaring pin, so a delivery waiting for a performance is known to have somewhere to go.
func (e *performances) checkNestedDelivery(flow *lower.ActionGraph, node ast.Node, path []ast.Node, pin string, value Value) error {
	for _, next := range path {
		flow = lower.NestedFlow(flow, node, next)
		if flow == nil {
			return fmt.Errorf("%w: %s holds no nested action %s", ErrNodePin, nodeDescription(node), ActionNodeName(next))
		}
		node = next
	}
	pins, err := e.nodePins(flow, node)
	if err != nil {
		return err
	}
	if !pins.declares(pin) {
		return fmt.Errorf("%w: %s declares no %s", ErrNodePin, nodeDescription(node), pin)
	}
	return e.ctx.checkNamedWrite(flow.Scopes[node], nodeDescription(node), pin, &value)
}

// takeDeliveries moves the oldest delivery at each pin of node into perf, so that
// performances of one node begun in turn each start with their own inputs, and
// forwards what waits for the nodes under it.
func (e *performances) takeDeliveries(f *actionFrame, node ast.Node, perf *actionFrame) error {
	queues := f.pending[node]
	for pin, values := range queues {
		perf.data[pin] = values[0]
		if len(values) == 1 {
			delete(queues, pin)
		} else {
			queues[pin] = values[1:]
		}
	}
	if len(queues) == 0 {
		delete(f.pending, node)
	}
	nested := f.nested[node]
	delete(f.nested, node)
	for _, d := range nested {
		if err := e.deliver(perf, lower.NestedFlow(perf.flow, node, d.path[0]), d.path[0], d.path[1:], d.pin, d.value); err != nil {
			return err
		}
	}
	return nil
}

// setFrameFeature writes a feature the performance holds, through the action's
// performance occurrence for the action's own features.
func (e *performances) setFrameFeature(f *actionFrame, name string, value Value) error {
	if f == e.root {
		if err := e.owner.setFeature(name, value); err != nil {
			return err
		}
		e.noteFrameWrite(f, name, value)
		return nil
	}
	if err := e.ctx.checkNamedWrite(f.scope, f.describe(), name, &value); err != nil {
		return err
	}
	f.data[f.key(name)] = value
	e.noteFrameWrite(f, name, value)
	return nil
}

// assignEnclosing writes name to the innermost block-local or performance feature
// around perf that holds it, else to what is around the root, reporting whether one did.
func (e *performances) assignEnclosing(perf *actionFrame, name string, value Value) (bool, error) {
	local, holder, ok := enclosingHolder(perf, name)
	switch {
	case !ok:
		return e.owner.assignAround(name, value)
	case local != nil:
		local[name] = value
		return true, nil
	default:
		return true, e.setFrameFeature(holder, name, value)
	}
}

// enclosingHolder finds the innermost block-local or performance around perf that
// holds name: the block's locals, else the performance.
func enclosingHolder(perf *actionFrame, name string) (local map[string]Value, holder *actionFrame, ok bool) {
	for f := perf; f != nil; f = f.parent {
		for i := len(f.locals) - 1; i >= 0; i-- {
			if _, ok := f.locals[i][name]; ok {
				return f.locals[i], nil, true
			}
		}
		if f.parent != nil && f.parent.holds(name) {
			return nil, f.parent, true
		}
	}
	return nil, nil, false
}

// lookupEnclosing reads name from the innermost binding around perf that holds
// a value for it, the frames around the root included.
func lookupEnclosing(perf *actionFrame, name string) (Value, bool) {
	for f := perf; f != nil; f = f.parent {
		for i := len(f.locals) - 1; i >= 0; i-- {
			if value, ok := f.locals[i][name]; ok {
				return value, true
			}
		}
		if f.parent != nil {
			if value, ok := f.parent.data[f.parent.key(name)]; ok {
				return value, true
			}
			continue
		}
		for i := len(f.outer) - 1; i >= 0; i-- {
			if value, ok := f.outer[i].lookup(name); ok {
				return value, true
			}
		}
	}
	return Value{}, false
}

// evalContextFor returns a context evaluating in scope with the performance and
// every frame around it in reach, innermost last.
func (e *performances) evalContextFor(perf *actionFrame, scope *symbols.Scope) *EvalContext {
	ec := NewEvalContextIn(e.ctx, scope, e.self)
	for _, f := range perf.lexicalFrames() {
		ec.pushFrame(f)
	}
	return ec
}

// evalContextAround returns a context evaluating in scope what is written at perf's
// node: the enclosing performances and the block-locals around the node, not perf's own.
func (e *performances) evalContextAround(perf *actionFrame, scope *symbols.Scope) *EvalContext {
	ec := NewEvalContextIn(e.ctx, scope, e.self)
	if perf.parent != nil {
		for _, f := range perf.parent.lexicalFrames() {
			ec.pushFrame(f)
		}
	}
	for _, local := range perf.locals {
		ec.pushFrame(mapFrame(local))
	}
	return ec
}

// lexicalValues merges the values a performance and the frames around it hold,
// the innermost winning, for a caller reading them as one map.
func lexicalValues(perf *actionFrame) map[string]Value {
	merged := make(map[string]Value)
	for _, f := range perf.lexicalFrames() {
		for name, value := range f.vars {
			merged[name] = value
		}
	}
	return merged
}

// collect reports the values the performance and its subactions hold, a node's
// under its path (`p.v`), the latest performance of each name standing for it.
func (f *actionFrame) collect(prefix string, into map[string]Value) {
	for name, value := range f.data {
		into[prefix+name] = value
	}
	latest := make(map[string]*actionFrame)
	for node, sub := range f.subactions {
		name := ActionNodeName(node)
		if name == "" {
			continue
		}
		if earlier, named := latest[name]; !named || earlier.began < sub.began {
			latest[name] = sub
		}
	}
	for name, sub := range latest {
		sub.collect(prefix+name+".", into)
	}
}

// bindInputPins seeds the pins a performance reads from the bindings at them, where nothing
// delivered ahead of it holds a value; bindings must agree, and an unvalued undirected end waits.
func (e *performances) bindInputPins(perf *actionFrame, activation int64) error {
	bound := make(map[string]boundEnd)
	for _, end := range e.bindingsAt(perf) {
		dir, err := e.boundPin(perf, end)
		if err != nil {
			return err
		}
		if dir == ast.DirOut {
			continue
		}
		earlier, alreadyBound := bound[end.Pin]
		if _, held := perf.data[perf.key(end.Pin)]; held && !alreadyBound {
			continue
		}
		if holder, node, _ := otherEnd(perf, end); node != nil && holder == perf {
			continue // a node of perf's own flow reads this pin once it runs
		}
		value, err := e.bindingOtherValue(perf, end, activation)
		if err != nil {
			if dir == ast.DirNone && e.unheldEnd(end, err) {
				continue
			}
			return err
		}
		if alreadyBound {
			if held := perf.data[perf.key(end.Pin)]; !equalValues(held, value) {
				return &BindingConflictError{
					Target:     end.pinText(),
					Left:       bindingEndText(earlier.Other),
					Right:      bindingEndText(end.Other),
					LeftValue:  held,
					RightValue: value,
				}
			}
			continue
		}
		if err := e.setFrameFeature(perf, end.Pin, value); err != nil {
			return err
		}
		bound[end.Pin] = end
	}
	return nil
}

// bindOutputPins carries what a performance's output pins hold to the other ends of the
// bindings at them, and what an undirected pin holds where its other end differs from it.
func (e *performances) bindOutputPins(perf *actionFrame) error {
	for _, end := range e.bindingsAt(perf) {
		dir, err := e.boundPin(perf, end)
		if err != nil {
			return err
		}
		value, ok := perf.data[perf.key(end.Pin)]
		switch dir {
		case ast.DirOut, ast.DirInOut:
			if !ok {
				return fmt.Errorf("%w: %s produced no value at %s to bind %s to",
					ErrBindingEnd, perf.describe(), end.Pin, bindingEndText(end.Other))
			}
		case ast.DirNone:
			if !ok {
				continue
			}
			if other, held := e.otherEndHeld(perf, end); held && equalValues(other, value) {
				continue
			}
		default:
			continue
		}
		if end.OtherNode != nil {
			if holder, node, _ := otherEnd(perf, end); node != nil && holder == perf {
				continue // a node of perf's own flow carried its value here as it ended
			}
			if err := e.writeOtherEnd(perf, end, value); err != nil {
				return err
			}
			continue
		}
		if end.OtherChain != nil {
			ec := e.evalContextAround(end.at, end.Scope)
			if err := writeThroughChain(ec, end.OtherChain, end.OtherFeature, value); err != nil {
				return fmt.Errorf("%w: %s is bound to %s: %w",
					ErrBindingEnd, end.pinText(), bindingEndText(end.Other), err)
			}
			continue
		}
		name := simpleEndName(end.Other)
		if name == "" {
			return fmt.Errorf("%w: %s is bound to %s, which names no feature to hold its value",
				ErrBindingEnd, end.pinText(), bindingEndText(end.Other))
		}
		written, err := e.assignEnclosing(end.at, name, value)
		if err != nil {
			return err
		}
		if !written {
			return fmt.Errorf("%w: %s is bound to %s, which no enclosing action holds",
				ErrBindingEnd, end.pinText(), name)
		}
	}
	return nil
}

// bindingsAt returns the bindings with an end at perf's pins: those of the flow its node
// is a node of written at it (`p.v`), and those an enclosing flow wrote reaching down to
// it through the performances between (`leg.inner.v`).
func (e *performances) bindingsAt(perf *actionFrame) []boundEnd {
	var at []boundEnd
	var path []ast.Node
	for anc := perf; anc.parent != nil; anc = anc.parent {
		for _, binding := range anc.flow.Bindings {
			if binding.Node == anc.node && slices.Equal(binding.Path, path) {
				at = append(at, boundEnd{PinBinding: binding, at: anc})
			}
		}
		path = append([]ast.Node{anc.node}, path...)
	}
	return at
}

// boundPin returns the direction of the pin a binding ends at, which must be a
// feature the performance holds.
func (e *performances) boundPin(perf *actionFrame, end boundEnd) (ast.FeatureDirection, error) {
	dir, declared := perf.features[perf.key(end.Pin)]
	if !declared {
		return ast.DirNone, fmt.Errorf("%w: %s names no parameter or attribute of %s",
			ErrBindingEnd, end.pinText(), perf.describe())
	}
	return dir, nil
}

// otherEnd locates the other end of a binding at perf's pin: the performance it is in reach
// of, and the node under that one it names with the path on to it, none when it is that
// performance's own pin. The end runs down through the performances between end.at and
// perf as far as it names their nodes, so it keeps to perf's own run of each, not the latest.
func otherEnd(perf *actionFrame, end boundEnd) (holder *actionFrame, node ast.Node, path []ast.Node) {
	if end.OtherNode != end.at.node {
		return end.at.parent, end.OtherNode, end.OtherPath
	}
	holder, path = end.at, end.OtherPath
	for len(path) > 0 {
		next := childToward(holder, perf)
		if next == nil || next.node != path[0] {
			break
		}
		holder, path = next, path[1:]
	}
	if len(path) == 0 {
		return holder, nil, nil
	}
	return holder, path[0], path[1:]
}

// childToward returns the performance under holder on the way to perf, nil at perf itself.
func childToward(holder, perf *actionFrame) *actionFrame {
	for f := perf; f != nil && f.parent != nil; f = f.parent {
		if f.parent == holder {
			return f
		}
	}
	return nil
}

// otherPerformance returns the performance holding the other end of a binding at perf's
// pin: one of those around perf, or the latest of the node it names under one; performed
// is false where that node, or one on the way to it, has not run.
func (e *performances) otherPerformance(perf *actionFrame, end boundEnd) (other *actionFrame, performed bool) {
	holder, node, path := otherEnd(perf, end)
	if node == nil {
		return holder, true
	}
	other, performed = holder.subactions[node]
	for _, node := range path {
		if !performed {
			return nil, false
		}
		other, performed = other.subactions[node]
	}
	return other, performed
}

// writeOtherEnd carries value to the node pin at the other end of a binding at perf's pin:
// into the performance around perf holding it, else delivered ahead of the node it names.
func (e *performances) writeOtherEnd(perf *actionFrame, end boundEnd, value Value) error {
	holder, node, path := otherEnd(perf, end)
	if node == nil {
		if !holder.declares(end.OtherPin) {
			return fmt.Errorf("%w: %s declares no %s", ErrNodePin, holder.describe(), end.OtherPin)
		}
		return e.setFrameFeature(holder, end.OtherPin, value)
	}
	flow := end.at.flow
	if holder != end.at.parent {
		flow = lower.NestedFlow(holder.flow, holder.node, node)
		if flow == nil {
			return fmt.Errorf("%w: %s holds no nested action %s", ErrNodePin, holder.describe(), ActionNodeName(node))
		}
	}
	return e.deliver(holder, flow, node, path, end.OtherPin, value)
}

// bindingOtherValue reads the value the other end of a binding at a performance's pin
// holds: another node's pin, or an expression over the enclosing performances.
func (e *performances) bindingOtherValue(perf *actionFrame, end boundEnd, activation int64) (Value, error) {
	if end.OtherNode != nil {
		other, performed := e.otherPerformance(perf, end)
		if !performed {
			return Value{}, fmt.Errorf("%w: %s is bound to %s, which is read before it runs",
				ErrNodeNotPerformed, end.pinText(), bindingEndText(end.Other))
		}
		value, err := other.pin(end.OtherPin)
		if err != nil {
			return Value{}, fmt.Errorf("%w: bound to %s", err, end.pinText())
		}
		return value, nil
	}
	ec := e.evalContextAround(end.at, end.Scope)
	ec.activation = activation
	value, err := ec.Eval(end.Other)
	if err != nil {
		return Value{}, fmt.Errorf("%w: %s is bound to %s: %w",
			ErrBindingEnd, end.pinText(), bindingEndText(end.Other), err)
	}
	return value, nil
}

// otherEndHeld reads what the other end of a binding holds now: a performed node's
// pin, an enclosing feature named outright, or the feature a chain reaches.
func (e *performances) otherEndHeld(perf *actionFrame, end boundEnd) (Value, bool) {
	if end.OtherNode != nil {
		other, performed := e.otherPerformance(perf, end)
		if !performed {
			return Value{}, false
		}
		value, held := other.data[other.key(end.OtherPin)]
		return value, held
	}
	if name := simpleEndName(end.Other); name != "" {
		return e.evalContextAround(end.at, end.Scope).Lookup(name)
	}
	if end.OtherChain != nil {
		value, err := e.evalContextAround(end.at, end.Scope).Eval(end.Other)
		return value, err == nil
	}
	return Value{}, false
}

// unheldEnd reports whether err, from reading the other end of a binding, says that end
// holds no value yet: an unperformed node's pin or an unvalued enclosing feature.
func (e *performances) unheldEnd(end boundEnd, err error) bool {
	var noValue *NoValueError
	if errors.Is(err, ErrNodeNotPerformed) || errors.As(err, &noValue) {
		return true
	}
	name := simpleEndName(end.Other)
	if end.OtherNode != nil || name == "" {
		return false
	}
	_, valued := e.evalContextAround(end.at, end.Scope).Lookup(name)
	_, _, holds := enclosingHolder(end.at, name)
	return !valued && holds
}

// simpleEndName returns the name a binding end written as one name states, ""
// for any other expression.
func simpleEndName(end ast.Node) string {
	switch n := end.(type) {
	case *ast.FeatureReference:
		return simpleEndName(n.Name)
	case *ast.QualifiedName:
		if len(n.Parts) == 1 {
			return n.Parts[0].Text
		}
	}
	return ""
}

// bindingEndText renders a binding end for a diagnostic.
func bindingEndText(end ast.Node) string {
	switch n := end.(type) {
	case *ast.FeatureReference:
		return bindingEndText(n.Name)
	case *ast.QualifiedName:
		return qualifiedNameText(n)
	case *ast.FeatureChainExpr:
		return bindingEndText(n.Operand) + "." + ast.SimpleName(n.Member)
	}
	return fmt.Sprintf("%T", end)
}

// performInvocation performs the action a node names as a subperformance of perf: the node's
// pins (its arguments among them) bind the callee's inputs, its final values become the node's,
// and its outputs return to enclosing features when the node's own performance ends.
func (e *performances) performInvocation(perf *actionFrame, inv actionInvocation) error {
	scope := nodeScope(perf.flow, perf.node)
	sym, err := resolveActionSymbol(e.ctx, scope, inv)
	if err != nil {
		return err
	}
	if e.ctx.actionDepth >= maxActionNestingDepth {
		return fmt.Errorf(
			"action invocation nested more than %d deep at %s (recursive action?)",
			maxActionNestingDepth, qualifiedNameText(inv.target),
		)
	}
	e.ctx.actionDepth++
	defer func() { e.ctx.actionDepth-- }()

	params := e.ctx.actionParametersOf(sym)
	in, out := parameterNames(params)
	inputs := make(map[string]Value, len(in))
	for _, name := range in {
		if value, ok := perf.data[perf.key(name)]; ok {
			inputs[name] = value
		}
	}
	if inv.expr == nil {
		for _, name := range in {
			if _, bound := inputs[name]; bound {
				continue
			}
			if value, ok := lookupEnclosing(perf, name); ok {
				inputs[name] = value
			}
		}
	}
	if err := checkInputsBound(inv, params, inputs); err != nil {
		return err
	}

	callee, err := e.ctx.performActionStep(sym, e.self, inputs)
	if err != nil {
		return fmt.Errorf("invoke action %s: %w", qualifiedNameText(inv.target), err)
	}
	perf.adopt(callee)
	sort.Strings(out)
	perf.outputs = out
	return nil
}

// adopt makes the completed performance of the action a node performed the node's
// own: its features' values, and its subactions, read as `call.inner.v`.
func (f *actionFrame) adopt(callee *ActionExecutor) {
	for name, value := range callee.root.data {
		f.data[f.key(name)] = value
	}
	f.performs = callee.graph
	if len(callee.root.subactions) > 0 && f.subactions == nil {
		f.subactions = make(map[ast.Node]*actionFrame, len(callee.root.subactions))
	}
	for node, sub := range callee.root.subactions {
		sub.parent = f
		f.subactions[node] = sub
	}
}

// checkInputsBound reports an input parameter that no argument, pin value or
// default binds, before the callee runs rather than when its body reads it.
func checkInputsBound(inv actionInvocation, params []actionParameter, inputs map[string]Value) error {
	for _, param := range params {
		if param.Direction != ast.DirIn && param.Direction != ast.DirInOut {
			continue
		}
		if _, bound := inputs[param.Name]; bound || param.Optional {
			continue
		}
		return fmt.Errorf("%w: action %s: input parameter %s is bound by no argument",
			ErrUnboundParameter, qualifiedNameText(inv.target), param.Name)
	}
	return nil
}

// performanceFrame is the frame an evaluation reads a performance's values
// through, which also answers for the nodes of its flow.
func performanceFrame(f *actionFrame) frame {
	return frame{vars: f.data, aliases: f.aliases, perf: f, run: f.run}
}
