package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// An action node owning a flow runs it as subperformances (`subactions :> subperformances`):
// a token moves into the node's performance, and the node completes when its last token retires.

// graphOf returns the flow a performance runs.
func (e *ActionExecutor) graphOf(frame *actionFrame) *lower.ActionGraph {
	return frame.graph
}

// tokenGraph returns the flow the token at this index is running in.
func (e *ActionExecutor) tokenGraph(tokenIdx int) *lower.ActionGraph {
	return e.graphOf(e.tokens[tokenIdx].frame)
}

// subflowOf returns the flow a node owns, and whether it owns one.
func (e *performances) subflowOf(graph *lower.ActionGraph, node ast.Node) (*lower.Subflow, bool) {
	sub, owns := graph.Subflows[node]
	return sub, owns && sub != nil
}

// enterSubflow moves a token into the flow its node owns, run by the node's performance;
// the flow was validated at initialize(), so an unbuildable one is an error here.
func (e *ActionExecutor) enterSubflow(tokenIdx int, perf *actionFrame) error {
	token := &e.tokens[tokenIdx]
	node := token.Location
	if perf.graph == nil || perf.graph.Initial == nil {
		return fmt.Errorf("%w: action node %s owns a flow that cannot be built",
			ErrInvalidActionFlow, ActionNodeName(node))
	}
	token.frame = perf
	token.Location = perf.graph.Initial
	token.Via = lower.ActionEdge{}
	token.moved = e.sweep
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeEnter(ActionNodeName(node))
	}
	return nil
}

// runSubflow performs the flow perf owns to completion where a body statement,
// not a token of the enclosing flow, performs its node: the tokens of that flow
// alone are stepped until its last one retires, pausing where a breakpoint is
// met as RunToCompletion does. Nothing outside can post a message meanwhile, so
// a token parked at an accept is a deadlock, as under RunToCompletion.
func (e *ActionExecutor) runSubflow(perf *actionFrame) error {
	node := perf.node
	if perf.graph == nil || perf.graph.Initial == nil {
		return fmt.Errorf("%w: %s owns a flow that cannot be built",
			ErrInvalidActionFlow, perf.describe())
	}
	perf.inBody = true
	e.tokens = append(e.tokens, Token{ID: e.nextTokenID, Location: perf.graph.Initial, frame: perf})
	e.nextTokenID++
	// The root performance, a case body's own flow, has no node and is traced by name.
	name := ActionNodeName(node)
	if node == nil {
		name = perf.describe()
	}
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeEnter(name)
	}
	for perf.live > 0 {
		if name := e.breakpointHit(); name != "" {
			if err := e.pauseRun(name); err != nil {
				return err
			}
		}
		if err := e.chargeActionStep(); err != nil {
			return err
		}
		moved, err := e.stepSubflow(perf)
		if err != nil {
			return err
		}
		if moved {
			continue
		}
		if len(e.waitingTokens(perf)) > 0 {
			return e.deadlockError(perf)
		}
		return fmt.Errorf("%w: %d token(s) stuck in %s, no progress made",
			ErrActionDeadlock, len(e.tokensIn(perf)), perf.describe())
	}
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeExit(name)
	}
	return nil
}

// stepSubflow steps every token of perf's flow once and reports whether any moved,
// which a retired, forked or relocated token did.
func (e *ActionExecutor) stepSubflow(perf *actionFrame) (bool, error) {
	before := e.subflowLocations(perf)
	defer e.beginSweep()()
	order := e.beginStepOrder()
	defer e.beginStepWrites(e.stepCount + 1)()
	var err error
	for i := len(e.tokens) - 1; i >= 0 && err == nil; i-- {
		if i >= len(e.tokens) || e.moving(e.tokens[i]) || !e.tokens[i].inFlowOf(perf) {
			continue
		}
		err = e.stepTokenNoting(i, &order)
	}
	e.noteTokenOrder(e.stepCount+1, order)
	if err != nil {
		return false, err
	}
	after := e.subflowLocations(perf)
	if len(after) != len(before) {
		return true, nil
	}
	for id, location := range after {
		if before[id] != location {
			return true, nil
		}
	}
	return false, nil
}

// subflowLocations returns where each token of perf's flow sits, by token ID.
func (e *ActionExecutor) subflowLocations(perf *actionFrame) map[int64]ast.Node {
	locations := make(map[int64]ast.Node)
	for _, idx := range e.tokensIn(perf) {
		locations[e.tokens[idx].ID] = e.tokens[idx].Location
	}
	return locations
}

// tokensIn returns the indices of the tokens running in perf's flow or one nested
// in it; every token's for nil.
func (e *ActionExecutor) tokensIn(perf *actionFrame) []int {
	var indices []int
	for i := range e.tokens {
		if e.tokens[i].inFlowOf(perf) {
			indices = append(indices, i)
		}
	}
	return indices
}

// inFlowOf reports whether the token runs in perf's flow or one nested in it;
// every token does of nil.
func (t Token) inFlowOf(perf *actionFrame) bool {
	if perf == nil {
		return true
	}
	for f := t.frame; f != nil; f = f.parent {
		if f == perf {
			return true
		}
	}
	return false
}

// positionIn returns the node of perf's flow the token stands at: its location, or the node
// owning the flow nested under perf it runs in; false for a token outside perf's flow.
func (t Token) positionIn(perf *actionFrame) (ast.Node, bool) {
	if t.frame == perf {
		return t.Location, true
	}
	for f := t.frame; f != nil; f = f.parent {
		if f.parent == perf {
			return f.node, true
		}
	}
	return nil, false
}

// leaveSubflow returns a token to the node whose flow has just completed, ends
// that node's performance and takes the node's own succession.
func (e *ActionExecutor) leaveSubflow(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	frame := token.frame
	token.frame = frame.parent
	token.Location = frame.node
	token.Via = lower.ActionEdge{}
	token.Wait = nil
	if tr := e.trace(); tr != nil {
		tr.RecordActionNodeExit(ActionNodeName(frame.node))
	}
	if err := e.endPerformance(frame); err != nil {
		return err
	}
	return e.completeNode(tokenIdx, frame)
}

// validateSubflows reports a nested node whose own flow could not be built, in
// graph's flow or in a block flow a body of it states. It runs at initialize(),
// not at construction, per the error-timing contract.
func (e *ActionExecutor) validateSubflows(graph *lower.ActionGraph) error {
	for _, node := range graph.Nodes {
		if sub, owns := e.subflowOf(graph, node); owns {
			if sub.Err != nil {
				return fmt.Errorf("%w: action node %s: %w",
					ErrInvalidActionFlow, ActionNodeName(node), sub.Err)
			}
			if sub.Graph.Initial == nil {
				return fmt.Errorf("%w: no initial node found in action node %s",
					ErrInvalidActionFlow, ActionNodeName(node))
			}
			if err := e.validateSubflows(sub.Graph); err != nil {
				return err
			}
		}
		for _, block := range lower.BlockFlows(graph.Bodies[node]) {
			if err := e.validateSubflows(block); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkResultParameters refuses an action, or a node of its flow, declaring a
// `return` parameter — only a function or expression owns one.
func (e *ActionExecutor) checkResultParameters() error {
	for _, param := range e.ctx.model.BehaviorParametersOf(e.action) {
		if param.IsResult {
			return fmt.Errorf("%w: action %s declares `return %s`; write `out %s`",
				ErrActionResultParameter, symbolText(e.action), param.Symbol.Name, param.Symbol.Name)
		}
	}
	return e.checkNodeResultParameters(e.graph)
}

func (e *ActionExecutor) checkNodeResultParameters(graph *lower.ActionGraph) error {
	if graph == nil {
		return nil
	}
	for _, node := range graph.Nodes {
		for _, f := range graph.Features[node] {
			if f.IsResult {
				return fmt.Errorf("%w: action node %s declares `return %s`; write `out %s`",
					ErrActionResultParameter, ActionNodeName(node), f.Name, f.Name)
			}
		}
		if sub, owns := e.subflowOf(graph, node); owns && sub.Graph != nil {
			if err := e.checkNodeResultParameters(sub.Graph); err != nil {
				return err
			}
		}
		for _, block := range lower.BlockFlows(graph.Bodies[node]) {
			if err := e.checkNodeResultParameters(block); err != nil {
				return err
			}
		}
	}
	return nil
}

// subflowNodeNames returns the names of the nodes of every flow nested under
// graph — the flows its nodes own and the block flows their bodies state — so
// a debugger can break on a step of a nested flow.
func (e *ActionExecutor) subflowNodeNames(graph *lower.ActionGraph) []string {
	var names []string
	for _, node := range graph.Nodes {
		if sub, owns := e.subflowOf(graph, node); owns && sub.Graph != nil {
			names = append(names, e.flowNodeNames(sub.Graph)...)
		}
		for _, block := range lower.BlockFlows(graph.Bodies[node]) {
			names = append(names, e.flowNodeNames(block)...)
		}
	}
	return names
}

// flowNodeNames returns the names of graph's nodes, then of the flows nested under
// it; a run of statements is a step of a block's flow but no node to break on.
func (e *ActionExecutor) flowNodeNames(graph *lower.ActionGraph) []string {
	var names []string
	for _, node := range graph.Nodes {
		if graph.StatementRuns[node] {
			continue
		}
		names = append(names, ActionNodeNames(node)...)
	}
	return append(names, e.subflowNodeNames(graph)...)
}

// joinConnections appends the connectors a nested flow declares to those around it.
func joinConnections(outer, inner []lower.Connection) []lower.Connection {
	if len(inner) == 0 {
		return outer
	}
	joined := make([]lower.Connection, 0, len(outer)+len(inner))
	joined = append(joined, outer...)
	return append(joined, inner...)
}
