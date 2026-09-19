package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// msgEndFeatureMultiplicity reports an end feature whose multiplicity is not
// exactly one (KerML validateFeatureEndFeatureMultiplicity).
const msgEndFeatureMultiplicity = "End feature must have multiplicity 1: an end relates exactly one thing per link; write `[1]` or take it from a feature the end subsets or redefines"

// checkFeatureEndFeatureMultiplicity warns on an end feature none of whose
// multiplicities, its own or a general's, is exactly one.
func (cc *constraintChecker) checkFeatureEndFeatureMultiplicity(sym *symbols.Symbol) {
	switch d := sym.Decl.(type) {
	case *ast.ConnectorEnd:
		if _, declares := d.DeclaredName(); !declares || cc.model.EndMultiplicityIsOne(sym) {
			return
		}
		cc.reportEndMultiplicity(d.Span())
	case *ast.Usage:
		if d.IsEnd && !cc.model.EndMultiplicityIsOne(sym) {
			cc.reportEndMultiplicity(d.Span())
		}
		for i, end := range d.ConnectorEnds {
			if end == nil {
				continue
			}
			if _, declares := end.DeclaredName(); declares || cc.model.ConnectorEndMultiplicityIsOne(sym, i) {
				continue
			}
			cc.reportEndMultiplicity(end.Span())
		}
	}
}

func (cc *constraintChecker) reportEndMultiplicity(span source.Span) {
	cc.diags = append(cc.diags, diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     span,
		Message:  msgEndFeatureMultiplicity,
		Code:     "end-feature-multiplicity",
		Source:   "constraint",
	})
}
