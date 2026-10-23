package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// occurrenceRunError performs the named occurrence def in src as an interaction
// and answers the error it fails with.
func occurrenceRunError(t *testing.T, name, src string) error {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefOccurrence)
	if sym == nil {
		t.Fatalf("occurrence def %s not found", name)
	}
	_, err := ctx.ExecuteOccurrence(sym)
	return err
}

// TestRuntimeRobustnessSequence pins the failure modes of performing an occurrence
// def as an interaction: each is a typed error, never a panic or a hang.
func TestRuntimeRobustnessSequence(t *testing.T) {
	t.Run("message end no lifeline owns", func(t *testing.T) {
		err := occurrenceRunError(t, "Seq", `package test {
			occurrence def Seq {
				part a { event occurrence sent; }
				part b;
				message m from a.sent to b;
			}
		}`)
		if !errors.Is(err, lower.ErrMessageEndUnowned) {
			t.Fatalf("error = %v, want ErrMessageEndUnowned", err)
		}
	})
	t.Run("message end naming nothing", func(t *testing.T) {
		err := occurrenceRunError(t, "Seq", `package test {
			occurrence def Seq {
				part a { event occurrence sent; }
				part b { event occurrence got; }
				message m from a.sent to b.missing;
			}
		}`)
		if !errors.Is(err, lower.ErrMessageEndUnowned) {
			t.Fatalf("error = %v, want ErrMessageEndUnowned", err)
		}
	})
	t.Run("succession cycle between events", func(t *testing.T) {
		err := occurrenceRunError(t, "Seq", `package test {
			occurrence def Seq {
				part a { event occurrence one; then event occurrence two; }
				part b { event occurrence got; }
				succession first a.two then a.one;
				message m from a.one to b.got;
			}
		}`)
		if !errors.Is(err, lower.ErrSequenceCycle) {
			t.Fatalf("error = %v, want ErrSequenceCycle", err)
		}
	})
	t.Run("message against its lifelines' order", func(t *testing.T) {
		err := occurrenceRunError(t, "Seq", `package test {
			occurrence def Seq {
				part a { event occurrence sent; then event occurrence got; }
				part b { event occurrence got; then event occurrence sent; }
				message m from a.sent to b.got;
				message n from b.sent to a.got;
				succession first b.got then a.sent;
			}
		}`)
		if !errors.Is(err, lower.ErrSequenceCycle) {
			t.Fatalf("error = %v, want ErrSequenceCycle", err)
		}
	})
	t.Run("no lifeline holds an event", func(t *testing.T) {
		err := occurrenceRunError(t, "Seq", `package test {
			occurrence def Seq {
				part a;
				part b;
			}
		}`)
		if !errors.Is(err, lower.ErrNoSequence) {
			t.Fatalf("error = %v, want ErrNoSequence", err)
		}
	})
	t.Run("unbound payload", func(t *testing.T) {
		err := occurrenceRunError(t, "Seq", `package test {
			item def Sig { attribute n; }
			occurrence def Seq {
				part a { event occurrence sent; }
				part b { event occurrence got; }
				message m of p : Sig = new Sig(n = 1 / 0) from a.sent to b.got;
			}
		}`)
		if !errors.Is(err, errUnboundPayload) {
			t.Fatalf("error = %v, want errUnboundPayload", err)
		}
	})
}
