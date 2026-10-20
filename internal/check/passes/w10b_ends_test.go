package passes

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// SysML v2.1 Beta 2 §8.3.13.3 and §8.3.16.2: the ends of a connection or flow
// definition are usages, the inherited ones included. A definition that
// specializes a kernel association inherits KerML end features and is reported
// once, at the definition; one whose own usage ends redefine them is clean, as
// is one specializing the SysML connection and flow definitions. Links::Link
// declares `participant` as a plain feature, so CLink has no end at all.
func TestW10BEndsAreUsages(t *testing.T) {
	const src = `package P {
		private import Links::*;
		private import Occurrences::HappensLink;
		private import Transfers::Transfer;
		private import Connections::*;
		private import Flows::*;
		abstract connection def CBin :> BinaryLink;
		abstract connection def CLink :> Link;
		abstract connection def CHappens :> HappensLink;
		abstract interface def IBin :> BinaryLink;
		abstract allocation def ABin :> BinaryLink;
		abstract flow def FBin :> BinaryLink;
		abstract flow def FTransfer :> Transfer;
		abstract connection def COk :> Connection;
		abstract connection def COkBinary :> BinaryConnection;
		abstract flow def FOk :> Flow;
		abstract flow def FOkMessage :> Message;
		connection def CDeclared :> BinaryLink { end a; end b; }
		connection def CPlain { end x; end y; }
	}`
	wantConnection := []string{"CBin", "CHappens", "IBin"}
	wantFlow := []string{"FBin", "FTransfer"}
	for _, warm := range []bool{false, true} {
		diags := w9cLibraryDiags(t, src, warm)
		var gotConnection, gotFlow []string
		for _, d := range diags {
			switch d.Message {
			case msgConnectionDefEndUsage, msgFlowDefEndUsage:
			default:
				continue
			}
			if d.Severity != diag.SeverityError {
				t.Errorf("warm=%v: %q severity = %v, want an error", warm, d.Message, d.Severity)
			}
			line := src[d.Span.Offset:]
			line = line[:strings.IndexByte(line, ';')]
			switch d.Message {
			case msgConnectionDefEndUsage:
				if d.Code != "connection-def-end-not-usage" {
					t.Errorf("warm=%v: code %q for %q", warm, d.Code, line)
				}
				gotConnection = append(gotConnection, w10bDefName(line))
			case msgFlowDefEndUsage:
				if d.Code != "flow-def-end-not-usage" {
					t.Errorf("warm=%v: code %q for %q", warm, d.Code, line)
				}
				gotFlow = append(gotFlow, w10bDefName(line))
			}
		}
		if !slices.Equal(gotConnection, wantConnection) {
			t.Errorf("warm=%v: connection definitions reported = %v, want %v", warm, gotConnection, wantConnection)
		}
		if !slices.Equal(gotFlow, wantFlow) {
			t.Errorf("warm=%v: flow definitions reported = %v, want %v", warm, gotFlow, wantFlow)
		}
	}
}

// w10bDefName is the name a `... def Name ...` declaration declares.
func w10bDefName(decl string) string {
	fields := strings.Fields(decl)
	for i, f := range fields {
		if f == "def" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return decl
}

// A document's language may be declared apart from its file name: KerML opened
// under a .sysml name still declares KerML end features, which a SysML connection
// definition inheriting them must be reported for.
func TestW10BEndsAreUsagesReadsTheDeclaredDocumentKind(t *testing.T) {
	const base = `package Base {
		assoc Pair { end feature a : Anything; end feature b : Anything; }
	}`
	const derived = `package P {
		private import Base::Pair;
		abstract connection def C :> Pair;
	}`
	baseRoot := parser.New(source.NewWithKind("base.sysml", []byte(base), source.KindKerML)).ParseFile()
	derivedFile := source.New("derived.sysml", []byte(derived))
	derivedRoot := parser.New(derivedFile).ParseFile()
	idx := newTestIndex()
	idx.AddDocumentWithKind("base.sysml", baseRoot, source.KindKerML)
	idx.AddDocument("derived.sysml", derivedRoot)
	var got []diag.Diagnostic
	for _, d := range Analyze("derived.sysml", derivedRoot, nil, idx) {
		if d.Code == "connection-def-end-not-usage" {
			got = append(got, d)
		}
	}
	if len(got) != 1 {
		t.Fatalf("got %d connection-def-end-not-usage diagnostics, want 1: %v", len(got), got)
	}
	if want := "abstract connection def C"; !strings.HasPrefix(derived[got[0].Span.Offset:], want) {
		t.Errorf("reported at %q, want the definition", derived[got[0].Span.Offset:got[0].Span.Offset+len(want)])
	}
}
