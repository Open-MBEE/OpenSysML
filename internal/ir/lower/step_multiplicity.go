package lower

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const (
	// StepMultiplicityNotFixedCode identifies a step that has no exact finite count.
	StepMultiplicityNotFixedCode = "action-step-multiplicity-not-fixed"
	// StepMultiplicityUnsupportedCode identifies a repeated step with unsupported links.
	StepMultiplicityUnsupportedCode = "action-step-multiplicity-unsupported"
	// StepOrderUnsatisfiableCode identifies succession multiplicities that exclude a step count.
	StepOrderUnsatisfiableCode = "action-step-order-unsatisfiable"
	// StepOrderOpenCode identifies a succession that does not establish repeated-step ordering.
	StepOrderOpenCode = "action-step-order-open"
)

// StepMultiplicityError describes a step multiplicity the executor cannot honor.
type StepMultiplicityError struct {
	Node          ast.Node
	Step          string
	Multiplicity  string
	Code          string
	Reason        string
	Declaration   ast.Node
	InheritedFrom string
	Err           error
}

func (e *StepMultiplicityError) Error() string {
	if e == nil {
		return "action step multiplicity is unsupported"
	}
	step := e.Step
	if step == "" {
		step = "?"
	}
	multiplicity := e.Multiplicity
	if multiplicity == "" {
		multiplicity = "<unspecified>"
	}
	reason := e.Reason
	if reason == "" {
		reason = "the action step multiplicity is unsupported"
	}
	message := fmt.Sprintf("action step %s%s: %s", step, multiplicity, reason)
	if e.InheritedFrom != "" {
		message += " (inherited from " + e.InheritedFrom + ")"
	}
	return message
}

func (e *StepMultiplicityError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// StepCount returns the fixed number of performances an owned or inherited
// multiplicity governs; when none does, the step performs once.
func (g *ActionGraph) StepCount(node ast.Node, model *semantics.Model) (int64, error) {
	if g == nil || node == nil {
		return 1, nil
	}
	evaluator := model
	if evaluator == nil {
		evaluator = semantics.NewModel(g.resolver)
	}
	multiplicity, scope, source, found := g.stepMultiplicity(node, evaluator)
	if !found {
		return 1, nil
	}
	if multiplicity == nil {
		rangeIn, ok := evaluator.MultiplicityOf(source)
		if !ok {
			return 1, nil
		}
		return g.fixedStepCount(node, rangeIn, model)
	}
	if err := g.unaddressableBoundError(node, multiplicity, scope, evaluator, nil); err != nil {
		return 0, err
	}
	rangeIn, ok := evaluator.RangeIn(scope, multiplicity)
	if !ok {
		return 0, g.stepError(node, model, StepMultiplicityNotFixedCode,
			"multiplicity is not a fixed count; the executor performs a step a fixed number of times", nil)
	}
	return g.fixedStepCount(node, rangeIn, model)
}

func (g *ActionGraph) fixedStepCount(node ast.Node, rangeIn semantics.Range, model *semantics.Model) (int64, error) {
	if !rangeIn.Lower.Known || !rangeIn.Upper.Known ||
		rangeIn.Lower.Infinite || rangeIn.Upper.Infinite ||
		rangeIn.Lower.Value != rangeIn.Upper.Value ||
		rangeIn.Lower.Value < 0 {
		return 0, g.stepError(node, model, StepMultiplicityNotFixedCode,
			"multiplicity is not a fixed count; the executor performs a step a fixed number of times", nil)
	}
	return rangeIn.Lower.Value, nil
}

// MultiplicityText returns the source text of node's effective multiplicity.
func (g *ActionGraph) MultiplicityText(node ast.Node, model *semantics.Model) string {
	if g == nil || node == nil {
		return ""
	}
	evaluator := model
	if evaluator == nil {
		evaluator = semantics.NewModel(g.resolver)
	}
	multiplicity, scope, source, _ := g.stepMultiplicity(node, evaluator)
	if multiplicity == nil {
		if source != nil {
			if rangeIn, ok := evaluator.MultiplicityOf(source); ok {
				return semanticsRangeText(rangeIn)
			}
		}
		return ""
	}
	if source != nil {
		if text := g.sourceTextAt(scope, multiplicity, model); text != "" {
			return text
		}
	}
	return g.multiplicityText(node, multiplicity, model)
}

// CheckStep classifies a declared multiplicity and its incident successions.
func (g *ActionGraph) CheckStep(node ast.Node, model *semantics.Model) error {
	if g == nil || node == nil {
		return nil
	}
	_, _, _, found := g.stepMultiplicity(node, model)
	if !found {
		return nil
	}
	count, err := g.StepCount(node, model)
	if err != nil {
		return err
	}
	if count == 1 {
		for _, edge := range g.Incoming(node) {
			if edge.SourceMultiplicity == nil && edge.TargetMultiplicity == nil {
				continue
			}
			if err := g.checkRepeatedEdgeOrder(node, edge, count, model); err != nil {
				return err
			}
		}
		for _, edge := range g.Edges[node] {
			if edge.SourceMultiplicity == nil && edge.TargetMultiplicity == nil {
				continue
			}
			if err := g.checkRepeatedEdgeOrder(node, edge, count, model); err != nil {
				return err
			}
		}
		return nil
	}
	if err := g.checkRepeatedPins(node, model); err != nil {
		return err
	}
	if isFixedMultiplicityStateBehavior(node) {
		return g.stepError(node, model, StepMultiplicityUnsupportedCode,
			"the state entry, do, and exit performances have multiplicity [1]", nil)
	}

	for _, edge := range g.Incoming(node) {
		if err := g.checkRepeatedEdge(node, edge, count, model); err != nil {
			return err
		}
	}
	for _, edge := range g.Edges[node] {
		if err := g.checkRepeatedEdge(node, edge, count, model); err != nil {
			return err
		}
	}
	if count == 0 && g.hasOpenZeroStepOrder(node) {
		return g.stepError(node, model, StepOrderOpenCode,
			"the zero-performance step leaves the order of its predecessor and successor open", nil)
	}
	return nil
}

func (g *ActionGraph) StepMultiplicity(node ast.Node, model *semantics.Model) (*ast.Multiplicity, *symbols.Scope) {
	if g == nil || node == nil {
		return nil, nil
	}
	evaluator := model
	if evaluator == nil {
		evaluator = semantics.NewModel(g.resolver)
	}
	multiplicity, scope, _, _ := g.stepMultiplicity(node, evaluator)
	return multiplicity, scope
}

// HasStepMultiplicity reports whether a node has an owned or effective step
// multiplicity, including recorded library facts.
func (g *ActionGraph) HasStepMultiplicity(node ast.Node, model *semantics.Model) bool {
	if g == nil || node == nil {
		return false
	}
	evaluator := model
	if evaluator == nil {
		evaluator = semantics.NewModel(g.resolver)
	}
	_, _, _, found := g.stepMultiplicity(node, evaluator)
	return found
}

func (g *ActionGraph) stepMultiplicity(node ast.Node, model *semantics.Model) (*ast.Multiplicity, *symbols.Scope, *symbols.Symbol, bool) {
	if model == nil {
		model = semantics.NewModel(g.resolver)
	}
	if multiplicity := g.Multiplicities[node]; multiplicity != nil {
		return multiplicity, g.nodeScope(node), nil, true
	}
	usage, ok := node.(*ast.Usage)
	if !ok || (usage.Kind != ast.UsageAction && !IsCaseNode(usage)) {
		return nil, nil, nil, false
	}
	sym := actionStepSymbol(node, g.nodeScope(node))
	if sym == nil {
		return nil, nil, nil, false
	}
	source, ok := model.GoverningMultiplicitySource(sym)
	if !ok {
		return nil, nil, nil, false
	}
	multiplicity := semantics.UsageMultiplicityOf(source)
	if multiplicity != nil {
		return multiplicity, source.OwnerScope, source, true
	}
	if source.Recorded() {
		return nil, source.OwnerScope, source, true
	}
	return nil, nil, nil, false
}

func semanticsRangeText(r semantics.Range) string {
	bound := func(value semantics.Bound) string {
		if value.Infinite {
			return "*"
		}
		if value.Known {
			return fmt.Sprint(value.Value)
		}
		return "?"
	}
	if r.Lower.Known && r.Upper.Known && r.Lower == r.Upper {
		return "[" + bound(r.Lower) + "]"
	}
	return "[" + bound(r.Lower) + ".." + bound(r.Upper) + "]"
}

func actionStepSymbol(node ast.Node, scope *symbols.Scope) *symbols.Symbol {
	if node == nil {
		return nil
	}
	for current := scope; current != nil; current = current.Parent() {
		for _, member := range current.AllMembers() {
			if member != nil && member.Decl == node {
				return member
			}
		}
	}
	return nil
}

func (g *ActionGraph) checkRepeatedEdge(node ast.Node, edge ActionEdge, count int64, model *semantics.Model) error {
	other := edge.Source
	if other == node {
		other = edge.Target
	}
	if edge.Guard != nil {
		// A guard runs at the edge's source, which a written end on an edge out
		// of the repeated step cannot constrain; into one it needs its written
		// target end to count every performance.
		if edge.Target != node || edge.TargetMultiplicity == nil {
			return g.stepError(node, model, StepMultiplicityUnsupportedCode,
				"guarded successions cannot order every performance of the repeated step", edge.Decl)
		}
		// The guard's grammar writes no source end, but the one performance it
		// leaves crosses once: order the edge with that end fixed at one.
		sourceEnd := &crossingRange{lower: 1, upper: 1, written: true}
		return g.checkEdgeOrder(node, edge, count, nil, sourceEnd, nil, model)
	}
	if edge.DeclaredOrder && count > 1 {
		return g.stepError(node, model, StepOrderOpenCode,
			"the body states no succession, so its declaration order is the executor's and does not order every performance", nil)
	}
	if isControlNode(other) {
		return g.checkControlEdge(node, edge, other, count, model)
	}
	return g.checkRepeatedEdgeOrder(node, edge, count, model)
}

// checkControlEdge orders an edge between a repeated step and a control node.
// An unwritten node runs once, unless its ends force the repeated step's
// count: a bijective crossing — every performance into a join, or the lone
// incoming edge of a merge — or the one performance a fork or the lone
// outgoing edge of a decision leaves.
func (g *ActionGraph) checkControlEdge(node ast.Node, edge ActionEdge, control ast.Node, count int64, model *semantics.Model) error {
	into := edge.Target == control
	sourceEnd, targetEnd := mandatedControlEnds(control, into)
	if err := g.checkMandatedEnd(node, edge.SourceMultiplicity, sourceEnd, control, model, edge.Decl); err != nil {
		return err
	}
	if err := g.checkMandatedEnd(node, edge.TargetMultiplicity, targetEnd, control, model, edge.Decl); err != nil {
		return err
	}
	var derived bool
	switch control.(type) {
	case *ast.JoinNode:
		// The join-in ends are mandated one each, so the crossing is bijective.
		derived = into
	case *ast.MergeNode:
		// One incoming edge plus the one incoming link every merge performance
		// owns make the crossing bijective.
		derived = into && len(g.Incoming(control)) == 1
	case *ast.ForkNode:
		// The fork-out ends are mandated one each, so the crossing is bijective.
		derived = !into
	case *ast.DecisionNode:
		// One outgoing edge plus the one outgoing link every decision performance
		// owns make the crossing bijective.
		derived = !into && len(g.Edges[control]) == 1
	}
	if !derived {
		counts := map[ast.Node]int64{control: 1}
		return g.checkEdgeOrder(node, edge, count, counts, sourceEnd, targetEnd, model)
	}
	// The control node performs once per performance of the repeated step, so
	// every other edge at it must still order under that count.
	counts := map[ast.Node]int64{control: count}
	check := func(other ActionEdge) error {
		if other == edge {
			return nil
		}
		s, t := mandatedControlEnds(control, other.Target == control)
		return g.checkEdgeOrder(node, other, count, counts, s, t, model)
	}
	for _, other := range g.Incoming(control) {
		if err := check(other); err != nil {
			return err
		}
	}
	for _, other := range g.Edges[control] {
		if err := check(other); err != nil {
			return err
		}
	}
	return nil
}

// checkMandatedEnd refuses a written end that contradicts the range SysML
// mandates at a control node, which no default may rescue.
func (g *ActionGraph) checkMandatedEnd(node ast.Node, written *ast.Multiplicity, mandated *crossingRange, control ast.Node, model *semantics.Model, declaration ast.Node) error {
	if written == nil || mandated == nil {
		return nil
	}
	rangeIn, err := g.crossingRange(node, written, declaration, model)
	if err != nil {
		return err
	}
	if rangeIn.lower != mandated.lower || rangeIn.upper != mandated.upper || rangeIn.upperInfinite != mandated.upperInfinite {
		return g.stepError(node, model, StepOrderUnsatisfiableCode,
			"the succession's written end multiplicity contradicts the one SysML requires at a "+controlKindName(control)+" node", declaration)
	}
	return nil
}

// mandatedControlEnds returns the range SysML mandates at the source and target
// ends of an edge incident to a control node: `into` means the edge leads into
// the node. A nil end has no mandate and keeps the usual defaults.
func mandatedControlEnds(control ast.Node, into bool) (sourceEnd, targetEnd *crossingRange) {
	exactOne := &crossingRange{lower: 1, upper: 1, written: true}
	zeroOrOne := &crossingRange{lower: 0, upper: 1, written: true}
	if into {
		targetEnd = exactOne
		switch control.(type) {
		case *ast.JoinNode:
			sourceEnd = exactOne
		case *ast.MergeNode:
			sourceEnd = zeroOrOne
		}
		return sourceEnd, targetEnd
	}
	sourceEnd = exactOne
	switch control.(type) {
	case *ast.ForkNode:
		targetEnd = exactOne
	case *ast.DecisionNode:
		targetEnd = zeroOrOne
	}
	return sourceEnd, targetEnd
}

func controlKindName(node ast.Node) string {
	switch node.(type) {
	case *ast.ForkNode:
		return "fork"
	case *ast.JoinNode:
		return "join"
	case *ast.MergeNode:
		return "merge"
	case *ast.DecisionNode:
		return "decision"
	}
	return "control"
}

// CrossesPerPerformance reports whether node, a step performed n times, leads
// its every performance into a join or merge: the edge is bijective, so the
// control node fires once per performance rather than behind a barrier.
func (g *ActionGraph) CrossesPerPerformance(node ast.Node, model *semantics.Model) bool {
	if g == nil || node == nil {
		return false
	}
	for _, edge := range g.Edges[node] {
		if edge.Guard != nil {
			continue
		}
		switch edge.Target.(type) {
		case *ast.JoinNode, *ast.MergeNode:
			return true
		}
	}
	return false
}

func (g *ActionGraph) checkRepeatedEdgeOrder(node ast.Node, edge ActionEdge, count int64, model *semantics.Model) error {
	return g.checkEdgeOrder(node, edge, count, nil, nil, nil, model)
}

// checkEdgeOrder is the order check of checkRepeatedEdgeOrder with explicit
// counts and mandated ranges substituted: counts overrides the step count an
// endpoint reports (a control node performing per performance), and each
// mandated end stands in for an unwritten one — a written end that differs
// contradicts it and is unsatisfiable.
func (g *ActionGraph) checkEdgeOrder(node ast.Node, edge ActionEdge, count int64, counts map[ast.Node]int64, mandatedSource, mandatedTarget *crossingRange, model *semantics.Model) error {
	if isStartNode(edge.Source) || isDoneNode(edge.Target) {
		return nil
	}
	if count == 0 {
		return nil
	}

	countOf := func(endpoint ast.Node) (int64, error) {
		if counts != nil {
			if c, ok := counts[endpoint]; ok {
				return c, nil
			}
		}
		return g.StepCount(endpoint, model)
	}
	sourceCount, err := countOf(edge.Source)
	if err != nil {
		return err
	}
	targetCount, err := countOf(edge.Target)
	if err != nil {
		return err
	}
	endRange := func(endpoint ast.Node, written *ast.Multiplicity, mandated *crossingRange) (crossingRange, error) {
		rangeIn, err := g.crossingRange(node, written, edge.Decl, model)
		if err != nil {
			return crossingRange{}, err
		}
		if mandated == nil {
			return rangeIn, nil
		}
		if !rangeIn.written {
			return *mandated, nil
		}
		if rangeIn.lower != mandated.lower || rangeIn.upper != mandated.upper || rangeIn.upperInfinite != mandated.upperInfinite {
			return crossingRange{}, g.stepError(node, model, StepOrderUnsatisfiableCode,
				"the succession's written end multiplicity contradicts the one SysML requires at a "+controlKindName(endpoint)+" node", edge.Decl)
		}
		return rangeIn, nil
	}
	sourceRange, err := endRange(edge.Source, edge.SourceMultiplicity, mandatedSource)
	if err != nil {
		return err
	}
	targetRange, err := endRange(edge.Target, edge.TargetMultiplicity, mandatedTarget)
	if err != nil {
		return err
	}
	unconstrained := crossingRange{upperInfinite: true}
	exactOne := crossingRange{lower: 1, upper: 1}
	orderOpen := false
	for _, defaultRange := range []crossingRange{unconstrained, exactOne} {
		s := sourceRange
		if !s.written {
			s = defaultRange
		}
		t := targetRange
		if !t.written {
			t = defaultRange
		}
		forced := s.lower >= sourceCount || t.lower >= targetCount
		if succession, ok := edge.Decl.(*ast.SuccessionEdge); ok &&
			succession.SourceImplied && succession.TargetImplied &&
			edge.SourceMultiplicity == nil && edge.TargetMultiplicity == nil &&
			mandatedSource == nil && mandatedTarget == nil {
			forced = false
		} else if edge.Decl == nil {
			forced = s.lower >= sourceCount || t.lower >= targetCount
		}
		admitted := s.admits(sourceCount) && t.admits(targetCount)
		if forced && !admitted {
			return g.stepError(node, model, StepOrderUnsatisfiableCode,
				"the succession's end multiplicities exclude the declared step count", edge.Decl)
		}
		if !forced || !admitted {
			orderOpen = true
		}
	}
	if orderOpen {
		return g.stepError(node, model, StepOrderOpenCode,
			"the succession does not establish an order between every performance", edge.Decl)
	}
	return nil
}

// CheckBehaviorOrderMultiplicity applies the action-step succession rule at one performance per end.
func CheckBehaviorOrderMultiplicity(scope *symbols.Scope, earlier, later ast.Node, earlierMultiplicity, laterMultiplicity *ast.Multiplicity, decl ast.Node, model *semantics.Model) error {
	if earlierMultiplicity == nil && laterMultiplicity == nil {
		return nil
	}
	graph := &ActionGraph{
		Scope:          scope,
		Multiplicities: make(map[ast.Node]*ast.Multiplicity),
	}
	return graph.checkRepeatedEdgeOrder(later, ActionEdge{
		Source:             earlier,
		Target:             later,
		Decl:               decl,
		SourceMultiplicity: earlierMultiplicity,
		TargetMultiplicity: laterMultiplicity,
	}, 1, model)
}

type crossingRange struct {
	lower         int64
	upper         int64
	upperInfinite bool
	written       bool
}

func (r crossingRange) admits(count int64) bool {
	return r.lower <= count && (r.upperInfinite || count <= r.upper)
}

func (g *ActionGraph) crossingRange(step ast.Node, multiplicity *ast.Multiplicity, edgeDecl ast.Node, model *semantics.Model) (crossingRange, error) {
	if multiplicity == nil {
		return crossingRange{}, nil
	}
	scope := g.Scope
	if g.declaredIn[edgeDecl] != nil {
		scope = g.declaredIn[edgeDecl]
	}
	if err := g.unaddressableBoundError(step, multiplicity, scope, model, multiplicity); err != nil {
		return crossingRange{}, err
	}
	evaluator := model
	if evaluator == nil {
		evaluator = semantics.NewModel(g.resolver)
	}
	r, ok := evaluator.RangeIn(scope, multiplicity)
	if !ok || !r.Lower.Known || r.Lower.Infinite || (!r.Upper.Known && !r.Upper.Infinite) {
		text := g.sourceTextAt(scope, multiplicity, model)
		if text == "" {
			text = multiplicityBoundsText(multiplicity)
		}
		err := g.stepError(step, model, StepMultiplicityNotFixedCode,
			"succession-end multiplicity "+text+" cannot be evaluated", multiplicity)
		return crossingRange{}, err
	}
	return crossingRange{
		lower:         r.Lower.Value,
		upper:         r.Upper.Value,
		upperInfinite: r.Upper.Infinite,
		written:       true,
	}, nil
}

func (g *ActionGraph) unaddressableBoundError(node ast.Node, multiplicity *ast.Multiplicity, scope *symbols.Scope, model *semantics.Model, declaration ast.Node) *StepMultiplicityError {
	evaluator := model
	if evaluator == nil {
		evaluator = semantics.NewModel(nil)
	}
	value, ok := evaluator.UnaddressableBoundIn(scope, multiplicity)
	if !ok {
		return nil
	}
	err := g.stepError(node, model, StepMultiplicityUnsupportedCode,
		fmt.Sprintf("bound %s is beyond the 64-bit range of a step count", value.FormatInt()), declaration)
	err.Err = semantics.ErrIntegerUnaddressable
	return err
}

func (g *ActionGraph) checkRepeatedPins(node ast.Node, model *semantics.Model) error {
	path := []string{getNodeName(node)}
	for graph := g; graph != nil; graph = graph.Enclosing {
		for source, flows := range graph.DataFlows {
			for _, flow := range flows {
				sourceEnd := getNodeName(source) + "." + flow.SourcePin
				targetEnd := getNodeName(flow.Target) + "." + flow.TargetPin
				if source == node || flow.Target == node ||
					connectionEndStartsAt(sourceEnd, path) || connectionEndStartsAt(targetEnd, path) {
					return g.stepError(node, model, StepMultiplicityUnsupportedCode,
						"object flows at pins of a repeated action step are unsupported", flow.Decl)
				}
			}
		}
		for _, binding := range graph.Bindings {
			if !bindingTouchesNode(binding, node) {
				continue
			}
			if !g.supportedRepeatedBinding(node, binding, model) {
				return g.stepError(node, model, StepMultiplicityUnsupportedCode,
					"bindings at pins of a repeated action step are unsupported", binding.Decl)
			}
			if err := g.checkRepeatedBindingEnd(node, binding, model); err != nil {
				return err
			}
		}
		for _, connection := range graph.Connections {
			for _, end := range connection.Ends {
				if connectionEndStartsAt(end, path) {
					return g.stepError(node, model, StepMultiplicityUnsupportedCode,
						"connections at features of a repeated action step are unsupported", nil)
				}
			}
		}
		if graph.Enclosing != nil {
			path = append([]string{getNodeName(graph.EnclosingNode)}, path...)
		}
	}
	return nil
}

func bindingTouchesNode(binding PinBinding, node ast.Node) bool {
	if binding.Node == node || binding.OtherNode == node {
		return true
	}
	for _, path := range [][]ast.Node{binding.Path, binding.OtherPath} {
		for _, step := range path {
			if step == node {
				return true
			}
		}
	}
	return false
}

// supportedRepeatedBinding reports whether the binding at a pin of node is one the
// executor honors per performance: `pin = e` written at the node itself with the
// other end an expression rather than another node's pin.
func (g *ActionGraph) supportedRepeatedBinding(node ast.Node, binding PinBinding, model *semantics.Model) bool {
	if binding.Node != node || len(binding.Path) != 0 || binding.OtherNode != nil {
		return false
	}
	for _, step := range binding.OtherPath {
		if step == node {
			return false
		}
	}
	return true
}

// checkRepeatedBindingEnd refuses a binding whose other end is statically known
// to hold more than one value, which a `bind pin = e` cannot distribute over the
// performances; ends of unknown width are decided at run time.
func (g *ActionGraph) checkRepeatedBindingEnd(node ast.Node, binding PinBinding, model *semantics.Model) error {
	if binding.FromValue || binding.Other == nil || g.resolver == nil {
		return nil
	}
	scope := binding.Scope
	if scope == nil {
		scope = g.nodeScope(node)
	}
	sym, ok := g.resolver.ResolveTarget(scope, binding.Other)
	if !ok || sym == nil {
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Multiplicity == nil {
		return nil
	}
	evaluator := model
	if evaluator == nil {
		evaluator = semantics.NewModel(nil)
	}
	r, ok := evaluator.RangeIn(sym.OwnerScope, usage.Multiplicity)
	if !ok || !r.Lower.Known || r.Lower.Infinite || r.Lower.Value <= 1 {
		return nil
	}
	return g.stepError(node, model, StepMultiplicityUnsupportedCode,
		"a binding distributes a multi-valued end over the performances in an assignment the model leaves open", binding.Decl)
}

func connectionEndStartsAt(end string, path []string) bool {
	segments := strings.Split(end, ".")
	if len(segments) <= len(path) {
		return false
	}
	for i, name := range path {
		if segments[i] != name {
			return false
		}
	}
	return true
}

func (g *ActionGraph) hasOpenZeroStepOrder(node ast.Node) bool {
	hasPredecessor, hasSuccessor := false, false
	for _, edge := range g.Incoming(node) {
		if !isStartNode(edge.Source) {
			hasPredecessor = true
			break
		}
	}
	for _, edge := range g.Edges[node] {
		if !isDoneNode(edge.Target) {
			hasSuccessor = true
			break
		}
	}
	return hasPredecessor && hasSuccessor
}

func (g *ActionGraph) nodeScope(node ast.Node) *symbols.Scope {
	if g.Scopes != nil && g.Scopes[node] != nil {
		return g.Scopes[node]
	}
	return g.Scope
}

func (g *ActionGraph) stepError(node ast.Node, model *semantics.Model, code, reason string, declaration ast.Node) *StepMultiplicityError {
	name := getNodeName(node)
	if name == "" {
		name = fmt.Sprintf("%T", node)
	}
	inheritedFrom := ""
	if g != nil {
		evaluator := model
		if evaluator == nil {
			evaluator = semantics.NewModel(g.resolver)
		}
		multiplicity, _, source, _ := g.stepMultiplicity(node, evaluator)
		stepSymbol := actionStepSymbol(node, g.nodeScope(node))
		if source == nil && stepSymbol != nil {
			source, _ = evaluator.GoverningMultiplicitySource(stepSymbol)
		}
		if source != nil && stepSymbol != nil && source != stepSymbol {
			for _, redefined := range evaluator.AllRedefinedFeatures(stepSymbol) {
				if redefined == source {
					inheritedFrom = symbols.FQNOf(source)
					break
				}
			}
		}
		if declaration == nil {
			if multiplicity != nil {
				declaration = multiplicity
			}
			if source != nil && declaration == nil {
				declaration = source.Decl
			}
		}
	}
	return &StepMultiplicityError{
		Node:          node,
		Step:          name,
		Multiplicity:  g.MultiplicityText(node, model),
		Code:          code,
		Reason:        reason,
		Declaration:   declaration,
		InheritedFrom: inheritedFrom,
	}
}

// StepError constructs the shared diagnostic used when a consumer refuses a
// repeated step for a reason outside the built-in succession checks.
func (g *ActionGraph) StepError(node ast.Node, model *semantics.Model, code, reason string, declaration ast.Node) *StepMultiplicityError {
	return g.stepError(node, model, code, reason, declaration)
}

func (g *ActionGraph) multiplicityText(node ast.Node, multiplicity *ast.Multiplicity, model *semantics.Model) string {
	if text := g.sourceTextWithModel(node, multiplicity, model); text != "" {
		return text
	}
	return multiplicityBoundsText(multiplicity)
}

func multiplicityBoundsText(multiplicity *ast.Multiplicity) string {
	lower := boundText(multiplicity.Lower)
	if !multiplicity.IsRange {
		return "[" + lower + "]"
	}
	return "[" + lower + ".." + boundText(multiplicity.Upper) + "]"
}

func (g *ActionGraph) sourceTextWithModel(node, textNode ast.Node, model *semantics.Model) string {
	if model == nil || model.SourceText() == nil {
		return ""
	}
	text := model.SourceText()(g.DocOf(node), textNode.Span())
	if text != "" {
		return strings.TrimSpace(text)
	}
	return ""
}

func (g *ActionGraph) sourceTextAt(scope *symbols.Scope, textNode ast.Node, model *semantics.Model) string {
	if scope == nil || textNode == nil || model == nil || model.SourceText() == nil {
		return ""
	}
	text := model.SourceText()(symbols.DocNameOf(scope), textNode.Span())
	return strings.TrimSpace(text)
}

func boundText(node ast.Node) string {
	switch value := node.(type) {
	case *ast.LiteralInteger:
		return value.Value
	case *ast.LiteralInfinity:
		return "*"
	case *ast.QualifiedName:
		return ast.SimpleName(value)
	}
	if path := FeaturePath(node); path != "" {
		return path
	}
	return "?"
}

func isStartNode(node ast.Node) bool {
	_, ok := node.(*ast.InitialNode)
	return ok
}

func isDoneNode(node ast.Node) bool {
	_, ok := node.(*ast.FinalNode)
	return ok
}

func isControlNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		return true
	default:
		return false
	}
}

func isFixedMultiplicityStateBehavior(node ast.Node) bool {
	usage, ok := node.(*ast.Usage)
	if !ok {
		return false
	}
	switch strings.ToLower(usage.Keyword) {
	case "entry", "do", "exit":
		return true
	default:
		return false
	}
}
