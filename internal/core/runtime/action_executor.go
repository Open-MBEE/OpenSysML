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
