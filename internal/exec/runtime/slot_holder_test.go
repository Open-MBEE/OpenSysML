package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestSlotHoldersSeeClassificationBetweenChoicesAtOneInstant(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Added { attribute value : Real; }
		item def Rack { item raw [1]; }
		item rack : Rack;
		item other : Added;
	}`)
	raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
	other := instantiateQualified(t, ctx, idx, "test::other")
	typ := idx.LookupQualified("test::Added")[0]
	sym := idx.LookupQualified("test::Added::value")[0]
	a := &StateExecutor{self: raw, graph: &lower.StateGraph{}}
	b := &StateExecutor{self: other, graph: &lower.StateGraph{}}
	p := lower.Place{Name: "value", Sym: sym, Direct: true}
	fps := []lower.Footprint{{Writes: []lower.Place{p}}, {Writes: []lower.Place{p}}}
	classify := func() ChoicePoint {
		c := ChoicePoint{Kind: ChoiceDueOrder, Alternatives: []string{"raw", "other"}}
		holder := ctx.slotHolders()
		classifyChoice(&c, fps, func(i, j int) lower.Relation { return ctx.relationBy(a, b, holder) })
		return c
	}
	now := ctx.clock.now
	if c := classify(); c.Independent || len(c.Touches) != 1 {
		t.Fatalf("before classification: independent=%t, touches=%v", c.Independent, c.Touches)
	}
	if err := ctx.classify(raw, typ); err != nil {
		t.Fatal(err)
	}
	if ctx.clock.now != now {
		t.Fatal("classification advanced the clock")
	}
	if c := classify(); !c.Independent || len(c.Touches) != 0 {
		t.Fatalf("after classification: independent=%t, touches=%v", c.Independent, c.Touches)
	}
	if !ctx.relation(a, b).Held(p, p) {
		t.Fatal("uncached relation does not see the added feature")
	}
}

func TestSlotHoldersAgreeWithUncachedLookups(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Base { attribute value : Real; }
		item def Derived :> Base { attribute :>> value; attribute extra : Real; }
		item alpha : Derived;
		item beta : Base;
	}`)
	first := instantiateQualified(t, ctx, idx, "test::alpha")
	second := instantiateQualified(t, ctx, idx, "test::beta")
	local := &symbols.Symbol{Decl: &ast.Usage{}}
	stateLocal := &symbols.Symbol{Decl: &ast.Usage{}}
	state := &StateExecutor{self: first, graph: &lower.StateGraph{
		Attributes:      []lower.Attribute{{Node: local.Decl}},
		StateAttributes: map[*ast.StateNode][]lower.Attribute{nil: {{Node: stateLocal.Decl}}},
	}}
	action := &ActionExecutor{graph: &lower.ActionGraph{Attributes: []lower.Attribute{{Node: local.Decl}}}}
	action.self = first
	waiters := []clockWaiter{state, action, &StateExecutor{self: second, graph: &lower.StateGraph{}}, &ActionExecutor{graph: &lower.ActionGraph{}}}
	syms := []*symbols.Symbol{nil, local, stateLocal, {Decl: &ast.Usage{}}}
	for _, name := range []string{"test::Base::value", "test::Derived::value", "test::Derived::extra"} {
		syms = append(syms, idx.LookupQualified(name)[0])
	}
	check := func() {
		holder := ctx.slotHolders()
		for range 2 {
			for _, w := range waiters {
				for _, sym := range syms {
					if got, want := holder(w, sym), ctx.slotHolder(w, sym); got != want {
						t.Fatalf("%T, %v: holder=%v, want %v", w, sym, got, want)
					}
				}
			}
		}
	}
	check()
	state.occurrence = second
	check()
	first.FeatureValues["value"].Feature = &EffectiveFeature{Symbol: syms[len(syms)-1]}
	check()
}
