package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessExpressionReads covers expression reads that cannot produce a value:
// destroyed objects, unbounded values, metadata and chains through scalars.
func TestRuntimeRobustnessExpressionReads(t *testing.T) {
	t.Run("usage_read_through_a_part_without_an_output", testUsageReadThroughAPartWithoutAnOutput)
	t.Run("operation_of_a_destroyed_object", testOperationOfADestroyedObject)
	t.Run("structured_attribute_chain_of_an_unknown_feature", testStructuredAttributeChainOfAnUnknownFeature)
	t.Run("elements_chain_of_a_non_numeric_collection", testElementsChainOfANonNumericCollection)
	t.Run("arithmetic_over_the_unbounded_value", testArithmeticOverTheUnboundedValue)
	t.Run("unbounded_value_compared_with_a_string", testUnboundedValueComparedWithAString)
	t.Run("metadata_of_a_value", testMetadataOfAValue)
	t.Run("metadata_of_an_unresolved_name", testMetadataOfAnUnresolvedName)
	t.Run("metadata_without_the_reflective_library", testMetadataWithoutTheReflectiveLibrary)
	t.Run("two_valued_member_in_scalar_context", testTwoValuedMemberInScalarContext)
	t.Run("expression_over_a_feature_value_holding_no_value", testExpressionOverAFeatureValueHoldingNoValue)
}

// testExpressionOverAFeatureValueHoldingNoValue: a valueless feature of a value type is
// read without an error and reports that it holds no value, while an expression
// computing over it reports which feature holds none rather than a number or a panic.
func testExpressionOverAFeatureValueHoldingNoValue(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Holder {
				attribute d : Real;
				attribute n : Real = d + 1.0;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
	if sym == nil {
		t.Fatal("Holder part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	fv, err := inst.GetFeatureValue(ctx, "d")
	if err != nil {
		t.Fatalf("feature value d: %v", err)
	}
	if !ctx.HoldsNoValue(fv.HeldValue()) {
		t.Errorf("feature value d holds %v, want no value", fv.HeldValue())
	}

	_, err = inst.GetFeatureValue(ctx, "n")
	var noValue *NoValueError
	if !errors.As(err, &noValue) {
		t.Fatalf("feature value n err = %v, want *NoValueError", err)
	}
	if noValue.Feature != "d" || noValue.Symbol == nil || noValue.Symbol.Name != "d" {
		t.Errorf("feature value n reports no value for %q (%v), want feature d", noValue.Feature, noValue.Symbol)
	}

	// A value naming an object the context does not hold answers the question
	// rather than panicking on the lookup.
	if ctx.HoldsNoValue(Value{Kind: ValInstance, Instance: 1 << 30}) {
		t.Error("a value naming no object reads as holding none")
	}
}

// testTwoValuedMemberInScalarContext: a `[0..*]` member holding two values is
// not the one value an operator, a `[1]` parameter or a library function takes;
// each refuses it with a typed error rather than a panic, a hang or a guess.
func testTwoValuedMemberInScalarContext(t *testing.T) {
	for _, tc := range []struct {
		name string
		expr string
		want error
	}{
		{"arithmetic", "q.zs + 1.0", ErrTypeMismatch},
		{"negation", "-q.zs", ErrTypeMismatch},
		{"calc parameter", "Inc(q.zs)", ErrMultiplicityViolation},
		{"library function", "RealFunctions::sqrt(q.zs)", ErrTypeMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `package test {
				private import ScalarValues::*;
				calc def Inc { in x : Real[1]; x + 1.0 }
				part def Holder { attribute zs : Real[0..*]; }
				calc def Two {
					attribute q : Holder = new Holder(zs = (1.0, 2.0));
					return r = ` + tc.expr + `;
				}
			}`
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			pkg := resolveSymbol(t, idx.DocumentRoot("<test>"), "test")
			sym := resolveSymbol(t, pkg.Scope, "Two")

			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("panic: %v", r)
					}
				}()
				_, err := ctx.InvokeCalc(sym, nil, pkg.Scope)
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, tc.want) {
					t.Errorf("InvokeCalc err = %v, want %v", err, tc.want)
				}
			case <-watchdog(5 * time.Second):
				t.Fatal("evaluating the two-valued member did not terminate")
			}
		})
	}
}

// testUsageReadThroughAPartWithoutAnOutput: a chain through a part that stops at
// a calc usage names the outputs to read instead of answering no value.
func testUsageReadThroughAPartWithoutAnOutput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer[1];
				out a = n + 1;
				out b = n * 2;
			}
			part holder {
				calc c : Two { in n = 5; }
			}
			calc def Probe {
				in n : Integer;
				holder.c
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Probe", []Value{constInt(1)}, 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "a, b") {
		t.Errorf("error should name the outputs to read, got: %v", err)
	}
}

// testOperationOfADestroyedObject: an operation invoked on a destroyed object is
// errOccurrenceDestroyed before it runs, even one reading no feature of it.
func testOperationOfADestroyedObject(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, `
		package test {
			private import OccurrenceFunctions::*;
			item def Ping;
			part def Beacon { action ping { first start; action fire { send Ping to tower; } succession first start then fire; } }
			calc def DestroyBeacon { in b : Beacon; return : Beacon = destroy(b); }
		}
	`)
	beacon := instantiate("Beacon")
	if _, err := invoke("DestroyBeacon", objectValue(beacon)); err != nil {
		t.Fatalf("destroy(beacon): %v", err)
	}
	if _, err := ctx.invokeOperation(beacon, "ping", nil); !errors.Is(err, errOccurrenceDestroyed) {
		t.Errorf("invoke ping on a destroyed object = %v; want %v", err, errOccurrenceDestroyed)
	}
	if sent := len(ctx.PendingMessages()); sent != 0 {
		t.Errorf("a destroyed object sent %d message(s); want none", sent)
	}
}

// testStructuredAttributeChainOfAnUnknownFeature: a structured attribute usage
// holds the features of its type, so a chain into one it does not have is
// reported rather than answered as an empty value.
func testStructuredAttributeChainOfAnUnknownFeature(t *testing.T) {
	src := `
		package test {
			attribute def Cost { attribute v : Real default 1.0; }
			attribute template : Cost;
			calc def Probe {
				in n : Integer;
				template.nope
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Probe", []Value{constInt(1)}, 10000)
	if err == nil {
		t.Fatal("expected chaining into an unknown feature to fail")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should name the feature that does not exist, got: %v", err)
	}
}

// testElementsChainOfANonNumericCollection: only a numerical vector is the
// sequence of its own elements, so `elements` of another collection is a member
// lookup on each of them, reported rather than answered with the collection.
func testElementsChainOfANonNumericCollection(t *testing.T) {
	src := `
		package test {
			calc def Probe {
				in n : Integer;
				return : String[*] = ("a", "b").elements;
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Probe", []Value{constInt(1)}, 10000)
	if err == nil {
		t.Fatal("expected chaining elements of a non-numeric collection to fail")
	}
	if !strings.Contains(err.Error(), "cannot chain through non-instance member") {
		t.Errorf("error should report the member lookup, got: %v", err)
	}
}

// testArithmeticOverTheUnboundedValue: `*` is no number, so arithmetic over it
// fails with a typed error naming the operation.
func testArithmeticOverTheUnboundedValue(t *testing.T) {
	for _, expr := range []string{"* + 1", "1 - *", "2 * *", "* / 2", "* % 2", "* ** 2", "-*", "1..*"} {
		_, _, err := evalDeclaredExpr(t, "package test {}", expr)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
}

// testUnboundedValueComparedWithAString: nothing orders `*` against a string.
func testUnboundedValueComparedWithAString(t *testing.T) {
	_, _, err := evalDeclaredExpr(t, "package test {}", `* > "a"`)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testMetadataOfAValue: only an element carries metadata, so reading it from a
// scalar fails rather than answering with the empty sequence.
func testMetadataOfAValue(t *testing.T) {
	for _, expr := range []string{"1.metadata", `"abc".metadata`} {
		_, _, err := evalDeclaredExpr(t, "package test {}", expr)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
}

// testMetadataOfAnUnresolvedName: a name that names no element is refused.
func testMetadataOfAnUnresolvedName(t *testing.T) {
	_, _, err := evalDeclaredExpr(t, "package test {}", "test::missing.metadata")
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference", err)
	}
}

// testMetadataWithoutTheReflectiveLibrary: without the KerML library the reflective
// metaobject has no metaclass, so `.metadata` is refused whole; `@` still answers.
func testMetadataWithoutTheReflectiveLibrary(t *testing.T) {
	const src = `
	package test {
		metadata def Safety { attribute level = 4; }
		part def Vehicle;
		part seatBelt : Vehicle { @Safety; }
		package probe {
			attribute all [*] = test::seatBelt.metadata;
			attribute safe = test::seatBelt @ Safety;
		}
	}`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(typedModel(model, resolver), 10000)
	probe := resolveSymbol(t, root, "test").Scope
	probe = resolveSymbol(t, probe, "probe").Scope
	valueOf := func(name string) (Value, error) {
		decl := resolveSymbol(t, probe, name).Decl.(*ast.Usage)
		return newEvalContext(ctx, probe).Eval(decl.Value)
	}
	for range 2 {
		got, err := valueOf("all")
		if err == nil {
			t.Fatalf("seatBelt.metadata = %s without the library, want ErrNoMetaclass", FormatValue(got))
		}
		if !errors.Is(err, ErrNoMetaclass) || !strings.Contains(err.Error(), "test::seatBelt") {
			t.Fatalf("seatBelt.metadata err = %v, want ErrNoMetaclass naming test::seatBelt", err)
		}
	}
	if n := len(ctx.metadataObjects); n != 0 {
		t.Errorf("%d annotation objects survive the refused read, want none", n)
	}
	if got, err := valueOf("safe"); err != nil || !got.isBool() || !got.Const.Bool {
		t.Errorf("seatBelt @ Safety = %s, %v; want true", FormatValue(got), err)
	}
}
