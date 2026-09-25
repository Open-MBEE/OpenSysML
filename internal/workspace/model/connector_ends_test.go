package model

import (
	"strings"
	"testing"
)

// allMessages returns every finding reported for one document.
func allMessages(t *testing.T, name, src string) []string {
	t.Helper()
	ws := NewWorkspace()
	uri := "file:///" + name + ".sysml"
	ws.Open(uri, []byte(src), 1)
	defer ws.Close(uri)
	var out []string
	for _, d := range ws.Diagnostics(uri) {
		out = append(out, d.Message)
	}
	return out
}

// TestConnectorEndNamesResolve covers SysML v2 7.13.2/7.14.2: the ends a
// connection, interface or flow-carrying interface usage names in its connect
// clause redefine the ends of the definition typing it, so they are
// declarations of the usage's own ends rather than unresolved references.
func TestConnectorEndNamesResolve(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"connection usage", `package P {
			part def TireBead;
			part def Rim;
			connection def PressureSeat {
				end [1] part bead : TireBead;
				end [1] part rim : Rim;
			}
			part wheelAssy {
				part t { part bead : TireBead; }
				part w { part rim : Rim; }
				connection : PressureSeat connect
					bead references t.bead to
					rim references w.rim;
			}
		}`},
		{"interface usage", `package P {
			port def FuelOutPort;
			port def FuelInPort;
			interface def FuelInterface {
				end supplierPort : FuelOutPort;
				end consumerPort : FuelInPort;
			}
			part vehicle {
				part tankAssy { port fuelTankPort : FuelOutPort; }
				part eng { port engineFuelPort : FuelInPort; }
				interface : FuelInterface connect
					supplierPort ::> tankAssy.fuelTankPort to
					consumerPort ::> eng.engineFuelPort;
			}
		}`},
		{"interface usage carrying flows", `package P {
			item def Fuel;
			port def FuelOutPort { out item fuelSupply : Fuel; }
			port def FuelInPort { in item fuelSupply : Fuel; }
			interface def FuelInterface {
				end supplierPort : FuelOutPort;
				end consumerPort : FuelInPort;

				flow supplierPort.fuelSupply to consumerPort.fuelSupply;
			}
			part vehicle {
				part tankAssy { port fuelTankPort : FuelOutPort; }
				part eng { port engineFuelPort : FuelInPort; }
				interface : FuelInterface connect
					supplierPort ::> tankAssy.fuelTankPort to
					consumerPort ::> eng.engineFuelPort;
			}
		}`},
		{"explicitly redefining end", `package P {
			part def TireBead;
			connection def PressureSeat {
				end [1] part bead : TireBead;
				end [1] part rim;
			}
			part wheelAssy {
				part t { part bead : TireBead; }
				part w { part rim; }
				connection : PressureSeat connect
					seatRim :>> PressureSeat::rim references w.rim to
					seatBead :>> PressureSeat::bead references t.bead;
			}
		}`},
		{"general whose ends are not enumerable", `package P {
			connection def PressureSeat;
			part wheelAssy {
				part t { part bead; }
				part w { part rim; }
				connection : PressureSeat connect
					bead references t.bead to
					rim references w.rim;
			}
		}`},
		{"explicitly redefining end by plain name", `package P {
			part def TireBead;
			connection def PressureSeat {
				end [1] part bead : TireBead;
				end [1] part rim;
			}
			part wheelAssy {
				part t { part bead : TireBead; }
				part w { part rim; }
				connection : PressureSeat connect
					seatRim :>> rim references w.rim to
					seatBead :>> bead references t.bead;
			}
		}`},
		{"body member redefining a declared end", `package P {
			part def TireBead;
			connection def PressureSeat {
				end [1] part bead : TireBead;
				end [1] part rim : TireBead;
			}
			part wheelAssy {
				part t { part bead : TireBead; }
				part w { part rim : TireBead; }
				connection : PressureSeat connect
					bead references t.bead to
					rim references w.rim {
						part p :>> bead;
					}
			}
		}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Two cases carry findings the reference reports too — an end-less
			// general relating fewer than two elements, and a non-end member
			// redefining an end — so those rules are not the subject here.
			confirmed := map[string]bool{
				"Must have at least two related elements":   true,
				"Redefining feature must be an end feature": true,
			}
			var found []string
			for _, msg := range allMessages(t, "ends", tc.src) {
				if !confirmed[msg] {
					found = append(found, msg)
				}
			}
			if len(found) != 0 {
				t.Fatalf("expected no findings in a well-formed model, got %d: %v", len(found), found)
			}
		})
	}
}

// TestConnectorEndArityMismatch covers the counterpart: an end beyond the last
// position of the connection definition typing the usage redefines nothing, so
// it is reported rather than silently accepted.
func TestConnectorEndArityMismatch(t *testing.T) {
	src := `package P {
		connection def Seat { end [1] part bead; }
		part wheelAssy {
			part t; part w;
			connection : Seat connect
				bead references t to
				rim references w;
		}
	}`
	// The one-ended definition also relates fewer than two elements, as it does
	// in the reference; the arity of the usage's ends is what is under test.
	var found []string
	for _, msg := range allMessages(t, "arity", src) {
		if msg != "Must have at least two related elements" {
			found = append(found, msg)
		}
	}
	if len(found) != 1 || !strings.Contains(found[0], "rim redefines no end of Seat") {
		t.Fatalf("expected one arity finding for the extra end, got %v", found)
	}
}
