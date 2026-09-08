package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A message built outside a send carries its payload by feature name: a feature
// redefined under another name is one slot, reported when two entries name it
// rather than resolved by map order, and an entry naming no feature is refused.
func TestMaterializeAcceptedBindsRedefinedFeatureOnce(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		attribute def Base { attribute a : Integer; attribute k : Integer; }
		attribute def Sub :> Base { attribute b redefines a; }
	}`))
	subs := idx.LookupQualified("P::Sub")
	if len(subs) != 1 {
		t.Fatalf("P::Sub matched %d symbols, want 1", len(subs))
	}
	message := func(payload map[string]Value) Message {
		return Message{SignalType: "Sub", Signal: subs[0], Payload: payload}
	}
	got, err := ctx.materializeAccepted(message(map[string]Value{"b": integerValue(4), "k": integerValue(6)}))
	if err != nil {
		t.Fatalf("materialize payload: %v", err)
	}
	inst, ok := ctx.Instance(got.Instance)
	if !ok {
		t.Fatalf("no instance %d", got.Instance)
	}
	for name, want := range map[string]int64{"b": 4, "a": 4, "k": 6} {
		if v := inst.FeatureValues[name].HeldValue(); v.Kind != ValConst || v.Const.Int != want {
			t.Errorf("%s = %+v, want %d", name, v, want)
		}
	}
	for _, tc := range []struct {
		payload map[string]Value
		want    string
	}{
		{map[string]Value{"a": integerValue(4), "b": integerValue(5)}, "a and b are one feature, bound twice"},
		{map[string]Value{"arg1": integerValue(4)}, `"arg1" names no feature it carries`},
	} {
		created := len(ctx.created)
		if _, err := ctx.materializeAccepted(message(tc.payload)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("payload %v: error %v, want %q", tc.payload, err, tc.want)
		}
		if len(ctx.created) != created {
			t.Errorf("payload %v left %d instance(s) behind", tc.payload, len(ctx.created)-created)
		}
	}
}

// Positional arguments bind features by name, so a feature named like a
// position (`arg1`) takes the argument at its own position and no other.
func TestSendNewBindsFeatureNamedLikeAPosition(t *testing.T) {
	assertIntOutput(t, sendNewOutputs(t, `item def Pair { attribute head : Integer; attribute arg1 : Integer; }`,
		"send new Pair(4, 6) to reader;", "assign got := p.head * 10 + p.arg1;", "Pair"), "got", 46)
}

// A feature named `value` is bound like any other: the accept still binds the
// constructed occurrence, not the value that feature holds.
func TestSendNewBindsFeatureNamedValue(t *testing.T) {
	assertIntOutput(t, sendNewOutputs(t, `item def Data { attribute value : Integer; }`,
		"send new Data(value = 7) to reader;", "assign got := p.value;", "Data"), "got", 7)
}

// sendNewOutputs runs a pipeline that sends as told and reads the accepted p into got.
func sendNewOutputs(t *testing.T, def, send, read, typ string) map[string]Value {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		`+def+`
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender { `+send+` }
			action reader accept p : `+typ+` { `+read+` }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("%s %v", send, err)
	}
	return outputs
}

// A constructor that labels one feature twice fails at the send, whichever
// spelling names it: the same label or its qualified form; the name a
// redefinition masks is no member. Nothing is validated first; the runtime alone rejects it.
func TestSendNewRejectsDuplicateLabelsAtSend(t *testing.T) {
	for _, tc := range []struct{ args, want string }{
		{"b = 1, b = 2", "b is bound twice"},
		{"b = 1, Sub::b = 2", "b is bound twice"},
		{"a = 1, b = 2", "a is not a feature of Sub"},
	} {
		t.Run(tc.args, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
				private import ScalarValues::*;
				attribute def Base { attribute a : Integer; attribute k : Integer; }
				attribute def Sub :> Base { attribute b redefines a; }
				action pipeline {
					first start;
					action sender { send new Sub(`+tc.args+`) to reader; }
					action reader accept got : Sub;
					done;
					succession first start then sender;
					succession first sender then reader;
					succession first reader then done;
				}
			}`))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
			if sym == nil {
				t.Fatal("action pipeline not found")
			}
			_, err := ctx.ExecuteAction(sym)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("send new Sub(%s): error %v, want %q", tc.args, err, tc.want)
			}
			if errors.Is(err, ErrAcceptDeadlock) {
				t.Errorf("send new Sub(%s): reported as a deadlock, want the send itself rejected", tc.args)
			}
		})
	}
}

// A qualified label is resolved whole at the send: `Sub::b`/`Base::a` bind their
// feature, while `Other::b` is rejected even though Sub has a `b`, unvalidated.
func TestSendNewResolvesQualifiedLabelsAtSend(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		attribute def Base { attribute a : Integer; attribute k : Integer; }
		attribute def Sub :> Base { attribute b redefines a; }
		attribute def Other { attribute b : Integer; attribute k : Integer; }
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender { send new Sub(%s) to reader; }
			action reader accept msg : Sub { assign got := msg.b * 10 + msg.k; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`
	for _, tc := range []struct {
		args string
		want int64
	}{
		{"Sub::b = 4, Sub::k = 6", 46},
		{"b = 4, Base::k = 6", 46},
		{"P::Sub::b = 4, k = 6", 46},
	} {
		t.Run(tc.args, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(model, tc.args)))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
			if sym == nil {
				t.Fatal("action pipeline not found")
			}
			outputs, err := ctx.ExecuteAction(sym)
			if err != nil {
				t.Fatalf("send new Sub(%s): %v", tc.args, err)
			}
			assertIntOutput(t, outputs, "got", tc.want)
		})
	}
	for _, tc := range []struct{ args, want string }{
		{"Other::b = 4, k = 6", "Other::b is not a feature of Sub"},
		{"b = 4, Other::k = 6", "Other::k is not a feature of Sub"},
		{"Base::a = 4, Base::k = 6", "Base::a is not a feature of Sub"},
		{"Sub::b = 4, Base::a = 6", "Base::a is not a feature of Sub"},
	} {
		t.Run(tc.args, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(model, tc.args)))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
			if sym == nil {
				t.Fatal("action pipeline not found")
			}
			_, err := ctx.ExecuteAction(sym)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("send new Sub(%s): error %v, want %q", tc.args, err, tc.want)
			}
			if errors.Is(err, ErrAcceptDeadlock) {
				t.Errorf("send new Sub(%s): reported as a deadlock, want the send itself rejected", tc.args)
			}
		})
	}
}

// A positional argument beyond the constructed type's features fails at the
// send, also when no accept ever consumes the message. Nothing is validated first.
func TestSendNewRejectsExcessPositionalArgumentsAtSend(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		item def Empty;
		item def One { attribute a : Integer; }
		action pipeline {
			first start;
			action sender { send new Empty(1) to nobody; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
		action two {
			first start;
			action sender { send new One(1, 2) to nobody; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`))
	for _, tc := range []struct{ action, want string }{
		{"pipeline", "new Empty takes 0 argument(s), found 1"},
		{"two", "new One takes 1 argument(s), found 2"},
	} {
		sym := findSymbolByName(idx.DocumentRoot("<test>"), tc.action, ast.DefAction)
		if sym == nil {
			t.Fatalf("action %s not found", tc.action)
		}
		if _, err := ctx.ExecuteAction(sym); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want %q", tc.action, err, tc.want)
		}
	}
}

// An argument the feature does not admit, by type or by count, fails at the send
// whether an accept consumes the message or none does. Nothing is validated first.
// An object is admitted by what it is, not by what its feature is typed by.
func TestSendNewRejectsInadmissibleArgumentsAtSend(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		item def Base;
		item def Telemetry :> Base;
		item def Reading { attribute n : Integer; attribute pair : Integer[2]; ref item src : Telemetry; }
		part station { part base : Base; part tele : Telemetry; }
		action pipeline {
			first start;
			action sender { send new Reading(%s) to %s; }
			action reader accept r : Reading;
			done;
			succession first start then sender;
			succession first sender then %s;
			succession first reader then done;
		}
	}`
	t.Run("special object admitted", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(model, `n = 7, pair = (1, 2), src = station.tele`, "reader", "reader")))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
		if sym == nil {
			t.Fatal("action pipeline not found")
		}
		if _, err := ctx.ExecuteAction(sym); err != nil {
			t.Fatalf("send new Reading(src = station.tele): %v", err)
		}
	})
	for _, tc := range []struct{ args, want string }{
		{`"seven", (1, 2)`, "feature value Reading.n: type mismatch"},
		{`n = 7, pair = 1`, "feature value Reading.pair: multiplicity violation"},
		{`n = 7, pair = (1, 2), src = station.base`, "feature value Reading.src: type mismatch"},
	} {
		for _, receiver := range []string{"reader", "nobody"} {
			t.Run(tc.args+" to "+receiver, func(t *testing.T) {
				next := "reader"
				if receiver == "nobody" {
					next = "done"
				}
				idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(model, tc.args, receiver, next)))
				sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
				if sym == nil {
					t.Fatal("action pipeline not found")
				}
				created := len(ctx.created)
				_, err := ctx.ExecuteAction(sym)
				if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "send new Reading") {
					t.Fatalf("send new Reading(%s): error %v, want the send rejected with %q", tc.args, err, tc.want)
				}
				if errors.Is(err, ErrAcceptDeadlock) {
					t.Errorf("send new Reading(%s): reported as a deadlock, want the send itself rejected", tc.args)
				}
				if len(ctx.created) != created {
					t.Errorf("send new Reading(%s) left %d instance(s) behind", tc.args, len(ctx.created)-created)
				}
			})
		}
	}
}

// A constructed message whose delivery fails (bad address, unjoined port) leaves
// nothing behind — not the occurrence, not an object its payload created, not the
// behaviors that object started — in an action or a state machine alike.
func TestSendNewLeavesNothingBehindWhenDeliveryFails(t *testing.T) {
	const src = `package test {
		private import ScalarValues::*;
		item def Ping;
		item def Tagged { ref source : Beacon; }
		part def Beacon {
			exhibit state machine { entry; then idle; state idle; }
		}
		part def Node {
			attribute count : Integer = 0;
			port lonely;
			action tag {
				first start;
				action sender { send new Tagged(source = station.beacon) via lonely; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
			action listen {
				first start;
				action sender { send new Ping() to alpha.count; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
			action ship {
				first start;
				action sender { send new Ping() via lonely; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
			state machine {
				entry; then sending;
				state sending { entry send new Ping() via lonely; }
			}
			state tagging {
				entry; then sending;
				state sending { entry send new Tagged(source = station.beacon) via lonely; }
			}
		}
		part alpha : Node;
		part station { part beacon : Beacon; }
	}`
	for _, tc := range []struct{ name, behavior string }{
		{"action addressed to an attribute", "test::Node::listen"},
		{"action through an unjoined port", "test::Node::ship"},
		{"action whose payload creates a behaving object", "test::Node::tag"},
		{"state machine through an unjoined port", "test::Node::machine"},
		{"state machine whose payload creates a behaving object", "test::Node::tagging"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
			alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
			sym := oneSymbol(t, idx, tc.behavior)
			var err error
			if sym.Kind == symbols.SymbolStateUsage {
				_, _, err = ctx.ExecuteStatePerformedBy(sym, alpha, nil)
			} else {
				_, err = ctx.ExecuteActionPerformedBy(sym, alpha, nil)
			}
			if !errors.Is(err, ErrUnroutableSend) {
				t.Fatalf("%s: %v, want %v", tc.behavior, err, ErrUnroutableSend)
			}
			for _, inst := range ctx.instances {
				if inst.Type != nil && (inst.Type.Name == "Ping" || inst.Type.Name == "Tagged" || inst.Type.Name == "station" || inst.Type.Name == "Beacon") {
					t.Errorf("a %s the failed send created is still alive as instance %d", inst.Type.Name, inst.ID)
				}
			}
			for _, behavior := range append(append([]*ObjectBehavior(nil), ctx.objectBehaviors...), ctx.pendingBehaviors...) {
				if _, live := ctx.instances[behavior.Object.ID]; !live {
					t.Errorf("%s still attached to abandoned instance %d", behavior.Describe(), behavior.Object.ID)
				}
			}
			if len(ctx.PendingMessages()) != 0 {
				t.Errorf("the failed send posted %+v", ctx.PendingMessages())
			}
		})
	}
}

// A constructed message that cannot be built — a later argument names no feature,
// or does not fit the one it names — leaves nothing an earlier argument created
// behind, not the object it materialized nor the behaviors that object started.
func TestSendNewLeavesNothingBehindWhenBuildFails(t *testing.T) {
	const src = `package test {
		private import ScalarValues::*;
		item def Tagged { ref source : Beacon; attribute count : Integer[2..3]; }
		part def Beacon {
			exhibit state machine { entry; then idle; state idle; }
		}
		part def Node {
			port lonely;
			action mislabel {
				first start;
				action sender { send new Tagged(source = station.beacon, bogus = 1) via lonely; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
			action misfit {
				first start;
				action sender { send new Tagged(source = station.beacon, count = 7) via lonely; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
			state mislabeling {
				entry; then sending;
				state sending { entry send new Tagged(source = station.beacon, bogus = 1) via lonely; }
			}
			state misfitting {
				entry; then sending;
				state sending { entry send new Tagged(source = station.beacon, count = 7) via lonely; }
			}
		}
		part alpha : Node;
		part station { part beacon : Beacon; }
	}`
	for _, tc := range []struct{ name, behavior, want string }{
		{"action with an unknown label", "test::Node::mislabel", "bogus is not a feature of Tagged"},
		{"action with a misfitting value", "test::Node::misfit", "multiplicity"},
		{"state machine with an unknown label", "test::Node::mislabeling", "bogus is not a feature of Tagged"},
		{"state machine with a misfitting value", "test::Node::misfitting", "multiplicity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
			alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
			sym := oneSymbol(t, idx, tc.behavior)
			var err error
			if sym.Kind == symbols.SymbolStateUsage {
				_, _, err = ctx.ExecuteStatePerformedBy(sym, alpha, nil)
			} else {
				_, err = ctx.ExecuteActionPerformedBy(sym, alpha, nil)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: error %v, want one mentioning %q", tc.behavior, err, tc.want)
			}
			for _, inst := range ctx.instances {
				if inst.Type != nil && (inst.Type.Name == "Tagged" || inst.Type.Name == "station" || inst.Type.Name == "Beacon") {
					t.Errorf("a %s the failed send created is still alive as instance %d", inst.Type.Name, inst.ID)
				}
			}
			for _, behavior := range append(append([]*ObjectBehavior(nil), ctx.objectBehaviors...), ctx.pendingBehaviors...) {
				if _, live := ctx.instances[behavior.Object.ID]; !live {
					t.Errorf("%s still attached to abandoned instance %d", behavior.Describe(), behavior.Object.ID)
				}
			}
			if len(ctx.PendingMessages()) != 0 {
				t.Errorf("the failed send posted %+v", ctx.PendingMessages())
			}
		})
	}
}

// A label names only a feature the constructor binds: an inherited library
// descriptor is refused at the send; a restatement of it in the type binds.
func TestSendNewLabelMustNameConstructibleFeature(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		item def Box { attribute w : Integer; }
		item def Solid :> Box { attribute :>> isSolid = false; }
		action inherited {
			first start;
			action sender { send new Box(w = 7, isSolid = true) to reader; }
			action reader accept b : Box;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
		action restated {
			attribute got : Integer = 0;
			attribute solid : Boolean = false;
			first start;
			action sender { send new Solid(w = 7, isSolid = true) to reader; }
			action reader accept b : Solid { assign got := b.w; assign solid := b.isSolid; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`))
	root := idx.DocumentRoot("<test>")
	sym := findSymbolByName(root, "inherited", ast.DefAction)
	if sym == nil {
		t.Fatal("action inherited not found")
	}
	const want = "isSolid is not a feature a constructor of Box binds"
	if _, err := ctx.ExecuteAction(sym); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("inherited: error %v, want %q", err, want)
	}
	sym = findSymbolByName(root, "restated", ast.DefAction)
	if sym == nil {
		t.Fatal("action restated not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("restated: %v", err)
	}
	assertIntOutput(t, outputs, "got", 7)
	if v := outputs["solid"]; v.Kind != ValConst || !v.Const.Bool {
		t.Errorf("solid = %+v, want true", v)
	}
}

// A constructor names a type: an unresolved name or a package is rejected at
// the send rather than posted as a message of that name. Nothing is validated first.
func TestSendNewRejectsNonTypeTargetsAtSend(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		package Q { item def Sig { attribute a : Integer; } }
		action missing {
			first start;
			action sender { send new Missing(a = 1) to nobody; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
		action pkg {
			first start;
			action sender { send new Q() to nobody; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`))
	for _, tc := range []struct{ action, want string }{
		{"missing", "send new Missing: unresolved reference: Missing"},
		{"pkg", "send new Q: Q is a package, not a type"},
	} {
		sym := findSymbolByName(idx.DocumentRoot("<test>"), tc.action, ast.DefAction)
		if sym == nil {
			t.Fatalf("action %s not found", tc.action)
		}
		_, err := ctx.ExecuteAction(sym)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want %q", tc.action, err, tc.want)
		}
		if errors.Is(err, ErrAcceptDeadlock) {
			t.Errorf("%s: reported as a deadlock, want the send itself rejected", tc.action)
		}
	}
}

// A usage is a type too: `new sig(a = 4)` with `item sig : Sig` constructs an
// occurrence an `accept : Sig` binds, carrying the argument.
func TestSendNewConstructsAUsage(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		item def Sig { attribute a : Integer; }
		item sig : Sig;
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender { send new sig(a = 4) to reader; }
			action reader accept msg : Sig { assign got := msg.a * 10; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("send new sig(a = 4): %v", err)
	}
	assertIntOutput(t, outputs, "got", 40)
}

// executeActionSource executes the named action declared in src.
func executeActionSource(t *testing.T, name, src string) (map[string]Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefAction)
	if sym == nil {
		t.Fatalf("action %s not found", name)
	}
	return ctx.ExecuteAction(sym)
}

// A send addresses a specific consumer: an accept action named by no send must
// not take a message addressed to a sibling. The accept suspends waiting for
// its own message, and since nothing else can post one the run deadlocks.
func testSendReachesOnlyItsAddressee(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender { send 7 to wanted; }
			action other accept n : Integer;
			action reader { assign got := n; }
			done;
			succession first start then sender;
			succession first sender then other;
			succession first other then reader;
			succession first reader then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: the message is addressed to `wanted`, not `other`")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
}

// An accept whose type no in-flight message carries waits for its own type
// rather than binding the wrong message or silently continuing, and reports
// the type it is still waiting for when the wait can never end.
func testAcceptOfUnsentTypeReports(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : String = "none";
			first start;
			action sender { send 7 to reader; }
			action reader accept text : String;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: only an Integer was sent")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
	if !strings.Contains(err.Error(), "String") {
		t.Errorf("expected the accepted type in the message, got: %v", err)
	}
}

// The bus is context-wide: a message an action sends reaches a state machine
// executed later in the same context, which is why the two are not per-executor
// queues.
func TestActionMessageReachesStateMachine(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		action pipeline {
			first start;
			action sender { send Ping to Driver; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
		state Driver {
			entry; then init;
			state init;
			state waiting;
			succession first init then waiting;
			transition first waiting when Ping then done;
		}
	}`))
	root := idx.DocumentRoot("<test>")
	actionSym := findSymbolByName(root, "pipeline", ast.DefAction)
	stateSym := findSymbolByName(root, "Driver", ast.DefState)
	if actionSym == nil || stateSym == nil {
		t.Fatal("pipeline or Driver not found")
	}
	if _, err := ctx.ExecuteAction(actionSym); err != nil {
		t.Fatalf("execute action: %v", err)
	}
	_, visits, err := ctx.ExecuteStateWithEvents(stateSym, nil)
	if err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	assertVisits(t, visits, "init", "waiting", "done")
}

// A message nobody accepts stays in flight rather than being consumed by an
// unrelated accept: it must remain observable on the bus.
func TestUnacceptedMessageStaysPending(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender {
				send 3 to reader;
				send "spare" to nobody;
			}
			action reader accept n : Integer;
			action recorder { assign got := n; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then recorder;
			succession first recorder then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 3)

	pending := ctx.PendingMessages()
	if len(pending) != 1 {
		t.Fatalf("expected 1 message still in flight, got %d: %v", len(pending), pending)
	}
	if pending[0].SignalType != "String" || pending[0].Target != "nobody" {
		t.Errorf("unexpected pending message: %+v", pending[0])
	}
}

// A send whose message names a type sends that type, carrying no payload, so a
// state machine transition triggered by that signal fires.
func TestSendOfNamedTypeReachesStateMachine(t *testing.T) {
	_, visits, err := executeStateSource(t, "Driver", `package P {
		item def Ping;
		state Driver {
			entry; then start;
			state start;
			state waiting { entry { send Ping to Driver; } }
			succession first start then waiting;
			transition first waiting when Ping then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	assertVisits(t, visits, "start", "waiting", "done")
}

// A signal no transition out of the active configuration accepts is not
// swallowed: the machine suspends and the message stays in flight.
func TestStateMachineLeavesForeignSignalPending(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		item def Pong;
		state Driver {
			entry; then start;
			state start;
			state waiting { entry { send Pong to Driver; } }
			succession first start then waiting;
			transition first waiting when Ping then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Driver", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Driver not found")
	}
	if _, _, err := ctx.ExecuteStateWithEvents(sym, nil); err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 || pending[0].SignalType != "Pong" {
		t.Fatalf("expected Pong still in flight, got %v", pending)
	}
}

// A send through a port reaches the accept listening on the port connected to
// it: the connection, not the accept's name, is what addresses the message.
func TestSendViaPortReachesConnectedAccept(t *testing.T) {
	outputs, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : Integer = 0;
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort; }
			action reader accept msg : Integer via inPort;
			action recorder { assign got := msg; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then recorder;
			succession first recorder then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 42)
}

const commandItem = "\n\titem def Command { attribute code : Integer; }\n" +
	"\tport def CommandPort { out item issued : Command; }\n"

// An item usage is a message of its own (`item cmd : Command { … }` then
// `send cmd via p`): the object is typed by the definition typing the usage, so
// an accept of that definition takes it and reads its features.
func TestSendOfAnItemObjectIsTypedByItsDefinition(t *testing.T) {
	outputs, err := executeActionSource(t, "ship", `package P {
		private import ScalarValues::*;`+commandItem+`
		action ship {
			attribute got : Integer = 0;
			item cmd : Command { attribute :>> code = 7; }
			port src : CommandPort;
			port dst : ~CommandPort;
			connect src to dst;
			first start;
			action sender { send cmd via src; }
			action reader accept order : Command via dst { assign got := order.code; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 7)
}

// The type of such a message is the definition, not the usage sent, so an accept
// naming the usage waits on: nothing was sent of that type.
func TestSendOfAnItemObjectIsNotTypedByTheUsageSent(t *testing.T) {
	_, err := executeActionSource(t, "ship", `package P {
		private import ScalarValues::*;`+commandItem+`
		action ship {
			item cmd : Command { attribute :>> code = 7; }
			port src : CommandPort;
			port dst : ~CommandPort;
			connect src to dst;
			first start;
			action sender { send cmd via src; }
			action reader accept order : cmd via dst;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("execute action: err = %v, want %v", err, ErrAcceptDeadlock)
	}
}

// A standard-library feature named receiver must not shadow the sibling
// receiving node a routed send addresses.
func TestSendViaPortToReceiverWithLibraries(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		action pipeline {
			attribute got : Integer = 0;
			port senderPort;
			port receiverPort;
			connect senderPort to receiverPort;
			first start;
			action sender {
				send 42 via senderPort to receiver;
			}
			action receiver accept value : Integer via receiverPort {
				assign got := value;
			}
			done;
			succession first start then sender;
			succession first sender then receiver;
			succession first receiver then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute action with libraries: %v", err)
	}
	assertIntOutput(t, outputs, "got", 42)
}

// A receiving node remains reachable through nested behavior scopes.
func TestSendViaPortToNestedReceiver(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		action pipeline {
			attribute got : Integer = 0;
			port senderPort;
			port receiverPort;
			connect senderPort to receiverPort;
			first start;
			action group {
				first emit;
				action emit {
					if true {
						send 1 via senderPort to receiver;
					}
				}
				done;
				succession first emit then done;
			}
			action receiver accept value : Integer via receiverPort {
				assign got := value;
			}
			done;
			succession first start then group;
			succession first group then receiver;
			succession first receiver then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute nested action with libraries: %v", err)
	}
	assertIntOutput(t, outputs, "got", 1)
}

// Library names must not turn an unknown routed receiver into a silent send.
func TestSendViaPortToUnknownReceiverWithLibraries(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		action pipeline {
			port senderPort;
			port receiverPort;
			connect senderPort to receiverPort;
			first start;
			action sender {
				send 42 via senderPort to recevier;
			}
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	_, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, ErrUnreachableSendReceiver) {
		t.Fatalf("expected ErrUnreachableSendReceiver, got: %v", err)
	}
}

// A port-routed message is only for the accept on the port it arrived at: an
// accept listening on no port does not take it, however well its type matches.
func TestPortRoutedMessageBypassesPortlessAccept(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort; }
			action reader accept msg : Integer;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock for an accept on no port, got: %v", err)
	}
}

// The converse: an accept listening on a port does not take an addressed
// message, which travelled over no connection.
func TestAddressedMessageBypassesPortAccept(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port inPort;
			first start;
			action sender { send 42 to reader; }
			action reader accept msg : Integer via inPort;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock for an accept on a port, got: %v", err)
	}
	if !strings.Contains(err.Error(), "via inPort") {
		t.Errorf("expected the awaited port in the message, got: %v", err)
	}
}

// A connection joins its ends without a direction, so a send through either end
// reaches the other: the accept listens on the end the send did not name.
func TestSendViaPortRoutesInEitherDirection(t *testing.T) {
	outputs, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : Integer = 0;
			port left;
			port right;
			connect right to left;
			first start;
			action sender { send 7 via left; }
			action reader accept msg : Integer via right;
			action recorder { assign got := msg; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then recorder;
			succession first recorder then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 7)
}

// A state machine's transitions accept signals, never ports, so a message routed
// to a port is not swallowed by a machine that would otherwise react to it.
func TestPortRoutedMessageDoesNotReachStateMachine(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		action pipeline {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send Ping via outPort; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
		state Driver {
			entry; then init;
			state init;
			state waiting;
			succession first init then waiting;
			transition first waiting when Ping then done;
		}
	}`))
	root := idx.DocumentRoot("<test>")
	actionSym := findSymbolByName(root, "pipeline", ast.DefAction)
	stateSym := findSymbolByName(root, "Driver", ast.DefState)
	if actionSym == nil || stateSym == nil {
		t.Fatal("pipeline or Driver not found")
	}
	if _, err := ctx.ExecuteAction(actionSym); err != nil {
		t.Fatalf("execute action: %v", err)
	}
	if _, _, err := ctx.ExecuteStateWithEvents(stateSym, nil); err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 || pending[0].Port != "inPort" {
		t.Fatalf("expected the port-routed Ping still in flight, got %v", pending)
	}
}

// The two judgements of whether a machine reacts to a message — matchesEvent,
// dispatching a queued occurrence to a transition, and acceptsSignalFrom,
// deciding whether a state takes a message in flight — agree for a transfer
// addressed to the performer, addressed to its port, and routed to its port: a
// via-less accept receives only what is addressed to the performer itself.
func TestAcceptRoutingAgreesBetweenDispatchAndAcceptance(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		port def PingPort {
			in item ping : Ping;
		}
		part def Node {
			port inPort : PingPort;
			exhibit state Listen {
				entry; then waiting;
				state waiting;
				state onPort;
				state received;
				transition byPort first waiting accept Ping via inPort then onPort;
				transition byPart first waiting accept Ping then received;
			}
		}
		part def Emitter {
			port outPort : ~PingPort;
			action routed {
				first start;
				action emit { send new Ping() via outPort; }
				succession first start then emit;
			}
		}
		part def Group {
			part alpha : Node;
			part emitter : Emitter;
			connect emitter.outPort to alpha.inPort;
			action toPort {
				first start;
				action emit { send new Ping() to alpha.inPort; }
				succession first start then emit;
			}
			action toPart {
				first start;
				action emit { send new Ping() to alpha; }
				succession first start then emit;
			}
		}
	}`))
	root := idx.DocumentRoot("<test>")
	group, err := ctx.Instantiate(oneSymbol(t, idx, "P::Group"))
	if err != nil {
		t.Fatalf("instantiate Group: %v", err)
	}
	alpha := instanceAtPath(t, ctx, group, "alpha")
	emitter := instanceAtPath(t, ctx, group, "emitter")
	exec := objectMachine(t, alpha, "")
	if err := exec.RunToQuiescence(); err != nil {
		t.Fatalf("settle Listen: %v", err)
	}
	active := exec.ActiveStates()
	if len(active) != 1 || active[0].Name != "waiting" {
		t.Fatalf("Listen settled in %v, want waiting", active)
	}
	waiting := active[0]

	// sent runs a send by self and returns the one message it left in flight.
	sent := func(owner, action string, self *Instance) Message {
		before := len(ctx.PendingMessages())
		sym := findSymbolByName(findSymbolByName(root, owner, ast.DefPart).Scope, action, ast.DefAction)
		if sym == nil {
			t.Fatalf("%s::%s not found", owner, action)
		}
		actionExec, err := ctx.CreateActionExecutorFor(sym, self)
		if err != nil {
			t.Fatalf("create %s: %v", action, err)
		}
		if err := actionExec.RunToCompletion(); err != nil {
			t.Fatalf("run %s: %v", action, err)
		}
		pending := ctx.PendingMessages()
		if len(pending) != before+1 {
			t.Fatalf("%s left %d messages in flight, want 1", action, len(pending)-before)
		}
		return pending[before]
	}

	tests := []struct {
		name  string
		msg   Message
		fires string
	}{
		{"port-addressed", sent("Group", "toPort", group), "byPort"},
		{"part-addressed", sent("Group", "toPart", group), "byPart"},
		{"port-routed", sent("Emitter", "routed", emitter), "byPort"},
	}
	for _, tc := range tests {
		var fired []string
		for _, trans := range exec.graph.Transitions[waiting] {
			matches, err := exec.matchesEvent(trans, &Event{Type: EventAccept, Payload: tc.msg})
			if err != nil {
				t.Fatalf("%s: matchesEvent %s: %v", tc.name, trans.Name, err)
			}
			if matches {
				fired = append(fired, trans.Name)
			}
		}
		if len(fired) != 1 || fired[0] != tc.fires {
			t.Errorf("%s: matchesEvent enables %v, want [%s]", tc.name, fired, tc.fires)
		}
		accepts, err := exec.acceptsSignalFrom(waiting, tc.msg)
		if err != nil {
			t.Fatalf("%s: acceptsSignalFrom: %v", tc.name, err)
		}
		if accepts != (len(fired) > 0) {
			t.Errorf("%s: acceptsSignalFrom = %v while matchesEvent enables %v", tc.name, accepts, fired)
		}
	}
}

// A call event fires only the transition triggered by the operation invoked,
// and only when the call carries every argument the trigger declares.
func TestCallEventMatchesOperationName(t *testing.T) {
	callTrigger := func(operation string, params ...string) *lower.Transition {
		declared := make([]ast.NameSegment, len(params))
		for i, param := range params {
			declared[i] = ast.NameSegment{Text: param}
		}
		trigger := &ast.CallEvent{Parameters: declared}
		if operation != "" {
			trigger.Operation = &ast.QualifiedName{Parts: []ast.NameSegment{{Text: operation}}}
		}
		return &lower.Transition{Trigger: trigger}
	}
	callEvent := func(operation string, args ...string) *Event {
		call := Call{Operation: operation, Args: make(map[string]Value, len(args))}
		for _, arg := range args {
			call.Args[arg] = Value{}
		}
		return &Event{Type: EventCall, Payload: call}
	}

	exec := &StateExecutor{}
	tests := []struct {
		name    string
		trans   *lower.Transition
		event   *Event
		matches bool
	}{
		{"same operation", callTrigger("open"), callEvent("open"), true},
		{"other operation", callTrigger("open"), callEvent("close"), false},
		{"unnamed operation accepts any", callTrigger(""), callEvent("close"), true},
		{"accept trigger ignores calls", &lower.Transition{Trigger: &ast.AcceptEvent{}}, callEvent("open"), false},
		{"declared parameter present", callTrigger("open", "angle"), callEvent("open", "angle"), true},
		{"declared parameter missing", callTrigger("open", "angle"), callEvent("open"), false},
		{"one declared parameter of several missing", callTrigger("open", "angle", "speed"), callEvent("open", "angle"), false},
		{"undeclared arguments ignored", callTrigger("open"), callEvent("open", "angle"), true},
	}
	for _, tc := range tests {
		got, err := exec.matchesEvent(tc.trans, tc.event)
		if err != nil {
			t.Fatalf("%s: matchesEvent: %v", tc.name, err)
		}
		if got != tc.matches {
			t.Errorf("%s: matchesEvent = %v, want %v", tc.name, got, tc.matches)
		}
	}
}

// A call whose guard rejects it must leave the machine's data as it was: its
// arguments are bound only for the guard, and undone when nothing fires.
func TestRejectedCallLeavesNoArgumentsBehind(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state waiting;
			state moving;
			succession first init then waiting;
			transition first waiting accept setSpeed(value) if value > 0 then moving;
		}
	}`)
	exec.InvokeOperation("setSpeed", map[string]Value{
		"value": {Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 0}},
	})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	if value, held := exec.stateData["value"]; held {
		t.Errorf("the rejected call's argument is still in the machine's data: %v", value)
	}
}

// An accept whose message has not arrived parks the token rather than failing:
// the action is suspended at that node, and the token records what it waits for.
func TestAcceptParksTokenUntilMessageArrives(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action reader accept n : Integer;
			action recorder { assign got := n; }
			done;
			succession first start then reader;
			succession first reader then recorder;
			succession first recorder then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	// Step until the accept parks: the executor waits rather than erroring.
	for i := 0; i < 10 && exec.State() != StateWaiting; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if exec.State() != StateWaiting {
		t.Fatalf("expected the executor to be waiting, got %v", exec.State())
	}
	tokens := exec.Tokens()
	if len(tokens) != 1 || tokens[0].Wait == nil {
		t.Fatalf("expected one parked token, got %+v", tokens)
	}
	if tokens[0].Wait.ParamName != "n" || tokens[0].Wait.SignalType != "Integer" {
		t.Errorf("unexpected wait: %+v", *tokens[0].Wait)
	}
	// Step 1 moves the token off `start` onto the accept; step 2 finds nothing
	// it can take and parks it there.
	if tokens[0].Wait.Since != 2 {
		t.Errorf("expected the token to have parked at step 2, got %d", tokens[0].Wait.Since)
	}

	// A message posted from outside the action resumes it.
	nine := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 9}}
	ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Value: &nine})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume after the message arrived: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected the resumed action to complete, got %v", exec.State())
	}
	assertIntOutput(t, exec.Results(), "got", 9)
}

// A parked token holds its place in the queue's matching order: an accept that
// suspended before a message it cannot take arrived still takes only its own.
func TestParkedAcceptTakesOnlyItsOwnMessage(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action reader accept n : Integer;
			action recorder { assign got := n; }
			done;
			succession first start then reader;
			succession first reader then recorder;
			succession first recorder then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	for i := 0; i < 10 && exec.State() != StateWaiting; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	// A String arrives first and must be left in flight; the Integer resumes it.
	text := NewStringValue("not for you")
	four := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 4}}
	ctx.PostMessage(Message{SignalType: "String", Target: "reader", Value: &text})
	ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Value: &four})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume after the message arrived: %v", err)
	}
	assertIntOutput(t, exec.Results(), "got", 4)

	pending := ctx.PendingMessages()
	if len(pending) != 1 || pending[0].SignalType != "String" {
		t.Fatalf("expected the String still in flight, got %v", pending)
	}
}

func assertVisits(t *testing.T, visits []string, want ...string) {
	t.Helper()
	if len(visits) != len(want) {
		t.Fatalf("visits = %v, want %v", visits, want)
	}
	for i, name := range want {
		if visits[i] != name {
			t.Fatalf("visits = %v, want %v", visits, want)
		}
	}
}

func assertIntOutput(t *testing.T, outputs map[string]Value, name string, want int64) {
	t.Helper()
	value, ok := outputs[name]
	if !ok {
		t.Fatalf("output %q missing from %v", name, outputs)
	}
	if value.Const.Int != want {
		t.Errorf("%s = %v, want %d", name, value.Const.Int, want)
	}
}

// Routing honors the selection a variation is bound to: a message sent through a
// port reaches the ports the selected `variant interface`'s connection joins it
// to, and not the ones an unselected variant would (SysML v2 §7.20). A
// connection belonging to no variation always routes.
func TestRoutingHonorsTheSelectedVariantConnection(t *testing.T) {
	conns := []lower.Connection{
		{Ends: []string{"outPort", "inPort"}, Variation: "link", Variant: "direct"},
		{Ends: []string{"outPort", "bypass"}, Variation: "link", Variant: "indirect"},
		{Ends: []string{"outPort", "always"}},
	}
	for _, tt := range []struct {
		selected string
		want     []string
	}{
		{"", []string{"always"}},
		{"direct", []string{"inPort", "always"}},
		{"indirect", []string{"bypass", "always"}},
	} {
		_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test { }`))
		if tt.selected != "" {
			ctx.selectedVariants[variantSelection{variation: "link"}] = tt.selected
		}
		if err := ctx.postVia(conns, Message{SignalType: "Ping"}, lower.Send{Target: "outPort", IsVia: true}, nil); err != nil {
			t.Fatalf("selection %q: %v", tt.selected, err)
		}
		var got []string
		for _, msg := range ctx.PendingMessages() {
			got = append(got, msg.Port)
		}
		if len(got) != len(tt.want) {
			t.Fatalf("selection %q routed to %v, want %v", tt.selected, got, tt.want)
		}
		for i, port := range tt.want {
			if got[i] != port {
				t.Fatalf("selection %q routed to %v, want %v", tt.selected, got, tt.want)
			}
		}
	}
}

// Two objects of one type each selecting a different variant of one variation
// route over their own selection: a message a behavior of one object sends
// reaches the ports that object's selected connection joins, and no port of the
// other object's (SysML v2 §7.20).
func TestRoutingIsPerOwnerVariantSelection(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		port def P;
		part def Sys {
			port outPort : P;
			port inPort : P;
			port bypass : P;
			variation interface link {
				variant interface direct connect outPort to inPort;
				variant interface indirect connect outPort to bypass;
			}
		}
		part alpha : Sys { interface :>> link = link::direct; }
		part beta : Sys { interface :>> link = link::indirect; }
	}`))
	for usage, want := range map[string]string{"test::alpha": "inPort", "test::beta": "bypass"} {
		self, err := ctx.Instantiate(oneSymbol(t, idx, usage))
		if err != nil {
			t.Fatalf("%s: %v", usage, err)
		}
		if err := ctx.postVia(nil, Message{SignalType: "Ping"}, lower.Send{Target: "outPort", IsVia: true}, self); err != nil {
			t.Fatalf("%s: %v", usage, err)
		}
		var got []string
		for _, msg := range ctx.PendingMessages() {
			if msg.Object != self.ID {
				t.Errorf("%s routed a message into object %d", usage, msg.Object)
			}
			got = append(got, msg.Port)
		}
		if len(got) != 1 || got[0] != want {
			t.Errorf("%s routed to %v, want [%s]", usage, got, want)
		}
		ctx.messages = nil
	}
}

// A `send … to r` naming a receiving node is for the object performing the
// sending behavior, not another object declaring the same node.
func TestAddressedSendStaysWithinTheSendingObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		item def Ping;
		part def Node {
			action listen {
				first start;
				action sender { send Ping() to reader; }
				action reader accept ping : Ping;
				done;
				succession first start then sender;
				succession first sender then reader;
				succession first reader then done;
			}
		}
		part alpha : Node;
		part beta : Node;
	}`))
	alpha, beta := instanceOfUsage(t, ctx, idx, "test::alpha"), instanceOfUsage(t, ctx, idx, "test::beta")
	send := lower.Send{Target: "reader", Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	if err := ctx.post(nil, Message{SignalType: "Ping"}, send, alpha); err != nil {
		t.Fatalf("post: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 {
		t.Fatalf("posted %d messages, want 1", len(pending))
	}
	if got := pending[0]; !got.reaches("reader", "", alpha.ID) || got.reaches("reader", "", beta.ID) {
		t.Errorf("message %+v is not confined to the sending object %d", got, alpha.ID)
	}
}

// An addressed target resolves through the instance graph: `alpha.inPort` names
// that object's port, not a same-named port of the sender.
func TestAddressedSendResolvesPortOfNamedObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Node {
			port inPort : PingPort;
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
		part beta : Node;
	}`))
	alpha, beta := instanceOfUsage(t, ctx, idx, "test::alpha"), instanceOfUsage(t, ctx, idx, "test::beta")
	send := lower.Send{Target: "alpha.inPort", TargetPath: true, Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	if err := ctx.post(nil, Message{SignalType: "Ping"}, send, beta); err != nil {
		t.Fatalf("post: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 {
		t.Fatalf("posted %d messages, want 1", len(pending))
	}
	got := pending[0]
	if got.Port != "inPort" || got.Object != alpha.ID {
		t.Errorf("addressed alpha.inPort delivered as %+v, want port inPort of object %d", got, alpha.ID)
	}
	if got.reaches("", "inPort", beta.ID) {
		t.Errorf("message %+v reached the sending object %d, which owns a same-named port", got, beta.ID)
	}
}

// A target descends composite features to the object owning the port: the
// addressee of `inner.inPort` is that part of the sending object, not the sender.
func TestAddressedSendDescendsToNestedPort(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Leaf { port inPort : PingPort; }
		part def Node {
			part inner : Leaf;
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
	}`))
	alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
	send := lower.Send{Target: "inner.inPort", TargetPath: true, Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	if err := ctx.post(nil, Message{SignalType: "Ping"}, send, alpha); err != nil {
		t.Fatalf("post: %v", err)
	}
	inner, ok, err := ctx.fvObject(alpha, "inner")
	if err != nil {
		t.Fatalf("alpha.inner: %v", err)
	}
	if !ok {
		t.Fatal("part inner materialized no object")
	}
	got := ctx.PendingMessages()[0]
	if got.Port != "inPort" || got.Object != inner.ID {
		t.Errorf("addressed inner.inPort delivered as %+v, want port inPort of object %d", got, inner.ID)
	}
}

// A target reaching no port of an addressable object is a typed error, not a
// delivery to whatever else carries the last segment's name.
func TestAddressedSendToUnreachablePortIsTyped(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		private import ScalarValues::*;
		item def Ping;
		part def Node {
			attribute count : Integer = 0;
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
	}`))
	send := lower.Send{Target: "alpha.count", TargetPath: true, Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	err := ctx.post(nil, Message{SignalType: "Ping"}, send, instanceOfUsage(t, ctx, idx, "test::alpha"))
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("post to alpha.count: %v, want ErrUnroutableSend", err)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("an unroutable send posted %+v", ctx.PendingMessages())
	}
}

// A receiver named by a qualified name is the element the name resolves to, not
// a path through the sender's features: `::` separates namespaces, `.` chains
// features.
func TestAddressedSendToQualifiedNameReachesReceiver(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		action pipeline {
			first start;
			action sender { send Ping to P::Driver; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
		state Driver {
			entry; then init;
			state init;
			state waiting;
			succession first init then waiting;
			transition first waiting when Ping then done;
		}
	}`))
	root := idx.DocumentRoot("<test>")
	if _, err := ctx.ExecuteAction(findSymbolByName(root, "pipeline", ast.DefAction)); err != nil {
		t.Fatalf("execute action: %v", err)
	}
	_, visits, err := ctx.ExecuteStateWithEvents(findSymbolByName(root, "Driver", ast.DefState), nil)
	if err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	assertVisits(t, visits, "init", "waiting", "done")
}

// A qualified name addresses the element it resolves to, so a same-named feature
// of the sending object is not the addressee: no object owns a receiver declared
// in a package, so a sending object cannot address it rather than reach its own.
func TestAddressedSendToQualifiedNameSkipsSameNamedFeature(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		package Other {
			action reader accept n : Integer;
		}
		part def Node {
			action reader accept n : Integer;
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
	}`))
	alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
	send := lower.Send{Target: "Other::reader", Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	err := ctx.post(nil, Message{SignalType: "Integer"}, send, alpha)
	if !errors.Is(err, ErrUnroutableSend) {
		t.Errorf("`send to Other::reader` from an object: %v, want %v", err, ErrUnroutableSend)
	}
	for _, msg := range ctx.PendingMessages() {
		if msg.reaches("reader", "", alpha.ID) {
			t.Errorf("%+v is deliverable to the sender's own reader", msg)
		}
	}
}

// A behavior no object performs has no object of its own to reach instead, so it
// still addresses a receiver of a package by name.
func TestAddressedSendToQualifiedNameFromNoObjectIsDelivered(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		package Other {
			action reader accept n : Integer;
		}
		action listen { first start; done; succession first start then done; }
	}`))
	send := lower.Send{Target: "Other::reader", Scope: declScope(oneSymbol(t, idx, "test::listen"))}
	if err := ctx.post(nil, Message{SignalType: "Integer"}, send, nil); err != nil {
		t.Fatalf("post: %v", err)
	}
	if got := ctx.PendingMessages()[0]; got.Target != "reader" || !got.reaches("reader", "", 0) {
		t.Errorf("`send to Other::reader` delivered as %+v, want the reader of Other", got)
	}
}

// An address naming an object alone names no receiver within it, so only that
// object's own accepts take it — not a behavior of unknown performer.
func TestAddressedSendToAnObjectNeedsThatObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def Leaf { attribute count : Integer = 0; }
		part def Node {
			part leaf : Leaf;
			action talk { first start; done; succession first start then done; }
		}
		part alpha : Node;
	}`))
	alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
	send := lower.Send{Target: "leaf", Scope: declScope(oneSymbol(t, idx, "test::Node::talk"))}
	if err := ctx.post(nil, Message{SignalType: "Integer"}, send, alpha); err != nil {
		t.Fatalf("post: %v", err)
	}
	got := ctx.PendingMessages()[0]
	if got.Target != "" || got.Object == 0 || got.Object == alpha.ID {
		t.Errorf("`send to leaf` delivered as %+v, want the object of leaf alone", got)
	}
	if got.reaches("anything", "", 0) {
		t.Errorf("%+v is deliverable to a behavior no object performs", got)
	}
}

// A receiver of another object carries that object's identity, so the sender's
// own same-named receiver cannot take the message.
func TestAddressedSendToReceiverOfAnotherObjectCarriesItsIdentity(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def Node { action reader accept n : Integer; }
		part def Talker {
			action reader accept n : Integer;
			action talk { first start; done; succession first start then done; }
		}
		part alpha : Node;
		part beta : Talker;
	}`))
	alpha, beta := instanceOfUsage(t, ctx, idx, "test::alpha"), instanceOfUsage(t, ctx, idx, "test::beta")
	send := lower.Send{Target: "alpha::reader", Scope: declScope(oneSymbol(t, idx, "test::Talker::talk"))}
	if err := ctx.post(nil, Message{SignalType: "Integer"}, send, beta); err != nil {
		t.Fatalf("post: %v", err)
	}
	got := ctx.PendingMessages()[0]
	if got.Target != "reader" || got.Object != alpha.ID {
		t.Errorf("`send to alpha::reader` delivered as %+v, want receiver reader of object %d", got, alpha.ID)
	}
	if got.reaches("reader", "", beta.ID) {
		t.Errorf("%+v is deliverable to the sending object %d", got, beta.ID)
	}
}

// A node of the sending behavior is nearer than a feature of the object, and a
// name resolving to it addresses that node rather than the object's feature.
func TestAddressedSendPrefersTheNearerDeclaration(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def Leaf { attribute count : Integer = 0; }
		part def Node {
			part reader : Leaf;
			action listen {
				first start;
				action reader accept n : Integer;
				succession first start then reader;
				succession first reader then done;
				done;
			}
		}
		part alpha : Node;
	}`))
	alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
	send := lower.Send{Target: "reader", Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	if err := ctx.post(nil, Message{SignalType: "Integer"}, send, alpha); err != nil {
		t.Fatalf("post: %v", err)
	}
	got := ctx.PendingMessages()[0]
	if got.Target != "reader" || got.Object != alpha.ID || got.Port != "" {
		t.Errorf("`send to reader` delivered as %+v, want node reader of object %d", got, alpha.ID)
	}
}

// A port a qualified name resolves to that the sender owns no occurrence of is
// unroutable, not a delivery to the sender's same-named port.
func TestAddressedSendToQualifiedPortOfAnotherTypeIsTyped(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Other { port inPort : PingPort; }
		part def Node {
			port inPort : PingPort;
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
	}`))
	send := lower.Send{Target: "Other::inPort", Scope: declScope(oneSymbol(t, idx, "test::Node::listen"))}
	err := ctx.post(nil, Message{SignalType: "Ping"}, send, instanceOfUsage(t, ctx, idx, "test::alpha"))
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("post to Other::inPort: %v, want ErrUnroutableSend", err)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("an unroutable send posted %+v", ctx.PendingMessages())
	}
}

// A namespace qualifies the object a path starts from, so `test::alpha.inPort`
// reaches alpha's port rather than being read as a port of the sender.
func TestAddressedSendThroughNamespaceQualifiedPathReachesObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Node {
			port inPort : PingPort;
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
		part beta : Node;
	}`))
	alpha, beta := instanceOfUsage(t, ctx, idx, "test::alpha"), instanceOfUsage(t, ctx, idx, "test::beta")
	send := lower.Send{
		Target:     "test.alpha.inPort",
		TargetPath: true,
		Scope:      declScope(oneSymbol(t, idx, "test::Node::listen")),
	}
	if err := ctx.post(nil, Message{SignalType: "Ping"}, send, beta); err != nil {
		t.Fatalf("post: %v", err)
	}
	got := ctx.PendingMessages()[0]
	if got.Port != "inPort" || got.Object != alpha.ID {
		t.Errorf("addressed test::alpha.inPort delivered as %+v, want port inPort of object %d", got, alpha.ID)
	}
}

// A run that cannot build the object a target names fails as that: an exhausted
// budget is not an address naming nothing.
func TestAddressedSendReportsWhyTheObjectCouldNotBeBuilt(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Node {
			port inPort : PingPort;
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
	}`))
	send := lower.Send{
		Target:     "alpha.inPort",
		TargetPath: true,
		Scope:      declScope(oneSymbol(t, idx, "test::Node::listen")),
	}
	ctx.maxSteps = 0
	err := ctx.post(nil, Message{SignalType: "Ping"}, send, nil)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("post to alpha.inPort: %v, want ErrStepLimitExceeded", err)
	}
	if errors.Is(err, ErrUnroutableSend) {
		t.Errorf("an exhausted budget was reported as a bad address: %v", err)
	}
}

// A path led by a part naming several occurrences reaches no one object, so it
// is unroutable rather than attributed to the sending object.
func TestAddressedSendThroughMultiplePartIsTyped(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Leaf { port inPort : PingPort; }
		part def Node {
			part nodes : Leaf[3];
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
	}`))
	send := lower.Send{
		Target:     "nodes.inPort",
		TargetPath: true,
		Scope:      declScope(oneSymbol(t, idx, "test::Node::listen")),
	}
	err := ctx.post(nil, Message{SignalType: "Ping"}, send, nil)
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("post to nodes.inPort: %v, want ErrUnroutableSend", err)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("an unroutable send posted %+v", ctx.PendingMessages())
	}
}

// A path through a multi-valued feature of the sending object denotes every
// element it holds (KerML §7.3.4.6): one send delivers one message per element,
// each on that element's own identity, with the port path kept for each.
func TestAddressedSendFansOutOverAMultiValuedFeature(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Leaf { port inPort : PingPort; }
		part def Node {
			part nodes : Leaf[3];
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
	}`))
	alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
	send := lower.Send{
		Target:     "nodes.inPort",
		TargetPath: true,
		Scope:      declScope(oneSymbol(t, idx, "test::Node::listen")),
	}
	if err := ctx.post(nil, Message{SignalType: "Ping"}, send, alpha); err != nil {
		t.Fatalf("post to nodes.inPort: %v", err)
	}
	elements, err := ctx.fvObjects(alpha, "nodes")
	if err != nil || len(elements) != 3 {
		t.Fatalf("nodes elements = %v, err = %v, want 3", elements, err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 3 {
		t.Fatalf("pending messages = %+v, want one per element", pending)
	}
	for i, msg := range pending {
		if msg.Port != "inPort" || msg.Object != elements[i].ID {
			t.Errorf("message %d delivered as %+v, want port inPort of object %d", i, msg, elements[i].ID)
		}
	}
}

// An object's behavior addresses a receiving node of an action it performs: the
// performance is no object of its own, so identity must not exclude it.
func TestAddressedSendReachesPerformedAction(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		action listener {
			first start;
			action reader accept n : Integer;
			done;
			succession first start then reader;
			succession first reader then done;
		}
		part def Node {
			action main {
				first start;
				action sender { send 7 to reader; }
				perform listener;
				done;
				succession first start then sender;
				succession first sender then listener;
				succession first listener then done;
			}
		}
		part solo : Node;
	}`))
	main := oneSymbol(t, idx, "test::Node::main")
	solo := instanceOfUsage(t, ctx, idx, "test::solo")
	if _, err := ctx.ExecuteActionPerformedBy(main, solo, nil); err != nil {
		t.Fatalf("performed by an object: %v", err)
	}
}

// An object performs a behavior as itself however deeply it is performed, so an
// accept two performances down takes a message addressed to that object; the
// same behavior performed by no object has no identity to present and waits.
func TestPerformedBehaviorRunsAsItsPerformer(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		action inner {
			first start;
			action reader accept n : Integer;
			done;
			succession first start then reader;
			succession first reader then done;
		}
		action outer {
			first start;
			perform inner;
			done;
			succession first start then inner;
			succession first inner then done;
		}
		part def Node {
			action main {
				first start;
				action sender { send 7 to solo; }
				perform outer;
				done;
				succession first start then sender;
				succession first sender then outer;
				succession first outer then done;
			}
		}
		part solo : Node;
	}`))
	main := oneSymbol(t, idx, "test::Node::main")
	solo := instanceOfUsage(t, ctx, idx, "test::solo")
	if _, err := ctx.ExecuteActionPerformedBy(main, solo, nil); err != nil {
		t.Fatalf("performed by the object addressed: %v", err)
	}

	idx, _, ctx = buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		action inner {
			first start;
			action reader accept n : Integer;
			done;
			succession first start then reader;
			succession first reader then done;
		}
		part def Node {
			action main {
				first start;
				action sender { send 7 to solo; }
				perform inner;
				done;
				succession first start then sender;
				succession first sender then inner;
				succession first inner then done;
			}
		}
		part solo : Node;
	}`))
	main = oneSymbol(t, idx, "test::Node::main")
	if _, err := ctx.ExecuteActionPerformedBy(main, nil, nil); !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("performed by no object: %v, want %v", err, ErrAcceptDeadlock)
	}
}

// A qualifier naming another object of the sender's own type chooses that
// object: the element resolved through it is a declaration the sender shares, so
// its identity has to come from the qualifier rather than from the sender.
func TestAddressedSendToQualifiedElementOfATwinObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		port def PingPort { in item ping : Integer; }
		part def Node {
			port inPort : PingPort;
			action reader accept n : Integer;
			action listen { first start; done; succession first start then done; }
		}
		part alpha : Node;
		part beta : Node;
	}`))
	alpha, beta := instanceOfUsage(t, ctx, idx, "test::alpha"), instanceOfUsage(t, ctx, idx, "test::beta")
	scope := declScope(oneSymbol(t, idx, "test::Node::listen"))
	for _, tc := range []struct{ target, port, receiver string }{
		{"alpha::inPort", "inPort", ""},
		{"alpha::reader", "", "reader"},
	} {
		ctx.messages = nil
		if err := ctx.post(nil, Message{SignalType: "Integer"}, lower.Send{Target: tc.target, Scope: scope}, beta); err != nil {
			t.Fatalf("post to %s: %v", tc.target, err)
		}
		got := ctx.PendingMessages()[0]
		if got.Object != alpha.ID || got.Port != tc.port || got.Target != tc.receiver {
			t.Errorf("`send to %s` from beta delivered as %+v, want object %d", tc.target, got, alpha.ID)
		}
		if got.reaches(tc.receiver, tc.port, beta.ID) {
			t.Errorf("`send to %s` is deliverable to the sending object %d", tc.target, beta.ID)
		}
	}
}

// A message injected from outside the model names its destination in its fields
// alone, so it is held to that destination rather than open to any consumer.
func TestInjectedMessageIsHeldToTheDestinationItNames(t *testing.T) {
	ctx := &Context{}
	ctx.PostMessage(Message{SignalType: "Integer", Target: "reader"})
	ctx.PostMessage(Message{SignalType: "Integer", Port: "inPort", Object: 1})
	ctx.PostMessage(Message{SignalType: "Integer"})
	pending := ctx.PendingMessages()
	if got := pending[0]; got.Delivery != DeliverReceiver || got.reaches("other", "", 0) {
		t.Errorf("injected for reader: %+v, taken by an unrelated consumer", got)
	}
	if !pending[0].reaches("reader", "", 0) {
		t.Errorf("injected for reader: %+v, not taken by reader", pending[0])
	}
	if got := pending[1]; got.Delivery != DeliverPort || got.reaches("", "inPort", 2) {
		t.Errorf("injected for a port of object 1: %+v, taken elsewhere", got)
	}
	if got := pending[2]; got.Delivery != DeliverAnyone || !got.reaches("other", "", 3) {
		t.Errorf("injected for no one: %+v, want any consumer to take it", got)
	}
}

// A consumer takes a message only by satisfying every part of the destination it
// carries; a behavior no object performs is the one identity that cannot tell.
func TestDeliveryHoldsAConsumerToTheWholeDestination(t *testing.T) {
	tests := []struct {
		name    string
		msg     Message
		who     string
		port    string
		object  int64
		reached bool
	}{
		{"unaddressed", Message{}, "reader", "", 2, true},
		{"unaddressed on a port", Message{}, "reader", "inPort", 2, false},
		{"receiver of the same object", Message{Target: "reader", Object: 1, Delivery: DeliverReceiver}, "reader", "", 1, true},
		{"receiver of another object", Message{Target: "reader", Object: 1, Delivery: DeliverReceiver}, "reader", "", 2, false},
		{"another receiver", Message{Target: "reader", Object: 1, Delivery: DeliverReceiver}, "other", "", 1, false},
		{"receiver of no object performer", Message{Target: "reader", Object: 1, Delivery: DeliverReceiver}, "reader", "", 0, true},
		{"receiver addressed by no object", Message{Target: "reader", Delivery: DeliverReceiver}, "reader", "", 1, true},
		{"port of the same object", Message{Port: "inPort", Object: 1, Delivery: DeliverPort}, "", "inPort", 1, true},
		{"port of another object", Message{Port: "inPort", Object: 1, Delivery: DeliverPort}, "", "inPort", 2, false},
		{"another port", Message{Port: "inPort", Object: 1, Delivery: DeliverPort}, "", "outPort", 1, false},
		{"routed receiver of same port and object", Message{Target: "reader", Port: "inPort", Object: 1, Delivery: DeliverPortReceiver}, "reader", "inPort", 1, true},
		{"routed sibling receiver", Message{Target: "reader", Port: "inPort", Object: 1, Delivery: DeliverPortReceiver}, "sibling", "inPort", 1, false},
		{"routed receiver on another port", Message{Target: "reader", Port: "inPort", Object: 1, Delivery: DeliverPortReceiver}, "reader", "outPort", 1, false},
		{"routed receiver of another object", Message{Target: "reader", Port: "inPort", Object: 1, Delivery: DeliverPortReceiver}, "reader", "inPort", 2, false},
		{"object itself", Message{Object: 1, Delivery: DeliverObject}, "reader", "", 1, true},
		{"object and no object performer", Message{Object: 1, Delivery: DeliverObject}, "reader", "", 0, false},
	}
	for _, tc := range tests {
		if got := tc.msg.reaches(tc.who, tc.port, tc.object); got != tc.reached {
			t.Errorf("%s: %+v reaches(%q, %q, %d) = %v, want %v", tc.name, tc.msg, tc.who, tc.port, tc.object, got, tc.reached)
		}
	}
}

// instanceOfUsage materializes the object a part usage occurs as, which is what
// an address naming that usage resolves to.
func instanceOfUsage(t *testing.T, ctx *Context, idx *symbols.Index, fqn string) *Instance {
	t.Helper()
	inst, err := ctx.occurrenceOf(oneSymbol(t, idx, fqn))
	if err != nil {
		t.Fatalf("instantiate %s: %v", fqn, err)
	}
	return inst
}

// A send deep in the nesting routes through a connector declared by a flow
// around it, not only by the action's own body or the send's own flow.
func TestSendRoutesThroughAnEnclosingNestedFlowsConnector(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action group {
				port senderPort;
				port receiverPort;
				connect senderPort to receiverPort;
				first inner;
				action inner {
					first emit;
					action emit {
						send 7 via senderPort to receiver;
					}
				}
				action receiver accept value : Integer via receiverPort {
					assign got := value;
				}
				succession first inner then receiver;
			}
			done;
			succession first start then group;
			succession first group then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute action sending from a nested flow: %v", err)
	}
	assertIntOutput(t, outputs, "got", 7)
}

// An accepted signal materializes one occurrence per message: the guard read
// during transition selection and the firing effect see the same object.
func TestAcceptedSignalMaterializesOneOccurrencePerMessage(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		attribute def Ping { attribute n : Real; }
		state sm {
			attribute total : Real = 0.0;
			entry; then idle;
			state idle;
			state seen;
			transition first idle accept p : Ping if p.n > 0.0 do assign total := total + p.n then seen;
		}
	}`))
	exec, err := newStateExecutor(ctx, oneSymbol(t, idx, "test::sm"), nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	ping := oneSymbol(t, idx, "test::Ping")
	exec.enqueueSignal(Message{SignalType: "Ping", Signal: ping, Payload: map[string]Value{
		"n": {Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 2}},
	}})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "seen")

	count := 0
	for _, inst := range ctx.instances {
		if inst.Type == ping {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected one Ping occurrence for one message, got %d", count)
	}
}

// A send's invocation names a message unless the name resolves to a
// calculation, and a library function is one only where the model resolves it:
// under an import or by its qualified name, never by a bare name the model
// does not import.
func TestSendInvocationIsACallOnlyWhereItResolvesToACalc(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
	private import ScalarValues::*;
	item def Ping;
	calc def double { in x : Integer; return : Integer = x * 2; }
	attribute qualified = SequenceFunctions::size((1, 2));
	attribute bare = size((1, 2));
	attribute own = double(2);
	attribute ping = Ping();
	package imported {
		private import SequenceFunctions::*;
		attribute bare = size((1, 2));
	}
}`)
	ec := NewEvalContext(ctx, nil)
	cases := map[string]bool{
		"test::qualified":      true,
		"test::bare":           false,
		"test::own":            true,
		"test::ping":           false,
		"test::imported::bare": true,
	}
	for fqn, want := range cases {
		sym := lookupOne(t, idx, fqn)
		inv, ok := sym.Decl.(*ast.Usage).Value.(*ast.InvocationExpr)
		if !ok {
			t.Fatalf("%s: value is %T, want an invocation", fqn, sym.Decl.(*ast.Usage).Value)
		}
		if got := ec.invokesCalc(sym.Scope, inv); got != want {
			t.Errorf("%s: invokesCalc = %v, want %v", fqn, got, want)
		}
	}
}

// A send classifies its invocation in the scope it is declared in: a library
// function imported only by the nested action that sends is a call there, so
// the value it computes is what travels, not a message named after it.
func TestActionSendCallsFunctionImportedByNestedAction(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender {
				private import SequenceFunctions::*;
				send size((1, 2, 3)) to reader;
			}
			action reader accept n : Integer;
			action recorder { assign got := n; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then recorder;
			succession first recorder then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 3)
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("expected no message left in flight, got %v", pending)
	}
}

// A send naming both a signal and a calc calls the calc, whichever import
// brought it into view first: the value it computes is what travels.
func TestActionSendCallsACalcSharingASignalsName(t *testing.T) {
	const src = `
		package A { item def Count; }
		package B { private import ScalarValues::*; calc def Count { in xs : Integer[0..*]; return : Integer = 3; } }
		package P {
			private import ScalarValues::*;
			%s
			action pipeline {
				attribute got : Integer = 0;
				first start;
				action sender { send Count((1, 2, 3)) to reader; }
				action reader accept n : Integer;
				action recorder { assign got := n; }
				done;
				succession first start then sender;
				succession first sender then reader;
				succession first reader then recorder;
				succession first recorder then done;
			}
		}`
	for _, imports := range []string{
		"private import A::*; private import B::*;",
		"private import B::*; private import A::*;",
	} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(src, imports)))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
		if sym == nil {
			t.Fatal("action pipeline not found")
		}
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			t.Fatalf("%s: execute action: %v", imports, err)
		}
		assertIntOutput(t, outputs, "got", 3)
	}
}

// A send invoking a feature typed by a calc performs that calc, as an expression
// does: the value it computes travels, not a message named after the feature.
func TestActionSendCallsAFeatureTypedByACalc(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		calc def Twice { in x : Integer; return : Integer = 2 * x; }
		ref count : Twice;
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender { send count(4) to reader; }
			action reader accept n : Integer;
			action recorder { assign got := n; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then recorder;
			succession first recorder then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 8)
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("expected no message left in flight, got %v", pending)
	}
}

// A feature typed by a library function — directly, or through a bodiless model calc
// specializing it — is a calc to send as well: the function's result travels, not a
// message named after the feature.
func TestActionSendCallsAFeatureTypedByALibraryFunction(t *testing.T) {
	for _, decls := range []string{
		"ref root : sqrt;",
		"calc def Root :> sqrt; ref root : Root;",
	} {
		t.Run(decls, func(t *testing.T) { testActionSendCallsLibraryTypedFeature(t, decls) })
	}
}

func testActionSendCallsLibraryTypedFeature(t *testing.T, decls string) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		private import RealFunctions::sqrt;
		`+decls+`
		action pipeline {
			attribute got : Real = 0.0;
			first start;
			action sender { send root(16.0) to reader; }
			action reader accept n : Real;
			action recorder { assign got := n; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then recorder;
			succession first recorder then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pipeline", ast.DefAction)
	if sym == nil {
		t.Fatal("action pipeline not found")
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	if got := outputs["got"]; got.Const.Kind != semantics.ValReal || got.Const.Real != 4 {
		t.Errorf("got = %+v, want 4.0", got)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("expected no message left in flight, got %v", pending)
	}
}

// The same holds for a state's entry: a function imported by the state alone is
// called and its value sent, so the transition on the value's type fires.
func TestStateSendCallsFunctionImportedByNestedBlock(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		state Driver {
			entry; then start;
			state start;
			state waiting {
				private import SequenceFunctions::*;
				entry send size((1, 2, 3)) to Driver;
			}
			succession first start then waiting;
			transition first waiting accept n : Integer then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Driver", ast.DefState)
	if sym == nil {
		t.Fatal("state Driver not found")
	}
	_, visits, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute state machine: %v", err)
	}
	assertVisits(t, visits, "start", "waiting", "done")
}
