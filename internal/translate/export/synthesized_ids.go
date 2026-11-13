package export

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// SynthesizedElementID is the elementId the conversion writes for sym, an element
// the semantic side tables synthesize; ownerID is the elementId of any element.
func SynthesizedElementID(sym *symbols.Symbol, ownerID func(*symbols.Symbol) string) (string, bool) {
	if sym == nil || sym.Expr == nil {
		return "", false
	}
	part := sym.Expr
	owner := ownerID(part.Owner)
	if part.Role == symbols.ExpressionArgument && part.Owner != nil && part.Owner.Expr != nil && part.Owner.Expr.Role == symbols.ExpressionParameter {
		// An operand is minted off the expression, though its parameter owns it.
		owner = ownerID(part.Owner.Expr.Owner)
	}
	if owner == "" {
		return "", false
	}
	switch part.Role {
	case symbols.ExpressionValue:
		return rdf.ExpressionNodeID(owner, pValue), true
	case symbols.ExpressionArgument:
		return rdf.ExpressionNodeID(owner, fmt.Sprintf("a%d", part.Index)), true
	case symbols.ExpressionParameter:
		return rdf.ExpressionNodeID(owner, fmt.Sprintf("in%d", part.Index)), true
	case symbols.ExpressionResult:
		return rdf.ExpressionNodeID(owner, "out"), true
	case symbols.ExpressionChangeWrapper:
		return rdf.ExpressionNodeID(rdf.ExpressionNodeID(owner, pValue), "a0"), true
	case symbols.ExpressionChangeCondition:
		return rdf.ExpressionNodeID(owner, "change"), true
	case symbols.ExpressionBody:
		return rdf.ExpressionNodeID(owner, "body"), true
	case symbols.ExpressionBodyResult:
		return rdf.ExpressionNodeID(owner, "result"), true
	case symbols.ExpressionChainFeature:
		if part.Slot == "" {
			return "", false
		}
		return rdf.ExpressionNodeID(owner, part.Slot), true
	case symbols.ExpressionGuard:
		return rdf.ExpressionNodeID(owner, xGuard), true
	case symbols.ExpressionTriggerAction:
		return memberElementID(owner, naming.TriggerSegment), true
	case symbols.ExpressionPayload:
		return memberElementID(owner, naming.Qualify("", naming.ParamName(sym.Decl), part.Index)), true
	case symbols.ExpressionTransitionSuccession:
		return owner + transitionSuccessionSuffix, true
	}
	return "", false
}

// memberElementID is the elementId of the member segment declares under the
// element with elementId owner, as EncodeElementID spells the qualified name.
func memberElementID(owner, segment string) string {
	return owner + "__" + rdf.EncodeElementID(segment)
}
