package model

import "testing"

// TestInheritedMembersAreVisible covers references that reach a member through
// what a feature specializes, redefines or is typed by, rather than through its
// own body. Each model is well formed, so none may report a finding.
func TestInheritedMembersAreVisible(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"variant through typed usage", `package P {
			part def Engine;
			variation part def EngineChoices :> Engine {
				variant part cyl4;
				variant part cyl6;
			}
			part family { part engine : EngineChoices; }
			part v :> family { part redefines engine = engine::cyl4; }
		}`},
		{"nested redefinition", `package P {
			part def Cylinder;
			part def Engine { part cyl : Cylinder[4..6]; }
			part def Vehicle { part eng : Engine; }
			part small : Vehicle { part redefines eng { part redefines cyl[4]; } }
		}`},
		{"member inherited from definition", `package P {
			private import ScalarValues::*;
			part def Vehicle { attribute mass : Real; }
			part v : Vehicle { attribute m = v::mass; }
		}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if found := diagnose(t, "inherited", tc.src); len(found) != 0 {
				t.Fatalf("expected no findings in a well-formed model, got %d: %v", len(found), found)
			}
		})
	}
}

func TestAliasSupertypeInheritanceWithGeometryAndUnits(t *testing.T) {
	src := `package LRU_Assembly {
		private import ShapeItems::Box;
		private import ShapeItems::CircularCylinder;
		private import SI::mm;
		part def AvionicsLRU :> Box {
			:>> length default = 100 [mm];
			:>> width default = 50 [mm];
			:>> height default = 20 [mm];
		}
		part def MountingBushing :> CircularCylinder {
			:>> radius = 5 [mm];
			:>> height = 1 [mm];
		}
		alias AvionicsLRUType for AvionicsLRU;
		part lru : AvionicsLRUType {
			:>> length = 100 [mm];
			:>> width = 50 [mm];
			:>> height = 20 [mm];
		}
	}`
	ws := NewWorkspace()
	const uri = "file:///alias_geometry.sysml"
	ws.Open(uri, []byte(src), 1)
	defer ws.Close(uri)
	if diagnostics := ws.Diagnostics(uri); len(diagnostics) != 0 {
		t.Fatalf("expected alias-based geometry model to be clean, got %d: %v", len(diagnostics), diagnostics)
	}
}

// TestRedefinitionDoesNotShadowItsTarget covers the misspelled counterpart of
// the inherited cases: a redefinition of a name no supertype declares must
// still report, or the inherited lookup would accept anything.
func TestRedefinitionDoesNotShadowItsTarget(t *testing.T) {
	src := `package P {
		part def Engine;
		variation part def EngineChoices :> Engine { variant part cyl4; }
		part family { part engine : EngineChoices; }
		part v :> family { part redefines engine = engine::cyl8; }
	}`
	if found := diagnose(t, "inherited_bad", src); len(found) != 1 {
		t.Fatalf("expected one finding for the undeclared variant, got %d: %v", len(found), found)
	}
}

// TestBodyLocalDeclarationsAreVisible covers names declared inside a loop body
// or a body expression, which are members of the loop or the expression rather
// than of the enclosing behavior.
func TestBodyLocalDeclarationsAreVisible(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"loop condition reads loop body", `package P {
			private import ScalarValues::*;
			action def Monitor { out charge : Real; }
			action def Charge {
				loop action charging {
					action monitor : Monitor { out charge; }
				} until charging.monitor.charge >= 100;
			}
		}`},
		{"for body statement reads for body", `package P {
			private import ScalarValues::*;
			action def Step { in x_in : Real; out x_out : Real; }
			action def Move {
				in attribute profile : Real[*];
				private attribute position : Real = 0;
				for power in profile {
					perform action step : Step { in x_in = position; out x_out; }
					then assign position := step.x_out;
				}
			}
		}`},
		{"if branch body reads its own declaration", `package P {
			private import ScalarValues::*;
			action def Step { out done : Boolean; }
			action def Drive {
				in attribute fast : Boolean;
				if fast {
					action accelerate : Step { out done; }
					assign fast := accelerate.done;
				}
			}
		}`},
		{"else branch reuses the then branch's name", `package P {
			private import ScalarValues::*;
			action def Step { out done : Boolean; }
			action def Drive {
				in attribute fast : Boolean;
				if fast {
					action move : Step { out done; }
					assign fast := move.done;
				} else {
					action move : Step { out done; }
					assign fast := move.done;
				}
			}
		}`},
		{"body expression parameter", `package P {
			private import ScalarValues::*;
			private import ControlFunctions::*;
			action def Sample {
				in attribute samples : Real[*];
				assert constraint { samples->forAll { in s : Real; s > 0 } }
			}
		}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if found := diagnose(t, "bodylocal", tc.src); len(found) != 0 {
				t.Fatalf("expected no findings in a well-formed model, got %d: %v", len(found), found)
			}
		})
	}
}

// TestBodyLocalNamesDoNotEscape covers the negative side: a loop body member is
// not a member of the enclosing behavior, and a body-expression parameter is
// not visible outside its body.
func TestBodyLocalNamesDoNotEscape(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"loop member from outside", `package P {
			private import ScalarValues::*;
			action def Charge {
				loop action charging { } until true;
				attribute c = charging;
			}
		}`},
		{"if branch member from outside", `package P {
			private import ScalarValues::*;
			action def Drive {
				in attribute fast : Boolean;
				if fast { action accelerate; }
				attribute a = accelerate;
			}
		}`},
		{"else branch member from the then branch", `package P {
			private import ScalarValues::*;
			action def Drive {
				in attribute fast : Boolean;
				if fast {
					attribute a = brake;
				} else {
					action brake;
				}
			}
		}`},
		{"body expression parameter from outside", `package P {
			private import ScalarValues::*;
			private import ControlFunctions::*;
			action def Sample {
				in attribute samples : Real[*];
				assert constraint { samples->forAll { in s : Real; s > 0 } & s > 0 }
			}
		}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if found := diagnose(t, "escape", tc.src); len(found) != 1 {
				t.Fatalf("expected one finding, got %d: %v", len(found), found)
			}
		})
	}
}
