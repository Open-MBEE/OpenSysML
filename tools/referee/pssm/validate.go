package pssm

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

// Validate parses and validates an emitted model with the front end and lowers
// its machine, returning every error diagnostic and the lowering error.
func Validate(m *Model) []string {
	ws := model.NewWorkspace()
	ws.Open(m.Name, []byte(m.Text), 1)
	var problems []string
	for _, d := range ws.Diagnostics(m.Name) {
		if d.Severity != passes.SeverityError {
			continue
		}
		problems = append(problems, fmt.Sprintf("%d:%d: %s", d.Span.Offset, d.Span.Len, d.Message))
	}
	syms := ws.LookupQualified(m.Qualified)
	if len(syms) != 1 {
		return append(problems, fmt.Sprintf("%s: %d symbols named", m.Qualified, len(syms)))
	}
	scope := syms[0].Scope
	if scope == nil {
		scope = syms[0].OwnerScope
	}
	if _, err := lower.ToStateGraph(syms[0].Decl, scope); err != nil {
		problems = append(problems, "lower: "+err.Error())
	}
	return problems
}
