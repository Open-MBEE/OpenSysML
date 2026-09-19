package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// The reproducer of validation/invalid/InterfaceUsage_Invalid: an interface end
// declared as a part rather than a port, in a definition and in a usage.
func TestW10BInterfaceEnds(t *testing.T) {
	const src = `package P {
		port def FuelOutPort;
		port def FuelInPort;
		part def Fuel;
		interface def FuelInterface {
			end supplierPort : FuelOutPort;
			end consumerPort : FuelInPort;
		}
		interface def Bad {
			end port supplierPort : FuelOutPort;
			end port consumerPort : FuelInPort;
			end part notPort : Fuel;
		}
		part vehicle {
			part tankAssy { port fuelTankPort : FuelOutPort; part fuel : Fuel; }
			part eng { port engineFuelPort : FuelInPort; }
			interface fi : FuelInterface connect tankAssy.fuelTankPort to eng.engineFuelPort;
			interface bad {
				end part ::> tankAssy.fuel;
				end port ::> eng.engineFuelPort;
			}
		}
	}`
	diags := w9cLibraryDiags(t, src, false)
	byMessage := map[string]int{}
	for _, d := range diags {
		switch d.Message {
		case msgInterfaceDefEndPort, msgInterfaceEndPort:
			byMessage[d.Message]++
			if d.Severity != diag.SeverityError {
				t.Errorf("%q severity = %v, want an error", d.Message, d.Severity)
			}
			if got := src[d.Span.Offset : d.Span.Offset+len("end p")]; got != "end p" {
				t.Errorf("%q starts at %q, want the end declaration", d.Message, got)
			}
		}
	}
	for msg, want := range map[string]int{
		msgInterfaceDefEndPort: 1,
		msgInterfaceEndPort:    1,
	} {
		if byMessage[msg] != want {
			t.Errorf("got %d %q diagnostics, want %d", byMessage[msg], msg, want)
		}
	}
}

// An interface whose ends are ports, written with or without the keyword, draws
// nothing, and neither does an n-ary association.
func TestW10BEndsClean(t *testing.T) {
	const src = `package P {
		port def PA;
		part def A;
		interface def Fine {
			end supplierPort : PA;
			end port consumerPort : PA;
		}
		assoc Ternary {
			end a : A;
			end b : A;
			end c : A;
		}
	}`
	for _, d := range w9cLibraryDiags(t, src, false) {
		switch d.Message {
		case msgInterfaceDefEndPort, msgInterfaceEndPort, msgFlowDefTooManyEnds:
			t.Errorf("unexpected %q at offset %d", d.Message, d.Span.Offset)
		}
	}
}

// A flow definition redefining its inherited source and target has two ends, not
// four; a third one is an error, at the definition when it is inherited.
func TestW10BFlowDefEndCount(t *testing.T) {
	const src = `package P {
		port def PA;
		part def Fuel;
		flow def Fine {
			end port from1 : PA;
			end port to1 : PA;
		}
		flow def Three {
			end port a : PA;
			end port b : PA;
			end port c : PA;
		}
		flow def Inherited :> Three;
	}`
	var got []string
	for _, d := range w9cLibraryDiags(t, src, false) {
		if d.Message == msgFlowDefTooManyEnds {
			got = append(got, strings.TrimSpace(src[d.Span.Offset:min(d.Span.Offset+12, len(src))]))
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d flow end-count diagnostics (%v), want 2", len(got), got)
	}
	if !strings.HasPrefix(got[0], "end port c") {
		t.Errorf("first diagnostic at %q, want the third declared end", got[0])
	}
	if !strings.HasPrefix(got[1], "flow def Inh") {
		t.Errorf("second diagnostic at %q, want the deriving definition", got[1])
	}
}

// Malformed input must not panic or invent end diagnostics.
func TestW10BEndsMalformed(t *testing.T) {
	for _, src := range []string{"interface def", "interface def I { end", "flow def { end end }"} {
		typeDiags(t, src)
	}
}
