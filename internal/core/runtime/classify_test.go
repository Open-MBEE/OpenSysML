package runtime

import (
	"errors"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// instantiateQualified materializes the object the qualified name declares.
func instantiateQualified(t *testing.T, ctx *Context, idx *symbols.Index, name string) *Instance {
	t.Helper()
	matches := idx.LookupQualified(name)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", name, len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate(%s): %v", name, err)
	}
	return inst
}

// readInstance reads a feature holding one object and returns that object.
func readInstance(t *testing.T, ctx *Context, inst *Instance, name string) *Instance {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", name, err)
	}
	held := fv.HeldValue()
	if held.Kind != ValInstance {
		t.Fatalf("%s = %s, want one object", name, FormatValue(held))
	}
	obj, ok := ctx.getInstance(held.Instance)
	if !ok {
		t.Fatalf("%s: object %d not registered", name, held.Instance)
	}
	return obj
}

func readInt(t *testing.T, ctx *Context, inst *Instance, name string) int64 {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", name, err)
	}
	return intValue(t, map[string]Value{name: fv.HeldValue()}, name)
}

func readBool(t *testing.T, ctx *Context, inst *Instance, name string) bool {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", name, err)
	}
	v := fv.HeldValue()
	if v.Kind != ValConst || v.Const.Kind != semantics.ValBool {
		t.Fatalf("%s = %s, want a Boolean", name, FormatValue(v))
	}
	return v.Const.Bool
}

const classifiedValueModel = `
	package test {
		private import ScalarValues::*;
		item def Segment { item ends [2]; }
		item def Loop { item edges : Segment [*]; }
		item def Square :> Loop {
			item :>> edges [4] = (e1, e2, e3, e4);
			item e1 [1];
			item e2 [1];
			item e3 [1];
			item e4 [1];
		}
		item sq : Square;
	}
`

// A feature's values are instances of its type (KerML 1.0 §7.3.4.1): an untyped member listed
// in `edges : Segment` is the same object, now a Segment, whichever is read first, and only once.
func TestListedValueIsClassifiedByTheFeatureType(t *testing.T) {
	for _, first := range []string{"e1", "edges"} {
		t.Run(first+"_first", func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, classifiedValueModel)
			sq := instantiateQualified(t, ctx, idx, "test::sq")
			segment := idx.LookupQualified("test::Segment")[0]
			if _, err := sq.GetFeatureValue(ctx, first); err != nil {
				t.Fatalf("GetFeatureValue(%s): %v", first, err)
			}

			e1 := readInstance(t, ctx, sq, "e1")
			edges, err := sq.GetFeatureValue(ctx, "edges")
			if err != nil {
				t.Fatalf("GetFeatureValue(edges): %v", err)
			}
			els := elementsOf(edges.HeldValue())
			if len(els) != 4 || els[0].Kind != ValInstance || els[0].Instance != e1.ID {
				t.Fatalf("edges = %s, want e1 (%d) first of four", FormatValue(edges.HeldValue()), e1.ID)
			}
			if e1.Type.Name != "e1" || !ctx.instanceConforms(e1, segment) {
				t.Fatalf("e1 is declared by %s and is a Segment: %v", e1.Type.Name, ctx.instanceConforms(e1, segment))
			}
			ends, err := e1.GetFeatureValue(ctx, "ends")
			if err != nil {
				t.Fatalf("GetFeatureValue(e1.ends): %v", err)
			}
			if got := len(elementsOf(ends.HeldValue())); got != 2 {
				t.Fatalf("e1.ends holds %d objects, want 2", got)
			}

			before := len(e1.classifiers)
			if err := ctx.classify(e1, segment); err != nil {
				t.Fatalf("classify again: %v", err)
			}
			if len(e1.classifiers) != before {
				t.Fatalf("classifying an object twice grew its classifiers from %d to %d", before, len(e1.classifiers))
			}
		})
	}
}

// A holding computed rather than listed — the object chosen by a condition, an index, an
// invocation or a body — classifies its object before it is first read all the same: the
// classifier's features and behavior are there whichever feature is read first.
func TestComputedHoldingIsClassifiedWhicheverIsReadFirst(t *testing.T) {
	holdings := map[string]string{
		"conditional": "if pickLead ? lead else trail",
		"indexed":     "(trail, lead)#(2)",
		"invocation":  "SequenceFunctions::head((lead, trail))",
		"body":        "(lead, trail)->select { in x; x == lead }",
	}
	for name, holding := range holdings {
		t.Run(name, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, `package test {
				private import ScalarValues::*;
				private import ControlFunctions::*;
				state def Glowing {
					attribute cycles : Integer = 0;
					entry; then lit;
					state lit { entry action count { assign cycles := cycles + 1; } }
				}
				item def Tallied { attribute tally : Integer = 7; exhibit state glow : Glowing; }
				item def Rack {
					attribute pickLead : Boolean = true;
					item lead [1];
					item trail [1];
					item tallied : Tallied [1] = `+holding+`;
				}
				item rack : Rack;
			}`)
			rack := instantiateQualified(t, ctx, idx, "test::rack")
			tallied := idx.LookupQualified("test::Tallied")[0]

			lead := readInstance(t, ctx, rack, "lead")
			if !ctx.instanceConforms(lead, tallied) {
				t.Fatalf("lead, read first, is classified by %v, want Tallied", lead.classifiers)
			}
			if got := readInt(t, ctx, lead, "tally"); got != 7 {
				t.Fatalf("lead.tally = %d, want 7", got)
			}
			glow, ok := lead.Behavior("glow")
			if !ok || glow.State == nil || glow.State.stateData["cycles"].Const.Int != 1 {
				t.Fatal("lead, read first, runs no glow state machine")
			}
			if held := readInstance(t, ctx, rack, "tallied"); held != lead {
				t.Fatalf("tallied holds object %d, want lead (%d)", held.ID, lead.ID)
			}
			if trail := readInstance(t, ctx, rack, "trail"); ctx.instanceConforms(trail, tallied) {
				t.Fatal("trail, which tallied does not hold, is a Tallied")
			}
		})
	}
}

// An argument a calc's returns never pass on is not held by the feature the call values:
// reading it neither computes that feature — which may fail — nor classifies the object
// the call does return, so no behavior starts on it.
func TestArgumentNotReturnedIsNotHeldByTheCall(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		state def Glowing {
			attribute cycles : Integer = 0;
			entry; then lit;
			state lit { entry action count { assign cycles := cycles + 1; } }
		}
		item def Tallied { attribute tally : Integer = 7; exhibit state glow : Glowing; }
		calc def pickChosen { in chosen; in other; return : Anything = chosen; }
		item def Rack {
			item lead [1];
			item trail [1];
			item tallied : Tallied [1] = pickChosen(lead, trail);
			item failing : Tallied [1] = pickChosen(3, trail);
		}
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	tallied := idx.LookupQualified("test::Tallied")[0]

	trail := readInstance(t, ctx, rack, "trail")
	if ctx.instanceConforms(trail, tallied) {
		t.Fatal("trail, which no call returns, is a Tallied")
	}
	if rack.FeatureValues["tallied"].Materialized || rack.FeatureValues["lead"].Materialized {
		t.Fatal("reading trail computed tallied, which does not hold it")
	}
	lead := readInstance(t, ctx, rack, "lead")
	if !ctx.instanceConforms(lead, tallied) {
		t.Fatalf("lead, read first, is classified by %v, want Tallied", lead.classifiers)
	}
	if glow, ok := lead.Behavior("glow"); !ok || glow.State == nil || glow.State.stateData["cycles"].Const.Int != 1 {
		t.Fatal("lead, read first, runs no glow state machine")
	}
	if _, err := rack.GetFeatureValue(ctx, "failing"); err == nil {
		t.Fatal("failing holds 3 as a Tallied without error")
	}
}

// A call through a feature chain, `picker.pickChosen(lead, trail)`, applies the calc the
// chain denotes: the feature it values holds the arguments that calc's returns pass on,
// and the chain itself is the callee, not an argument.
func TestChainCallReturnedArgumentsAreHeldByTheCall(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Tallied { attribute tally : Integer = 7; }
		item def Picker {
			calc pickChosen { in chosen; in other; return : Anything = chosen; }
			calc pickOther { in chosen; in other; return : Anything = other; }
		}
		item def Rack {
			item picker : Picker;
			item lead [1];
			item trail [1];
			item spare [1];
			item tallied : Tallied [1] = picker.pickChosen(lead, trail);
			item named : Tallied [1] = picker.pickOther(other = spare, chosen = trail);
		}
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	tallied := idx.LookupQualified("test::Tallied")[0]

	trail := readInstance(t, ctx, rack, "trail")
	if ctx.instanceConforms(trail, tallied) {
		t.Fatal("trail, which neither call returns, is a Tallied")
	}
	if rack.FeatureValues["tallied"].Materialized || rack.FeatureValues["named"].Materialized {
		t.Fatal("reading trail computed a call that does not hold it")
	}
	lead := readInstance(t, ctx, rack, "lead")
	if !ctx.instanceConforms(lead, tallied) {
		t.Fatalf("lead, read first, is classified by %v, want Tallied", lead.classifiers)
	}
	spare := readInstance(t, ctx, rack, "spare")
	if !ctx.instanceConforms(spare, tallied) {
		t.Fatalf("spare, read first, is classified by %v, want Tallied", spare.classifiers)
	}
	if picker := readInstance(t, ctx, rack, "picker"); ctx.instanceConforms(picker, tallied) {
		t.Fatal("picker, the callee's holder, is a Tallied")
	}
}

// A holder that cannot materialize holds nothing: the held feature reads alike in either
// order, and the holder's own error is reported when the holder is read.
func TestFailingHolderDoesNotFailTheFeatureItWouldHold(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		item def Gauge;
		item def Tallied { attribute tally : Integer = 7; }
		item def Rack {
			item lead : Gauge [1];
			item trail : Gauge [1];
			item both : Gauge [0..*] = (lead, trail);
			item stowed : Gauge [3] = both;
			item tallied : Tallied [1] = lead;
			attribute count : Integer = size(both);
		}
		item rack : Rack;
	}`
	for _, order := range [][]string{{"both", "stowed"}, {"stowed", "both"}, {"count", "stowed"}, {"lead", "tallied"}, {"tallied", "lead"}} {
		ctx, idx := libraryShapeContext(t, src)
		rack := instantiateQualified(t, ctx, idx, "test::rack")
		for _, name := range order {
			_, err := rack.GetFeatureValue(ctx, name)
			switch name {
			case "stowed":
				if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "lower bound 3") {
					t.Fatalf("reading %v: stowed: %v, want its own multiplicity violation", order, err)
				}
			case "tallied":
				if !errors.Is(err, ErrTypeMismatch) {
					t.Fatalf("reading %v: tallied: %v, want a type mismatch", order, err)
				}
			default:
				if err != nil {
					t.Fatalf("reading %v: %s: %v", order, name, err)
				}
			}
		}
		if got := len(elementsOf(rack.FeatureValues["both"].HeldValue())); rack.FeatureValues["both"].Materialized && got != 2 {
			t.Fatalf("reading %v: both holds %d objects, want 2", order, got)
		}
		if got := readInt(t, ctx, rack, "count"); got != 2 {
			t.Fatalf("reading %v: count = %d, want 2", order, got)
		}
	}
}

// Only a feature that may answer another's objects as its own holds them: an attribute
// computing from a feature, a condition tested, and an argument a calc's returns do not pass
// on hold nothing, so reading those features forces no other; a chain holds its last member
// read from the objects before it, not those objects. A function whose body the model does
// not state may return any argument.
func TestOnlyFeaturesPassingObjectsOnHoldThem(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		item def Gauge { item cell [1]; }
		item def Tallied { attribute tally : Integer = 7; }
		calc def pickFirst { in chosen : Gauge; in other : Gauge; return : Gauge = chosen; }
		calc def pickAt { in gauges : Gauge [*]; in n : Integer; gauges#(n) }
		calc def pickWhen {
			in gauges : Gauge [*]; in fallback : Gauge; in wanted : Boolean;
			if wanted { for g in gauges { return : Gauge = g; } }
			return : Gauge = fallback;
		}
		item def Rack {
			attribute pickLead : Boolean = true;
			attribute count : Integer = 2;
			attribute doubled : Integer = count * 2;
			item lead : Gauge [1];
			item trail : Gauge [1];
			item spare : Gauge [1];
			item tallied : Tallied [1] = if pickLead ? lead else trail;
			item cells [2] = (lead, trail).cell;
			item picked = (lead, trail)#(count - 1);
			item led : Tallied [1] = pickFirst(lead, trail);
			item trailed : Tallied [1] = pickFirst(chosen = trail, other = lead);
			item indexed : Tallied [1] = pickAt((lead, trail), count);
			item looped : Tallied [1] = pickWhen((lead, trail), spare, pickLead);
			item headed : Tallied [1] = head((lead, count));
		}
		item rack : Rack;
	}`)
	rack := idx.LookupQualified("test::Rack")[0]

	want := map[string][]string{
		"lead":       {"tallied", "picked", "led", "indexed", "looped", "headed"},
		"trail":      {"tallied", "picked", "trailed", "indexed", "looped"},
		"spare":      {"looped"},
		"lead.cell":  {"cells"},
		"trail.cell": {"cells"},
		"pickLead":   nil,
		"count":      nil,
		"doubled":    nil,
	}
	for name, holders := range want {
		if got := ctx.holdingFeatures(rack, name); !slices.Equal(got, holders) {
			t.Errorf("holdingFeatures(Rack, %s) = %v, want %v", name, got, holders)
		}
	}
}

// A collect body's parameter stands for the operand's elements: a body answering it, directly,
// through a feature it declares, or in a nested body, passes the operand's objects on, so the
// collecting feature holds them whichever is read first; a body answering another feature
// passes that one on instead, and one chaining through the parameter passes on the chain's
// last member of the operand's objects.
func TestCollectedObjectsAreHeldByTheCollectingFeature(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		item def Tallied { attribute tally : Integer = 7; }
		item def Rack {
			item lead [1] { item cell [1]; }
			item trail [1] { item cell [1]; }
			item spare [1];
			item same : Tallied [2] = (lead, trail).{in x; x};
			item local : Tallied [2] = (lead, trail).{in x; private item held = x; held};
			item nested : Tallied [*] = (lead, trail).{in x; (x, spare).{in y; y}};
			item other : Tallied [2] = (lead, trail).{in x; spare};
			item cells [2] = (lead, trail).{in x; x.cell};
		}
		item rack : Rack;
	}`
	ctx, idx := libraryShapeContext(t, src)
	rack := idx.LookupQualified("test::Rack")[0]
	want := map[string][]string{
		"lead":       {"same", "local", "nested"},
		"trail":      {"same", "local", "nested"},
		"spare":      {"nested", "other"},
		"lead.cell":  {"cells"},
		"trail.cell": {"cells"},
	}
	for name, holders := range want {
		if got := ctx.holdingFeatures(rack, name); !slices.Equal(got, holders) {
			t.Errorf("holdingFeatures(Rack, %s) = %v, want %v", name, got, holders)
		}
	}

	for _, order := range [][]string{{"lead", "same"}, {"same", "lead"}} {
		ctx, idx := libraryShapeContext(t, src)
		rack := instantiateQualified(t, ctx, idx, "test::rack")
		tallied := idx.LookupQualified("test::Tallied")[0]
		for _, name := range order {
			if _, err := rack.GetFeatureValue(ctx, name); err != nil {
				t.Fatalf("reading %v: %s: %v", order, name, err)
			}
			if lead := readInstance(t, ctx, rack, "lead"); !ctx.instanceConforms(lead, tallied) {
				t.Errorf("reading %v, after %s lead is classified by %v, want Tallied",
					order, name, lead.classifiers)
			}
		}
	}
}

// Calcs returning through each other are analysed as one: a parameter one passes on only
// through the other's returns is passed on, whichever calc is met first and wherever the
// direct return stands beside the call.
func TestMutuallyRecursiveCalcsPassArgumentsThroughEachOther(t *testing.T) {
	for name, holdings := range map[string]string{
		"through_first_met": "item viaA : Tallied [1] = pickA(lead, trail, true); item viaB : Tallied [1] = pickB(lead, trail, true);",
		"through_last_met":  "item viaB : Tallied [1] = pickB(lead, trail, true); item viaA : Tallied [1] = pickA(lead, trail, true);",
	} {
		t.Run(name, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, `package test {
				private import ScalarValues::*;
				item def Gauge;
				item def Tallied { attribute tally : Integer = 7; }
				calc def pickA {
					in a : Gauge; in b : Gauge; in again : Boolean;
					return : Gauge = if again ? pickB(a, b, false) else a;
				}
				calc def pickB {
					in c : Gauge; in d : Gauge; in again : Boolean;
					return : Gauge = if again ? pickA(d, c, false) else d;
				}
				item def Rack {
					item lead : Gauge [1];
					item trail : Gauge [1];
					`+holdings+`
				}
			}`)
			rack := idx.LookupQualified("test::Rack")[0]
			for _, held := range []string{"lead", "trail"} {
				got := ctx.holdingFeatures(rack, held)
				sort.Strings(got)
				if want := []string{"viaA", "viaB"}; !slices.Equal(got, want) {
					t.Errorf("holdingFeatures(Rack, %s) = %v, want %v", held, got, want)
				}
			}
		})
	}
}

// A calc returning a local written from an input passes that input on, so the typed feature
// receiving the result classifies the argument whichever is read first; a data local passes nothing.
func TestCalcLocalsPassArgumentsOnToTheReturn(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		item def Gauge { item cell [1]; }
		item def Tallied :> Gauge { attribute tally : Integer = 7; }
		calc def viaDeclared { in chosen : Gauge; in other : Gauge; attribute held = chosen; return : Gauge = held; }
		calc def viaAssigned {
			in chosen : Gauge; in other : Gauge;
			attribute held = other;
			assign held := chosen;
			return : Gauge = held;
		}
		calc def viaBlock {
			in chosen : Gauge; in other : Gauge; in wanted : Boolean;
			if wanted { attribute held = chosen; return : Gauge = held; }
			return : Gauge = other;
		}
		calc def viaLater {
			in chosen : Gauge; in other : Gauge;
			attribute held = chosen;
			attribute passed = held;
			return : Gauge = passed;
		}
		calc def viaMember { in chosen : Gauge; in other : Gauge; attribute held = chosen; return = held.cell; }
		calc def viaSelf { in chosen : Gauge; in other : Gauge; attribute held = chosen; assign held := held; return : Gauge = held; }
		calc def viaComputed { in chosen : Gauge; in n : Integer; attribute held = n + 1; return : Integer = held; }
		item def Rack {
			attribute count : Integer = 2;
			item lead : Gauge [1];
			item trail : Gauge [1];
			item declared : Tallied [1] = viaDeclared(lead, trail);
			item assigned : Tallied [1] = viaAssigned(lead, trail);
			item blocked : Tallied [1] = viaBlock(lead, trail, true);
			item later : Tallied [1] = viaLater(lead, trail);
			item celled : Tallied [1] = viaMember(lead, trail);
			item selfed : Tallied [1] = viaSelf(lead, trail);
			attribute computed : Integer = viaComputed(lead, count);
		}
		item rack : Rack;
	}`
	ctx, idx := libraryShapeContext(t, src)
	rack := idx.LookupQualified("test::Rack")[0]
	want := map[string][]string{
		"lead":  {"declared", "assigned", "blocked", "later", "selfed"},
		"trail": {"assigned", "blocked"},
		"count": nil,
	}
	for name, holders := range want {
		if got := ctx.holdingFeatures(rack, name); !slices.Equal(got, holders) {
			t.Errorf("holdingFeatures(Rack, %s) = %v, want %v", name, got, holders)
		}
	}

	for _, order := range [][]string{{"lead", "declared"}, {"declared", "lead"}, {"lead", "assigned"}, {"lead", "blocked"}, {"lead", "later"}, {"lead", "selfed"}} {
		ctx, idx := libraryShapeContext(t, src)
		rack := instantiateQualified(t, ctx, idx, "test::rack")
		for _, name := range order {
			if _, err := rack.GetFeatureValue(ctx, name); err != nil {
				t.Fatalf("reading %v: %s: %v", order, name, err)
			}
		}
		lead := readInstance(t, ctx, rack, "lead")
		if got := readInt(t, ctx, lead, "tally"); got != 7 {
			t.Errorf("reading %v: lead.tally = %d, want 7 from the classifier", order, got)
		}
	}
}

// The object a selected variant materialized is held like any other: a typed feature
// holding a variation's value classifies that object, which then carries the feature's
// own features and runs its behaviors, and stays the variant's object.
func TestSelectedVariantObjectIsClassifiedByTheFeatureHoldingIt(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		state def Glowing {
			attribute cycles : Integer = 0;
			entry; then lit;
			state lit { entry action count { assign cycles := cycles + 1; } }
		}
		item def Engine { attribute cylinders : Integer = 4; }
		item def Tallied :> Engine { attribute tally : Integer = 7; exhibit state glow : Glowing; }
		item def Car {
			variation item engine : Engine [1] {
				variant item small : Engine;
				variant item big : Engine { :>> cylinders = 8; }
			}
		}
		item def Garage {
			item car : Car { :>> engine = engine::big; }
			item tallied : Tallied [1] = car.engine { attribute label : String = "kept"; }
		}
		item garage : Garage;
	}`)
	garage := instantiateQualified(t, ctx, idx, "test::garage")
	tallied := idx.LookupQualified("test::Tallied")[0]

	fv, err := garage.GetFeatureValue(ctx, "tallied")
	if err != nil {
		t.Fatalf("garage.tallied: %v", err)
	}
	if fv.Value.Kind != ValVariant || fv.Value.Variant().Name != "big" || fv.Value.Instance == 0 {
		t.Fatalf("garage.tallied = %+v, want the materialized big variant", fv.Value)
	}
	car := readInstance(t, ctx, garage, "car")
	engine, err := car.GetFeatureValue(ctx, "engine")
	if err != nil || engine.Value.Instance != fv.Value.Instance {
		t.Fatalf("car.engine = %+v, %v; want the same object tallied holds (%d)", engine.Value, err, fv.Value.Instance)
	}
	big := ctx.instances[fv.Value.Instance]
	if !ctx.instanceConforms(big, tallied) {
		t.Fatalf("the big variant's object is classified by %v, want Tallied", big.classifiers)
	}
	if got := readInt(t, ctx, big, "cylinders"); got != 8 {
		t.Fatalf("big.cylinders = %d, want the variant's 8", got)
	}
	if got := readInt(t, ctx, big, "tally"); got != 7 {
		t.Fatalf("big.tally = %d, want 7", got)
	}
	if label, err := big.GetFeatureValue(ctx, "label"); err != nil || label.Value.Str() != "kept" {
		t.Fatalf("big.label = %+v, %v; want the holding feature's \"kept\"", label, err)
	}
	glow, ok := big.Behavior("glow")
	if !ok || glow.State == nil || glow.State.stateData["cycles"].Const.Int != 1 {
		t.Fatal("the big variant's object runs no glow state machine")
	}
}

// A feature valued by a chain holds the chain's last member of the objects before it: reading
// that member on a nested object — through one owner, several, or a collection — classifies
// it before its classifier's features are read, while the objects along the chain and the
// chains other features read stay untouched.
func TestChainedHoldingClassifiesTheLastMemberWhicheverIsReadFirst(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		item def Engine { attribute cylinders : Integer = 4; }
		item def Tallied :> Engine { attribute tally : Integer = 7; }
		item def Car {
			variation item engine : Engine [1] {
				variant item small : Engine;
				variant item big : Engine { :>> cylinders = 8; }
			}
		}
		item def Plain { item engine : Engine [1]; }
		item def Lot { item parked : Plain [1]; }
		item def Garage {
			item car : Car { :>> engine = engine::big; }
			item spare : Car { :>> engine = engine::small; }
			item fleet : Plain [2];
			item lot : Lot [1];
			item tallied : Tallied [1] = car.engine;
			item spared : Tallied [1] = spare.engine;
			item fleeted : Tallied [2] = fleet.engine;
			item deep : Tallied [1] = lot.parked.engine;
			attribute isTallied : Boolean = car.engine istype Tallied;
		}
		item garage : Garage;
	}`
	engineOf := func(t *testing.T, ctx *Context, holder *Instance) *Instance {
		t.Helper()
		fv, err := holder.GetFeatureValue(ctx, "engine")
		if err != nil {
			t.Fatalf("%s.engine: %v", holder.Type.Name, err)
		}
		id, ok := fv.HeldValue().Object()
		if !ok {
			t.Fatalf("%s.engine = %s, want one object", holder.Type.Name, FormatValue(fv.HeldValue()))
		}
		return ctx.instances[id]
	}

	for _, first := range []string{"car.engine", "tallied"} {
		t.Run(first+"_first", func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, src)
			garage := instantiateQualified(t, ctx, idx, "test::garage")
			tallied := idx.LookupQualified("test::Tallied")[0]
			car := readInstance(t, ctx, garage, "car")
			if first == "tallied" {
				if _, err := garage.GetFeatureValue(ctx, "tallied"); err != nil {
					t.Fatalf("garage.tallied: %v", err)
				}
			}
			engine := engineOf(t, ctx, car)
			if !ctx.instanceConforms(engine, tallied) {
				t.Fatalf("car.engine is classified by %v, want Tallied", engine.classifiers)
			}
			if got := readInt(t, ctx, engine, "tally"); got != 7 {
				t.Fatalf("car.engine.tally = %d, want 7", got)
			}
			if got := readInt(t, ctx, engine, "cylinders"); got != 8 {
				t.Fatalf("car.engine.cylinders = %d, want the big variant's 8", got)
			}
			if ctx.instanceConforms(car, tallied) {
				t.Fatalf("car, an object the chain passes through, is classified by %v", car.classifiers)
			}
			if fv := garage.FeatureValues["tallied"]; fv == nil || !fv.Materialized {
				t.Fatal("garage.tallied is not read once car.engine is")
			}
			if held, ok := garage.FeatureValues["tallied"].HeldValue().Object(); !ok || held != engine.ID {
				t.Fatalf("garage.tallied holds %s, want car.engine (%d)", FormatValue(garage.FeatureValues["tallied"].HeldValue()), engine.ID)
			}
			for _, other := range []string{"spared", "fleeted", "deep"} {
				if garage.FeatureValues[other].Materialized {
					t.Errorf("garage.%s, a chain through other objects, is read along with car.engine", other)
				}
			}
			if !readBool(t, ctx, garage, "isTallied") {
				t.Error("car.engine istype Tallied = false")
			}
		})
	}

	t.Run("over_a_collection", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, src)
		garage := instantiateQualified(t, ctx, idx, "test::garage")
		tallied := idx.LookupQualified("test::Tallied")[0]
		fleet, err := garage.GetFeatureValue(ctx, "fleet")
		if err != nil {
			t.Fatalf("garage.fleet: %v", err)
		}
		for _, el := range elementsOf(fleet.HeldValue()) {
			plain := ctx.instances[el.Instance]
			engine := engineOf(t, ctx, plain)
			if !ctx.instanceConforms(engine, tallied) {
				t.Fatalf("fleet.engine is classified by %v, want Tallied", engine.classifiers)
			}
			if ctx.instanceConforms(plain, tallied) {
				t.Fatalf("a fleet car is classified by %v", plain.classifiers)
			}
		}
		if got := len(elementsOf(garage.FeatureValues["fleeted"].HeldValue())); got != 2 {
			t.Fatalf("garage.fleeted holds %d objects, want the two engines", got)
		}
	})

	t.Run("through_nested_owners", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, src)
		garage := instantiateQualified(t, ctx, idx, "test::garage")
		tallied := idx.LookupQualified("test::Tallied")[0]
		lot := readInstance(t, ctx, garage, "lot")
		parked := readInstance(t, ctx, lot, "parked")
		engine := engineOf(t, ctx, parked)
		if !ctx.instanceConforms(engine, tallied) {
			t.Fatalf("lot.parked.engine is classified by %v, want Tallied", engine.classifiers)
		}
		if ctx.instanceConforms(parked, tallied) || ctx.instanceConforms(lot, tallied) {
			t.Fatal("an object along lot.parked.engine is classified as Tallied")
		}
		if held := readInstance(t, ctx, garage, "deep"); held != engine {
			t.Fatalf("garage.deep holds object %d, want lot.parked.engine (%d)", held.ID, engine.ID)
		}
	})
}

// A value none of whose types is comparable with the feature's type is refused: a number,
// an object of an unrelated definition, or one already classified by one, cannot become a Segment.
func TestIncomparableValueIsRefusedByTheFeatureType(t *testing.T) {
	cases := map[string]struct {
		decl string
		want string
	}{
		"integer": {
			decl: `item :>> edges [1] = (count); attribute count : Integer = 3;`,
			want: "cannot write 3 (an Integer) to a feature typed by Segment",
		},
		"unrelated_object": {
			decl: `item :>> edges [1] = (bolt); item bolt : Bolt;`,
			want: "to a feature typed by Segment",
		},
		"object_classified_by_an_unrelated_definition": {
			decl: `item :>> edges [1] = (raw); item raw [1]; item fastener : Bolt [1] = raw;`,
			want: "to a feature typed by Segment",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, `package test {
				private import ScalarValues::*;
				item def Segment;
				item def Bolt;
				item def Loop { item edges : Segment [*]; }
				item def Broken :> Loop { `+tc.decl+` }
				item broken : Broken;
			}`)
			broken := instantiateQualified(t, ctx, idx, "test::broken")
			_, err := broken.GetFeatureValue(ctx, "edges")
			if !errors.Is(err, ErrTypeMismatch) {
				t.Fatalf("edges: %v, want ErrTypeMismatch", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("edges: %v, want %q in the message", err, tc.want)
			}
		})
	}
}

const heldObjectModel = `
	package test {
		item def Segment { item ends [2]; }
		item def Bolt;
		item def Rack {
			item slot : Segment [0..1] = raw;
			item raw [1];
			item loose [1];
		}
		item rack : Rack;
		item def Bin {
			item pair : Segment [2] = (spare, bolt);
			item spare [1];
			item bolt : Bolt;
		}
		item bin : Bin;
		attribute def Load { item seg : Segment; }
	}
`

// isUnclassified reports whether an object still is what it was declared alone.
func isUnclassified(inst *Instance) bool {
	_, ends := inst.FeatureValues["ends"]
	return len(inst.classifiers) == 0 && !ends
}

// A declared holding classifies its object and a probe undoes both; a refused value
// classifies nothing, and a write or message admits an object by what it already is.
func TestClassificationIsKeptOrUndoneWithTheChange(t *testing.T) {
	ctx, idx := libraryShapeContext(t, heldObjectModel)
	rack := instantiateQualified(t, ctx, idx, "test::rack")

	// Reading raw reads slot, which holds it, first.
	end := ctx.beginProbe()
	raw := readInstance(t, ctx, rack, "raw")
	if isUnclassified(raw) {
		t.Fatal("raw is not a Segment while it is held by slot")
	}
	end()
	if !isUnclassified(raw) {
		t.Fatalf("after the probe, raw is classified by %v with %v", raw.classifiers, sortedNames(raw.FeatureValues))
	}
	if rack.FeatureValues["slot"].Materialized {
		t.Fatal("after the probe, slot still holds its value")
	}

	bin := instantiateQualified(t, ctx, idx, "test::bin")
	if _, err := bin.GetFeatureValue(ctx, "pair"); !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("pair = %v, want ErrTypeMismatch: bolt is a Bolt", err)
	}
	for _, inst := range ctx.instances {
		if !isUnclassified(inst) {
			t.Fatalf("a refused value classified object %d, a %s, by %v", inst.ID, symbolText(inst.Type), inst.classifiers)
		}
	}

	loose := readInstance(t, ctx, rack, "loose")
	looseVal := Value{Kind: ValInstance, Instance: loose.ID}
	if err := rack.SetFeatureValue(ctx, "slot", looseVal); !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("SetFeatureValue(slot, loose) = %v, want ErrTypeMismatch: loose is no Segment", err)
	}
	load := idx.LookupQualified("test::Load")[0]
	if _, err := ctx.SignalMessage(load, map[string]Value{"seg": looseVal}, rack); !errors.Is(err, ErrSignalArgument) {
		t.Fatalf("SignalMessage(Load, seg = loose) = %v, want ErrSignalArgument", err)
	}
	if !isUnclassified(loose) {
		t.Fatal("a refused write or message classified its object")
	}

	raw = readInstance(t, ctx, rack, "raw")
	rawVal := Value{Kind: ValInstance, Instance: raw.ID}
	if isUnclassified(raw) {
		t.Fatal("raw is not a Segment once slot holds it")
	}
	if err := rack.SetFeatureValue(ctx, "slot", rawVal); err != nil {
		t.Fatalf("SetFeatureValue(slot, raw), a Segment now: %v", err)
	}
	if _, err := ctx.SignalMessage(load, map[string]Value{"seg": rawVal}, rack); err != nil {
		t.Fatalf("SignalMessage(Load, seg = raw), a Segment now: %v", err)
	}
}

// sortedNames is the feature names an object carries, sorted.
func sortedNames(values map[string]*FeatureValue) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// A qualified name of an enclosing type's feature, written in a nested usage, reads the enclosing
// object's value (KerML 1.0 §7.4.4); one naming a declaration outside the object reads that.
func TestQualifiedOuterFeatureReadsTheEnclosingObject(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		package Consts { attribute unit : Real = 7.0; }
		item def Frame {
			attribute span : Real;
			attribute gap : Real;
			attribute scale : Real = Consts::unit;
			item bar { attribute len : Real = Frame::span; }
			item strut { attribute len : Real = bar.len + Frame::gap; }
		}
		item frame : Frame { :>> span = 4.0; :>> gap = 1.0; }
		item other : Frame { :>> span = 10.0; :>> gap = 1.0; }
	}`)
	for _, tc := range []struct{ object, feature, want string }{
		{"test::frame", "bar.len", "4.0"},
		{"test::frame", "strut.len", "5.0"},
		{"test::frame", "scale", "7.0"},
		{"test::other", "bar.len", "10.0"},
		{"test::other", "strut.len", "11.0"},
	} {
		inst := instantiateQualified(t, ctx, idx, tc.object)
		path := strings.Split(tc.feature, ".")
		for _, step := range path[:len(path)-1] {
			inst = readInstance(t, ctx, inst, step)
		}
		fv, err := inst.GetFeatureValue(ctx, path[len(path)-1])
		if err != nil {
			t.Fatalf("%s.%s: %v", tc.object, tc.feature, err)
		}
		if got := FormatTraceValue(fv.HeldValue()); got != tc.want {
			t.Fatalf("%s.%s = %s, want %s", tc.object, tc.feature, got, tc.want)
		}
	}
}

// A feature valued by a chain over a collection holds the chain's values from every member,
// in member order, and the members' objects are made only when the chain is read.
func TestChainValueCollectsAcrossTheCollection(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Corner;
		item def Side { item corners : Corner [2]; }
		item def Panel {
			item sides : Side [3];
			item corners : Corner [*] = sides.corners;
		}
		item panel : Panel;
	}`)
	panel := instantiateQualified(t, ctx, idx, "test::panel")
	if panel.FeatureValues["sides"].Materialized || panel.FeatureValues["corners"].Materialized {
		t.Fatal("sides or corners materialized on instantiation")
	}
	made := len(ctx.instances)

	corners, err := panel.GetFeatureValue(ctx, "corners")
	if err != nil {
		t.Fatalf("GetFeatureValue(corners): %v", err)
	}
	got := elementsOf(corners.HeldValue())
	if len(got) != 6 {
		t.Fatalf("corners holds %d objects, want 6", len(got))
	}
	if len(ctx.instances)-made != 9 {
		t.Fatalf("reading corners made %d objects, want 3 sides and 6 corners", len(ctx.instances)-made)
	}
	sides, err := panel.GetFeatureValue(ctx, "sides")
	if err != nil {
		t.Fatalf("GetFeatureValue(sides): %v", err)
	}
	for i, side := range elementsOf(sides.HeldValue()) {
		obj, _ := ctx.getInstance(side.Instance)
		fv, err := obj.GetFeatureValue(ctx, "corners")
		if err != nil {
			t.Fatalf("sides.%d.corners: %v", i+1, err)
		}
		for j, c := range elementsOf(fv.HeldValue()) {
			if want := got[2*i+j]; want.Instance != c.Instance {
				t.Fatalf("corners.%d = %s, want sides.%d.corners.%d = %s", 2*i+j+1, FormatValue(want), i+1, j+1, FormatValue(c))
			}
		}
	}
}

// A binding end whose path crosses a collection reaches that feature on every object of it, in
// order: the end holds their values together, and a value the other end holds on its own
// determines none of the objects' parts (KerML 1.0 §7.3.4.6, §7.4.9.2).
func TestBindingEndAcrossACollection(t *testing.T) {
	model := func(shelf string) string {
		return `package test {
			private import ScalarValues::*;
			item def Thing;
			item def Group {
				item items : Thing [2];
				attribute weights : Real [2] = (1.0, 2.0);
				attribute shares : Real [2];
			}
			item def Shelf {
				item groups : Group [2];
				item allItems : Thing [0..*];
				attribute allWeights : Real [0..*] nonunique;
				attribute allShares : Real [0..*] = (0.1, 0.2, 0.3, 0.4);
				` + shelf + `
			}
			item shelf : Shelf;
		}`
	}
	values := func(t *testing.T, ctx *Context, inst *Instance, name string) string {
		t.Helper()
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return FormatValue(fv.HeldValue())
	}
	groupsOf := func(t *testing.T, ctx *Context, shelf *Instance) []*Instance {
		t.Helper()
		fv, err := shelf.GetFeatureValue(ctx, "groups")
		if err != nil {
			t.Fatalf("groups: %v", err)
		}
		var groups []*Instance
		for _, g := range elementsOf(fv.HeldValue()) {
			obj, _ := ctx.getInstance(g.Instance)
			groups = append(groups, obj)
		}
		return groups
	}
	union := func(t *testing.T, ctx *Context, groups []*Instance, name string) []Value {
		t.Helper()
		var all []Value
		for _, g := range groups {
			fv, err := g.GetFeatureValue(ctx, name)
			if err != nil {
				t.Fatalf("group.%s: %v", name, err)
			}
			all = append(all, elementsOf(fv.HeldValue())...)
		}
		return all
	}
	const bound = `bind [0..*] groups.items = [0..*] allItems;
		bind [0..*] groups.weights = [0..*] allWeights;
		bind [0..*] groups.shares = [0..*] allShares;`

	for _, order := range []string{"union_first", "members_first"} {
		t.Run(order, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model(bound))
			shelf := instantiateQualified(t, ctx, idx, "test::shelf")
			if order == "members_first" {
				union(t, ctx, groupsOf(t, ctx, shelf), "items")
			} else if shelf.FeatureValues["groups"].Materialized {
				t.Fatal("groups materialized on instantiation")
			}
			all, err := shelf.GetFeatureValue(ctx, "allItems")
			if err != nil {
				t.Fatalf("allItems: %v", err)
			}
			got := elementsOf(all.HeldValue())
			want := union(t, ctx, groupsOf(t, ctx, shelf), "items")
			if len(got) != 4 || len(want) != 4 {
				t.Fatalf("allItems holds %d objects, the groups' items %d, want 4 each", len(got), len(want))
			}
			for i := range want {
				if got[i].Instance != want[i].Instance {
					t.Fatalf("allItems.%d = %s, want %s, the groups' items in order", i+1, FormatValue(got[i]), FormatValue(want[i]))
				}
			}
			if got := values(t, ctx, shelf, "allWeights"); got != "[1.0, 2.0, 1.0, 2.0]" {
				t.Fatalf("allWeights = %s, want the groups' weights in order", got)
			}
		})
	}

	// The union's own value determines none of the members' parts: a member keeps what it
	// holds on its own, and is undetermined without a value of its own, in either order.
	for _, order := range []string{"union_first", "member_first"} {
		t.Run("members_undetermined_by_the_union_"+order, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model(bound))
			shelf := instantiateQualified(t, ctx, idx, "test::shelf")
			groups := groupsOf(t, ctx, shelf)
			if order == "union_first" {
				if got := values(t, ctx, shelf, "allShares"); got != "[0.1, 0.2, 0.3, 0.4]" {
					t.Fatalf("allShares = %s, want its own value", got)
				}
			}
			_, err := groups[0].GetFeatureValue(ctx, "shares")
			var undetermined *UndeterminedBindingError
			if !errors.As(err, &undetermined) {
				t.Fatalf("groups.1.shares = %v, want UndeterminedBindingError", err)
			}
			if want := "binding end cannot be resolved: Group.shares is bound by `bind [0..*] groups.shares = [0..*] allShares` " +
				"through every object groups holds; the model does not determine which of the bound values Group.shares holds"; err.Error() != want {
				t.Fatalf("error = %q\nwant    %q", err.Error(), want)
			}
			if got := values(t, ctx, shelf, "allShares"); got != "[0.1, 0.2, 0.3, 0.4]" {
				t.Fatalf("allShares = %s, want its own value", got)
			}
			if got := values(t, ctx, groups[1], "weights"); got != "[1.0, 2.0]" {
				t.Fatalf("groups.2.weights = %s, want its own default", got)
			}
		})
	}

	t.Run("union_disagrees_with_its_own_value", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, model(bound+"\n:>> allWeights = (9.0, 9.0, 9.0, 9.0);"))
		shelf := instantiateQualified(t, ctx, idx, "test::shelf")
		_, err := shelf.GetFeatureValue(ctx, "allWeights")
		var conflict *BindingConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("allWeights = %v, want BindingConflictError", err)
		}
	})

	// An end's multiplicity counts the values of every object reached together: four
	// weights satisfy [4] though each group holds two, and fall short of [5..*].
	t.Run("end_multiplicity_over_the_union", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, model("attribute four : Real [4] nonunique; bind [4] groups.weights = [4] four;"))
		shelf := instantiateQualified(t, ctx, idx, "test::shelf")
		if got := values(t, ctx, shelf, "four"); got != "[1.0, 2.0, 1.0, 2.0]" {
			t.Fatalf("four = %s, want the groups' weights, four in all", got)
		}
		ctx, idx = libraryShapeContext(t, model("bind [5..*] groups.weights = [5..*] allWeights;"))
		shelf = instantiateQualified(t, ctx, idx, "test::shelf")
		_, err := shelf.GetFeatureValue(ctx, "allWeights")
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Fatalf("allWeights = %v, want ErrMultiplicityViolation", err)
		}
		if want := "`bind [5..*] groups.weights = [5..*] allWeights` links [5..*] of groups.weights, which holds 4 value(s)"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q\nwant it to contain %q", err.Error(), want)
		}
	})

	t.Run("path_reaching_no_object", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, `package test {
			item def Thing;
			item def Group { item items : Thing [2]; }
			item def Shelf {
				item groups : Group [0..*];
				item allItems : Thing [0..*];
				bind [0..*] groups.items = [0..*] allItems;
			}
			item shelf : Shelf;
		}`)
		shelf := instantiateQualified(t, ctx, idx, "test::shelf")
		all, err := shelf.GetFeatureValue(ctx, "allItems")
		if err != nil {
			t.Fatalf("allItems: %v", err)
		}
		if n := len(elementsOf(all.HeldValue())); n != 0 {
			t.Fatalf("allItems holds %d objects, want none: groups holds no object", n)
		}
	})

	// A step of the path that holds no object, or a feature the reached objects lack, is a typed refusal.
	for _, tc := range []struct{ name, shelf, want string }{
		{"non_object_step", "bind [0..*] allWeights.x = [0..*] allItems;", `binding end cannot be resolved "allWeights.x": allWeights is not an object`},
		{"missing_feature", "bind [0..*] groups.nothing = [0..*] allItems;", `binding end cannot be resolved "groups.nothing": feature nothing not found`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model(":>> allWeights = (1.0, 2.0);\n"+tc.shelf))
			shelf := instantiateQualified(t, ctx, idx, "test::shelf")
			_, err := shelf.GetFeatureValue(ctx, "allItems")
			if !errors.Is(err, ErrBindingEnd) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("allItems = %v, want ErrBindingEnd containing %q", err, tc.want)
			}
		})
	}
}

// A collection short of its lower bound is made up through its optional subsetters first, within
// their upper bounds, and through anonymous members only past them; a required one is never made twice.
func TestOptionalSubsetterFillsTheCollection(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Side;
		item def Panel {
			item sides : Side [4];
			item left : Side [1] :> sides;
			item top : Side [0..1] :> sides;
			item spare : Side [0..2] :> sides;
		}
		item panel : Panel;
	}`)
	panel := instantiateQualified(t, ctx, idx, "test::panel")
	sides, err := panel.GetFeatureValue(ctx, "sides")
	if err != nil {
		t.Fatalf("GetFeatureValue(sides): %v", err)
	}
	if got := len(elementsOf(sides.HeldValue())); got != 4 {
		t.Fatalf("sides holds %d objects, want 4", got)
	}
	top := readInstance(t, ctx, panel, "top")
	spare, err := panel.GetFeatureValue(ctx, "spare")
	if err != nil {
		t.Fatalf("GetFeatureValue(spare): %v", err)
	}
	if got := len(elementsOf(spare.HeldValue())); got != 2 {
		t.Fatalf("spare holds %d objects, want 2", got)
	}
	els := elementsOf(sides.HeldValue())
	if left := readInstance(t, ctx, panel, "left"); els[0].Instance != left.ID || els[1].Instance != top.ID {
		t.Fatalf("sides = %s, want left (%d) then top (%d) first", FormatValue(sides.HeldValue()), left.ID, top.ID)
	}
}

// A probe filling an optional subsetter that already holds an object is undone whole: the
// objects made up go, and what the subsetter held before, written as it was, stays.
func TestProbeFillingASubsetterKeepsWhatItHeld(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Side;
		item def Panel {
			item sides : Side [4];
			item left : Side [1] :> sides;
			item spare : Side [0..3] :> sides;
		}
		item panel : Panel;
		item extra : Side;
	}`)
	panel := instantiateQualified(t, ctx, idx, "test::panel")
	extra := instantiateQualified(t, ctx, idx, "test::extra")
	held := sequenceOf([]Value{{Kind: ValInstance, Instance: extra.ID}})
	if err := panel.SetFeatureValue(ctx, "spare", held); err != nil {
		t.Fatalf("SetFeatureValue(spare): %v", err)
	}
	before := *panel.FeatureValues["spare"]
	objects := len(ctx.instances)

	end := ctx.beginProbe()
	sides, err := panel.GetFeatureValue(ctx, "sides")
	if err != nil {
		t.Fatalf("GetFeatureValue(sides): %v", err)
	}
	if got := len(elementsOf(sides.HeldValue())); got != 4 {
		t.Fatalf("sides holds %d objects in the probe, want 4", got)
	}
	if got := len(elementsOf(panel.FeatureValues["spare"].HeldValue())); got != 3 {
		t.Fatalf("spare holds %d objects in the probe, want extra and two made up", got)
	}
	end()

	if got := len(ctx.instances); got != objects {
		t.Fatalf("the probe left %d objects behind", got-objects)
	}
	if after := *panel.FeatureValues["spare"]; !after.Materialized || !after.Written ||
		FormatValue(after.HeldValue()) != FormatValue(before.HeldValue()) {
		t.Fatalf("the probe changed spare from %s (written) to %s (materialized %t, written %t)",
			FormatValue(before.HeldValue()), FormatValue(after.HeldValue()), after.Materialized, after.Written)
	}
	if sides := panel.FeatureValues["sides"]; sides.Materialized {
		t.Fatalf("the probe left sides materialized as %s", FormatValue(sides.HeldValue()))
	}
}

// A collection read is one change: charged whole before any object is made, and a read
// the budget refuses leaves no object, filled subsetter or charged element behind.
func TestCollectionReadRefusedByTheBudgetFillsNothing(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Side;
		item def Panel {
			item sides : Side [4];
			item left : Side [1] :> sides;
			item top : Side [0..1] :> sides;
		}
		item panel : Panel;
	}`)
	panel := instantiateQualified(t, ctx, idx, "test::panel")
	left := readInstance(t, ctx, panel, "left")
	objects := len(ctx.instances)

	end := ctx.beginRun()
	defer end()
	ctx.maxElements = 3
	_, err := panel.GetFeatureValue(ctx, "sides")
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("sides under a budget of 3: %v, want ErrElementLimitExceeded", err)
	}
	if ctx.run.elements != 0 {
		t.Fatalf("a refused read left %d elements charged", ctx.run.elements)
	}
	if got := len(ctx.instances); got != objects {
		t.Fatalf("a refused read left %d objects behind", got-objects)
	}
	if top := panel.FeatureValues["top"]; len(elementsOf(top.HeldValue())) != 0 {
		t.Fatalf("a refused read filled top with %s", FormatValue(top.HeldValue()))
	}

	// The budget runs out at each point of the read in turn; each refusal leaves nothing behind.
	ctx.maxElements = 4
	maxSteps, refused := ctx.maxSteps, 0
	var sides *FeatureValue
	for extra := int64(0); sides == nil; extra++ {
		ctx.maxSteps = ctx.run.steps + extra
		fv, err := panel.GetFeatureValue(ctx, "sides")
		ctx.maxSteps = maxSteps
		if err == nil {
			sides = fv
			break
		}
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("sides with %d steps: %v, want ErrStepLimitExceeded", extra, err)
		}
		refused++
		if got := len(ctx.instances); got != objects {
			t.Fatalf("a read refused after %d steps left %d objects behind", extra, got-objects)
		}
		if top := panel.FeatureValues["top"]; len(elementsOf(top.HeldValue())) != 0 {
			t.Fatalf("a read refused after %d steps filled top with %s", extra, FormatValue(top.HeldValue()))
		}
		if ctx.run.elements != 0 {
			t.Fatalf("a read refused after %d steps left %d elements charged", extra, ctx.run.elements)
		}
	}
	if refused < 3 {
		t.Fatalf("only %d step budgets refused the read; the objects it makes each take one", refused)
	}
	els := elementsOf(sides.HeldValue())
	if top := readInstance(t, ctx, panel, "top"); len(els) != 4 || els[0].Instance != left.ID || els[1].Instance != top.ID {
		t.Fatalf("sides = %s, want left (%d) then top (%d) first of four", FormatValue(sides.HeldValue()), left.ID, top.ID)
	}
}

// Types conform one way or the other (KerML 1.0 §8.4.4.4, binding connector type conformance):
// an object held by a Segment feature and a Line :> Segment feature is both, in either order;
// held by two siblings under one general type, it is refused, as the pilot warns for such a binding.
func TestClassifiersMustBeComparable(t *testing.T) {
	model := `package test {
		item def Common;
		item def Segment :> Common;
		item def Line :> Segment;
		item def Bolt :> Common;
		item def Rack { item raw [1]; }
		item rack : Rack;
	}`
	for _, order := range [][2]string{{"Segment", "Line"}, {"Line", "Segment"}} {
		t.Run(order[0]+"_then_"+order[1], func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model)
			raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
			for _, name := range order {
				typ := idx.LookupQualified("test::" + name)[0]
				if err := ctx.classify(raw, typ); err != nil {
					t.Fatalf("classify(raw, %s): %v", name, err)
				}
			}
			for _, name := range []string{"Common", "Segment", "Line"} {
				if typ := idx.LookupQualified("test::" + name)[0]; !ctx.instanceConforms(raw, typ) {
					t.Fatalf("raw, a Segment and a Line, is no %s", name)
				}
			}
		})
	}
	t.Run("siblings", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, model)
		raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
		bolt, segment := idx.LookupQualified("test::Bolt")[0], idx.LookupQualified("test::Segment")[0]
		if err := ctx.classify(raw, bolt); err != nil {
			t.Fatalf("classify(raw, Bolt): %v", err)
		}
		if ctx.canClassify(raw, segment) {
			t.Fatal("a Bolt can become a Segment, though neither conforms to the other")
		}
		if !ctx.canClassify(raw, idx.LookupQualified("test::Common")[0]) {
			t.Fatal("a Bolt cannot be held as the Common it already is")
		}
	})
}

// A classification the classifier's body makes impossible (two names of one redefined
// feature valued) is refused whole outside any probe: the object keeps what it had.
func TestRefusedClassificationLeavesTheObjectAsItWas(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Ring { attribute ringCost : Real; }
		item def Band :> Ring { attribute bandCost :>> ringCost = 400.0; attribute :>> ringCost = 500.0; }
		item def Rack { item raw [1]; }
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	raw := readInstance(t, ctx, rack, "raw")
	before := sortedNames(raw.FeatureValues)

	band := idx.LookupQualified("test::Band")[0]
	if err := ctx.classify(raw, band); !errors.Is(err, ErrConflictingRedefinition) {
		t.Fatalf("classify(raw, Band) = %v, want ErrConflictingRedefinition", err)
	}
	if len(raw.classifiers) != 0 {
		t.Fatalf("a refused classification left raw classified by %v", raw.classifiers)
	}
	if after := sortedNames(raw.FeatureValues); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Fatalf("a refused classification changed raw's features from %v to %v", before, after)
	}
}

// An object classified by a type exhibiting a behavior runs it (KerML 1.0 §7.4.9), once,
// however many features hold it; a probe undoes the run with the holding.
func TestClassificationStartsTheClassifierBehaviors(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		state def Glowing {
			attribute cycles : Integer = 0;
			entry; then lit;
			state lit { entry action count { assign cycles := cycles + 1; } }
		}
		part def Lamp { exhibit state glow : Glowing; }
		part def Room {
			part lamp : Lamp [1] = bulb;
			part spare : Lamp [1] = bulb;
			part bulb [1];
		}
		part room : Room;
	}`)
	room := instantiateQualified(t, ctx, idx, "test::room")

	end := ctx.beginProbe()
	bulb := readInstance(t, ctx, room, "lamp")
	if _, ok := bulb.Behavior("glow"); !ok {
		t.Fatal("bulb, a Lamp while lamp holds it, runs no glow behavior")
	}
	end()
	if len(bulb.behaviors) != 0 || len(ctx.objectBehaviors) != 0 {
		t.Fatalf("after the probe, bulb runs %d behaviors and the context %d", len(bulb.behaviors), len(ctx.objectBehaviors))
	}

	if bulb = readInstance(t, ctx, room, "lamp"); readInstance(t, ctx, room, "spare") != bulb {
		t.Fatal("lamp and spare hold different objects")
	}
	if len(bulb.behaviors) != 1 {
		t.Fatalf("bulb, held by two Lamp features, runs %d behaviors, want 1", len(bulb.behaviors))
	}
	behavior, ok := bulb.Behavior("glow")
	if !ok || behavior.State == nil {
		t.Fatal("bulb runs no glow state machine")
	}
	if got := behavior.State.stateData["cycles"].Const.Int; got != 1 {
		t.Fatalf("glow ran to cycles = %d, want 1", got)
	}
}

// A classification whose behaviors fail to start is undone whole: what a behavior wrote to
// a feature the object already had goes with the features and behaviors it added.
func TestRefusedClassificationUndoesWhatItsBehaviorsDid(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Counter { attribute hits : Integer = 0; }
		item def Tallied :> Counter {
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign hits := hits + 1; } }
			}
			exhibit state break {
				entry; then on;
				state on { entry action fail { assign hits := 1 / 0; } }
			}
		}
		item def Rack { item raw : Counter [1]; }
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	raw := readInstance(t, ctx, rack, "raw")
	hits, err := raw.GetFeatureValue(ctx, "hits")
	if err != nil {
		t.Fatalf("hits: %v", err)
	}
	before := FormatValue(hits.HeldValue())

	if err := ctx.classify(raw, idx.LookupQualified("test::Tallied")[0]); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("classify(raw, Tallied) = %v, want ErrDivisionByZero", err)
	}
	if len(raw.classifiers) != 0 || len(raw.behaviors) != 0 || len(ctx.objectBehaviors) != 0 {
		t.Fatalf("a refused classification left raw classified by %v running %d behaviors (context: %d)",
			raw.classifiers, len(raw.behaviors), len(ctx.objectBehaviors))
	}
	if _, ok := raw.FeatureValues["tally"]; ok {
		t.Fatal("a refused classification left raw with the tally feature")
	}
	if after := FormatValue(hits.HeldValue()); raw.FeatureValues["hits"] != hits || after != before {
		t.Fatalf("a refused classification changed hits from %s to %s", before, after)
	}
}

// A variant a refused classification's behavior selected goes with it: a value variant
// stands for no object to abandon, so the selection itself is undone.
func TestRefusedClassificationUndoesTheVariantsItSelected(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Counter { attribute hits : Integer = 0; }
		item def Car {
			variation attribute power : Real {
				variant attribute strong = 150.0;
				variant attribute weak = 120.0;
			}
		}
		item def Tallied :> Counter {
			item car : Car [1] { attribute :>> power = power::strong; }
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign hits := car.power / hits; } }
			}
		}
		item def Rack { item raw : Counter [1]; item own : Car [1] { attribute :>> power = power::weak; } }
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	raw := readInstance(t, ctx, rack, "raw")
	own := readInstance(t, ctx, rack, "own")
	if _, err := own.GetFeatureValue(ctx, "power"); err != nil {
		t.Fatalf("own.power: %v", err)
	}
	before := maps.Clone(ctx.selectedVariants)
	if got := before[variantSelection{owner: own.ID, variation: "power"}]; got != "weak" {
		t.Fatalf("own selected %q before classifying, want weak", got)
	}

	if err := ctx.classify(raw, idx.LookupQualified("test::Tallied")[0]); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("classify(raw, Tallied) = %v, want ErrDivisionByZero", err)
	}
	if !maps.Equal(ctx.selectedVariants, before) {
		t.Fatalf("a refused classification changed the variants selected from %v to %v", before, ctx.selectedVariants)
	}
}

// A collection is classified whole: when a later object's classification fails, the objects
// classified before it are left as they were, behaviors, writes and all.
func TestRefusedCollectionClassificationUndoesTheEarlierObjects(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Counter { attribute hits : Rational; }
		item def Tallied :> Counter {
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign hits := 10 / hits; } }
			}
		}
		item def Rack {
			item lead : Counter [1] { :>> hits = 1; }
			item trail : Counter [1] { :>> hits = 0; }
			item tallied : Tallied [2] = (lead, trail);
		}
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")

	// lead is classified before trail's behavior fails; the read materializes it all the same.
	if _, err := rack.GetFeatureValue(ctx, "tallied"); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("rack.tallied = %v, want ErrDivisionByZero", err)
	}
	held := rack.FeatureValues["lead"].HeldValue()
	lead, ok := ctx.getInstance(held.Instance)
	if held.Kind != ValInstance || !ok {
		t.Fatalf("lead = %s, want the object the failed read materialized", FormatValue(held))
	}
	if len(lead.classifiers) != 0 || len(lead.behaviors) != 0 || len(ctx.objectBehaviors) != 0 {
		t.Fatalf("a refused collection left lead classified by %v running %d behaviors (context: %d)",
			lead.classifiers, len(lead.behaviors), len(ctx.objectBehaviors))
	}
	if _, ok := lead.FeatureValues["tally"]; ok {
		t.Fatal("a refused collection left lead with the tally feature")
	}
	if hits := lead.FeatureValues["hits"]; hits.Written {
		t.Fatalf("a refused collection left lead.hits written to %s", FormatValue(hits.HeldValue()))
	}
}

// The objects a classification's behaviors make go with it when it is undone: a refused
// collection leaves the context holding what it held, however often the read is retried.
func TestRefusedCollectionClassificationAbandonsWhatItsBehaviorsMade(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Counter { attribute hits : Rational; }
		item def Gauge { attribute reading : Integer = 1; }
		item def Tallied :> Counter {
			item gauge : Gauge [1];
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign hits := gauge.reading / hits; } }
			}
		}
		item def Rack {
			item lead : Counter [1] { :>> hits = 1; }
			item trail : Counter [1] { :>> hits = 0; }
			item tallied : Tallied [2] = (lead, trail);
		}
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")

	// The read materializes rack's lead and trail, which stay; lead's gauge, made by the
	// tally lead ran as a Tallied, does not.
	var held []int64
	var occurrences map[*symbols.Symbol]int64
	for attempt := 1; attempt <= 3; attempt++ {
		if _, err := rack.GetFeatureValue(ctx, "tallied"); !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("attempt %d: rack.tallied = %v, want ErrDivisionByZero", attempt, err)
		}
		if attempt == 1 {
			held, occurrences = ctx.InstanceIDs(), maps.Clone(ctx.occurrences)
			if len(held) != 3 {
				t.Fatalf("a refused collection left the objects %v, want rack, lead and trail only", held)
			}
		}
		if after := ctx.InstanceIDs(); !slices.Equal(after, held) {
			t.Fatalf("attempt %d: a refused collection changed the objects held from %v to %v", attempt, held, after)
		}
		if !maps.Equal(ctx.occurrences, occurrences) {
			t.Fatalf("attempt %d: a refused collection changed the occurrences held to %v", attempt, ctx.occurrences)
		}
		if len(ctx.created) != len(held) || len(ctx.objectBehaviors) != 0 {
			t.Fatalf("attempt %d: a refused collection left %d creations and %d behaviors",
				attempt, len(ctx.created), len(ctx.objectBehaviors))
		}
		for _, name := range []string{"lead", "trail"} {
			inst, ok := ctx.getInstance(rack.FeatureValues[name].HeldValue().Instance)
			if !ok {
				t.Fatalf("attempt %d: %s holds no object", attempt, name)
			}
			if _, ok := inst.FeatureValues["gauge"]; ok || len(inst.classifiers) != 0 || len(inst.behaviors) != 0 {
				t.Fatalf("attempt %d: a refused collection left %s classified by %v running %d behaviors",
					attempt, name, inst.classifiers, len(inst.behaviors))
			}
		}
	}
}

// A classifier redefining a feature the object carries refines it (KerML 1.0 §7.3.4.5): the
// object reads the redefinition's default, type and multiplicity; what it held is kept where
// the redefinition admits it, and refused whole where it does not.
func TestNarrowerClassifierRefinesTheCarriedFeatures(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Engine;
		item def V8 :> Engine;
		item def Car {
			attribute doors : Integer default = 4;
			attribute tags : String [0..2] default = ("a", "b");
			item engine : Engine [1];
		}
		item def Coupe :> Car { attribute :>> doors default = 2; item :>> engine : V8; }
		item def Tagged :> Car { attribute :>> tags : String [1]; }
		item def Rack {
			item raw : Car [1];
			item sporty : Car [1] { attribute :>> doors default = 3; }
			item coupe : Coupe [1] = raw;
			item coupe2 : Coupe [1] = sporty;
		}
		item rack : Rack;
		item car : Car;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	raw := readInstance(t, ctx, rack, "raw")
	coupe := idx.LookupQualified("test::Coupe")[0]
	if !ctx.instanceConforms(raw, coupe) {
		t.Fatalf("raw held by coupe is classified by %v, want Coupe", raw.classifiers)
	}
	if doors := readInt(t, ctx, raw, "doors"); doors != 2 {
		t.Errorf("raw.doors = %d as a Coupe, want Coupe's default 2", doors)
	}
	if feat := raw.FeatureValues["doors"].Feature; feat.OwnerType != coupe {
		t.Errorf("raw.doors reads %s's declaration, want Coupe's", feat.OwnerType.Name)
	}
	if feat := raw.FeatureValues["tags"].Feature; feat.OwnerType == coupe {
		t.Error("raw.tags, which Coupe does not redefine, reads a declaration of Coupe")
	}
	engine := readInstance(t, ctx, raw, "engine")
	if v8 := idx.LookupQualified("test::V8")[0]; !ctx.instanceConforms(engine, v8) {
		t.Errorf("raw.engine held by a Coupe's engine : V8 is classified by %v, want V8", engine.classifiers)
	}

	// A usage's own redefinition is not refined by a classifier redefining what it redefines.
	sporty := readInstance(t, ctx, rack, "sporty")
	if doors := readInt(t, ctx, sporty, "doors"); doors != 3 {
		t.Errorf("sporty.doors = %d as a Coupe, want the usage's own default 3", doors)
	}

	// A written value stays through the refinement, read by the refining declaration.
	car := instantiateQualified(t, ctx, idx, "test::car")
	if err := car.SetFeatureValue(ctx, "doors", integerValue(5)); err != nil {
		t.Fatalf("write car.doors: %v", err)
	}
	if err := ctx.classify(car, coupe); err != nil {
		t.Fatalf("classify(car, Coupe): %v", err)
	}
	if doors := car.FeatureValues["doors"]; !doors.Written || doors.HeldValue().Const.Int != 5 || doors.Feature.OwnerType != coupe {
		t.Errorf("car.doors = %s (written %t, declared by %s) after classifying, want the written 5 read by Coupe's declaration",
			FormatValue(doors.HeldValue()), doors.Written, doors.Feature.OwnerType.Name)
	}

	// A held value the refined declaration does not admit refuses the classification whole.
	if err := car.SetFeatureValue(ctx, "tags", sequenceOf([]Value{NewStringValue("x"), NewStringValue("y")})); err != nil {
		t.Fatalf("write car.tags: %v", err)
	}
	if err := ctx.classify(car, idx.LookupQualified("test::Tagged")[0]); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("classify(car, Tagged) = %v, want ErrMultiplicityViolation for two values in tags [1]", err)
	}
	if len(car.classifiers) != 1 || car.FeatureValues["tags"].Feature.OwnerType == idx.LookupQualified("test::Tagged")[0] {
		t.Errorf("a refused classification left car classified by %v reading tags from %s",
			car.classifiers, car.FeatureValues["tags"].Feature.OwnerType.Name)
	}
}

// An object's relationships are those of every type classifying it — declared type first,
// then each classifier — and one a classifier inherits from a type already counted is not listed twice.
func TestRelationshipsComeFromEveryTypeOfTheObject(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Base {
			attribute a : Integer;
			attribute b : Integer = 1;
			bind a = b;
			attribute xs : Integer [*];
			attribute x1 : Integer [*] :> xs = (1);
			item p; item q;
			connect p to q;
		}
		item def Wide :> Base {
			attribute c : Integer = 2;
			bind a = c;
			attribute x2 : Integer [*] :> xs = (2);
			item r;
			connect q to r;
		}
		item def Rack { item raw : Base [1]; }
		item rack : Rack;
	}`)
	raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
	if err := ctx.classify(raw, idx.LookupQualified("test::Wide")[0]); err != nil {
		t.Fatalf("classify(raw, Wide): %v", err)
	}
	bindings := ctx.bindingsOf(raw, "a")
	if len(bindings) != 2 || bindings[0].Ends[1].Path != "b" || bindings[1].Ends[1].Path != "c" {
		t.Fatalf("bindings of a = %+v, want Base's a = b then Wide's a = c", bindings)
	}
	var subsetters []string
	for _, feat := range ctx.subsettingFeaturesOf(raw, "xs") {
		subsetters = append(subsetters, feat.Name)
	}
	if strings.Join(subsetters, ",") != "x1,x2" {
		t.Fatalf("subsetters of xs = %v, want x1 then x2", subsetters)
	}
	var conns []string
	for _, conn := range ctx.connectionsOf(raw) {
		if ends := strings.Join(conn.Ends, "-"); ends == "p-q" || ends == "q-r" {
			conns = append(conns, ends)
		}
	}
	if strings.Join(conns, ",") != "p-q,q-r" {
		t.Fatalf("connections = %v, want Base's p-q then Wide's q-r, each once", conns)
	}
	if anon := ctx.anonymousConnectorsOf(raw.types()); len(anon) != 2 {
		t.Fatalf("anonymous connectors = %d, want Base's and Wide's", len(anon))
	}
}

// Two comparable classifiers declaring one name without redefinition read the narrower
// type's declaration, whichever was added first — as an object created as that type would.
func TestSameNameFeaturesOfClassifiersReadTheNarrowerDeclaration(t *testing.T) {
	const model = `package test {
		private import ScalarValues::*;
		item def Common;
		item def A :> Common { attribute n : Real = 1.0; }
		item def B :> A { attribute n : Real = 2.0; }
		item def C :> A { attribute n : Integer [2] = (3, 4); }
		item def Rack { item raw [1]; }
		item rack : Rack;
	}`
	classified := func(t *testing.T, ctx *Context, idx *symbols.Index, order ...string) *Instance {
		t.Helper()
		raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
		for _, name := range order {
			if err := ctx.classify(raw, idx.LookupQualified("test::" + name)[0]); err != nil {
				t.Fatalf("classify(raw, %s) after %v: %v", name, order, err)
			}
		}
		return raw
	}
	for _, order := range [][]string{{"A", "B"}, {"B", "A"}} {
		ctx, idx := libraryShapeContext(t, model)
		raw := classified(t, ctx, idx, order...)
		fv, err := raw.GetFeatureValue(ctx, "n")
		if err != nil {
			t.Fatalf("%v: n: %v", order, err)
		}
		if got := realValue(t, fv.HeldValue()); got != 2.0 || fv.Feature.OwnerType != idx.LookupQualified("test::B")[0] {
			t.Errorf("%v: n = %v read by %s, want B's default 2.0 read by B's declaration", order, got, fv.Feature.OwnerType.Name)
		}
	}
	for _, order := range [][]string{{"A", "C"}, {"C", "A"}} {
		ctx, idx := libraryShapeContext(t, model)
		raw := classified(t, ctx, idx, order...)
		fv, err := raw.GetFeatureValue(ctx, "n")
		if err != nil {
			t.Fatalf("%v: n: %v", order, err)
		}
		if got := FormatValue(fv.HeldValue()); got != "[3, 4]" || fv.Feature.OwnerType != idx.LookupQualified("test::C")[0] {
			t.Errorf("%v: n = %s read by %s, want C's [3, 4] read by C's declaration", order, got, fv.Feature.OwnerType.Name)
		}
	}

	// A held value the narrower declaration does not admit refuses the classification, leaving the object as it was.
	ctx, idx := libraryShapeContext(t, model)
	raw := classified(t, ctx, idx, "A")
	if err := raw.SetFeatureValue(ctx, "n", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1.5}}); err != nil {
		t.Fatalf("write raw.n: %v", err)
	}
	err := ctx.classify(raw, idx.LookupQualified("test::C")[0])
	if !errors.Is(err, ErrMultiplicityViolation) && !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("classify(raw, C) with n = 1.5 = %v, want a typed refusal of a Real in n : Integer [2]", err)
	}
	fv := raw.FeatureValues["n"]
	if len(raw.classifiers) != 1 || fv.Feature.OwnerType != idx.LookupQualified("test::A")[0] || realValue(t, fv.HeldValue()) != 1.5 {
		t.Errorf("refused classification left raw classified by %v with n = %s read by %s, want A's declaration holding 1.5",
			raw.classifiers, FormatValue(fv.HeldValue()), fv.Feature.OwnerType.Name)
	}
}

// Walks over an object's feature values cover the features its classifiers add — their defaults,
// their errors and the objects they hold — after the declared type's, each shared value once.
func TestObjectWalksCoverTheFeaturesClassifiersAdd(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Common;
		item def Base { attribute mass : Real; }
		item def Loaded :> Base {
			attribute grossMass :>> mass = (1.0, 2.0);
			attribute bad : Integer [1] = (5, 6);
			item inner : Common;
		}
		item def Rack { item raw : Base [1]; }
		item rack : Rack;
	}`)
	raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
	if errs, _ := ctx.MaterializationErrors(raw); len(errs) != 0 {
		t.Fatalf("MaterializationErrors as a Base = %v, want none", errs)
	}
	if nested := ctx.nestedObjects(raw); len(nested) != 0 {
		t.Fatalf("nestedObjects as a Base = %d objects, want none", len(nested))
	}
	if err := ctx.classify(raw, idx.LookupQualified("test::Loaded")[0]); err != nil {
		t.Fatalf("classify(raw, Loaded): %v", err)
	}

	var names []string
	for _, of := range ctx.FeaturesOfObject(raw) {
		names = append(names, of.Name)
	}
	if last := len(names) - 3; last < 1 || names[0] != "mass" || !slices.Equal(names[last:], []string{"grossMass", "bad", "inner"}) {
		t.Errorf("FeaturesOfObject = %v, want the declared type's features then the classifier's grossMass, bad, inner", names)
	}
	errs, _ := ctx.MaterializationErrors(raw)
	if len(errs) != 2 {
		t.Fatalf("MaterializationErrors as a Loaded = %v, want mass/grossMass once and bad once", errs)
	}
	for _, err := range errs {
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("err = %v, want ErrMultiplicityViolation", err)
		}
	}
	nested := ctx.nestedObjects(raw)
	if len(nested) != 1 || nested[0].feature != "inner" || !ctx.instanceConforms(nested[0].instance, idx.LookupQualified("test::Common")[0]) {
		t.Errorf("nestedObjects as a Loaded = %v, want the Common held by inner", nested)
	}
}

// A classifier's behavior writes the features the classifier itself declares: the
// performer is every type it is classified by, not only the one it was created as.
func TestClassifierBehaviorsWriteTheFeaturesTheClassifierAdds(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Counter { attribute hits : Integer = 0; }
		item def Tallied :> Counter {
			attribute tallies : Integer = 0;
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign tallies := tallies + 1; } }
			}
		}
		item def Rack { item raw : Counter [1]; }
		item rack : Rack;
	}`)
	raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
	if err := ctx.classify(raw, idx.LookupQualified("test::Tallied")[0]); err != nil {
		t.Fatalf("classify(raw, Tallied): %v", err)
	}
	if tallies := readInt(t, ctx, raw, "tallies"); tallies != 1 {
		t.Errorf("raw.tallies = %d after Tallied's behavior ran, want 1", tallies)
	}
}

// istype and hastype judge an object by every type it is classified by (KerML 1.0
// §7.4.9): hastype exactly, istype by conformance.
func TestTypePredicatesSeeTheClassifiersOfAnObject(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Curve;
		item def Line :> Curve;
		item def Segment :> Line;
		item def Shape {
			item raw [1];
			item loose [1];
			item edge : Line [1] = raw;
			attribute looseIsLine = loose istype Line;
			attribute isLine = raw istype Line;
			attribute isCurve = raw istype Curve;
			attribute isSegment = raw istype Segment;
			attribute hasLine = raw hastype Line;
			attribute hasCurve = raw hastype Curve;
			attribute hasSegment = raw hastype Segment;
		}
		item shape : Shape;
	}`)
	shape := instantiateQualified(t, ctx, idx, "test::shape")
	if readInstance(t, ctx, shape, "edge") != readInstance(t, ctx, shape, "raw") {
		t.Fatal("edge holds an object other than raw")
	}
	for name, want := range map[string]bool{
		"looseIsLine": false,
		"isLine":      true, "isCurve": true, "isSegment": false,
		"hasLine": true, "hasCurve": false, "hasSegment": false,
	} {
		if got := readBool(t, ctx, shape, name); got != want {
			t.Errorf("%s = %t once edge : Line holds raw, want %t", name, got, want)
		}
	}
}

// A selected variant is exactly the variant chosen, and its object is judged by each type
// it is classified by once a typed feature holds it, like any object.
func TestTypePredicatesSeeTheTypesOfASelectedVariantObject(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Engine { attribute cylinders : Integer = 4; }
		item def Tallied :> Engine { attribute tally : Integer = 7; }
		item def Car {
			variation item engine : Engine [1] {
				variant item small : Engine;
				variant item big : Engine { :>> cylinders = 8; }
			}
		}
		item def Garage {
			item car : Car { :>> engine = engine::big; }
			item tallied : Tallied [1] = car.engine;
			attribute isEngine = car.engine istype Engine;
			attribute hasEngine = car.engine hastype Engine;
			attribute isTallied = car.engine istype Tallied;
			attribute hasTallied = car.engine hastype Tallied;
			attribute hasCar = car.engine hastype Car;
			attribute isData = car.engine istype ScalarValues::ScalarValue;
		}
		item garage : Garage;
	}`)
	garage := instantiateQualified(t, ctx, idx, "test::garage")
	fv, err := garage.GetFeatureValue(ctx, "tallied")
	if err != nil || fv.Value.Kind != ValVariant || fv.Value.Instance == 0 {
		t.Fatalf("garage.tallied = %+v, %v; want the materialized big variant", fv, err)
	}
	for name, want := range map[string]bool{
		"isEngine": true, "hasEngine": false,
		"isTallied": true, "hasTallied": true,
		"hasCar": false, "isData": false,
	} {
		if got := readBool(t, ctx, garage, name); got != want {
			t.Errorf("%s = %t once tallied : Tallied holds the big variant, want %t", name, got, want)
		}
	}
}

// A type an object already conforms to declares nothing it lacks, but a feature of that
// type holding it still makes it a direct type of the object: hastype answers alike in
// either classification order, and classifying by a type twice records it once.
func TestHoldingByAWiderTypeRecordsItAsADirectType(t *testing.T) {
	for _, order := range [][]string{{"Line", "Segment"}, {"Segment", "Line"}} {
		t.Run(strings.Join(order, "-then-"), func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, `package test {
				item def Curve;
				item def Line :> Curve { attribute slope : Real; }
				item def Segment :> Line { attribute span : Real; }
				item def Rack { item raw [1]; }
				item rack : Rack;
			}`)
			pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
			if !ok || pkg.Scope == nil {
				t.Fatal("test package not indexed")
			}
			raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
			for _, name := range order {
				if err := ctx.classify(raw, idx.LookupQualified("test::" + name)[0]); err != nil {
					t.Fatalf("classify(raw, %s): %v", name, err)
				}
			}
			features := len(raw.FeatureValues)
			if err := ctx.classify(raw, idx.LookupQualified("test::Line")[0]); err != nil {
				t.Fatalf("classify(raw, Line) again: %v", err)
			}
			if len(raw.classifiers) != 2 || len(raw.FeatureValues) != features {
				t.Fatalf("classifiers = %v with %d feature values after classifying by Line again, want Line and Segment once with %d",
					raw.classifiers, len(raw.FeatureValues), features)
			}
			for _, name := range []string{"slope", "span"} {
				if _, ok := raw.FeatureValues[name]; !ok {
					t.Errorf("raw carries no %s", name)
				}
			}
			for src, want := range map[string]bool{
				"rack.raw hastype Line": true, "rack.raw hastype Segment": true,
				"rack.raw hastype Curve": false, "rack.raw istype Curve": true,
			} {
				val, err := evalIn(t, ctx, pkg.Scope, src)
				if err != nil || val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
					t.Fatalf("%s = %s, %v; want a Boolean", src, FormatValue(val), err)
				}
				if val.Const.Bool != want {
					t.Errorf("%s = %t, want %t", src, val.Const.Bool, want)
				}
			}
		})
	}
}

// Membership in an enumeration is decided by equality with its enumerated values, its only
// instances (SysML v2 §8.3.7); hastype reads the value's own type alone (KerML 1.0 §7.4.9.2).
func TestEnumerationClassifiesByItsEnumeratedValues(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		metadata def Hot;
		enum def Level :> Integer { low = 1; high = 3 { attribute n = 9; @Hot; } }
		enum def Grade :> Real { a = 4.0; b = 3.0; }
		enum def Color { red; green; blue; }
		enum def Rank :> Integer { one = 1; three = 3; }
		enum def Size :> Real { = 60.0; = 70.0; }
		attribute def Even :> Integer;
		attribute two : Integer = 2;
		attribute three : Integer = 3;
		attribute lvl : Level = Level::high;
		attribute held : Level = three;
		attribute cast : Level[0..1] = 3 as Level;
		attribute ranked : Rank = Level::high;
		attribute c : Color = Color::red;
		calc def isLevel { in x : Level; return : Boolean = x hastype Level; }
		calc def asLevel { in n : Integer; return : Level = n; }
		calc def viaBody { in n : Integer; attribute doubled = n + n; return : Level = doubled - n; }
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	for src, want := range map[string]bool{
		"3 istype Level": true, "2 istype Level": false, "three istype Level": true, "two istype Level": false,
		"3 @ Level": true, "2 @ Level": false, "(1, 2, 3) @ Level": true, "(2, 4) @ Level": false,
		"3 hastype Level": false, "3 hastype Integer": true, "three hastype Level": false,
		"Level::high hastype Level": true, "Level::high hastype Integer": false,
		"Level::high istype Integer": true, "Level::high istype Level": true, "Level::high istype Even": false,
		"lvl hastype Level": true, "lvl hastype Integer": false, "lvl istype Integer": true,
		"held hastype Level": true, "cast hastype Level": true,
		"Level::high istype Rank": true, "Level::high hastype Rank": false,
		"(Level::high as Rank) hastype Rank": true, "(Level::high as Rank) hastype Level": false,
		"ranked hastype Rank": true, "ranked hastype Level": false, "Level::low istype Rank": true,
		"60.0 istype Size": true, "65.0 istype Size": false, "(60.0 as Size) hastype Size": true, "60.0 hastype Size": false,
		"(3 as Level) hastype Level": true, "(3 as Level) hastype Integer": false, "(3 as Level) == 3": true,
		"Level::high == 3": true, "Level::high + 1 == 4": true, "(Level::high + 1) hastype Integer": true,
		"4 istype Grade": true, "4.0 istype Grade": true, "2.5 istype Grade": false, "3 istype Grade": true,
		"Grade::a hastype Grade": true, "Grade::a hastype Real": false,
		"3 istype Color": false, "Color::red istype Color": true, "Color::red hastype Color": true,
		"c hastype Color": true, "Color::red istype Level": false, "Level::high istype Color": false,
		"(Color::red as Color) hastype Color": true,
		"isLevel(3)":                          true, "isLevel(three)": true, "asLevel(3) hastype Level": true, "asLevel(3) hastype Integer": false,
		"viaBody(3) hastype Level": true, "viaBody(3) hastype Integer": false,
		"Level::high.n == 9": true, "lvl.n == 9": true, "(3 as Level).n == 9": true,
		"Level::high @ Hot": true, "Level::low @ Hot": false,
	} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil || val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
			t.Fatalf("%s = %s, %v; want a Boolean", src, FormatValue(val), err)
		}
		if val.Const.Bool != want {
			t.Errorf("%s = %t, want %t", src, val.Const.Bool, want)
		}
	}
	for src, want := range map[string]string{
		"3 as Level": "3", "2 as Level": "[]", "(1, 2, 3, 4) as Level": "[1, 3]", "three as Level": "3",
		"Color::red as Color": "Color::red", "Color::red as Level": "[]", "4 as Grade": "4.0",
	} {
		val, err := evalIn(t, ctx, pkg.Scope, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got := FormatValue(val); got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
	if _, err := evalIn(t, ctx, pkg.Scope, "5 as Even"); !errors.Is(err, ErrUndecidedClassification) {
		t.Errorf("5 as Even: %v, want ErrUndecidedClassification", err)
	}
	for _, src := range []string{"isLevel(2)", "asLevel(2)"} {
		if _, err := evalIn(t, ctx, pkg.Scope, src); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: %v, want ErrTypeMismatch", src, err)
		}
	}
}

// A feature typed by an enumeration admits an enumerated value, held as the literal it
// equals, and refuses any other by the write-conformance rule.
func TestEnumerationTypedFeatureAdmitsOnlyEnumeratedValues(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		enum def Level :> Integer { low = 1; high = 3; }
		attribute two : Integer = 2;
		attribute three : Integer = 3;
		part def Dial {
			attribute setting : Level = three;
			attribute isHigh = setting hastype Level;
		}
		part def Broken { attribute setting : Level = two; }
		part dial : Dial;
		part broken : Broken;
	}`)
	dial := instantiateQualified(t, ctx, idx, "test::dial")
	if got := readInt(t, ctx, dial, "setting"); got != 3 {
		t.Fatalf("dial.setting = %d, want 3", got)
	}
	if !readBool(t, ctx, dial, "isHigh") {
		t.Error("dial.setting hastype Level = false once 3 is held as a Level")
	}
	broken, err := ctx.Instantiate(idx.LookupQualified("test::broken")[0])
	if err == nil {
		_, err = broken.GetFeatureValue(ctx, "setting")
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("broken.setting = 2: %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "Level") {
		t.Errorf("error = %v, want the feature's type named", err)
	}
}
