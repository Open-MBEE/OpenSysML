package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// StateBehavior is one behavior a state machine performs: an entry, do or exit
// behavior of a state, or one effect of a transition. Every form it may be
// written in — an inline action body, an assignment, a send, a performed action
// — is lowered into the statements the runtime executes.
type StateBehavior struct {
	// Node is the behavior's declaration, unwrapped of the membership the parser
	// puts it in, and is what a diagnostic about it names.
	Node ast.Node
	// Name is the name the behavior was declared with, "" for an anonymous one.
	Name string
	// Body are the behavior's statements in declaration order. An inline action
	// body is one block, since it is a namespace of its own; every other form
	// lowers to the one statement it states.
	Body []Statement
	// Nodes are the action nodes the blocks of Body declare, in declaration order,
	// each of which performs as a subperformance of the behavior's.
	Nodes []ast.Node
	// Scope is the scope the behavior was declared in, which its own statements
	// and the action name it performs resolve in.
	Scope *symbols.Scope
	// Owner is the state whose attributes the behavior reads and writes: the
	// state it belongs to, or the source of the transition it is an effect of.
	Owner *ast.StateNode
}

// StateBehaviors are the lowered entry, do and exit behaviors of one state, each
// in declaration order.
type StateBehaviors struct {
	Entry []StateBehavior
	Do    []StateBehavior
	Exit  []StateBehavior
}

// LowerBehaviors lowers the actions of an entry, do or exit member, or the
// effects of a transition, in the scope they were declared in.
func LowerBehaviors(actions []ast.Node, scope *symbols.Scope) []StateBehavior {
	if len(actions) == 0 {
		return nil
	}
	behaviors := make([]StateBehavior, 0, len(actions))
	for _, action := range actions {
		if actual := unwrapMembership(action); actual != nil {
			behaviors = append(behaviors, lowerStateBehavior(actual, scope))
		}
	}
	return behaviors
}

// lowerStateBehavior lowers one behavior into the statements it states: the
// statements of an inline action body, or the one statement every other form is.
func lowerStateBehavior(action ast.Node, scope *symbols.Scope) StateBehavior {
	behavior := StateBehavior{Node: action, Scope: scope}
	switch node := action.(type) {
	case *ast.Usage:
		behavior.Name, _ = ast.EffectiveName(node)
		switch {
		case node.Kind == ast.UsageAction && node.HasBody && performsAction(node) && declaresOnlyFeatures(node.Members):
			// The body binds the pins of the action performed: the usage is the one
			// node of the behavior's flow, performed as a node of an action body is.
			behavior.Body = []Statement{Block{
				Node:  node,
				Scope: scope,
				Graph: lowerBlockFlow([]ast.Node{node}, scope, false),
				Own:   true,
			}}
		case node.Kind == ast.UsageAction && node.HasBody && performsAction(node):
			// Which of the two the behavior performs would be a silent pick.
			behavior.Body = []Statement{Unsupported{
				Description: "performing an action and stating a body of its own",
				Node:        node,
				Scope:       scope,
			}}
		case node.Kind == ast.UsageAction && node.HasBody:
			// The body is a namespace of its own, so its locals are declared in the
			// block's frame rather than in the state machine's data.
			behavior.Body = []Statement{lowerBehaviorBody(node, childScope(scope, node))}
		default:
			behavior.Body = []Statement{Effect{Kind: EffectPerform, Node: node, Scope: scope}}
		}
	case *ast.ActionExecutionNode:
		behavior.Name = node.Name
		behavior.Body = lowerActionExecution(node, scope)
	default:
		behavior.Body = []Statement{lowerStatement(action, scope)}
	}
	behavior.Nodes = blockNodesOf(behavior.Body, nil)
	return behavior
}

// lowerBehaviorBody lowers an inline action body of a behavior. A body stating
// successions or control nodes is the token flow a standalone action's body is
// (ToActionGraph), starting at its one unpreceded node where no `first` says;
// one stating none runs its statements in declaration order.
func lowerBehaviorBody(node *ast.Usage, scope *symbols.Scope) Statement {
	if !statesOwnFlow(node.Members) {
		return lowerBlock(node, node.Members, scope)
	}
	graph, err := ToActionGraph(node, scope)
	if err != nil {
		return Unsupported{
			Description: "the flow the body states: " + err.Error(),
			Node:        node,
			Scope:       scope,
		}
	}
	StartFlow(graph)
	return Block{Node: node, Scope: scope, Graph: graph, Own: true, Stated: true}
}

// lowerActionExecution lowers the step form of a behavior: an expression whose
// value the step names, or a reference to the action it performs.
func lowerActionExecution(node *ast.ActionExecutionNode, scope *symbols.Scope) []Statement {
	switch {
	case node.Expression != nil:
		return []Statement{Declare{Name: node.Name, Value: node.Expression, Node: node, Scope: scope}}
	case node.ActionRef != nil:
		return []Statement{Effect{Kind: EffectPerform, Node: node, Scope: scope}}
	default:
		// A step stating neither states no behavior, which executes as nothing.
		return []Statement{}
	}
}

// declaresOnlyFeatures reports whether a body declares parameters and attributes
// alone (`inout n = ticks;`), stating no step of its own.
func declaresOnlyFeatures(members []ast.Node) bool {
	for _, member := range members {
		actual := unwrapMembership(member)
		if actual == nil || statesNoStep(actual) {
			continue
		}
		if m, ok := actual.(*ast.Usage); !ok || !DeclaresNodeFeature(m) {
			return false
		}
	}
	return true
}

// performsAction reports whether a nested action usage names the action it
// performs — by typing, by reference subsetting or by an invocation — rather than
// stating its own body.
func performsAction(usage *ast.Usage) bool {
	if usage.PerformedInvocation() != nil {
		return true
	}
	for _, rel := range usage.Relationships {
		if rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences {
			continue
		}
		if _, ok := rel.Target.(*ast.QualifiedName); ok {
			return true
		}
	}
	return false
}
