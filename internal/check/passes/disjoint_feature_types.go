package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
					sym.Name, a.Name, b.Name),
				Code:   "disjoint-feature-types",
				Source: "constraint",
			})
			return
		}
	}
}
