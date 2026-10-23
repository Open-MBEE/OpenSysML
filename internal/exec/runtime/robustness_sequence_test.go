package runtime

import (
	"errors"
	"testing"
	"time"

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
	t.Run("constraint an event asserts does not hold", func(t *testing.T) {
		err := occurrenceRunError(t, "Seq", `package test {
			part def Node { attribute ready : ScalarValues::Boolean = false; }
			occurrence def Seq {
				part a : Node { event occurrence sent; }
				part b : Node { event occurrence got { assert constraint { ready } } }
				message m from a.sent to b.got;
			}
		}`)
		var violation *ViolationError
		if !errors.As(err, &violation) {
			t.Fatalf("error = %v, want a ViolationError", err)
		}
	})
	t.Run("message reaching a nested interaction use", func(t *testing.T) {
		err := occurrenceRunError(t, "Seq", `package test {
			occurrence def Inner {
				part a { event occurrence sent; }
				part b { event occurrence got; }
				message m from a.sent to b.got;
			}
			occurrence def Seq {
				part a { event occurrence sent; }
				occurrence inner : Inner;
				message m from a.sent to inner;
			}
		}`)
		if !errors.Is(err, lower.ErrMessageEndUnowned) {
			t.Fatalf("error = %v, want ErrMessageEndUnowned", err)
		}
	})
	t.Run("body-less performs at events complete", func(t *testing.T) {
		// A migrated sequence with body-less performs, create and signatureless
		// messages must return with every transfer taken, never block.
		done := make(chan error, 1)
		go func() {
			done <- occurrenceRunError(t, "Acquire", `package test {
				part Operator;
				part def Stage { action detectCharging; }
				part def Image { action data; }
				occurrence def Acquire {
					ref part operator :> Operator {
						event occurrence positionSent;
						then event occurrence createSent;
						then event occurrence chargingSent;
					}
					ref part stage : Stage {
						event occurrence positionReceived;
						then event occurrence chargingReceived {
							perform action detectChargingCall ::> Stage::detectCharging;
						}
						then event occurrence dataSent;
					}
					ref part image : Image {
						event occurrence createReceived;
						then event occurrence dataReceived {
							perform action dataCall ::> Image::data;
						}
					}
					message position from operator.positionSent to stage.positionReceived;
					message create from operator.createSent to image.createReceived;
					message detectCharging from operator.chargingSent to stage.chargingReceived;
					message data from stage.dataSent to image.dataReceived;
				}
			}`)
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("error = %v, want the run to complete", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("the run did not return within its deadline")
		}
	})
	t.Run("interaction using itself", func(t *testing.T) {
		err := occurrenceRunError(t, "Seq", `package test {
			occurrence def Seq {
				part a { event occurrence sent; }
				part b { event occurrence got; }
				message m from a.sent to b.got;
				occurrence again : Seq;
				succession first b.got then again;
			}
		}`)
		if !errors.Is(err, errNestedRecursion) {
			t.Fatalf("error = %v, want errNestedRecursion", err)
		}
	})
}
