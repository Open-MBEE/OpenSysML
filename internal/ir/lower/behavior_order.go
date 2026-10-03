package lower

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// BehaviorOrder is a succession outside a behavior body, ordering performances of the objects it relates.
type BehaviorOrder struct {
	Decl      ast.Node
	Name      string
	File      string
	Span      source.Span
	Featuring *symbols.Symbol
	Earlier   BehaviorOrderEnd
	Later     BehaviorOrderEnd
	Refusal   *BehaviorOrderRefusal
}

// BehaviorOrderEnd is one endpoint relative to its featuring instance or package.
type BehaviorOrderEnd struct {
	Path         []*symbols.Symbol
	Multiplicity *ast.Multiplicity
	Span         source.Span
}

// BehaviorOrderRefusal records why a lowered order cannot constrain execution.
type BehaviorOrderRefusal struct {
	Code   string
	Reason string
	Span   source.Span
}

// BehaviorOrders lowers succession declarations owned outside behavior bodies.
func BehaviorOrders(model *semantics.Model, roots ...*symbols.Scope) []BehaviorOrder {
	if model == nil {
		return nil
	}
	seen := make(map[ast.Node]bool)
	var orders []BehaviorOrder
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil || !model.BehaviorSuccessionOwner(scope.Owner()) {
			return
		}
		owner := scope.Owner()
		for _, succession := range model.DeclaredSuccessions(scope, owner, behaviorOrderMembers(scope.Node())) {
			if seen[succession.Decl] || !isBehaviorOrderSuccession(succession.Decl) {
				continue
			}
			seen[succession.Decl] = true
			left, right, ok := behaviorOrderSyntaxEnds(succession.Decl)
			if !ok {
				continue
			}
			leftPath := model.SuccessionEndPath(scope, owner, left.node)
			rightPath := model.SuccessionEndPath(scope, owner, right.node)
			if len(leftPath) == 0 && succession.Source.Symbol != nil {
				leftPath = []*symbols.Symbol{succession.Source.Symbol}
			}
			if len(rightPath) == 0 && succession.Target.Symbol != nil {
				rightPath = []*symbols.Symbol{succession.Target.Symbol}
			}
			if len(leftPath) == 0 && succession.Source.Node != nil {
				if sym := scope.MemberDeclaring(succession.Source.Node); sym != nil {
					leftPath = []*symbols.Symbol{sym}
				}
			}
			if len(rightPath) == 0 && succession.Target.Node != nil {
				if sym := scope.MemberDeclaring(succession.Target.Node); sym != nil {
					rightPath = []*symbols.Symbol{sym}
				}
			}

			featuring, wellFormed := model.BehaviorSuccessionFeaturingType(
				owner, [][]*symbols.Symbol{leftPath, rightPath}, left.node, right.node)
			file := scope.DocName()
			if owner != nil && owner.DocName != "" {
				file = owner.DocName
			}
			order := BehaviorOrder{
				Decl:      succession.Decl,
				Name:      behaviorOrderName(succession.Decl),
				File:      file,
				Span:      succession.Decl.Span(),
				Featuring: featuring,
				Earlier: BehaviorOrderEnd{
					Path:         leftPath,
					Multiplicity: succession.Source.Multiplicity,
					Span:         succession.Source.Span,
				},
				Later: BehaviorOrderEnd{
					Path:         rightPath,
					Multiplicity: succession.Target.Multiplicity,
					Span:         succession.Target.Span,
				},
			}
			if order.Earlier.Span == (source.Span{}) && left.node != nil {
				order.Earlier.Span = left.node.Span()
			}
			if order.Later.Span == (source.Span{}) && right.node != nil {
				order.Later.Span = right.node.Span()
			}
			if !wellFormed {
				order.Refusal = &BehaviorOrderRefusal{
					Code:   "connector-type-featuring",
					Reason: "the succession's ends have no common featuring type",
					Span:   succession.Decl.Span(),
				}
				orders = append(orders, order)
				continue
			}
			if reason, end := behaviorOrderNothingReason(model, leftPath, rightPath); reason != "" {
				order.Refusal = &BehaviorOrderRefusal{
					Code:   "succession-orders-nothing",
					Reason: reason,
					Span:   behaviorOrderNodeSpan(end),
				}
				orders = append(orders, order)
				continue
			}
			if err := CheckBehaviorOrderMultiplicity(scope, left.node, right.node,
				order.Earlier.Multiplicity, order.Later.Multiplicity, succession.Decl, model); err != nil {
				var refusal *StepMultiplicityError
				if !errors.As(err, &refusal) {
					continue
				}
				order.Refusal = &BehaviorOrderRefusal{
					Code:   refusal.Code,
					Reason: refusal.Reason,
					Span:   succession.Decl.Span(),
				}
			}
			orders = append(orders, order)
		}
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	return orders
}

func behaviorOrderName(decl ast.Node) string {
	switch n := decl.(type) {
	case *ast.Usage:
		name, _ := ast.EffectiveName(n)
		return name
	case *ast.TransitionMember:
		return n.Name
	default:
		return ""
	}
}

func behaviorOrderNodeSpan(node ast.Node) source.Span {
	if node == nil {
		return source.Span{}
	}
	return node.Span()
}

type behaviorOrderSyntaxEnd struct {
	node ast.Node
}

func isBehaviorOrderSuccession(decl ast.Node) bool {
	switch n := decl.(type) {
	case *ast.Usage:
		return n.Kind == ast.UsageSuccession && len(n.ConnectorEnds) == 2 || n.IsSuccessionFlow()
	case *ast.InitialNode:
		return n.Successor != nil
	case *ast.SuccessionEdge, *ast.ControlFlowEdge:
		return true
	case *ast.TransitionMember:
		return n.Target != nil
	default:
		return false
	}
}

func behaviorOrderSyntaxEnds(decl ast.Node) (behaviorOrderSyntaxEnd, behaviorOrderSyntaxEnd, bool) {
	switch n := decl.(type) {
	case *ast.Usage:
		if n.Kind == ast.UsageSuccession && len(n.ConnectorEnds) == 2 {
			return behaviorOrderSyntaxEnd{node: n.ConnectorEnds[0]},
				behaviorOrderSyntaxEnd{node: n.ConnectorEnds[1]}, true
		}
		if n.IsSuccessionFlow() && n.FlowEnds != nil {
			return behaviorOrderSyntaxEnd{node: n.FlowEnds.From},
				behaviorOrderSyntaxEnd{node: n.FlowEnds.To}, true
		}
	case *ast.InitialNode:
		if n.Successor != nil {
			return behaviorOrderSyntaxEnd{node: n.First},
				behaviorOrderSyntaxEnd{node: n.Successor}, true
		}
	case *ast.SuccessionEdge:
		left, right := ast.Node(n.Source), ast.Node(n.Target)
		if n.SourceMember != nil {
			left = n.SourceMember
		}
		if n.TargetMember != nil {
			right = n.TargetMember
		}
		return behaviorOrderSyntaxEnd{node: left}, behaviorOrderSyntaxEnd{node: right}, true
	case *ast.ControlFlowEdge:
		left, right := ast.Node(n.Source), ast.Node(n.Target)
		if n.SourceMember != nil {
			left = n.SourceMember
		}
		if n.TargetMember != nil {
			right = n.TargetMember
		}
		return behaviorOrderSyntaxEnd{node: left}, behaviorOrderSyntaxEnd{node: right}, true
	case *ast.TransitionMember:
		return behaviorOrderSyntaxEnd{node: n.Source},
			behaviorOrderSyntaxEnd{node: n.Target}, n.Target != nil
	}
	return behaviorOrderSyntaxEnd{}, behaviorOrderSyntaxEnd{}, false
}

func behaviorOrderMembers(node ast.Node) []ast.Node {
	switch n := node.(type) {
	case *ast.RootNamespace:
		return n.Members
	case *ast.Package:
		return n.Members
	case *ast.Namespace:
		return n.Members
	case *ast.Definition:
		return n.Members
	case *ast.Usage:
		return n.Members
	default:
		return nil
	}
}

func behaviorOrderNothingReason(model *semantics.Model, earlier, later []*symbols.Symbol) (string, ast.Node) {
	for _, path := range [][]*symbols.Symbol{earlier, later} {
		if len(path) == 0 || behaviorOrderExecutable(model, path) {
			continue
		}
		end := path[len(path)-1]
		name, kind := end.Name, end.Kind.Family()
		if end.Kind.IsDefinition() {
			kind += " definition"
		} else {
			kind += " usage"
		}
		reason := name + " is a " + kind + "; it does not bind an object behavior performance"
		if end.Kind == symbols.SymbolRequirementUsage || end.Kind == symbols.SymbolRequirementDef {
			reason = name + " is a " + kind + "; its evaluations are checked as verdicts and are not occurrences of a run"
		}
		return reason, end.Decl
	}
	return "", nil
}

// BehaviorOrderHasExecutableEnd reports whether an order can name an object behavior.
func BehaviorOrderHasExecutableEnd(model *semantics.Model, order BehaviorOrder) bool {
	return behaviorOrderExecutable(model, order.Earlier.Path) ||
		behaviorOrderExecutable(model, order.Later.Path)
}

func behaviorOrderExecutable(model *semantics.Model, path []*symbols.Symbol) bool {
	if len(path) == 0 {
		return false
	}
	sym := path[len(path)-1]
	if sym == nil || sym.Decl == nil || model == nil || model.IsSelf(sym) {
		return false
	}
	_, classifier := ClassifierBehaviorOf(sym.Decl)
	if classifier {
		return true
	}
	_, startable := StartableBehaviorOf(sym.Decl)
	if !startable {
		return false
	}
	owner := sym.Owner()
	return owner != nil &&
		owner.Kind != symbols.SymbolPackage &&
		owner.Kind != symbols.SymbolNamespace &&
		!model.IsBehaviorType(owner)
}
