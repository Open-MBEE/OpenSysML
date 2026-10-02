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
	Node         ast.Node
	Step         string
	Multiplicity string
	Code         string
	Reason       string
	Declaration  ast.Node
	Err          error
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
	return fmt.Sprintf("action step %s%s: %s", step, multiplicity, reason)
}

func (e *StepMultiplicityError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// StepCount returns the fixed number of performances declared for node.
// An absent declaration preserves the historical single-performance behavior.
func (g *ActionGraph) StepCount(node ast.Node, model *semantics.Model) (int64, error) {
	if g == nil || node == nil {
		return 1, nil
	}
	multiplicity := g.Multiplicities[node]
	if multiplicity == nil {
		return 1, nil
	}
	if err := g.unaddressableBoundError(node, multiplicity, g.nodeScope(node), model, nil); err != nil {
		return 0, err
	}
	evaluator := model
	if evaluator == nil {
		evaluator = semantics.NewModel(nil)
	}
	rangeIn, ok := evaluator.RangeIn(g.nodeScope(node), multiplicity)
	if !ok || !rangeIn.Lower.Known || !rangeIn.Upper.Known ||
		rangeIn.Lower.Infinite || rangeIn.Upper.Infinite ||
		rangeIn.Lower.Value != rangeIn.Upper.Value ||
		rangeIn.Lower.Value < 0 {
		return 0, g.stepError(node, model, StepMultiplicityNotFixedCode,
			"multiplicity is not a fixed count; the executor performs a step a fixed number of times", nil)
	}
	return rangeIn.Lower.Value, nil
}

// MultiplicityText returns the source text of node's declared multiplicity.
func (g *ActionGraph) MultiplicityText(node ast.Node, model *semantics.Model) string {
	if g == nil || node == nil {
		return ""
	}
	multiplicity := g.Multiplicities[node]
	if multiplicity == nil {
		return ""
	}
	return g.multiplicityText(node, multiplicity, model)
}

// CheckStep classifies a declared multiplicity and its incident successions.
func (g *ActionGraph) CheckStep(node ast.Node, model *semantics.Model) error {
	if g == nil || node == nil || g.Multiplicities[node] == nil {
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

func (g *ActionGraph) checkRepeatedEdge(node ast.Node, edge ActionEdge, count int64, model *semantics.Model) error {
	other := edge.Source
	if other == node {
		other = edge.Target
	}
	if edge.Guard != nil || isControlNode(other) {
		reason := "control-node successions require a single crossing at the repeated step"
		if edge.Guard != nil {
			reason = "guarded successions cannot order every performance of the repeated step"
		}
		return g.stepError(node, model, StepMultiplicityUnsupportedCode, reason, edge.Decl)
	}
	return g.checkRepeatedEdgeOrder(node, edge, count, model)
}

func (g *ActionGraph) checkRepeatedEdgeOrder(node ast.Node, edge ActionEdge, count int64, model *semantics.Model) error {
	if isStartNode(edge.Source) || isDoneNode(edge.Target) {
		return nil
	}
	if count == 0 {
		return nil
	}

	sourceCount, err := g.StepCount(edge.Source, model)
	if err != nil {
		return err
	}
	targetCount, err := g.StepCount(edge.Target, model)
	if err != nil {
		return err
	}
	sourceRange, err := g.crossingRange(node, edge.Source, edge.SourceMultiplicity, model)
	if err != nil {
		return err
	}
	targetRange, err := g.crossingRange(node, edge.Target, edge.TargetMultiplicity, model)
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

type crossingRange struct {
	lower         int64
	upper         int64
	upperInfinite bool
	written       bool
}

func (r crossingRange) admits(count int64) bool {
	return r.lower <= count && (r.upperInfinite || count <= r.upper)
}

func (g *ActionGraph) crossingRange(step, endpoint ast.Node, multiplicity *ast.Multiplicity, model *semantics.Model) (crossingRange, error) {
	if multiplicity == nil {
		return crossingRange{}, nil
	}
	if err := g.unaddressableBoundError(step, multiplicity, g.Scope, model, multiplicity); err != nil {
		return crossingRange{}, err
	}
	evaluator := model
	if evaluator == nil {
		evaluator = semantics.NewModel(nil)
	}
	r, ok := evaluator.RangeIn(g.Scope, multiplicity)
	if !ok || !r.Lower.Known || r.Lower.Infinite || (!r.Upper.Known && !r.Upper.Infinite) {
		err := g.stepError(step, model, StepMultiplicityNotFixedCode,
			"succession-end multiplicity "+g.multiplicityText(step, multiplicity, model)+" cannot be evaluated", multiplicity)
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
	return &StepMultiplicityError{
		Node:         node,
		Step:         name,
		Multiplicity: g.MultiplicityText(node, model),
		Code:         code,
		Reason:       reason,
		Declaration:  declaration,
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
