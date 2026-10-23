package repl

import (
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// occurrenceModel is a migrated use case in the shape a modelling tool's default
// sequence diagram migrates to: a lifeline subsetting a part, events performing
// body-less actions of the lifeline's type, signal payloads and signatureless messages.
const occurrenceModel = `package Instrument {
    part Operator;
    part def Stage { action detectCharging; }
    item def PositionStage;
    occurrence def Acquire {
        ref part operator :> Operator {
            event occurrence positionSent;
            then event occurrence chargingSent;
        }
        ref part stage : Stage {
            event occurrence positionReceived;
            then event occurrence chargingReceived {
                perform action detectChargingCall ::> Stage::detectCharging;
            }
        }
        message position of PositionStage[1] from operator.positionSent to stage.positionReceived;
        message detectCharging from operator.chargingSent to stage.chargingReceived;
    }
}`

func occurrenceSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(occurrenceModel)
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	return s
}

// withinCommandDeadline runs a session command, failing the test if it has not
// returned in time: a command blocked on the session is a hang at the prompt.
func withinCommandDeadline(t *testing.T, what string, run func() []string) []string {
	t.Helper()
	done := make(chan []string, 1)
	go func() { done <- run() }()
	select {
	case out := <-done:
		return out
	case <-time.After(20 * time.Second):
		t.Fatalf("%s did not return: the command is blocked", what)
		return nil
	}
}

// TestOccurrenceCommandReturnsAtThePrompt pins that `%occurrence` performs the
// interaction and prints its transfers from the prompt, where the session is
// already held for the command, and that the prompt takes the next line after it.
func TestOccurrenceCommandReturnsAtThePrompt(t *testing.T) {
	s := occurrenceSession(t)
	out := withinCommandDeadline(t, "%occurrence", func() []string {
		lines, _, err := s.RunMeta("%occurrence Instrument::Acquire")
		if err != nil {
			t.Errorf("%%occurrence: %v", err)
		}
		return lines
	})
	want := []string{
		"message position: operator.positionSent -> stage.positionReceived",
		"message detectCharging: operator.chargingSent -> stage.chargingReceived",
	}
	if strings.Join(out, "\n") != strings.Join(want, "\n") {
		t.Errorf("%%occurrence printed:\n%s\nwant:\n%s", strings.Join(out, "\n"), strings.Join(want, "\n"))
	}
	next := withinCommandDeadline(t, "%help after %occurrence", func() []string {
		lines, _, _ := s.RunMeta("%help")
		return lines
	})
	if len(next) == 0 {
		t.Errorf("%%help after %%occurrence printed nothing")
	}
	verdict := s.RunOccurrence("Instrument::Acquire")
	if verdict.Status != VerdictHolds || len(verdict.Values) != 2 {
		t.Errorf("RunOccurrence = %+v, want two transfers", verdict)
	}
}

// TestOccurrenceCommandNamesItsError pins that a run that cannot be performed is
// reported at the prompt with the construct at fault rather than silence.
func TestOccurrenceCommandNamesItsError(t *testing.T) {
	s := occurrenceSession(t)
	res := s.Submit(`package Broken {
        occurrence def Seq {
            part a { event occurrence sent; }
            part b;
            message m from a.sent to b;
        }
    }`)
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	out := withinCommandDeadline(t, "%occurrence of a broken interaction", func() []string {
		lines, _, _ := s.RunMeta("%occurrence Broken::Seq")
		return lines
	})
	text := strings.Join(out, "\n")
	if !strings.Contains(text, "error") || !strings.Contains(text, "b") {
		t.Errorf("%%occurrence on a message end no lifeline owns printed:\n%s", text)
	}
}
