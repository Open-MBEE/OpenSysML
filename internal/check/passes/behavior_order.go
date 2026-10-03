package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const behaviorOrderSource = "constraint"

// BehaviorOrderPass validates executable orders declared outside behavior bodies.
type BehaviorOrderPass struct{}

func (BehaviorOrderPass) Level() kit.PassLevel { return kit.LevelConstraint }

func (BehaviorOrderPass) Run(ctx *kit.Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	scope := ctx.Index.DocumentRoot(name)
	if scope == nil {
		return nil
	}
	model := ctx.Model()
	orders := lower.BehaviorOrders(model, scope)
	var diags []diag.Diagnostic
	for _, order := range orders {
		if order.Refusal == nil || ctx.DownstreamOfFailure(order.Decl) {
			continue
		}
		if order.Refusal.Code == "succession-orders-nothing" &&
			!lower.BehaviorOrderHasExecutableEnd(model, order) {
			continue
		}
		severity := diag.SeverityWarning
		message := order.Refusal.Reason
		if order.Refusal.Code == "connector-type-featuring" {
			severity = diag.SeverityError
			message = msgConnectorTypeFeaturing
		}
		diags = append(diags, diag.Diagnostic{
			Severity: severity,
			Span:     order.Refusal.Span,
			Message:  message,
			Code:     order.Refusal.Code,
			Source:   behaviorOrderSource,
		})
	}
	for _, order := range behaviorOrderCycles(model, orders) {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Span:     order.Decl.Span(),
			Message:  "the succession orders performances in a cycle",
			Code:     "succession-order-cycle",
			Source:   behaviorOrderSource,
		})
	}
	return diags
}

func behaviorOrderCycles(model *semantics.Model, orders []lower.BehaviorOrder) []lower.BehaviorOrder {
	if model == nil {
		return nil
	}
	featuringTypes := make(map[*symbols.Symbol]bool)
	for _, order := range orders {
		if order.Refusal == nil {
			featuringTypes[order.Featuring] = true
		}
	}
	seenDecls := make(map[ast.Node]bool)
	var cycles []lower.BehaviorOrder
	for featuring := range featuringTypes {
		var group []lower.BehaviorOrder
		for _, order := range orders {
			if order.Refusal != nil {
				continue
			}
			if featuring == nil && order.Featuring != nil ||
				featuring != nil && (order.Featuring == nil || !model.Conforms(featuring, order.Featuring)) {
				continue
			}
			group = append(group, order)
		}
		for i, order := range group {
			if len(order.Earlier.Path) == 0 || len(order.Later.Path) == 0 ||
				!behaviorOrderHasPath(model, group, order.Later, order.Earlier) {
				continue
			}
			if !seenDecls[order.Decl] {
				seenDecls[order.Decl] = true
				cycles = append(cycles, group[i])
			}
		}
	}
	return cycles
}

func behaviorOrderHasPath(model *semantics.Model, orders []lower.BehaviorOrder, start, target lower.BehaviorOrderEnd) bool {
	if len(start.Path) == 0 || len(target.Path) == 0 {
		return false
	}
	var seen []lower.BehaviorOrderEnd
	var visit func(lower.BehaviorOrderEnd) bool
	visit = func(current lower.BehaviorOrderEnd) bool {
		if behaviorOrderSameEnd(model, current, target) {
			return true
		}
		for _, previous := range seen {
			if behaviorOrderSameEnd(model, previous, current) {
				return false
			}
		}
		seen = append(seen, current)
		for _, order := range orders {
			if !behaviorOrderSameEnd(model, order.Earlier, current) {
				continue
			}
			if visit(order.Later) {
				return true
			}
		}
		return false
	}
	return visit(start)
}

func behaviorOrderSameEnd(model *semantics.Model, left, right lower.BehaviorOrderEnd) bool {
	if len(left.Path) != len(right.Path) {
		return false
	}
	for i := range left.Path {
		if !behaviorOrderSameFeature(model, left.Path[i], right.Path[i]) {
			return false
		}
	}
	return true
}

func behaviorOrderSameFeature(model *semantics.Model, left, right *symbols.Symbol) bool {
	if left == nil || right == nil {
		return false
	}
	if symbols.SameElement(left, right) {
		return true
	}
	for _, target := range model.AllRedefinedFeatures(left) {
		if symbols.SameElement(target, right) {
			return true
		}
	}
	for _, target := range model.AllRedefinedFeatures(right) {
		if symbols.SameElement(target, left) {
			return true
		}
	}
	return false
}
