package fuml

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// Validate parses and validates an emitted model with the front end and lowers
// its action definition, returning every error diagnostic and the lowering error.
func Validate(em *Emitted) []string {
	ws := model.NewWorkspace()
	ws.Open(em.Name, []byte(em.Text), 1)
	var problems []string
	for _, d := range ws.Diagnostics(em.Name) {
		if d.Severity != diag.SeverityError {
			continue
		}
		problems = append(problems, fmt.Sprintf("%d:%d: %s", d.Span.Offset, d.Span.Len, d.Message))
	}
	syms := ws.LookupQualified(em.Qualified)
	if len(syms) != 1 {
		return append(problems, fmt.Sprintf("%s: %d symbols named", em.Qualified, len(syms)))
	}
	scope := syms[0].Scope
	if scope == nil {
		scope = syms[0].OwnerScope
	}
	if _, err := lower.ToActionGraph(syms[0].Decl, scope); err != nil {
		problems = append(problems, "lower: "+err.Error())
	}
	return problems
}
