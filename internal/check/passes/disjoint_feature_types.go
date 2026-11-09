package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// checkDisjointFeatureTypes reports a feature two of whose types (its typings' and
// its subsetted features', KerML 1.1 §8.3.3.3.4) are disjoint, so it has no values.
func (cc *constraintChecker) checkDisjointFeatureTypes(sym *symbols.Symbol) {
	if sym == nil || !sym.IsFeature() || sym.Recorded() {
		return
	}
	types := cc.model.FeatureTypeSet(sym)
	for i, a := range types {
		for _, b := range types[i+1:] {
			if !cc.model.Disjoint(a, b) {
				continue
			}
			cc.diags = append(cc.diags, diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Span:     nameSpanOf(sym),
				Message: fmt.Sprintf(
					"%s is typed by %s and %s, which are disjoint, so it can have no values",
					disjointSubject(sym), a.Name, b.Name),
				Code:   "disjoint-feature-types",
				Source: "constraint",
			})
			return
		}
	}
}

// disjointSubject names the feature reported; a connector's end, which has no
// name of its own, is named by its connector and the feature it attaches to.
func disjointSubject(sym *symbols.Symbol) string {
	if sym.Name != "" {
		return sym.Name
	}
	if sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil {
		return "an unnamed feature"
	}
	owner := sym.OwnerScope.Owner()
	if u, ok := owner.Decl.(*ast.Usage); !ok || !u.Kind.IsEdge() {
		return "an unnamed feature"
	}
	subject := "the end of this " + connectorKindName(owner)
	if owner.Name != "" {
		subject = "the end of " + connectorKindName(owner) + " " + owner.Name
	}
	if end, ok := sym.Decl.(*ast.ConnectorEnd); ok {
		if text := targetText(end.AttachedTarget()); text != "" {
			subject += " at " + text
		}
	}
	return subject
}
