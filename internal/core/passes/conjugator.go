package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// msgAtMostOneConjugator reports a type declaring a second conjugation (KerML
// validateTypeAtMostOneConjugator).
const msgAtMostOneConjugator = "Cannot have more than one conjugator: a type conjugates one type; remove the other `~`"

// checkAtMostOneConjugator reports each conjugation a type declares beyond its first.
func (cc *constraintChecker) checkAtMostOneConjugator(sym *symbols.Symbol) {
	seen := false
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || !rel.Conjugated || rel.Kind != ast.RelSpecializes {
			continue
		}
		if !seen {
			seen = true
			continue
		}
		cc.diags = append(cc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     rel.Span(),
			Message:  msgAtMostOneConjugator,
			Code:     "type-conjugators",
			Source:   "constraint",
		})
	}
}
