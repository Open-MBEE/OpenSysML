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
	if err := g.checkRepeatedFeatureReads(node, model); err != nil {
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
	sourceRange, err := g.crossingRange(node, edge.SourceMultiplicity, edge.Decl, model)
	if err != nil {
		return err
	}
	targetRange, err := g.crossingRange(node, edge.TargetMultiplicity, edge.Decl, model)
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
			edge.SourceMultiplicity == nil && edge.TargetMultiplicity == nil {
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
			if bindingTouchesNode(binding, node) {
				return g.stepError(node, model, StepMultiplicityUnsupportedCode,
					"bindings at pins of a repeated action step are unsupported", binding.Decl)
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

func (g *ActionGraph) checkRepeatedFeatureReads(node ast.Node, model *semantics.Model) error {
	check := func(expression ast.Node, scope *symbols.Scope) error {
		var found *ast.FeatureChainExpr
		ast.Inspect(expression, func(candidate ast.Node) bool {
			chain, ok := candidate.(*ast.FeatureChainExpr)
			if !ok {
				return true
			}
			base, segments := flattenChain(chain)
			if len(segments) == 0 {
				return true
			}
			if g.chainNamesNode(base, segments, node, scope) {
				found = chain
				return false
			}
			return true
		})
		if found == nil {
			return nil
		}
		return g.stepError(node, model, StepMultiplicityUnsupportedCode,
			"features of a repeated action step cannot be read from outside the step", found)
	}
	outermost := g
	for outermost.Enclosing != nil {
		outermost = outermost.Enclosing
	}
	visited := make(map[*ActionGraph]bool)
	var scanGraph func(*ActionGraph) error
	var scanStatement func(Statement) error
	scanStatement = func(statement Statement) error {
		switch s := statement.(type) {
		case Block:
			if err := scanGraph(s.Graph); err != nil {
				return err
			}
			for _, nested := range s.Statements {
				if err := scanStatement(nested); err != nil {
					return err
				}
			}
		case Loop:
			return scanStatement(s.Body)
		case If:
			if err := scanStatement(s.Then); err != nil {
				return err
			}
			if s.Else != nil {
				return scanStatement(*s.Else)
			}
		}
		return nil
	}
	isInsideRepeatedStep := func(graph *ActionGraph) bool {
		for current := graph; current != nil && current != outermost; current = current.Enclosing {
			if current.EnclosingNode == node {
				return true
			}
		}
		return false
	}
	scanGraph = func(graph *ActionGraph) error {
		if graph == nil || visited[graph] || isInsideRepeatedStep(graph) {
			return nil
		}
		visited[graph] = true
		for _, attribute := range graph.Attributes {
			scope := attribute.Scope
			if scope == nil {
				scope = graph.Scope
			}
			if err := check(attribute.Value, scope); err != nil {
				return err
			}
		}
		for owner, features := range graph.Features {
			if owner == node {
				continue
			}
			for _, feature := range features {
				scope := feature.Scope
				if scope == nil {
					scope = graph.nodeScope(owner)
				}
				if err := check(feature.Value, scope); err != nil {
					return err
				}
			}
		}
		for owner, accept := range graph.Accepts {
			if owner != node {
				if err := check(accept.Trigger, accept.Scope); err != nil {
					return err
				}
			}
		}
		for source, edges := range graph.Edges {
			for _, edge := range edges {
				if err := check(edge.Guard, graph.nodeScope(source)); err != nil {
					return err
				}
			}
		}
		for owner, statements := range graph.Bodies {
			if owner == node {
				continue
			}
			for _, statement := range statements {
				for _, expression := range statementExpressions(statement) {
					if err := check(expression, graph.nodeScope(owner)); err != nil {
						return err
					}
				}
				if err := scanStatement(statement); err != nil {
					return err
				}
			}
		}
		for owner, subflow := range graph.Subflows {
			if owner == node || subflow == nil {
				continue
			}
			if err := scanGraph(subflow.Graph); err != nil {
				return err
			}
		}
		return nil
	}
	return scanGraph(outermost)
}

func (g *ActionGraph) chainNamesNode(base ast.Node, segments []string, node ast.Node, scope *symbols.Scope) bool {
	if g.resolver != nil {
		if scope == nil {
			scope = g.Scope
		}
		if symbol, ok := g.resolver.ResolveTarget(scope, base); ok && symbol != nil && symbol.Decl == node {
			return true
		}
	}
	path := FeaturePath(base)
	if cut := strings.LastIndex(path, "::"); cut >= 0 {
		path = path[cut+2:]
	}
	names := strings.Split(path, ".")
	names = append(names, segments...)
	nodePath := []string{getNodeName(node)}
	for graph := g; graph != nil && graph.Enclosing != nil; graph = graph.Enclosing {
		nodePath = append([]string{getNodeName(graph.EnclosingNode)}, nodePath...)
	}
	if len(names) <= len(nodePath) {
		return false
	}
	for i, name := range nodePath {
		if names[i] != name {
			return false
		}
	}
	return true
}

func statementExpressions(statement Statement) []ast.Node {
	switch s := statement.(type) {
	case Send:
		return []ast.Node{s.Message, s.TargetExpr, s.ReceiverExpr}
	case Assign:
		expressions := []ast.Node{s.Value}
		if s.Chain != nil {
			expressions = append(expressions, s.Chain.Base)
		}
		return expressions
	case Declare:
		return []ast.Node{s.Value}
	case Block:
		return blockStatementExpressions(s.Statements)
	case Loop:
		return append([]ast.Node{s.Condition, s.Until, s.Collection}, blockStatementExpressions(s.Body.Statements)...)
	case If:
		expressions := append([]ast.Node{s.Condition}, blockStatementExpressions(s.Then.Statements)...)
		if s.Else != nil {
			expressions = append(expressions, blockStatementExpressions(s.Else.Statements)...)
		}
		return expressions
	case Return:
		return []ast.Node{s.Value}
	case Effect:
		return []ast.Node{s.TargetExpr}
	}
	return nil
}

func blockStatementExpressions(statements []Statement) []ast.Node {
	var expressions []ast.Node
	for _, statement := range statements {
		expressions = append(expressions, statementExpressions(statement)...)
	}
	return expressions
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
