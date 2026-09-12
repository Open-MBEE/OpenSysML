package repl

import (
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
)

// Every exported entry point holds the session lock, so a frontend calling them
// from more than one goroutine — readline's Tab against the loop's command — never
// races the lazily built index, name table and runtime. Run under -race.
func TestExportedEntryPointsSerialize(t *testing.T) {
	const model = `package M {
		import ScalarValues::*;
		part def Vehicle { attribute mass : Real = 1.0; constraint light { mass < 10.0 } }
		calc def Add { in a : Real; in b : Real; return : Real = a + b; }
		action def Twice { in x : Real; out y : Real = x * 2.0; }
		state def Lamp { entry; then off; state off; state on; transition first off then on; }
	}`
	commands := []func(s *Session){
		func(s *Session) { s.RunCalc("Add 2.0 3.0") },
		func(s *Session) { s.RunAction("Twice") },
		func(s *Session) { s.RunStateMachine("Lamp") },
		func(s *Session) { s.InstantiateNamed("Vehicle") },
		func(s *Session) { s.InstantiateReport("Vehicle") },
		func(s *Session) { s.CheckConstraint("Vehicle::light") },
		func(s *Session) { s.CheckRequirement("Vehicle::light") },
		func(s *Session) { s.CheckSatisfy("") },
		func(s *Session) { s.EvalExpr("1 + 1") },
		func(s *Session) { s.View("Vehicle") },
		func(s *Session) { s.RunDocumentQuery("Vehicle") },
		func(s *Session) { s.RenderDocumentMarkdown("Vehicle", docrender.MarkdownOptions{}) },
		func(s *Session) { s.RenderDocumentSetMarkdown(docrender.MarkdownOptions{}) },
		func(s *Session) { s.Complete("%instantiate Veh", len("%instantiate Veh")) },
		func(s *Session) { s.Complete("Add", 3) },
		func(s *Session) { s.RunMeta("%features Vehicle") },
		func(s *Session) { s.Submit("package N { part def Trailer; }") },
		func(s *Session) { s.Diagnostics() },
		func(s *Session) { s.DiagnosticLines() },
		func(s *Session) { s.LocatedDiagnostics() },
		func(s *Session) { s.HasErrors() },
		func(s *Session) { s.SetTracing(true) },
		func(s *Session) { s.Tracing() },
		func(s *Session) { s.SetVerbosity(s.Verbosity()) },
		func(s *Session) { s.SetConformanceMode(s.ConformanceMode()) },
		func(s *Session) { s.SetBudgets(s.Budgets()) },
		func(s *Session) { s.SetRenderWidth(80) },
		func(s *Session) { s.List() },
	}
	for round := 0; round < 4; round++ {
		s := NewSession()
		s.SubmitFiles([]SourceFile{{Name: "m.sysml", Text: model}})
		if s.HasErrors() {
			t.Fatalf("model did not analyse cleanly:\n%s", strings.Join(s.DiagnosticLines(), "\n"))
		}
		var wg sync.WaitGroup
		for _, run := range commands {
			wg.Add(1)
			go func() {
				defer wg.Done()
				run(s)
			}()
		}
		wg.Wait()
		v := s.RunCalc("Add 2.0 3.0")
		if v.Status != VerdictHolds {
			t.Fatalf("round %d: Add after concurrent commands: %v", round, v.Lines)
		}
	}
}
