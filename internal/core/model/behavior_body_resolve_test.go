package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

// unresolvedMessages returns the name-resolution findings for one document.
func unresolvedMessages(ws *Workspace, name string) []string {
	var out []string
	for _, d := range ws.Diagnostics(name) {
		if d.Code == "unresolved" || d.Code == "ambiguous" {
			out = append(out, d.Message)
		}
	}
	return out
}

// diagnose opens src under a synthetic name, collects its name-resolution
// findings, and closes it again.
func diagnose(t *testing.T, name, src string) []string {
	t.Helper()
	ws := NewWorkspace()
	uri := "file:///" + name + ".sysml"
	ws.Open(uri, []byte(src), 1)
	defer ws.Close(uri)
	return unresolvedMessages(ws, uri)
}

// TestBehaviorBodyReferencesAreResolved covers the expression positions inside
// behavioral bodies. Each case references an undeclared name, so each must
// produce exactly one finding; before behavior bodies were walked these were
// all silently accepted.
func TestBehaviorBodyReferencesAreResolved(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"calc return parameter value", `package P { calc c { return r = zzz; } }`},
		{"calc result operand", `package P { calc c { zzz + 1 } }`},
		{"calc def return parameter value", `package P { calc def C { return r = zzz; } }`},
		{"constraint assertion", `package P { constraint k { zzz > 1 } }`},
		{"constraint def assertion", `package P { constraint def K { zzz > 1 } }`},
		{"requirement assumption", `package P { requirement r { assume constraint { zzz > 1 } } }`},
		{"requirement condition", `package P { requirement r { require constraint { zzz > 1 } } }`},
		{"action assignment value", `package P {
			private import ScalarValues::*;
			action a { attribute v : Integer = 0; assign v := zzz; }
		}`},
		{"state entry action", `package P {
			private import ScalarValues::*;
			state S {
				attribute v : Integer = 0;
				entry; then i;
				state i;
				state a { entry { assign v := zzz; } }
				succession first i then a;
			}
		}`},
		{"state do action", `package P {
			private import ScalarValues::*;
			state S {
				attribute v : Integer = 0;
				entry; then i;
				state i;
				state a { do { assign v := zzz; } }
				succession first i then a;
			}
		}`},
		{"state exit action", `package P {
			private import ScalarValues::*;
			state S {
				attribute v : Integer = 0;
				entry; then i;
				state i;
				state a { exit { assign v := zzz; } }
				succession first i then a;
			}
		}`},
		{"transition guard", `package P {
			state S { entry; then i; state i; state a; state b; succession first i then a; transition first a if zzz then b; }
		}`},
		{"transition effect", `package P {
			private import ScalarValues::*;
			state S {
				attribute v : Integer = 0;
				entry; then i; state i; state a; state b; succession first i then a;
				transition first a do { assign v := zzz; } then b;
			}
		}`},
		{"transition body member", `package P {
			state S { entry; then i; state i; state a; state b; succession first i then a; transition t first a then b { attribute w : zzz; } }
		}`},
		{"unnamed transition body member", `package P {
			state S { entry; then i; state i; state a; state b; succession first i then a; transition first a then b { attribute w : zzz; } }
		}`},
		{"initial succession body member", `package P {
			action A { action a1; action a2; first a1 then a2 { attribute w : zzz; } }
		}`},
		{"action accept change trigger", `package P {
			action A { first start; action wait accept when zzz > 1; done; succession first start then wait; succession first wait then done; }
		}`},
		{"action accept absolute time trigger", `package P {
			action A { first start; action wait accept at zzz; done; succession first start then wait; succession first wait then done; }
		}`},
		{"action accept relative time trigger", `package P {
			action A { first start; action wait accept after zzz; done; succession first start then wait; succession first wait then done; }
		}`},
		// A call trigger's parameters belong to its own transition: another
		// transition's guard must not see them.
		{"call trigger parameter outside its transition", `package P {
			state S {
				entry; then i; state i; state a; state b; state c; succession first i then a;
				transition first a accept setSpeed(zzz) then b;
				transition first b if zzz > 0 then c;
			}
		}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			found := diagnose(t, "body", tc.src)
			if len(found) != 1 {
				t.Fatalf("expected one unresolved-reference finding, got %d: %v", len(found), found)
			}
			if !strings.Contains(found[0], "zzz") {
				t.Errorf("expected the finding to name zzz, got: %s", found[0])
			}
		})
	}
}

// TestBehaviorDeclarationsAreVisible covers the declarations behavioral bodies
// refer to. Each model is well formed, so none may produce a finding: a missing
// symbol-table entry would turn body resolution into a false-positive machine.
func TestBehaviorDeclarationsAreVisible(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"nested substate", `package P {
			state S { entry; then i; state i; state outer { state inner; succession first inner then inner; } i then outer; }
		}`},
		{"region states", `package P {
			state S parallel { state r { entry; then s; state s; state x; succession first s then x; } }
		}`},
		{"sibling regions reuse state names", `package P {
			state S parallel {
				state left { entry; then start; state start; state a; succession first start then a; }
				state right { entry; then start; state start; state b; succession first start then b; }
			}
		}`},
		{"region body reads outer feature", `package P {
			private import ScalarValues::*;
			state S parallel {
				attribute v : Integer = 0;
				state r { entry; then s; state s; state x { entry { assign v := 1; } } succession first s then x; }
			}
		}`},
		{"named pseudostates", `package P {
			state S {
				entry; then i; state i; state a; fork f; join j;
				succession first i then a;
				transition first a then f;
				transition first f then j;
				transition first j then done;
			}
		}`},
		{"signal trigger", `package P {
			state S { entry; then i; state i; state a; state b; i then a; transition first a when sigX then b; }
		}`},
		{"call trigger parameter in guard", `package P {
			state S { entry; then i; state i; state a; state b; succession first i then a; transition first a accept setSpeed(value) if value > 0 then b; }
		}`},
		{"call trigger parameter in effect", `package P {
			private import ScalarValues::*;
			state S {
				attribute v : Integer = 0;
				entry; then i; state i; state b; succession first i then a;
				state a;
				accept setSpeed(value) do { assign v := value; } then b;
			}
		}`},
		{"accept payload in effect and guard", `package P {
			private import ScalarValues::*;
			item def Warning;
			state S {
				attribute level : Integer = 0;
				entry; then i; state i; state a; state b; succession first i then a;
				transition first a accept w : Warning if w != null do assign level := 1 then b;
			}
		}`},
		{"action accept trigger names", `package P {
			private import ScalarValues::*;
			action A {
				attribute maxTemp : Integer = 100;
				attribute temp : Integer = 0;
				first start;
				action wait accept when temp > maxTemp;
				done;
				succession first start then wait;
				succession first wait then done;
			}
		}`},
		{"first label written before the action it starts at", `package P {
			action def Focus { out image; }
			action def TakePicture {
				first focus then shoot;
				action focus : Focus { out image; }
				action shoot : Focus { out image; }
				flow from focus.image to shoot.image;
				flow from TakePicture::focus.image to TakePicture::shoot.image;
			}
		}`},
		{"named transition trigger parameters", `package P {
			private import ScalarValues::*;
			item def Warning;
			state S {
				attribute level : Integer = 0;
				entry; then i; state i; state a; state b; state c; succession first i then a;
				transition alert first a accept w : Warning if w != null do assign level := 1 then b;
				transition brake first b accept setSpeed(value) if value > 0 then c;
			}
		}`},
		{"requirement actor binding", `package P {
			private import ScalarValues::*;
			attribute userId : Integer = 42;
			requirement U { actor user = userId; require constraint { user > 0 } }
		}`},
		{"requirement subject", `package P {
			private import ScalarValues::*;
			part def Vehicle { attribute speed : Integer = 0; }
			requirement R { subject v : Vehicle; require constraint { v.speed > 0 } }
		}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if found := diagnose(t, "decl", tc.src); len(found) != 0 {
				t.Fatalf("expected no findings in a well-formed model, got %d: %v", len(found), found)
			}
		})
	}
}

// TestTriggerParametersDoNotEscapeTheirTransition covers where a trigger's
// parameters are visible: to the transition's own guard and effect, and nowhere
// else, so a recursive import of the state does not bring them into scope.
func TestTriggerParametersDoNotEscapeTheirTransition(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		item def Warning;
		state S {
			attribute level : Integer = 0;
			entry; then i; state i; state a; state b; state c; succession first i then a;
			transition alert first a accept w : Warning if w != null do assign level := 1 then b;
			transition brake first b accept setSpeed(value) if value > 0 then c;
		}
	}
	package Q {
		private import P::S::**;
		attribute payload = w;
		attribute speed = value;
	}`
	if got := diagnose(t, "trigparams", src); len(got) != 2 {
		t.Errorf("references to imported trigger parameters reported %v, want three findings", got)
	}
}

// TestTriggerParametersReachNestedEffects covers the other side: the effect
// sees the parameters however deeply it nests, a succession in the effect names
// the effect's own members, and a named transition's effect stays its feature.
func TestTriggerParametersReachNestedEffects(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		item def Warning { attribute level : Integer; }
		state S {
			attribute level : Integer = 0;
			port line;
			state a; state b;
			transition alert first a accept w : Warning do action {
				first start;
				then action record { in what = w.level; }
				then action raise { in cause = w; }
			} then b;
			transition setSpeed first a accept req(value) if value > 0
				do action set { in target = value; } then b;
			transition relay first a accept w : Warning
				do send w via line { in payload = w; } then b;
		}
		package Q {
			private import P::S::**;
			attribute effectOf = setSpeed.set;
			attribute relayed = relay.payload;
		}
	}`
	if got := diagnose(t, "nestedeffects", src); len(got) != 0 {
		t.Errorf("expected no findings in a well-formed model, got %v", got)
	}
	// The nested bodies are walked: a name they misspell is reported.
	misspelled := strings.NewReplacer("w.level", "ww.level", "= value;", "= vaule;", "relay.payload", "relay.paylaod").Replace(src)
	if got := diagnose(t, "nestedeffects", misspelled); len(got) != 3 {
		t.Errorf("misspelled parameters in nested effects reported %v, want three findings", got)
	}
}

// TestKeywordNamedDeclarationIsReferenceable covers a declaration whose name is
// a keyword: the name must reach the symbol table, or references to it resolve
// against nothing while the misspelled control case reports nothing either.
func TestKeywordNamedDeclarationIsReferenceable(t *testing.T) {
	resolved := `package P {
		action flow { }
		action caller { action a : flow; }
	}`
	if got := diagnose(t, "kwname", resolved); len(got) != 0 {
		t.Errorf("reference to the keyword-named action reported %v, want none", got)
	}

	misspelled := `package P {
		action flow { }
		action caller { action a : flwo; }
	}`
	if got := diagnose(t, "kwname_bad", misspelled); len(got) != 1 {
		t.Errorf("reference to an undeclared name reported %v, want one finding", got)
	}
}

func TestReservedKeywordNameErrorDoesNotGateLaterTiers(t *testing.T) {
	ws := NewWorkspace()
	uri := "file:///kwwarn.sysml"
	ws.Open(uri, []byte(`package P {
		part p {
			attribute x = 1;
			action flow { assign x := zzz; }
		}
	}`), 1)
	defer ws.Close(uri)

	var errors []passes.Diagnostic
	for _, d := range ws.Diagnostics(uri) {
		if d.Severity == passes.SeverityError {
			errors = append(errors, d)
		}
	}

	if len(errors) != 2 {
		t.Fatalf("errors = %v, want the keyword name and unresolved reference", errors)
	}
	if errors[0].Code != "reserved-keyword-name" || !strings.Contains(errors[0].Message, `"flow" is a reserved keyword`) {
		t.Errorf("keyword error = %+v", errors[0])
	}
	if !strings.Contains(errors[1].Message, "unresolved reference: zzz") {
		t.Errorf("name-resolution error = %+v", errors[1])
	}
}
