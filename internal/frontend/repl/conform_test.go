package repl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

func TestConformTraceTimeoutDoesNotStopNextRun(t *testing.T) {
	s := NewSession()
	s.Submit(`package Probe {
		private import ScalarValues::*;
		part def Controller {
			attribute status : String = "idle";
			exhibit state machine { entry; then waiting; state waiting; }
		}
		part controller : Controller;
	}`)
	if s.HasErrors() {
		t.Fatalf("model diagnostics: %v", s.DiagnosticLines())
	}
	b := Behavior{Name: "Probe::Controller::machine", Performer: []string{"Probe::controller"}}
	rows := []runtime.TraceRow{{Observe: map[string]runtime.Value{"status": runtime.NewStringValue("idle")}}}

	report, err := s.ConformTrace(b, rows, runtime.CheckBudget{}, time.Nanosecond)
	if err != nil || report.Status != runtime.TraceInconclusive || report.Reason != context.DeadlineExceeded.Error() {
		t.Fatalf("with deadline: report %+v, error %v; want inconclusive deadline", report, err)
	}
	bad := Behavior{Name: b.Name, Performer: []string{"Probe::controller.typo"}}
	_, err = s.ConformTrace(bad, rows, runtime.CheckBudget{}, 0)
	var setup *runtime.SetupError
	if !errors.As(err, &setup) {
		t.Fatalf("unresolved performer: error %v; want invalid setup error", err)
	}
	report, err = s.ConformTrace(b, rows, runtime.CheckBudget{}, 0)
	if err != nil || report.Status != runtime.TraceMatched {
		t.Fatalf("after deadline: report %+v, error %v; want match", report, err)
	}
}
