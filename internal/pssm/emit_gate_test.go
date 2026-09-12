package pssm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// TestEmitSuite is the gate over the pinned suite: every expressible test
// translates and its model parses, validates and lowers clean.
func TestEmitSuite(t *testing.T) {
	s := loadSuite(t)
	keep := os.Getenv("OPENSYSML_PSSM_KEEP")
	for _, tt := range s.Tests {
		if !Classify(tt).Class.Expressible() {
			continue
		}
		t.Run(strings.ReplaceAll(tt.Name, " ", "_"), func(t *testing.T) {
			m, err := Emit(s, tt)
			if err != nil {
				t.Errorf("emit: %v", err)
				return
			}
			if keep != "" {
				if err := os.WriteFile(filepath.Join(keep, m.Name), []byte(m.Text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if problems := Validate(m); len(problems) > 0 {
				t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
			}
		})
	}
}
