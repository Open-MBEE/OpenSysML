package runtime

import (
	"errors"
	"strings"
	"testing"
)

// indexedEndAssembly declares an assembly whose connectors are the members
// given: a two-element port and attribute to index into, and an index known
// only at run time.
func indexedEndAssembly(members string) string {
	return `
		package test {
			private import ScalarValues::*;
			port def P { attribute value : Real default 0.0; }
			part def Source { port y : P[2]; }
			part def Sink { port u : P; attribute v : Real; }
			connection def C { end source[1] : P; end target[1] : P; }
			part def Asm {
				part s : Source;
				part k : Sink;
				attribute xs : Real[2] = (1.5, 2.5);
				attribute unset : Real[2];
				attribute third : Integer = 3;
				attribute fraction : Real = 1.5;
				` + members + `
			}
		}`
}

func TestRuntimeRobustnessIndexedConnectorEnd(t *testing.T) {
	t.Run("connection_end_holds_the_selected_element", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Asm", indexedEndAssembly(`
			connection c : C connect [1] s.y#(2) to [1] k.u;`))
		conn := fvInstance(t, ctx, inst, "c")
		if len(conn.Ends) != 2 {
			t.Fatalf("connector has %d ends, want two", len(conn.Ends))
		}
		source := fvInstance(t, ctx, inst, "s")
		y, err := source.GetFeatureValue(ctx, "y")
		if err != nil {
			t.Fatal(err)
		}
		elements, err := ctx.HeldElements(y.HeldValue())
		if err != nil {
			t.Fatal(err)
		}
		if len(elements) != 2 {
			t.Fatalf("s.y holds %d elements, want two", len(elements))
		}
		if end := conn.Ends[0].Value; end.Kind != ValInstance || end.Instance != elements[1].Instance {
			t.Errorf("end source holds %v, want the second element of s.y (%v)", end, elements[1])
		}
		if end := conn.Ends[0].Value; end.Instance == elements[0].Instance {
			t.Errorf("end source holds the first element of s.y; the index selects the second")
		}
		if conn.Ends[1].Value.Instance != fvInstance(t, ctx, inst, "k", "u").ID {
			t.Errorf("end target holds %v, want k.u", conn.Ends[1].Value)
		}
	})

	t.Run("index_out_of_range_at_run_time", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Asm", indexedEndAssembly(`
			connection c : C connect [1] s.y#(third) to [1] k.u;`))
		before := len(ctx.instances)
		_, err := inst.GetFeatureValue(ctx, "c")
		if !errors.Is(err, ErrConnectorEnd) || !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("GetFeatureValue(c) = %v, want a connector end error wrapping ErrIndexOutOfRange", err)
		}
		var endErr *ConnectorEndError
		if !errors.As(err, &endErr) || endErr.End != "s.y#(third)" {
			t.Errorf("error = %+v, want a *ConnectorEndError naming the end as written", err)
		}
		if len(ctx.instances) != before {
			t.Errorf("failed connector materialized %d object(s)", len(ctx.instances)-before)
		}
	})

	t.Run("non_integer_index", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Asm", indexedEndAssembly(`
			connection c : C connect [1] s.y#(fraction) to [1] k.u;`))
		_, err := inst.GetFeatureValue(ctx, "c")
		if !errors.Is(err, ErrConnectorEnd) {
			t.Fatalf("GetFeatureValue(c) = %v, want a connector end error", err)
		}
		var endErr *ConnectorEndError
		if !errors.As(err, &endErr) || endErr.End != "s.y#(fraction)" || endErr.Err == nil {
			t.Errorf("error = %+v, want a *ConnectorEndError naming the end and its cause", err)
		}
	})

	t.Run("index_into_a_feature_holding_no_value", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Asm", indexedEndAssembly(`
			connect unset#(1) to k.v;`))
		conns, err := inst.OwnedConnectors(ctx)
		if err == nil {
			t.Fatalf("OwnedConnectors = %d connector(s), want an error: the end's feature holds no value", len(conns))
		}
		if !errors.Is(err, ErrConnectorEnd) || !errors.Is(err, ErrUninitializedFeatureValue) {
			t.Fatalf("OwnedConnectors = %v, want a connector end error wrapping ErrUninitializedFeatureValue", err)
		}
	})

	t.Run("binding_end_reads_the_selected_element", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Asm", indexedEndAssembly(`
			bind k.v = xs#(2);`))
		sink := fvInstance(t, ctx, inst, "k")
		v, err := sink.GetFeatureValue(ctx, "v")
		if err != nil {
			t.Fatalf("k.v: %v", err)
		}
		if got := v.HeldValue(); got.Kind != ValConst || got.Const.Real != 2.5 {
			t.Errorf("k.v = %v, want 2.5, the second element of xs", got)
		}
	})

	t.Run("binding_index_out_of_range_at_run_time", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Asm", indexedEndAssembly(`
			bind k.v = xs#(third);`))
		sink := fvInstance(t, ctx, inst, "k")
		_, err := sink.GetFeatureValue(ctx, "v")
		if !errors.Is(err, ErrBindingEnd) || !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("k.v = %v, want a binding end error wrapping ErrIndexOutOfRange", err)
		}
	})

	// A feature holding no value has no element to select: the bound feature
	// holds no value either, as it would bound to the whole feature, and
	// nothing fails or panics.
	t.Run("binding_index_into_a_feature_holding_no_value", func(t *testing.T) {
		read := func(members string) (Value, error) {
			inst, ctx := instantiatePart(t, "Asm", indexedEndAssembly(members))
			v, err := fvInstance(t, ctx, inst, "k").GetFeatureValue(ctx, "v")
			if err != nil {
				return Value{}, err
			}
			return v.HeldValue(), nil
		}
		whole, wholeErr := read(`bind k.v = unset;`)
		indexed, indexedErr := read(`bind k.v = unset#(1);`)
		if indexedErr != nil || wholeErr != nil {
			t.Fatalf("k.v = %v (whole feature: %v), want a read that fails on neither", indexedErr, wholeErr)
		}
		if indexed.Kind != ValInvalid || whole.Kind != ValInvalid {
			t.Errorf("k.v = %v (whole feature: %v), want no value held in either case", indexed, whole)
		}
	})
}

// A binding between two selected elements constrains them without determining either
// feature: equal elements pass, unequal ones are a binding conflict found from either
// side, and neither feature takes the other's value through an index.
func TestRuntimeRobustnessIndexedBindingBetweenSelectedElements(t *testing.T) {
	read := func(t *testing.T, members, name string) (Value, error) {
		t.Helper()
		inst, ctx := instantiatePart(t, "Asm", indexedEndAssembly(`
			attribute ys : Real[2] = (2.5, 4.5);`+members))
		v, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			return Value{}, err
		}
		return v.HeldValue(), nil
	}

	t.Run("equal_selected_elements_leave_both_features_their_own_values", func(t *testing.T) {
		for name, want := range map[string][]float64{"xs": {1.5, 2.5}, "ys": {2.5, 4.5}} {
			got, err := read(t, `bind xs#(2) = ys#(1);`, name)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			elements := elementsOf(got)
			if len(elements) != 2 || elements[0].Const.Real != want[0] || elements[1].Const.Real != want[1] {
				t.Errorf("%s = %v, want %v", name, got, want)
			}
		}
	})

	t.Run("unequal_selected_elements_are_a_binding_conflict_from_either_side", func(t *testing.T) {
		for _, name := range []string{"xs", "ys"} {
			_, err := read(t, `bind xs#(1) = ys#(1);`, name)
			if !errors.Is(err, ErrBindingConflict) {
				t.Errorf("%s = %v, want a binding conflict: xs#(1) is 1.5 and ys#(1) is 2.5", name, err)
			}
		}
	})

	t.Run("selected_element_out_of_range_is_a_binding_end_error_from_either_side", func(t *testing.T) {
		for _, name := range []string{"xs", "ys"} {
			_, err := read(t, `bind xs#(third) = ys#(1);`, name)
			if !errors.Is(err, ErrBindingEnd) || !errors.Is(err, ErrIndexOutOfRange) {
				t.Errorf("%s = %v, want a binding end error wrapping ErrIndexOutOfRange", name, err)
			}
		}
	})

	t.Run("stated_link_counts_are_checked_against_the_one_selected_value", func(t *testing.T) {
		for _, name := range []string{"xs", "ys"} {
			_, err := read(t, `binding [2] bind xs#(2) = ys#(1);`, name)
			if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "binding [2]") {
				t.Errorf("%s = %v, want a multiplicity violation naming `binding [2]`: the ends select one value", name, err)
			}
			if _, err := read(t, `binding [1] bind [1] xs#(2) = [1] ys#(1);`, name); err != nil {
				t.Errorf("%s: %v, want the stated one link of one value to pass", name, err)
			}
		}
	})

	t.Run("a_feature_holding_no_value_takes_nothing_through_an_index", func(t *testing.T) {
		got, err := read(t, `bind unset#(1) = ys#(1);`, "unset")
		if err != nil {
			t.Fatalf("unset: %v", err)
		}
		if got.Kind != ValInvalid && len(elementsOf(got)) != 0 {
			t.Errorf("unset = %v, want no value: a binding assigns nothing through an index", got)
		}
	})
}
