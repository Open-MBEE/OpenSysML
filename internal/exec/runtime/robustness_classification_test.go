package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessClassification covers type classification and casts: unresolved types,
// undecided casts, enumerations and difference-typed features.
func TestRuntimeRobustnessClassification(t *testing.T) {
	t.Run("type_classification_unresolved_type", testTypeClassificationUnresolvedType)
	t.Run("type_classification_undetermined_value_type", testTypeClassificationUndeterminedValueType)
	t.Run("cast_to_an_unresolved_type", testCastToAnUnresolvedType)
	t.Run("cast_undecided_by_the_value", testCastUndecidedByTheValue)
	t.Run("enumeration_typed_feature_holding_an_unenumerated_value", testEnumerationTypedFeatureHoldingAnUnenumeratedValue)
	t.Run("enumeration_whose_literal_value_cannot_be_evaluated", testEnumerationWhoseLiteralValueCannotBeEvaluated)
	t.Run("cast_of_a_quantity_to_a_constrained_subtype", testCastOfAQuantityToAConstrainedSubtype)
	t.Run("difference_typed_feature_holding_a_subtracted_object", testDifferenceTypedFeatureHoldingASubtractedObject)
	t.Run("enumeration_name_that_is_not_a_literal", testEnumerationNameThatIsNotALiteral)
	t.Run("chain_through_a_literal_without_that_attribute", testChainThroughALiteralWithoutThatAttribute)
	t.Run("classification_outside_the_evaluable_subset", testClassificationOutsideTheEvaluableSubset)
	t.Run("meta_cast_failure_modes", testMetaCastFailureModes)
}

// testClassificationOutsideTheEvaluableSubset: a classification the evaluator
// cannot judge — no subject to classify, a subject that is a datum, an
// unresolved metadata type, or a subject naming nothing — reports
// ErrFilterUnevaluable rather than silently answering false.
func testClassificationOutsideTheEvaluableSubset(t *testing.T) {
	const model = `
		metadata def Safety;
		#Safety part def Belt;
		attribute level = 3;
	`
	for _, tc := range []struct{ name, cond string }{
		{"implicit subject outside an object", "@Safety"},
		{"self outside an object", "self @ Safety"},
		{"a datum subject", "42 @ Safety"},
		{"a string subject", `"belt" @ Safety`},
		{"an unresolved metadata type", "Belt @ Nonexistent"},
	} {
		src := model + "\nconstraint c { " + tc.cond + " }"
		got, err := constraintVerdict(t, src, "c")
		if got {
			t.Errorf("%s: `%s` was satisfied, want a report", tc.name, tc.cond)
		}
		if !errors.Is(err, semantics.ErrFilterUnevaluable) {
			t.Errorf("%s: `%s` err = %v, want ErrFilterUnevaluable", tc.name, tc.cond, err)
		}
	}
	// A subject naming nothing is the unresolved reference it is, not a verdict.
	if got, err := constraintVerdict(t, model+"\nconstraint c { Missing @ Safety }", "c"); got || err == nil {
		t.Errorf("`Missing @ Safety` = %v err=%v, want a report", got, err)
	}
}

// testMetaCastFailureModes: `x meta T` reflects on the element x names, so a
// datum, an unresolved type or a feature the metaclass lacks or does not derive
// each report a typed error naming what is wrong, never a guessed metaobject.
// A member read through a cast that matched nothing is the empty sequence
// every feature chain over `()` is. `Comment::body` of a model never given its
// notation (no SetSourceText) is underived, not an empty string.
func testMetaCastFailureModes(t *testing.T) {
	const model = `
	package test {
		part def Vehicle {
			doc /* Carries. */
		}
		part seatBelt : Vehicle;
	}`
	_, got, err := evalDeclaredExpr(t, model, "(test::seatBelt meta SysML::PartDefinition).declaredName")
	if err != nil || got.Kind != ValSequence || got.Sequence().Size() != 0 {
		t.Errorf("member of an empty cast = %s, %v; want ()", FormatValue(got), err)
	}
	for _, tc := range []struct {
		name, expr, names string
		want              error
	}{
		{"a datum subject", "42 meta KerML::Feature", "element", semantics.ErrFilterUnevaluable},
		{"a string subject", `"belt" meta KerML::Feature`, "element", semantics.ErrFilterUnevaluable},
		{"an unresolved subject", "test::nope meta KerML::Feature", "nope", ErrUnresolvedReference},
		{"an unresolved type", "test::seatBelt meta KerML::Nonexistent", "KerML::Nonexistent", errUnresolvedType},
		{"a feature the metaclass lacks", "(test::seatBelt meta KerML::Feature).wheels", "wheels", ErrNoSuchFeature},
		{"a feature the runtime does not derive", "(test::seatBelt meta KerML::Feature).ownedRelationship", "ownedRelationship", ErrReflectiveFeatureUnsupported},
		{"a documentation body without the notation", "(test::Vehicle meta KerML::Element).documentation.body", "body", ErrReflectiveFeatureUnsupported},
	} {
		_, got, err := evalDeclaredExpr(t, model, tc.expr)
		if err == nil {
			t.Errorf("%s: `%s` = %s, want a typed error", tc.name, tc.expr, FormatValue(got))
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: `%s` err = %v, want %v", tc.name, tc.expr, err, tc.want)
		}
		if !strings.Contains(err.Error(), tc.names) {
			t.Errorf("%s: `%s` err = %q, want it to name %q", tc.name, tc.expr, err, tc.names)
		}
	}
}

func testTypeClassificationUnresolvedType(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		item def Integer;
		calc classify { return : Boolean = 1 istype MissingType; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	calc := resolveSymbol(t, pkg.Scope, "classify")
	_, err := NewContext(typedModel(model, resolver), 1000).InvokeCalc(calc, nil, pkg.Scope)
	if err == nil {
		t.Fatal("expected unresolved type classification to fail")
	}
	if !errors.Is(err, errUnresolvedType) {
		t.Fatalf("expected errUnresolvedType, got: %v", err)
	}
	if !strings.Contains(err.Error(), "MissingType") {
		t.Errorf("error = %v, want unresolved type name", err)
	}
}

// A value whose type the model cannot name is a typed error; `null` is the empty
// sequence (KerML 8.3.4.8.16), of every type, so it is not that value.
func testTypeClassificationUndeterminedValueType(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		item def Integer;
		calc classify { return : Boolean = 1.5 istype Integer; }
		calc empty { return : Boolean = null istype Integer; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	calc := resolveSymbol(t, pkg.Scope, "classify")
	_, err := NewContext(typedModel(model, resolver), 1000).InvokeCalc(calc, nil, pkg.Scope)
	if err == nil {
		t.Fatal("expected undetermined value type classification to fail")
	}
	if !errors.Is(err, errUndeterminedValueType) {
		t.Fatalf("expected errUndeterminedValueType, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Rational") {
		t.Errorf("error = %v, want the type the model has no name for", err)
	}
	empty := resolveSymbol(t, pkg.Scope, "empty")
	got, err := invokeValue(NewContext(typedModel(model, resolver), 1000), empty, nil, pkg.Scope)
	if err != nil {
		t.Fatalf("null istype Integer: %v", err)
	}
	if FormatValue(got) != "true" {
		t.Errorf("null istype Integer = %s, want true", FormatValue(got))
	}
}

func testCastToAnUnresolvedType(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		item def Integer;
		calc narrow { return : Integer = 1 as MissingType; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	calc := resolveSymbol(t, pkg.Scope, "narrow")
	_, err := NewContext(typedModel(model, resolver), 1000).InvokeCalc(calc, nil, pkg.Scope)
	if err == nil {
		t.Fatal("expected a cast to an unresolved type to fail")
	}
	if !errors.Is(err, errUnresolvedType) {
		t.Fatalf("expected errUnresolvedType, got: %v", err)
	}
	if !strings.Contains(err.Error(), "MissingType") {
		t.Errorf("error = %v, want unresolved type name", err)
	}
}

// testCastUndecidedByTheValue: a target narrower than the value's own type that
// the value does not settle — 5 states nothing about being an Even — fails
// rather than dropping a value that may well be one of the target's.
func testCastUndecidedByTheValue(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		attribute def Integer;
		attribute def Even :> Integer;
		calc narrow { return : Even = 5 as Even; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	calc := resolveSymbol(t, pkg.Scope, "narrow")
	_, err := NewContext(typedModel(model, resolver), 1000).InvokeCalc(calc, nil, pkg.Scope)
	if err == nil {
		t.Fatal("expected an undecidable cast to fail")
	}
	if !errors.Is(err, errUndecidedClassification) {
		t.Fatalf("expected errUndecidedClassification, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Even") {
		t.Errorf("error = %v, want the target type named", err)
	}
}

// testEnumerationTypedFeatureHoldingAnUnenumeratedValue: an enumeration-typed feature
// refuses a value equal to no enumerated one by the write-conformance rule, decidedly.
func testEnumerationTypedFeatureHoldingAnUnenumeratedValue(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		attribute def Integer;
		enum def Level :> Integer { low = 1; high = 3; }
		attribute two : Integer = 2;
		part def Dial { attribute setting : Level = two; }
		part dial : Dial;
		calc narrow { return : Level[0..1] = two as Level; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	ctx := NewContext(typedModel(model, resolver), 1000)
	dial, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "dial"))
	if err == nil {
		_, err = dial.GetFeatureValue(ctx, "setting")
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("expected ErrTypeMismatch, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Level") {
		t.Errorf("error = %v, want the feature's type named", err)
	}
	result, err := ctx.InvokeCalc(resolveSymbol(t, pkg.Scope, "narrow"), nil, pkg.Scope)
	if err != nil {
		t.Fatalf("2 as Level: %v, want the empty sequence", err)
	}
	if got := FormatValue(result); got != "[]" {
		t.Errorf("2 as Level = %s, want the empty sequence", got)
	}
}

// testEnumerationWhoseLiteralValueCannotBeEvaluated: a literal whose value fails to
// evaluate fails the classification with that error rather than answering false.
func testEnumerationWhoseLiteralValueCannotBeEvaluated(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		attribute def Integer;
		enum def Level :> Integer { low = 1; high = 1 / 0; }
		attribute isLevel = 2 istype Level;
	}`)
	pkg := resolveSymbol(t, root, "P")
	sym := resolveSymbol(t, pkg.Scope, "isLevel")
	_, err := NewContext(typedModel(model, resolver), 1000).EvalWithScope(sym.Decl.(*ast.Usage).Value, pkg.Scope)
	if err == nil {
		t.Fatal("expected the literal's failing value to fail the classification")
	}
	if !strings.Contains(err.Error(), "high") {
		t.Errorf("error = %v, want the failing literal named", err)
	}
}

// testDifferenceTypedFeatureHoldingASubtractedObject: a feature typed by a
// difference refuses an object one of the subtracted types classifies, whether
// the object was declared by it or classified by it since.
func testDifferenceTypedFeatureHoldingASubtractedObject(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		part def Vehicle;
		part def Car :> Vehicle;
		part def Electric;
		part def ElectricCar :> Car, Electric;
		part def CombustionVehicle differences Vehicle, Electric;
		part sedan : Car;
		part def Shop { part retrofit : ElectricCar = sedan; }
		part shop : Shop;
		part def Depot { part burner : CombustionVehicle = shop.retrofit; }
		part depot : Depot;
		attribute held = depot.burner istype Vehicle;
	`)
	sym := resolveSymbol(t, root, "held")
	_, err := NewContext(typedModel(model, resolver), 10000).Eval(sym.Decl.(*ast.Usage).Value)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("expected ErrTypeMismatch, got: %v", err)
	}
	if !strings.Contains(err.Error(), "CombustionVehicle") {
		t.Errorf("error = %v, want the feature's type named", err)
	}
}

// testCastOfAQuantityToAConstrainedSubtype: a quantity subtype inheriting its
// measurement reference narrows lengths by something a magnitude and a unit do
// not state, so a bare length is undecided rather than kept by its dimension.
func testCastOfAQuantityToAConstrainedSubtype(t *testing.T) {
	err := calcErrorWithLibraries(t, `
		package test {
			private import SI::*;
			attribute def RoomLength :> ISQBase::LengthValue;
			calc narrow { return : RoomLength = 5 [m] as RoomLength; }
		}`, "narrow", nil, 1000)
	if !errors.Is(err, errUndecidedClassification) {
		t.Fatalf("expected errUndecidedClassification, got: %v", err)
	}
	if !strings.Contains(err.Error(), "RoomLength") {
		t.Errorf("error = %v, want the target type named", err)
	}
}

// testEnumerationNameThatIsNotALiteral: a name qualified by an enumeration
// designates one of its literals, so one it does not declare is reported with
// the literals it does, never answered as an empty value.
func testEnumerationNameThatIsNotALiteral(t *testing.T) {
	src := `
	package test {
		enum def Color { red; green; blue; }
		part def Car { attribute c : Color = Color::purple; }
	}`
	got, err := variationFeatureValueInSource(t, src, "test::Car", "c")
	if !errors.Is(err, errNotALiteral) {
		t.Fatalf("c = (%v, %v), want errNotALiteral", got, err)
	}
	for _, name := range []string{"purple", "red", "green", "blue"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

// testChainThroughALiteralWithoutThatAttribute: a literal carries only the
// features it declares, so reading another one off it is reported rather than
// materializing an empty feature value.
func testChainThroughALiteralWithoutThatAttribute(t *testing.T) {
	src := `
	package test {
		enum def Level { low { attribute n = 1; } high { attribute n = 9; } }
		part def Sensor { attribute missing = Level::low.label; }
	}`
	got, err := variationFeatureValueInSource(t, src, "test::Sensor", "missing")
	if err == nil {
		t.Fatalf("missing = %v, want an error naming the unknown member", got)
	}
	if !strings.Contains(err.Error(), "label") {
		t.Errorf("error %q does not name the unknown member", err)
	}
}
