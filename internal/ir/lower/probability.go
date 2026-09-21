package lower

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ProbabilityFQN names the OpenSysML metadata type by which a model weights the
// successions out of a decision node; ProbabilityFeature is the weight it binds.
const (
	ProbabilityFQN     = semantics.ProbabilityFQN
	ProbabilityFeature = semantics.ProbabilityFeature
)

// ProbabilityTolerance is how far the constant weights out of one decision may
// sum from 1.0 and still be taken as summing to it.
const ProbabilityTolerance = 1e-6

// Probability is the weight a succession's `@Probability { p = ...; }` states for
// its branch: the expression bound to p, read where the succession's guard is.
type Probability struct {
	Expr ast.Node
	// Node is the annotation, for the span a message about the weight points at.
	Node ast.Node
}

// Span is where the annotation is written.
func (p *Probability) Span() source.Span { return p.Node.Span() }

// Constant is the weight when its expression is a number computed from literals
// alone (0.7, 1 - 0.3); a weight computed from features is known only when drawn.
func (p *Probability) Constant() (float64, bool) {
	v, ok := semantics.EvalConst(p.Expr)
	if !ok || !v.IsNumeric() {
		return 0, false
	}
	return v.AsReal(), true
}

// constantNonNumber reports a weight computed from literals alone that is no
// number at all (true, *): a value the draw could never take as a probability.
func (p *Probability) constantNonNumber() bool {
	v, ok := semantics.EvalConst(p.Expr)
	return ok && !v.IsNumeric()
}

// WeightInRange reports whether a drawn or constant weight is a probability.
func WeightInRange(w float64) bool {
	return !math.IsNaN(w) && w >= 0 && w <= 1
}

// ErrProbability is the typed error every ill-formed use of Probability wraps.
var ErrProbability = errors.New("invalid Probability metadata")

// ProbabilityError reports Probability metadata the lowering cannot give a
// meaning: at what node, and why.
type ProbabilityError struct {
	Node   ast.Node
	Reason string
}

func (e *ProbabilityError) Error() string {
	return fmt.Sprintf("%v: %s", ErrProbability, e.Reason)
}

// Is makes every ProbabilityError match ErrProbability.
func (e *ProbabilityError) Is(target error) bool { return target == ErrProbability }

// Span is where the offending annotation or succession is written.
func (e *ProbabilityError) Span() source.Span {
	if e.Node == nil {
		return source.Span{}
	}
	return e.Node.Span()
}

// probabilityReader reads Probability annotations by resolving the type each
// annotation names; nil reads none, as a lowering with no resolver must.
type probabilityReader struct {
	resolver *resolve.Resolver
	scope    *symbols.Scope
}

// read returns the Probability the members of a succession's body state, nil for
// none, and refuses a body that states it twice or binds anything but p.
func (r *probabilityReader) read(members []ast.Node) (*Probability, error) {
	if r == nil || r.resolver == nil {
		return nil, nil
	}
	var found *Probability
	for _, member := range members {
		typ, body, ok := annotationOf(unwrapMembership(member))
		if !ok || !r.isProbability(typ) {
			continue
		}
		if found != nil {
			return nil, &ProbabilityError{Node: unwrapMembership(member),
				Reason: "a succession states its probability once; this one states it twice"}
		}
		p, err := r.probabilityOf(unwrapMembership(member), body)
		if err != nil {
			return nil, err
		}
		found = p
	}
	return found, nil
}

// refuseStray refuses a Probability written as a member of the action body itself,
// which annotates the action, not the succession that follows it.
func (r *probabilityReader) refuseStray(member ast.Node) error {
	if r == nil || r.resolver == nil {
		return nil
	}
	typ, _, ok := annotationOf(member)
	if !ok || !r.isProbability(typ) {
		return nil
	}
	return &ProbabilityError{Node: member, Reason: "Probability annotates the action here, not a succession; " +
		"write it in the succession's body: first d then t { @Probability { p = <weight>; } }"}
}

// refuseStrayIn refuses a Probability among a flow node's prefixes or body
// members, which annotate the node rather than a succession out of it.
func (r *probabilityReader) refuseStrayIn(prefixes []*ast.PrefixMetadata, members []ast.Node) error {
	for _, prefix := range prefixes {
		if err := r.refuseStray(prefix); err != nil {
			return err
		}
	}
	for _, member := range members {
		if err := r.refuseStray(unwrapMembership(member)); err != nil {
			return err
		}
	}
	return nil
}

// annotationOf returns the type name and body of a metadata annotation written as
// a member: `@Probability { ... }` or `metadata : Probability { ... }`.
func annotationOf(member ast.Node) (*ast.QualifiedName, []ast.Node, bool) {
	switch n := member.(type) {
	case *ast.PrefixMetadata:
		return n.Type, n.Body, n.Type != nil
	case *ast.Usage:
		if n.Kind != ast.UsageMetadata {
			return nil, nil, false
		}
		for _, rel := range n.Relationships {
			if rel == nil || rel.Kind != ast.RelTyping {
				continue
			}
			if qn, ok := rel.Target.(*ast.QualifiedName); ok {
				return qn, n.Members, true
			}
		}
	}
	return nil, nil, false
}

// isProbability reports whether a metadata type name resolves to Probability.
func (r *probabilityReader) isProbability(typ *ast.QualifiedName) bool {
	sym, ok := r.resolver.ResolveQualified(r.scope, typ)
	if !ok || sym == nil {
		return false
	}
	if target, aliasOK := r.resolver.ResolveAliasTarget(sym); aliasOK {
		sym = target
	}
	return symbols.FQNOf(sym) == ProbabilityFQN
}

// probabilityOf reads the one binding of p an annotation body makes.
func (r *probabilityReader) probabilityOf(annotation ast.Node, body []ast.Node) (*Probability, error) {
	var p *Probability
	for _, member := range body {
		usage, ok := unwrapMembership(member).(*ast.Usage)
		if !ok {
			continue
		}
		name := metadataFeatureName(usage)
		if name != ProbabilityFeature {
			return nil, &ProbabilityError{Node: usage,
				Reason: fmt.Sprintf("Probability declares %s and nothing named %q", ProbabilityFeature, name)}
		}
		if usage.Value == nil {
			return nil, &ProbabilityError{Node: usage, Reason: "p is declared without a value"}
		}
		if p != nil {
			return nil, &ProbabilityError{Node: usage, Reason: "p is bound twice"}
		}
		p = &Probability{Expr: usage.Value, Node: annotation}
	}
	if p == nil {
		return nil, &ProbabilityError{Node: annotation,
			Reason: "Probability binds no p; write @Probability { p = <weight>; }"}
	}
	return p, nil
}

// metadataFeatureName is the feature a metadata body member binds: the name it
// declares (`p = 0.7`) or the one it redefines (`:>> p = 0.7`).
func metadataFeatureName(u *ast.Usage) string {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		if qn, ok := rel.Target.(*ast.QualifiedName); ok && len(qn.Parts) > 0 {
			return qn.Parts[len(qn.Parts)-1].Text
		}
	}
	return u.Ident.Name
}

// checkProbabilities refuses weights where they have no meaning: on a succession out
// of anything but a decision node, on some but not all of a decision's, a constant
// outside 0..1, or constants out of one decision that do not sum to 1.
func checkProbabilities(graph *ActionGraph) error {
	for _, node := range graph.Nodes {
		edges := graph.Edges[node]
		weighted := 0
		for _, edge := range edges {
			if edge.Probability != nil {
				weighted++
			}
		}
		if weighted == 0 {
			continue
		}
		if _, decision := node.(*ast.DecisionNode); !decision {
			for _, edge := range edges {
				if edge.Probability != nil {
					return &ProbabilityError{Node: edge.Probability.Node, Reason: fmt.Sprintf(
						"the succession out of %s is weighted, but only a succession out of a decision node can be",
						controlNodeName(node))}
				}
			}
		}
		if weighted < len(edges) {
			for _, edge := range edges {
				if edge.Probability == nil {
					return &ProbabilityError{Node: successionNode(edge), Reason: fmt.Sprintf(
						"decision node %s weights %d of its %d successions; weight every one or none",
						controlNodeName(node), weighted, len(edges))}
				}
			}
		}
		if err := checkConstantWeights(node, edges); err != nil {
			return err
		}
	}
	return nil
}

// checkConstantWeights refuses a constant weight outside 0..1 and, where every
// weight out of the node is constant, a sum other than 1 within ProbabilityTolerance.
func checkConstantWeights(node ast.Node, edges []ActionEdge) error {
	sum, allConstant := 0.0, true
	for _, edge := range edges {
		if edge.Probability.constantNonNumber() {
			return &ProbabilityError{Node: edge.Probability.Node, Reason: fmt.Sprintf(
				"p = %s is not a number", writtenValue(edge.Probability.Expr))}
		}
		w, ok := edge.Probability.Constant()
		if !ok {
			allConstant = false
			continue
		}
		if !WeightInRange(w) {
			return &ProbabilityError{Node: edge.Probability.Node, Reason: fmt.Sprintf(
				"p = %s lies outside 0.0..1.0", writtenValue(edge.Probability.Expr))}
		}
		sum += w
	}
	if allConstant && math.Abs(sum-1) > ProbabilityTolerance {
		return &ProbabilityError{Node: node, Reason: fmt.Sprintf(
			"the probabilities out of decision node %s sum to %s, not 1.0",
			controlNodeName(node), strconv.FormatFloat(sum, 'g', -1, 64))}
	}
	return nil
}

// successionNode is the declaration a message about an edge points at: its own,
// or its source node's for an implied succession.
func successionNode(edge ActionEdge) ast.Node {
	if edge.Decl != nil {
		return edge.Decl
	}
	return edge.Source
}

// TriggerName describes a transition's trigger for a diagnostic and a choice
// line, as the runtime spells it: the signal or operation accepted, or the
// event's kind alone.
func TriggerName(trigger ast.Node) string {
	switch t := trigger.(type) {
	case nil:
		return ""
	case *ast.AcceptEvent:
		return "accept " + orAnyName(ast.SimpleName(t.SignalType))
	case *ast.CallEvent:
		return "call " + orAnyName(ast.SimpleName(t.Operation))
	case *ast.TimeEvent:
		return "time"
	case *ast.ChangeEvent:
		return "change"
	default:
		return fmt.Sprintf("%T", trigger)
	}
}

// orAnyName names a type or operation, or reports that any is accepted.
func orAnyName(name string) string {
	if name == "" {
		return "any"
	}
	return name
}

// TriggerKey is the spelling of the event a transition reacts to: the
// transitions out of one state sharing a key compete for the same occurrences
// and are checked for their weights as one group; "" keys the completion group.
func TriggerKey(trans *Transition) string {
	key := TriggerName(trans.Trigger)
	switch t := trans.Trigger.(type) {
	case nil:
		return ""
	case *ast.AcceptEvent:
		if t.Subsets != nil {
			key = "accept :> " + triggerOperand(t.Subsets)
		}
	case *ast.CallEvent:
		key = "call " + triggerOperand(t.Operation)
	case *ast.TimeEvent:
		keyword := "after"
		if t.Absolute {
			keyword = "at"
		}
		key = "accept " + keyword + " " + writtenValue(t.Duration)
	case *ast.ChangeEvent:
		key = "accept when " + writtenValue(t.Condition)
	}
	if trans.Via != "" {
		key += " via " + trans.Via
	}
	return key
}

// triggerOperand renders the name a trigger carries, or its expression.
func triggerOperand(node ast.Node) string {
	if path := FeaturePath(node); path != "" {
		return path
	}
	return writtenValue(node)
}

// TransitionGroups returns the positions of the transitions out of source that
// compete for one event: every branch out of a pseudostate, and for a state the
// completion transitions and the transitions of each trigger spelling, each
// group in declaration order.
func TransitionGroups(source ast.Node, transitions []*Transition) [][]int {
	if _, pseudostate := source.(*ast.PseudostateNode); pseudostate {
		all := make([]int, len(transitions))
		for i := range transitions {
			all[i] = i
		}
		return [][]int{all}
	}
	var keys []string
	byKey := map[string][]int{}
	for i, trans := range transitions {
		key := TriggerKey(trans)
		if _, seen := byKey[key]; !seen {
			keys = append(keys, key)
		}
		byKey[key] = append(byKey[key], i)
	}
	groups := make([][]int, 0, len(keys))
	for _, key := range keys {
		groups = append(groups, byKey[key])
	}
	return groups
}

// transitionSources lists the vertices transitions leave, states then
// pseudostates in declaration order, so the first refusal is deterministic.
func (g *StateGraph) transitionSources() []ast.Node {
	seen := make(map[ast.Node]bool, len(g.Transitions))
	var sources []ast.Node
	add := func(node ast.Node) {
		if node == nil || seen[node] || len(g.Transitions[node]) == 0 {
			return
		}
		seen[node] = true
		sources = append(sources, node)
	}
	for _, state := range g.States {
		add(state)
	}
	for _, ps := range g.Pseudostates {
		add(ps)
	}
	var rest []ast.Node
	for node := range g.Transitions {
		if !seen[node] {
			rest = append(rest, node)
		}
	}
	slices.SortFunc(rest, func(a, b ast.Node) int { return a.Span().Offset - b.Span().Offset })
	return append(sources, rest...)
}

// checkTransitionProbabilities refuses transition weights that have no meaning:
// on some but not all of the transitions of one group — the completion
// transitions of a state, the transitions of one trigger spelling, or every
// branch out of a pseudostate — a constant outside 0..1, or the constants of
// one group that do not sum to 1.
func checkTransitionProbabilities(graph *StateGraph) error {
	for _, source := range graph.transitionSources() {
		transitions := graph.Transitions[source]
		for _, group := range TransitionGroups(source, transitions) {
			if err := checkTransitionGroup(source, transitions, group); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkTransitionGroup refuses the weights of one group of competing
// transitions, as checkConstantWeights refuses a decision's.
func checkTransitionGroup(source ast.Node, transitions []*Transition, group []int) error {
	weighted := 0
	for _, pos := range group {
		if transitions[pos].Probability != nil {
			weighted++
		}
	}
	if weighted == 0 {
		return nil
	}
	competing := "transitions"
	if _, isState := source.(*ast.StateNode); isState {
		if key := TriggerKey(transitions[group[0]]); key != "" {
			competing += " on " + key
		} else {
			competing = "completion transitions"
		}
	}
	if weighted < len(group) {
		for _, pos := range group {
			if transitions[pos].Probability == nil {
				return &ProbabilityError{Node: transitionDecl(transitions[pos], source), Reason: fmt.Sprintf(
					"%s weights %d of its %d %s; %s carries no weight — weight every one or none",
					transitionVertexName(source), weighted, len(group), competing,
					transitionLabel(transitions, pos))}
			}
		}
	}
	sum, allConstant := 0.0, true
	for _, pos := range group {
		p := transitions[pos].Probability
		if p.constantNonNumber() {
			return &ProbabilityError{Node: p.Node, Reason: fmt.Sprintf(
				"weight of %s out of %s: p = %s is not a number",
				transitionLabel(transitions, pos), transitionVertexName(source), writtenValue(p.Expr))}
		}
		w, ok := p.Constant()
		if !ok {
			allConstant = false
			continue
		}
		if !WeightInRange(w) {
			return &ProbabilityError{Node: p.Node, Reason: fmt.Sprintf(
				"weight of %s out of %s: p = %s lies outside 0.0..1.0",
				transitionLabel(transitions, pos), transitionVertexName(source), writtenValue(p.Expr))}
		}
		sum += w
	}
	if allConstant && math.Abs(sum-1) > ProbabilityTolerance {
		return &ProbabilityError{Node: transitionDecl(transitions[group[0]], source), Reason: fmt.Sprintf(
			"the probabilities of the %s out of %s sum to %s, not 1.0",
			competing, transitionVertexName(source), strconv.FormatFloat(sum, 'g', -1, 64))}
	}
	return nil
}

// transitionVertexName names a transition's source vertex for a message.
func transitionVertexName(source ast.Node) string {
	switch n := source.(type) {
	case *ast.StateNode:
		return "state " + orAnonymous(n.Name)
	case *ast.PseudostateNode:
		return fmt.Sprintf("%s %s", n.Kind, n.Name)
	}
	return "vertex"
}

// transitionDecl is the node a message about a transition points at: its
// declaration, or its source when it has none.
func transitionDecl(trans *Transition, source ast.Node) ast.Node {
	if trans.Decl != nil {
		return trans.Decl
	}
	return source
}

// transitionLabel names a transition out of a vertex by its own name or by
// declared position and target, as a choice line does.
func transitionLabel(transitions []*Transition, pos int) string {
	if transitions[pos].Name != "" {
		return "transition " + transitions[pos].Name
	}
	return fmt.Sprintf("%d->%s", pos+1, vertexNameOf(transitions[pos].Target))
}

// vertexNameOf names a transition's endpoint vertex, "" for anything else.
func vertexNameOf(node ast.Node) string {
	switch n := node.(type) {
	case *ast.StateNode:
		return n.Name
	case *ast.PseudostateNode:
		return n.Name
	}
	return ""
}
