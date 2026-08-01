package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ActionExecutor executes action bodies using token-flow semantics.
type ActionExecutor struct {
	ctx         *Context
	action      *symbols.Symbol
	tokens      []Token
	state       ExecutionState
	nextTokenID int64
	breakpoints map[string]bool
	results     map[string]Value // Accumulated results from consumed final tokens
	
	// Graph structure
	nodes       []ast.Node
	edges       map[ast.Node][]ast.Node          // Successor edges
	guards      map[ast.Node]map[ast.Node]ast.Node // Guards: source → target → guard expression
	dataFlows   map[ast.Node][]objectFlow        // Object flow edges
	mergeVisited map[ast.Node]bool               // Track merge node visits
}

type objectFlow struct {
	SourcePin string
	TargetPin string
	Target    ast.Node
}

// newActionExecutor creates an action executor.
func newActionExecutor(ctx *Context, action *symbols.Symbol) (*ActionExecutor, error) {
	if action.Kind != symbols.SymbolActionUsage && action.Kind != symbols.SymbolActionDef {
		return nil, fmt.Errorf("symbol %s is not an action", action.Name)
	}
	
	exec := &ActionExecutor{
		ctx:          ctx,
		action:       action,
		tokens:       make([]Token, 0),
		state:        StateReady,
		nextTokenID:  1,
		breakpoints:  make(map[string]bool),
		results:      make(map[string]Value),
		edges:        make(map[ast.Node][]ast.Node),
		guards:       make(map[ast.Node]map[ast.Node]ast.Node),
		dataFlows:    make(map[ast.Node][]objectFlow),
		mergeVisited: make(map[ast.Node]bool),
	}
	
	// Extract graph structure from AST
	if err := exec.extractGraph(); err != nil {
		return nil, fmt.Errorf("extract graph: %w", err)
	}
	
	return exec, nil
}

// Step advances execution by one step for all active tokens.
// Safely handles token slice modifications (fork/join) by collecting indices first.
// Returns error if deadlock detected (no progress made).
func (e *ActionExecutor) Step() error {
	if e.state == StateCompleted {
		return nil // Already completed
	}
	
	if e.state == StateReady {
		return fmt.Errorf("executor not initialized (call initialize first)")
	}
	
	// Snapshot token state before step (for deadlock detection)
	tokenCountBefore := len(e.tokens)
	tokenLocationsBefore := make([]ast.Node, len(e.tokens))
	for i, t := range e.tokens {
		tokenLocationsBefore[i] = t.Location
	}
	
	// Collect token indices to step (snapshot before iteration)
	tokenIndices := make([]int, len(e.tokens))
	for i := range e.tokens {
		tokenIndices[i] = i
	}
	
	// Step tokens in reverse order to handle removal safely
	// (removing token at higher index doesn't affect lower indices)
	for i := len(tokenIndices) - 1; i >= 0; i-- {
		// Check if token still exists (may have been removed by join/final)
		if i >= len(e.tokens) {
			continue
		}
		
		err := e.stepToken(i)
		if err != nil {
			return err
		}
	}
	
	// Deadlock detection: check if any progress was made
	progressMade := false
	
	// Progress indicators:
	// 1. Token count changed (fork/join/final consumed/created tokens)
	if len(e.tokens) != tokenCountBefore {
		progressMade = true
	}
	
	// 2. At least one token moved to different location
	if !progressMade && len(e.tokens) > 0 {
		for i := 0; i < len(e.tokens) && i < len(tokenLocationsBefore); i++ {
			if e.tokens[i].Location != tokenLocationsBefore[i] {
				progressMade = true
				break
			}
		}
	}
	
	// 3. All tokens consumed (completion)
	if len(e.tokens) == 0 {
		progressMade = true
	}
	
	// If no progress and tokens remain, deadlock detected
	if !progressMade && len(e.tokens) > 0 {
		return fmt.Errorf("deadlock detected: %d token(s) stuck, no progress made", len(e.tokens))
	}
	
	return nil
}

// RunToCompletion executes until StateCompleted or error.
// Includes infinite loop protection.
func (e *ActionExecutor) RunToCompletion() error {
	const maxSteps = 10000
	steps := 0
	
	for e.state == StateRunning {
		if steps >= maxSteps {
			return fmt.Errorf("execution exceeded max steps (%d), possible infinite loop", maxSteps)
		}
		
		err := e.Step()
		if err != nil {
			return err
		}
		
		steps++
	}
	
	return nil
}

// extractGraph builds node and edge maps from action AST.
func (e *ActionExecutor) extractGraph() error {
	// Get action node
	actionNode, ok := e.action.Decl.(*ast.Usage)
	if !ok {
		actionDef, ok := e.action.Decl.(*ast.Definition)
		if !ok {
			return fmt.Errorf("action symbol has invalid node type")
		}
		actionNode = &ast.Usage{Members: actionDef.Members}
	}
	
	// Extract nodes and edges from members
	for _, member := range actionNode.Members {
		// Unwrap Membership if present
		actualMember := member
		if membership, ok := member.(*ast.Membership); ok {
			actualMember = membership.Member
		}
		
		switch n := actualMember.(type) {
		case *ast.InitialNode, *ast.FinalNode, *ast.ForkNode, *ast.JoinNode,
			*ast.MergeNode, *ast.DecisionNode, *ast.ActionExecutionNode:
			e.nodes = append(e.nodes, actualMember)
		case *ast.SuccessionEdge:
			// Build edge map (source → target)
			sourceNode := e.findNodeByName(n.Source)
			targetNode := e.findNodeByName(n.Target)
			if sourceNode == nil {
				return fmt.Errorf("succession edge references undefined source node")
			}
			if targetNode == nil {
				return fmt.Errorf("succession edge references undefined target node")
			}
			e.edges[sourceNode] = append(e.edges[sourceNode], targetNode)
		case *ast.ControlFlowEdge:
			// Decision edges (with guards)
			sourceNode := e.findNodeByName(n.Source)
			targetNode := e.findNodeByName(n.Target)
			if sourceNode == nil {
				return fmt.Errorf("control flow edge references undefined source node")
			}
			if targetNode == nil {
				return fmt.Errorf("control flow edge references undefined target node")
			}
			e.edges[sourceNode] = append(e.edges[sourceNode], targetNode)
			
			// Store guard expression
			if n.Guard != nil {
				if e.guards[sourceNode] == nil {
					e.guards[sourceNode] = make(map[ast.Node]ast.Node)
				}
				e.guards[sourceNode][targetNode] = n.Guard
			}
		case *ast.ObjectFlowEdge:
			// Data flow edges: pin-to-pin data routing
			sourceNode, sourcePin := e.parsePinReference(n.Source)
			targetNode, targetPin := e.parsePinReference(n.Target)
			
			if sourceNode == nil {
				return fmt.Errorf("object flow edge references undefined source node")
			}
			if targetNode == nil {
				return fmt.Errorf("object flow edge references undefined target node")
			}
			
			// Store data flow: sourceNode → {targetNode, sourcePin, targetPin}
			e.dataFlows[sourceNode] = append(e.dataFlows[sourceNode], objectFlow{
				SourcePin: sourcePin,
				TargetPin: targetPin,
				Target:    targetNode,
			})
		}
	}
	
	return nil
}

// findNodeByName looks up a node by its qualified name.
func (e *ActionExecutor) findNodeByName(qname *ast.QualifiedName) ast.Node {
	if qname == nil || len(qname.Parts) == 0 {
		return nil
	}
	
	targetName := qname.Parts[len(qname.Parts)-1].Text
	for _, node := range e.nodes {
		nodeName := getNodeName(node)
		if nodeName == targetName {
			return node
		}
	}
	return nil
}

// parsePinReference extracts node and pin name from qualified reference.
// Expects format: nodeName.pinName (e.g., "action1.output")
// Returns (node, pinName). If no pin specified, returns (node, "").
func (e *ActionExecutor) parsePinReference(qname *ast.QualifiedName) (ast.Node, string) {
	if qname == nil || len(qname.Parts) == 0 {
		return nil, ""
	}
	
	// Single part: just node name, no pin
	if len(qname.Parts) == 1 {
		nodeName := qname.Parts[0].Text
		node := e.findNodeByName(&ast.QualifiedName{Parts: []ast.NameSegment{{Text: nodeName}}})
		return node, ""
	}
	
	// Two parts: nodeName.pinName
	nodeName := qname.Parts[0].Text
	pinName := qname.Parts[1].Text
	node := e.findNodeByName(&ast.QualifiedName{Parts: []ast.NameSegment{{Text: nodeName}}})
	return node, pinName
}

// getNodeName extracts the name from a behavioral node.
func getNodeName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return n.Name
	case *ast.FinalNode:
		return n.Name
	case *ast.ForkNode:
		return n.Name
	case *ast.JoinNode:
		return n.Name
	case *ast.MergeNode:
		return n.Name
	case *ast.DecisionNode:
		return n.Name
	case *ast.ActionExecutionNode:
		return n.Name
	default:
		return ""
	}
}

// initialize spawns initial token at InitialNode.
func (e *ActionExecutor) initialize() error {
	// Find initial node
	var initialNode *ast.InitialNode
	for _, node := range e.nodes {
		if n, ok := node.(*ast.InitialNode); ok {
			initialNode = n
			break
		}
	}
	
	if initialNode == nil {
		return fmt.Errorf("no initial node found in action %s", e.action.Name)
	}
	
	// Spawn initial token
	token := Token{
		ID:       e.nextTokenID,
		Location: initialNode,
		Data:     make(map[string]Value),
	}
	e.nextTokenID++
	e.tokens = append(e.tokens, token)
	
	e.state = StateRunning
	return nil
}

// stepToken advances a specific token by index.
func (e *ActionExecutor) stepToken(tokenIdx int) error {
	if tokenIdx < 0 || tokenIdx >= len(e.tokens) {
		return fmt.Errorf("invalid token index %d", tokenIdx)
	}
	
	token := &e.tokens[tokenIdx]
	
	switch node := token.Location.(type) {
	case *ast.InitialNode:
		return e.stepInitialNode(tokenIdx)
	case *ast.FinalNode:
		return e.stepFinalNode(tokenIdx)
	case *ast.ForkNode:
		return e.stepForkNode(tokenIdx)
	case *ast.JoinNode:
		return e.stepJoinNode(tokenIdx)
	case *ast.MergeNode:
		return e.stepMergeNode(tokenIdx)
	case *ast.DecisionNode:
		return e.stepDecisionNode(tokenIdx)
	case *ast.ActionExecutionNode:
		return e.stepActionExecutionNode(tokenIdx)
	default:
		return fmt.Errorf("unsupported node type: %T", node)
	}
}

// stepInitialNode advances token from initial node to successors.
func (e *ActionExecutor) stepInitialNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	successors := e.edges[token.Location]
	
	if len(successors) == 0 {
		return fmt.Errorf("initial node has no successors")
	}
	
	// Move token to first successor (initial should have exactly 1)
	token.Location = successors[0]
	return nil
}

// stepFinalNode consumes token and checks for completion.
func (e *ActionExecutor) stepFinalNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	
	// Save token data to results before consuming
	for k, v := range token.Data {
		e.results[k] = v
	}
	
	// Remove token
	e.tokens = append(e.tokens[:tokenIdx], e.tokens[tokenIdx+1:]...)
	
	// Check if all tokens consumed
	if len(e.tokens) == 0 {
		e.state = StateCompleted
	}
	
	return nil
}

// stepForkNode spawns N tokens (one per successor).
func (e *ActionExecutor) stepForkNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	node := token.Location.(*ast.ForkNode)
	
	successors := e.edges[node]
	if len(successors) == 0 {
		return fmt.Errorf("fork node %s has no successors", node.Name)
	}
	
	// Create N tokens (one per successor)
	newTokens := make([]Token, 0, len(successors))
	for _, succ := range successors {
		newToken := Token{
			ID:       e.nextTokenID,
			Location: succ,
			Data:     copyTokenData(token.Data), // Copy data to each fork
		}
		e.nextTokenID++
		newTokens = append(newTokens, newToken)
	}
	
	// Remove original token, add new tokens
	e.tokens = append(e.tokens[:tokenIdx], e.tokens[tokenIdx+1:]...)
	e.tokens = append(e.tokens, newTokens...)
	
	return nil
}

// stepJoinNode synchronizes tokens from all incoming edges.
// Waits for tokens on ALL incoming edges before firing.
func (e *ActionExecutor) stepJoinNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	node := token.Location.(*ast.JoinNode)
	
	// Get incoming edges
	incomingEdges := e.getIncomingEdges(node)
	
	// Count tokens at this join node
	tokensAtJoin := 0
	for _, t := range e.tokens {
		if t.Location == node {
			tokensAtJoin++
		}
	}
	
	// Wait until all incoming edges have tokens
	if tokensAtJoin < len(incomingEdges) {
		// Not ready yet - barrier synchronization requires ALL incoming tokens.
		// Returns nil (no-op) until all tokens arrive. Deadlock detection handled separately (Task 11).
		return nil
	}
	
	// Ready: collect all join tokens and remaining tokens
	joinTokens := make([]Token, 0, tokensAtJoin)
	remainingTokens := make([]Token, 0, len(e.tokens)-tokensAtJoin)
	
	for _, t := range e.tokens {
		if t.Location == node {
			joinTokens = append(joinTokens, t)
		} else {
			remainingTokens = append(remainingTokens, t)
		}
	}
	
	// Merge token data (last-write-wins)
	mergedData := make(map[string]Value)
	for _, t := range joinTokens {
		for k, v := range t.Data {
			mergedData[k] = v
		}
	}
	
	// Get successor
	successors := e.edges[node]
	if len(successors) == 0 {
		return fmt.Errorf("join node %s has no successors", node.Name)
	}
	if len(successors) > 1 {
		return fmt.Errorf("join node %s has multiple successors", node.Name)
	}
	
	// Create output token at successor
	outputToken := Token{
		ID:       e.nextTokenID,
		Location: successors[0],
		Data:     mergedData,
	}
	e.nextTokenID++
	
	// Replace tokens: remove join tokens, add output token
	e.tokens = append(remainingTokens, outputToken)
	
	return nil
}

// getIncomingEdges finds all nodes that have edges targeting the given node.
func (e *ActionExecutor) getIncomingEdges(node ast.Node) []ast.Node {
	incoming := make([]ast.Node, 0)
	for source, targets := range e.edges {
		for _, target := range targets {
			if target == node {
				incoming = append(incoming, source)
				break // Only count each source once
			}
		}
	}
	return incoming
}

// stepMergeNode implements OR-join semantics (first-token-wins).
func (e *ActionExecutor) stepMergeNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	mergeNode, ok := token.Location.(*ast.MergeNode)
	if !ok {
		return fmt.Errorf("expected MergeNode, got %T", token.Location)
	}
	
	// Check if merge already visited
	if e.mergeVisited[mergeNode] {
		// Discard token (first-wins)
		e.tokens = append(e.tokens[:tokenIdx], e.tokens[tokenIdx+1:]...)
		return nil
	}
	
	// Mark merge visited, pass token through
	e.mergeVisited[mergeNode] = true
	
	successors := e.edges[mergeNode]
	if len(successors) == 0 {
		return fmt.Errorf("merge node %s has no successors", mergeNode.Name)
	}
	if len(successors) > 1 {
		return fmt.Errorf("merge node %s has multiple successors (not yet supported)", mergeNode.Name)
	}
	
	token.Location = successors[0]
	return nil
}

// stepDecisionNode evaluates guards and routes token to matching branch.
func (e *ActionExecutor) stepDecisionNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	decisionNode, ok := token.Location.(*ast.DecisionNode)
	if !ok {
		return fmt.Errorf("expected DecisionNode, got %T", token.Location)
	}
	
	// Get successors (outgoing edges from decision)
	successors := e.edges[decisionNode]
	if len(successors) == 0 {
		return fmt.Errorf("decision node %s has no successors", decisionNode.Name)
	}
	
	// Evaluate guards for each successor
	ec := NewEvalContext(e.ctx, nil)
	ec.Push(token.Data) // Make token data available to guard expressions
	
	// Two-pass evaluation:
	// 1. Evaluate all guarded edges first
	// 2. If none match, use unguarded edge as fallback (else branch)
	
	var unguardedEdge ast.Node
	
	// Pass 1: Check guarded edges
	for _, succ := range successors {
		// Get guard for this edge (if any)
		var guard ast.Node
		if guards, ok := e.guards[decisionNode]; ok {
			guard = guards[succ]
		}
		
		// No guard = remember for fallback
		if guard == nil {
			unguardedEdge = succ
			continue
		}
		
		// Evaluate guard
		result, err := ec.Eval(guard)
		if err != nil {
			return fmt.Errorf("eval guard: %w", err)
		}
		
		// Guard must be boolean
		if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
			return fmt.Errorf("decision node %s: guard must evaluate to boolean, got %v", decisionNode.Name, result.Kind)
		}
		
		// Check if guard is true
		if result.Const.Bool {
			token.Location = succ
			return nil
		}
	}
	
	// Pass 2: Use unguarded edge as fallback
	if unguardedEdge != nil {
		token.Location = unguardedEdge
		return nil
	}
	
	return fmt.Errorf("decision node %s: no true guard", decisionNode.Name)
}

// copyTokenData creates a shallow copy of token data map.
// This is sufficient as Value structs are copied by value, and pointer
// fields (Sequence, Set) are intended to be shared across forked tokens.
func copyTokenData(data map[string]Value) map[string]Value {
	copy := make(map[string]Value)
	for k, v := range data {
		copy[k] = v
	}
	return copy
}

// stepActionExecutionNode evaluates inline expression or invokes nested action.
func (e *ActionExecutor) stepActionExecutionNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	node, ok := token.Location.(*ast.ActionExecutionNode)
	if !ok {
		return fmt.Errorf("expected ActionExecutionNode, got %T", token.Location)
	}
	
	if node.Expression != nil {
		// Evaluate inline expression
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(token.Data) // Make token data available
		result, err := ec.Eval(node.Expression)
		if err != nil {
			return fmt.Errorf("eval expression: %w", err)
		}
		
		// Store result: check if dataFlows specify output pin, else use "result"
		outputPin := "result"
		if flows, ok := e.dataFlows[node]; ok && len(flows) > 0 {
			// Use source pin from first data flow as output pin
			if flows[0].SourcePin != "" {
				outputPin = flows[0].SourcePin
			}
		}
		token.Data[outputPin] = result
	} else if node.ActionRef != nil {
		return fmt.Errorf("nested action invocation not yet implemented")
	}
	
	// Advance to successor
	successors := e.edges[token.Location]
	if len(successors) == 0 {
		return fmt.Errorf("action node %s has no successors", node.Name)
	}
	if len(successors) > 1 {
		return fmt.Errorf("action node %s has multiple successors (decision nodes not yet supported)", node.Name)
	}
	
	// Apply data flows: transfer data from this node's output pins to target input pins
	e.applyDataFlows(token, node)
	
	token.Location = successors[0]
	return nil
}

// applyDataFlows transfers data along object flow edges.
// Copies data from source pins to target pins for all outgoing data flows.
func (e *ActionExecutor) applyDataFlows(token *Token, sourceNode ast.Node) {
	flows, ok := e.dataFlows[sourceNode]
	if !ok || len(flows) == 0 {
		return
	}
	
	for _, flow := range flows {
		// Get data from source pin
		sourceData, ok := token.Data[flow.SourcePin]
		if !ok {
			// No data at source pin - skip this flow
			continue
		}
		
		// Store in target pin (will be available when token reaches target)
		token.Data[flow.TargetPin] = sourceData
	}
}

// --- Public accessor methods for REPL debugging ---

// Tokens returns a copy of active tokens.
func (e *ActionExecutor) Tokens() []Token {
	tokens := make([]Token, len(e.tokens))
	copy(tokens, e.tokens)
	return tokens
}

// State returns current execution state.
func (e *ActionExecutor) State() ExecutionState {
	return e.state
}

// Results returns accumulated results from final nodes.
func (e *ActionExecutor) Results() map[string]Value {
	return e.results
}

// SetBreakpoint adds a breakpoint at the given node name.
func (e *ActionExecutor) SetBreakpoint(nodeName string) {
	e.breakpoints[nodeName] = true
}

// ClearBreakpoints removes all breakpoints.
func (e *ActionExecutor) ClearBreakpoints() {
	e.breakpoints = make(map[string]bool)
}

// ActionSymbol returns the action being executed.
func (e *ActionExecutor) ActionSymbol() *symbols.Symbol {
	return e.action
}
