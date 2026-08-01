package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
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
	
	// Graph structure
	nodes       []ast.Node
	edges       map[ast.Node][]ast.Node          // Successor edges
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
		edges:        make(map[ast.Node][]ast.Node),
		dataFlows:    make(map[ast.Node][]objectFlow),
		mergeVisited: make(map[ast.Node]bool),
	}
	
	// Extract graph structure from AST
	if err := exec.extractGraph(); err != nil {
		return nil, fmt.Errorf("extract graph: %w", err)
	}
	
	return exec, nil
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
		case *ast.ObjectFlowEdge:
			// Data flow edges (deferred to Task 9)
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
		token.Data["result"] = result
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
	token.Location = successors[0]
	return nil
}
