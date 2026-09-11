package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const carrySrc = `
package test {
	private import ScalarValues::*;
	part def Ship {
		attribute cost : Real = 5.0;
		calc weigh { in n : Real; return : Real = cost * n; }
	}
	part ship : Ship;
	part def Fleet { part flagship : Ship; }
	part fleet : Fleet;
}`

// carryContexts are two contexts of one model, a scope reading the model's names,
// and an evaluator for the first.
func carryContexts(t *testing.T) (held, row *Context, scope *symbols.Scope, eval func(string) Value) {
	t.Helper()
	model, resolver, root := parseAndBuildModel(t, carrySrc)
	m := NewModel(model, resolver)
	held, row = NewContext(m, 10000), NewContext(m, 10000)
	pkg, _ := root.LookupLocal("test")
	scope = pkg.Scope
	eval = func(src string) Value {
		val, err := held.EvalWithScope(parseExpr(t, src), scope)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		return val
	}
	return held, row, scope, eval
}

// A value of one context is carried into another of the same model: a number, a
// string or a quantity as it is; an object, wherever it sits — alone, among the
// elements of a collection, as the self of a function — as the one brought for it,
// each held object asked for once; a function reads its calc again in the new
// context, so applying it there reads the object brought.
func TestCarryTakesAValueIntoAnotherContext(t *testing.T) {
	_, row, scope, eval := carryContexts(t)
	ship := eval("ship")
	heldShip, _ := ship.Object()
	if heldShip == 0 {
		t.Fatalf("ship evaluates to %s, want an object", FormatValue(ship))
	}
	var rowShip *Instance
	brought := 0
	bring := func(id int64) (*Instance, error) {
		brought++
		if id != heldShip {
			t.Fatalf("bring(#%d), want the held ship #%d", id, heldShip)
		}
		if rowShip == nil {
			sym, _ := scope.LookupLocal("Ship")
			inst, err := row.Instantiate(sym)
			if err != nil {
				t.Fatal(err)
			}
			rowShip = inst
		}
		return rowShip, nil
	}

	for _, src := range []string{"7.0", "\"seven\"", "ship.cost + 2.0", "(1, 2, 3)"} {
		val := eval(src)
		carried, err := row.Carry(val, bring)
		if err != nil {
			t.Fatalf("Carry(%s): %v", src, err)
		}
		if FormatValue(carried) != FormatValue(val) {
			t.Errorf("Carry(%s) = %s, want %s", src, FormatValue(carried), FormatValue(val))
		}
	}
	if brought != 0 {
		t.Errorf("carrying values naming no object brought %d", brought)
	}

	carried, err := row.Carry(NewSequenceValue(&Sequence{elements: []Value{ship, eval("2.0"), ship}}), bring)
	if err != nil {
		t.Fatalf("Carry(sequence): %v", err)
	}
	if brought != 2 || rowShip == nil {
		t.Fatalf("carrying a sequence naming the ship twice brought %d, want 2", brought)
	}
	for i, elem := range []int{0, 2} {
		if id, _ := carried.Sequence().elements[elem].Object(); id != rowShip.ID {
			t.Errorf("element %d = %s, want the row's ship #%d", i, FormatValue(carried.Sequence().elements[elem]), rowShip.ID)
		}
	}

	weigh := eval("ship.weigh")
	if weigh.Kind != ValFunction || weigh.FunctionSelf() == nil || weigh.FunctionSelf().ID != heldShip {
		t.Fatalf("ship.weigh = %s with self %v, want a function of the held ship", FormatValue(weigh), weigh.FunctionSelf())
	}
	fn, err := row.Carry(weigh, bring)
	if err != nil {
		t.Fatalf("Carry(ship.weigh): %v", err)
	}
	if fn.FunctionSelf() != rowShip {
		t.Fatalf("the carried function's self is %v, want the row's ship", fn.FunctionSelf())
	}
	if err := rowShip.SetFeatureValue(row, "cost", eval("10.0")); err != nil {
		t.Fatal(err)
	}
	f := fn.function()
	result, err := row.invokeCalcShapeIn(f.shape, calcArgs{positional: []Value{eval("3.0")}}, f.scope, f.self, f.enclosing)
	if err != nil {
		t.Fatalf("applying the carried function: %v", err)
	}
	if got := FormatValue(result); got != "30.0" {
		t.Errorf("weigh(3.0) in the row = %s, want 30.0, the row's ship weighed", got)
	}
	if got := FormatValue(eval("ship.cost")); got != "5.0" {
		t.Errorf("the held ship's cost is %s, want 5.0 untouched", got)
	}
}

// What bring refuses refuses the carry, and a value closed over the run that made it
// — a function of a calc declared in a behavior body — cannot be carried at all.
func TestCarryRefusesWhatNoOtherContextHolds(t *testing.T) {
	_, row, _, eval := carryContexts(t)
	refused := errors.New("no object stands for it")
	_, err := row.Carry(eval("fleet.flagship"), func(int64) (*Instance, error) { return nil, refused })
	if !errors.Is(err, refused) {
		t.Errorf("Carry(fleet.flagship) = %v, want bring's refusal", err)
	}

	sym := &symbols.Symbol{Name: "inBody"}
	fn := Value{Kind: ValFunction, ref: &functionValue{
		shape:     &calcShape{Sym: sym, Name: "inBody"},
		enclosing: []frame{{vars: map[string]Value{"k": integerValue(1)}, run: 1}},
	}}
	var notPortable *NotPortableError
	_, err = row.Carry(fn, func(int64) (*Instance, error) { t.Fatal("bring called"); return nil, nil })
	if !errors.As(err, &notPortable) || notPortable.Kind != ValFunction {
		t.Errorf("Carry(function over a body) = %v, want a NotPortableError of a function", err)
	}
}
