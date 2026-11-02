package runtime

import (
	"errors"
	"strings"
	"testing"
)

// The bindings an interface or connection owns bind the features its ends attach;
// what they cannot bind is reported as the ordinary binding and connector errors.
func TestRuntimeRobustnessConnectorOwnedBindings(t *testing.T) {
	t.Run("binding_over_an_unconnected_end", testBindingOverAnUnconnectedEnd)
	t.Run("conflicting_values_from_both_sides", testConflictingValuesFromBothSides)
	t.Run("binding_rooted_at_no_end", testBindingRootedAtNoEnd)
	t.Run("absent_connector_binds_nothing", testAbsentConnectorBindsNothing)
}

// featureValueAt reads the feature value a dotted path names from inst, through
// the objects the path's earlier features hold.
func featureValueAt(t *testing.T, ctx *Context, inst *Instance, path string) (*FeatureValue, error) {
	t.Helper()
	segments := strings.Split(path, ".")
	cur := inst
	for _, name := range segments[:len(segments)-1] {
		fv, err := cur.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s) = %v", name, err)
		}
		id, isObject := fv.HeldValue().Object()
		if !isObject {
			t.Fatalf("%s holds %s, want an object", name, fv.HeldValue().Kind)
		}
		next, held := ctx.Instance(id)
		if !held {
			t.Fatalf("%s holds object %d, which the context does not", name, id)
		}
		cur = next
	}
	return cur.GetFeatureValue(ctx, segments[len(segments)-1])
}

// wantUnset asserts that the feature value a dotted path names holds no value
// and reports no error: nothing binds it.
func wantUnset(t *testing.T, ctx *Context, inst *Instance, path, why string) {
	t.Helper()
	fv, err := featureValueAt(t, ctx, inst, path)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s) = %v, want an unset value: %s", path, err, why)
	}
	if got := fv.HeldValue(); got.Kind != ValInvalid {
		t.Fatalf("%s = %v, want unset: %s", path, got, why)
	}
}

const connectorOwnedBindingParts = `
	port def Pw { attribute voltage : Integer; }
	interface def L { end a : Pw; end b : ~Pw; bind a.voltage = b.voltage; }
	part def Src { port src : Pw { attribute :>> voltage = 28; } }
	part def Snk { port dst : ~Pw; }
`

func testBindingOverAnUnconnectedEnd(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorOwnedBindingParts+`
			part def Sys {
				part s : Src;
				part k : Snk;
				interface i : L;
			}
		}`)
	wantUnset(t, ctx, inst, "k.dst.voltage", "an end attached to nothing binds nothing")
}

func testConflictingValuesFromBothSides(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorOwnedBindingParts+`
			part def Sys {
				part s : Src;
				part k : Snk { port :>> dst { attribute :>> voltage = 12; } }
				interface i : L connect s.src to k.dst;
			}
		}`)
	_, err := featureValueAt(t, ctx, inst, "k.dst.voltage")
	if !errors.Is(err, errBindingConflict) {
		t.Fatalf("GetFeatureValue(k.dst.voltage) = %v, want errBindingConflict", err)
	}
	var conflict *bindingConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T, want *bindingConflictError", err)
	}
	if conflict.Left != "s.src.voltage" || conflict.Right != "k.dst.voltage" {
		t.Errorf("conflict names ends %q and %q, want the connected features s.src.voltage and k.dst.voltage", conflict.Left, conflict.Right)
	}
}

func testBindingRootedAtNoEnd(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			private import ScalarValues::*;
			port def Pw { attribute voltage : Integer; }
			interface def Monitored {
				end a : Pw;
				end b : ~Pw;
				attribute reading : Integer;
				bind a.voltage = reading;
			}
			part def Src { port src : Pw { attribute :>> voltage = 28; } }
			part def Snk { port dst : ~Pw; }
			part def Sys {
				part s : Src;
				part k : Snk;
				interface i : Monitored connect s.src to k.dst;
			}
		}`)
	wantUnset(t, ctx, inst, "k.dst.voltage", "a binding to the connector's own feature binds no connected feature")
}

func testAbsentConnectorBindsNothing(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorOwnedBindingParts+`
			part def Sys {
				part s : Src;
				part k : Snk;
				interface i : L [0..*] connect s.src to k.dst;
			}
		}`)
	wantUnset(t, ctx, inst, "k.dst.voltage", "a connector that may not exist binds nothing")
}
