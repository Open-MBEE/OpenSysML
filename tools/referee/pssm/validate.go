package pssm

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Validate parses and validates an emitted model with the front end and lowers
// its machine, returning every error diagnostic and the lowering error.
func Validate(m *Model) []string {
	ws := model.NewWorkspace()
	ws.Open(m.Name, []byte(m.Text), 1)
	var problems []string
	for _, d := range ws.Diagnostics(m.Name) {
		if d.Severity != diag.SeverityError {
			continue
		}
		problems = append(problems, fmt.Sprintf("%d:%d: %s", d.Span.Offset, d.Span.Len, d.Message))
	}
	syms := ws.LookupQualified(m.Qualified)
	if len(syms) != 1 {
		return append(problems, fmt.Sprintf("%s: %d symbols named", m.Qualified, len(syms)))
	}
	if _, err := ws.StateGraph(syms[0]); err != nil {
		problems = append(problems, "lower: "+err.Error())
	}
	return problems
}
