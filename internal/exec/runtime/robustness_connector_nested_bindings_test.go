package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessConnectorNestedBindings: bindings a connector routes through its
// own features, through connectors nested in its definition, or through the feature
// values of its end bodies, resolve against the standard library's value types — the
// features that hold an object of their own when nothing binds them.
func TestRuntimeRobustnessConnectorNestedBindings(t *testing.T) {
	t.Run("cycle_through_connector_owned_features", testCycleThroughConnectorOwnedFeatures)
	t.Run("chain_through_connector_owned_features_in_declaration_order", testChainThroughConnectorOwnedFeatures)
	t.Run("nested_connector_with_an_unattachable_end", testNestedConnectorWithAnUnattachableEnd)
	t.Run("end_body_value_conflicting_with_the_connected_port", testEndBodyValueConflictingWithThePort)
	t.Run("end_body_default_yields_to_the_connected_port", testEndBodyDefaultYieldsToThePort)
	t.Run("end_body_value_of_an_anonymous_connector_binds_its_ends", testEndBodyValueOfAnAnonymousConnector)
	t.Run("failing_end_body_default_yields_to_an_explicit_binding", testFailingEndBodyDefaultYieldsToAnExplicitBinding)
	t.Run("end_body_default_yields_to_an_outer_binding", testEndBodyDefaultYieldsToAnOuterBinding)
	t.Run("failing_outer_binding_is_not_masked_by_an_end_body_default", testFailingOuterBindingIsNotMaskedByADefault)
	t.Run("end_body_value_of_a_required_connector_collection", testEndBodyValueOfARequiredConnectorCollection)
}

// instantiatePartWithLibraries is instantiatePart over an index carrying the standard library.
func instantiatePartWithLibraries(t *testing.T, name, src string) (*Instance, *Context) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefPart)
	if sym == nil {
		t.Fatalf("part def %s not found", name)
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate %s: %v", name, err)
	}
	return inst, ctx
}

func wantIntegerAt(t *testing.T, ctx *Context, inst *Instance, path string, want int64) {
	t.Helper()
	fv, err := featureValueAt(t, ctx, inst, path)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", path, err)
	}
	if !ctx.equalValues(fv.HeldValue(), integerValue(want)) {
		t.Errorf("%s = %v, want %d", path, fv.HeldValue(), want)
	}
}

// wantOwnObject: the feature holds an object of its own, as a library-typed attribute no
// binding determines does, rather than a value bound through a connector.
func wantOwnObject(t *testing.T, ctx *Context, inst *Instance, path, why string) {
	t.Helper()
	fv, err := featureValueAt(t, ctx, inst, path)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s) = %v, want an object of its own: %s", path, err, why)
	}
	if _, isObject := fv.HeldValue().Object(); !isObject {
		t.Fatalf("%s = %v, want an object of its own: %s", path, fv.HeldValue(), why)
	}
}

const connectorNestedBindingParts = `
	port def Pw { attribute voltage : Integer; }
	part def Src { port src : Pw { attribute :>> voltage = 28; } }
	part def Snk { port dst : ~Pw; }
`

// testCycleThroughConnectorOwnedFeatures: bindings among a connector's own features that
// close a cycle back onto one end bind that end's feature to themselves and nothing to the
// other end; the resolution ends, leaving the unbound port unset.
func testCycleThroughConnectorOwnedFeatures(t *testing.T) {
	inst, ctx := instantiatePartWithLibraries(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorNestedBindingParts+`
			interface def Loop {
				end a : Pw;
				end b : ~Pw;
				attribute s1 : Integer;
				attribute s2 : Integer;
				bind a.voltage = s1;
				bind s1 = s2;
				bind s2 = a.voltage;
			}
			part def Sys {
				part s : Src;
				part k : Snk;
				interface i : Loop connect s.src to k.dst;
			}
		}`)
	wantIntegerAt(t, ctx, inst, "i.s2", 28)
	wantOwnObject(t, ctx, inst, "k.dst.voltage", "a cycle through the connector's features binds nothing to the other end")
}

// testChainThroughConnectorOwnedFeatures: a value crossing two connector features reaches
// the far port whichever side is materialized first, and the connector's features hold it.
func testChainThroughConnectorOwnedFeatures(t *testing.T) {
	for _, order := range [][]string{{"k.dst.voltage", "i.s1", "i.s2"}, {"i.s2", "k.dst.voltage", "i.s1"}, {"s.src.voltage", "i.s1", "i.s2", "k.dst.voltage"}} {
		inst, ctx := instantiatePartWithLibraries(t, "Sys", `
			package test {
				private import ScalarValues::*;`+connectorNestedBindingParts+`
				interface def Chain {
					end a : Pw;
					end b : ~Pw;
					attribute s1 : Integer;
					attribute s2 : Integer;
					bind a.voltage = s1;
					bind s1 = s2;
					bind s2 = b.voltage;
				}
				part def Sys {
					part k : Snk;
					part s : Src;
					interface i : Chain connect s.src to k.dst;
				}
			}`)
		for _, path := range order {
			wantIntegerAt(t, ctx, inst, path, 28)
		}
	}
}

// testNestedConnectorWithAnUnattachableEnd: an anonymous interface a connection definition
// owns binds one of the definition's own parts, which no path from the holder reaches; it
// binds nothing on the holder, and the sibling named interface still does.
func testNestedConnectorWithAnUnattachableEnd(t *testing.T) {
	inst, ctx := instantiatePartWithLibraries(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorNestedBindingParts+`
			interface def L { end a : Pw; end b : ~Pw; bind a.voltage = b.voltage; }
			connection def Wired {
				end a : Src;
				end b : Snk;
				part spare : Snk;
				interface : L connect a.src to spare.dst;
				interface w : L connect a.src to b.dst;
			}
			part def Sys {
				part s : Src;
				part k : Snk;
				part other : Snk;
				connection c : Wired connect s to k;
			}
		}`)
	wantIntegerAt(t, ctx, inst, "k.dst.voltage", 28)
	wantOwnObject(t, ctx, inst, "other.dst.voltage", "a part the connection does not relate is not bound")
}

// testEndBodyValueConflictingWithThePort: `attribute :>> voltage = 12` in an end's body and
// `= 5` on the connected port state two values for one feature: a binding conflict.
func testEndBodyValueConflictingWithThePort(t *testing.T) {
	inst, ctx := instantiatePartWithLibraries(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorNestedBindingParts+`
			interface def L { end a : Pw; end b : ~Pw { attribute :>> voltage = 12; } }
			part def Sys {
				part s : Src;
				part k : Snk { port :>> dst { attribute :>> voltage = 5; } }
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
	if conflict.Left != "k.dst.voltage" {
		t.Errorf("conflict names %q and %q, want the connected feature k.dst.voltage against the end's value", conflict.Left, conflict.Right)
	}
}

// testEndBodyDefaultYieldsToThePort: a `default =` in an end's body applies to a port with
// no value of its own and yields to one that has, or to another binding's value.
func testEndBodyDefaultYieldsToThePort(t *testing.T) {
	inst, ctx := instantiatePartWithLibraries(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorNestedBindingParts+`
			interface def L { end a : Pw; end b : ~Pw { attribute :>> voltage default = 12; } }
			part def Sys {
				part s : Src;
				part k : Snk;
				part valued : Snk { port :>> dst { attribute :>> voltage = 5; } }
				part bound : Snk;
				interface i : L connect s.src to k.dst;
				interface j : L connect s.src to valued.dst;
				interface n : L connect s.src to bound.dst;
				bind bound.dst.voltage = s.src.voltage;
			}
		}`)
	wantIntegerAt(t, ctx, inst, "k.dst.voltage", 12)
	wantIntegerAt(t, ctx, inst, "valued.dst.voltage", 5)
	wantIntegerAt(t, ctx, inst, "bound.dst.voltage", 28)
}

// testEndBodyValueOfAnAnonymousConnector: an anonymous connector's end-body value is read on
// the connector object the holder materialized for it, and binds the connected port.
func testEndBodyValueOfAnAnonymousConnector(t *testing.T) {
	inst, ctx := instantiatePartWithLibraries(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorNestedBindingParts+`
			interface def L { end a : Pw; end b : ~Pw { attribute :>> voltage = 12; } }
			part def Sys {
				part s : Src;
				part k : Snk;
				interface : L connect s.src to k.dst;
			}
		}`)
	wantIntegerAt(t, ctx, inst, "k.dst.voltage", 12)
}

// testFailingEndBodyDefaultYieldsToAnExplicitBinding: a `default =` whose expression fails is
// never evaluated against a port another binding values; it fails only where it applies.
func testFailingEndBodyDefaultYieldsToAnExplicitBinding(t *testing.T) {
	inst, ctx := instantiatePartWithLibraries(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorNestedBindingParts+`
			interface def L {
				end a : Pw;
				end b : ~Pw { attribute :>> voltage default = 1 / divisor; }
				attribute divisor : Integer = 0;
			}
			part def Sys {
				part s : Src;
				part k : Snk;
				part lone : Snk;
				interface i : L connect s.src to k.dst;
				interface j : L connect s.src to lone.dst;
				bind k.dst.voltage = s.src.voltage;
			}
		}`)
	wantIntegerAt(t, ctx, inst, "k.dst.voltage", 28)
	if _, err := featureValueAt(t, ctx, inst, "lone.dst.voltage"); err == nil {
		t.Errorf("GetFeatureValue(lone.dst.voltage) = nil error, want the failing default's error where it applies")
	}
}

// testEndBodyValueOfARequiredConnectorCollection: a connector declared `[2]` holds two objects,
// each stating the end-body value; the value they agree on binds the port.
func testEndBodyValueOfARequiredConnectorCollection(t *testing.T) {
	inst, ctx := instantiatePartWithLibraries(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorNestedBindingParts+`
			interface def L { end a : Pw; end b : ~Pw { attribute :>> voltage = 12; } }
			part def Sys {
				part s : Src;
				part k : Snk;
				interface i : L [2] connect s.src to k.dst;
			}
		}`)
	wantIntegerAt(t, ctx, inst, "k.dst.voltage", 12)
}

// testEndBodyDefaultYieldsToAnOuterBinding: a binding an outer owner states displaces an inner
// end body's `default =`, failing or not, and the failing one is still reported where it applies.
func testEndBodyDefaultYieldsToAnOuterBinding(t *testing.T) {
	inst, ctx := instantiatePartWithLibraries(t, "Sys", `
		package test {
			private import ScalarValues::*;`+connectorNestedBindingParts+`
			interface def Fails { end a : Pw; end b : ~Pw { attribute :>> voltage default = 1 / divisor; } attribute divisor : Integer = 0; }
			interface def Ok { end a : Pw; end b : ~Pw { attribute :>> voltage default = 12; } }
			part def Inner { port inp : Pw; part k : Snk; interface i : Fails connect inp to k.dst; }
			part def InnerOk { port inp : Pw; part k : Snk; interface i : Ok connect inp to k.dst; }
			part def Sys {
				part s : Src;
				part inner : Inner;
				part lone : Inner;
				part innerOk : InnerOk;
				part loneOk : InnerOk;
				bind inner.k.dst.voltage = s.src.voltage;
				bind innerOk.k.dst.voltage = s.src.voltage;
			}
		}`)
	wantIntegerAt(t, ctx, inst, "inner.k.dst.voltage", 28)
	wantIntegerAt(t, ctx, inst, "innerOk.k.dst.voltage", 28)
	wantIntegerAt(t, ctx, inst, "loneOk.k.dst.voltage", 12)
	if _, err := featureValueAt(t, ctx, inst, "lone.k.dst.voltage"); !errors.Is(err, errBindingEnd) {
		t.Errorf("GetFeatureValue(lone.k.dst.voltage) error = %v, want the failing default's errBindingEnd where it applies", err)
	}
}

// testFailingOuterBindingIsNotMaskedByADefault: an outer binding's own failure is reported,
// not replaced by the inner `default =` it would have displaced.
func testFailingOuterBindingIsNotMaskedByADefault(t *testing.T) {
	inst, ctx := instantiatePartWithLibraries(t, "Sys", `
		package test {
			private import ScalarValues::*;
			port def Pr { attribute voltage : Real; }
			part def Snk { port dst : ~Pr; }
			interface def Ok { end a : Pr; end b : ~Pr { attribute :>> voltage default = 12.0; } }
			part def InnerOk { port inp : Pr; part k : Snk; interface i : Ok connect inp to k.dst; }
			part def Sys {
				attribute zero : Integer = 0;
				attribute bad : Real = 1 / zero;
				part innerOk : InnerOk;
				bind innerOk.k.dst.voltage = bad;
			}
		}`)
	if fv, err := featureValueAt(t, ctx, inst, "innerOk.k.dst.voltage"); err == nil {
		t.Errorf("GetFeatureValue(innerOk.k.dst.voltage) = %v, nil error, want the outer binding's failure", fv.Value)
	}
}
