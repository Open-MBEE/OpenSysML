package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const declaredReaderModel = `
package DerivedRepro {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;

	part def Stage {
		attribute dryMass :> ISQ::mass;
		attribute propellantMass :> ISQ::mass;
		attribute mass :> ISQ::mass = dryMass + propellantMass;
		attribute count : Integer default 2;
		attribute doubled : Integer = count * 2;
		attribute heavy : Boolean = mass > 2000000 [kg];
		attribute label : String = if heavy ? "heavy" else "light";
	}
	part def FirstStage :> Stage {
		attribute :>> dryMass default = 130000 [kg];
		attribute :>> propellantMass = 2160000 [kg];
	}
	part def Vehicle {
		part s1 : FirstStage;
		part s2 : FirstStage {
			attribute :>> dryMass = 120000 [kg];
			attribute :>> count = 5;
		}
		attribute liftoffMass :> ISQ::mass = s1.mass + s2.mass;
		attribute stages : Integer = s1.count + s2.count;
	}
	part rocket : Vehicle;

	part def Cyclic {
		attribute a : Real = b;
		attribute b : Real = a;
	}
	part def Unbound {
		attribute dryMass :> ISQ::mass;
		attribute propellantMass :> ISQ::mass = 10 [kg];
		attribute mass :> ISQ::mass = dryMass + propellantMass;
	}
	part def Parametric {
		calc scaled { in factor : Real; return : Real = factor * 2.0; }
		attribute n : Real = scaled();
	}
	part def Behaving {
		attribute started : Boolean = false;
		exhibit state running {
			entry action { assign started := true; }
		}
	}
}
`

func declaredReaderFixture(t *testing.T) (*DeclaredReader, *resolve.Resolver, *symbols.Scope) {
	t.Helper()
	model, resolver, root := parseAndBuildLibraryModel(t, declaredReaderModel)
	pkg, ok := root.LookupLocal("DerivedRepro")
	if !ok || pkg == nil || pkg.Scope == nil {
		t.Fatal("DerivedRepro not found")
	}
	return NewDeclaredReader(model, resolver), resolver, pkg.Scope
}

func symbolAt(t *testing.T, resolver *resolve.Resolver, scope *symbols.Scope, path string) *symbols.Symbol {
	t.Helper()
	qn := &ast.QualifiedName{}
	for _, part := range strings.Split(path, "::") {
		qn.Parts = append(qn.Parts, ast.NameSegment{Text: part})
	}
	sym, ok := resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		t.Fatalf("%s not found", path)
	}
	return sym
}

func quantityText(t *testing.T, val Value) string {
	t.Helper()
	q := val.Quantity()
	if q == nil {
		t.Fatalf("value %s is not a quantity", describeValue(val))
	}
	return q.String()
}

func TestDeclaredReaderEvaluatesDerivedValuesFromTheCarrier(t *testing.T) {
	reader, resolver, scope := declaredReaderFixture(t)
	cases := []struct {
		element, feature, want string
	}{
		// Type-level redefinition: FirstStage's values feed Stage's expression.
		{"FirstStage", "mass", "2290000 [kg]"},
		{"Vehicle::s1", "mass", "2290000 [kg]"},
		// Usage-level redefinition wins over the type's default.
		{"Vehicle::s2", "dryMass", "120000 [kg]"},
		{"Vehicle::s2", "mass", "2280000 [kg]"},
		// Feature chains into owned parts, from the carrier.
		{"Vehicle", "liftoffMass", "4570000 [kg]"},
		{"rocket", "liftoffMass", "4570000 [kg]"},
	}
	for _, tc := range cases {
		val, err := reader.Read(symbolAt(t, resolver, scope, tc.element), tc.feature)
		if err != nil {
			t.Errorf("%s.%s: %v", tc.element, tc.feature, err)
			continue
		}
		if got := quantityText(t, val); got != tc.want {
			t.Errorf("%s.%s = %s, want %s", tc.element, tc.feature, got, tc.want)
		}
	}
}

func TestDeclaredReaderPreservesIntegersAndEvaluatesConditionals(t *testing.T) {
	reader, resolver, scope := declaredReaderFixture(t)
	cases := []struct {
		element, feature string
		want             semantics.Value
	}{
		{"Vehicle::s1", "doubled", semantics.Value{Kind: semantics.ValInt, Int: 4}},
		{"Vehicle::s2", "doubled", semantics.Value{Kind: semantics.ValInt, Int: 10}},
		{"rocket", "stages", semantics.Value{Kind: semantics.ValInt, Int: 7}},
		{"Vehicle::s1", "heavy", semantics.Value{Kind: semantics.ValBool, Bool: true}},
	}
	for _, tc := range cases {
		val, err := reader.Read(symbolAt(t, resolver, scope, tc.element), tc.feature)
		if err != nil {
			t.Errorf("%s.%s: %v", tc.element, tc.feature, err)
			continue
		}
		if val.Kind != ValConst || val.Const != tc.want {
			t.Errorf("%s.%s = %s, want %v", tc.element, tc.feature, describeValue(val), tc.want)
		}
	}
	val, err := reader.Read(symbolAt(t, resolver, scope, "Vehicle::s1"), "label")
	if err != nil {
		t.Fatalf("label: %v", err)
	}
	if val.Kind != ValString || val.Str() != "heavy" {
		t.Errorf("label = %s, want \"heavy\"", describeValue(val))
	}
}

func TestDeclaredReaderReportsAnUnboundLeafAsNoValue(t *testing.T) {
	reader, resolver, scope := declaredReaderFixture(t)
	unbound := symbolAt(t, resolver, scope, "Unbound")

	_, err := reader.Read(unbound, "dryMass")
	var noValue *NoValueError
	if !errors.As(err, &noValue) || noValue.Symbol == nil || noValue.Symbol.Name != "dryMass" {
		t.Fatalf("Unbound.dryMass err = %v, want NoValueError naming dryMass", err)
	}

	_, err = reader.Read(unbound, "mass")
	noValue = nil
	if !errors.As(err, &noValue) || noValue.Symbol == nil || noValue.Symbol.Name != "dryMass" {
		t.Fatalf("Unbound.mass err = %v, want NoValueError naming the unbound leaf dryMass", err)
	}
}

func TestDeclaredReaderReportsCyclesAndNonConstantValues(t *testing.T) {
	reader, resolver, scope := declaredReaderFixture(t)

	_, err := reader.Read(symbolAt(t, resolver, scope, "Cyclic"), "a")
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Errorf("Cyclic.a err = %v, want ErrCyclicFeatureValue", err)
	}

	_, err = reader.Read(symbolAt(t, resolver, scope, "Parametric"), "n")
	if err == nil {
		t.Error("Parametric.n evaluated, want an error for the unbound parameter")
	}
	var noValue *NoValueError
	if errors.As(err, &noValue) {
		t.Errorf("Parametric.n err = %v, want an error other than NoValueError", err)
	}
}

func TestDeclaredReaderRunsNoBehavior(t *testing.T) {
	reader, resolver, scope := declaredReaderFixture(t)
	val, err := reader.Read(symbolAt(t, resolver, scope, "Behaving"), "started")
	if err != nil {
		t.Fatalf("Behaving.started: %v", err)
	}
	if val.Kind != ValConst || val.Const.Kind != semantics.ValBool || val.Const.Bool {
		t.Errorf("Behaving.started = %s, want the declared false", describeValue(val))
	}
}

func TestDeclaredReaderAgreesWithTheRun(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, declaredReaderModel)
	pkg, _ := root.LookupLocal("DerivedRepro")
	rocket := symbolAt(t, resolver, pkg.Scope, "rocket")

	ctx := NewContext(NewModel(model, resolver), DefaultMaxSteps)
	inst, err := ctx.Instantiate(rocket)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "liftoffMass")
	if err != nil {
		t.Fatalf("run liftoffMass: %v", err)
	}
	ran := quantityText(t, fv.HeldValue())

	val, err := NewDeclaredReader(model, resolver).Read(rocket, "liftoffMass")
	if err != nil {
		t.Fatalf("declared liftoffMass: %v", err)
	}
	if declared := quantityText(t, val); declared != ran || ran != "4570000 [kg]" {
		t.Errorf("declared %s, run %s, want both 4570000 [kg]", declared, ran)
	}
}
