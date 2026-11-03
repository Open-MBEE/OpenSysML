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
	t.Run("each_connector_of_one_definition_is_checked", testEachConnectorOfOneDefinitionIsChecked)
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

const sharedDefinitionPartialBindings = `package P {
	item def Node;
	part def Holder { item nodes : Node [2]; }
	connection def Share { end a : Holder; end b : Holder; binding [1] bind [0..1] a.nodes = [0..1] b.nodes; }
	part def Mesh {
		part p : Holder;
		part q : Holder;
		part r : Holder;
		connection toQ : Share connect p to q;
		connection toR : Share connect p to r;
	}
}`

// testEachConnectorOfOneDefinitionIsChecked: a binding one connection def owns binds
// different features through each connection of the definition, and validation
// checks each, not only the first.
func testEachConnectorOfOneDefinitionIsChecked(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<shared-definition-partial-bindings>", parseAndBuild(t, sharedDefinitionPartialBindings))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Mesh"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	report, err := ctx.ValidateObject(inst, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	// The connector objects carry the definition's binding over their own ends;
	// the verdicts about the mesh are the ones over the connected features.
	var onMesh []ObjectVerdict
	for _, v := range report.Verdicts {
		if v.Subject == inst {
			onMesh = append(onMesh, v)
		}
	}
	if len(onMesh) != 2 {
		t.Fatalf("%d verdict(s) on the mesh, want one per connection: %+v", len(onMesh), onMesh)
	}
	for i, want := range []string{"p.nodes and q.nodes share 0 value(s) where the binding links 1", "p.nodes and r.nodes share 0 value(s) where the binding links 1"} {
		v := onMesh[i]
		if v.Kind != AssertionBinding || v.Status != ValidationViolated || !errors.Is(v.Err, ErrViolated) {
			t.Errorf("verdict %d = %s %s %v, want a violated binding", i+1, v.Kind, v.Status, v.Err)
		}
		if v.Err == nil || !strings.Contains(v.Err.Error(), want) {
			t.Errorf("verdict %d error %v does not name %q", i+1, v.Err, want)
		}
	}
	if report.Valid() {
		t.Error("an object violating a connector-owned binding is reported valid")
	}
}
