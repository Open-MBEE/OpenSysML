package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"strings"
	"testing"
)

// partChainModel declares a calc usage nested in a part, whose outputs a feature
// chain through the part reads: `lander.mass.mDry`.
const partChainModel = `
package test {
	private import ScalarValues::*;
	calc def MassEstimate {
		in mPropUsable : Real;
		out mProp = mPropUsable * 1.02;
		out mDry = 100.0 + mPropUsable * 0.4;
		out mWet = mDry + mProp;
	}
	part lander {
		calc mass : MassEstimate { in mPropUsable = 250.0; }
		attribute dryMass : Real = mass.mDry;
		part tank {
			attribute volume : Real = 3.0;
		}
	}
	attribute throughPart : Real = lander.mass.mDry;
	attribute throughAttribute : Real = lander.dryMass;
	attribute throughNestedPart : Real = lander.tank.volume;
}
`

// evalNamedAttribute evaluates the value binding of the named attribute of
// package test in src.
func evalNamedAttribute(t *testing.T, src, name string) (Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg == nil || pkg.Scope == nil {
		t.Fatal("package test not found")
	}
	sym, ok := pkg.Scope.LookupLocal(name)
	if !ok || sym == nil {
		t.Fatalf("attribute %s not found", name)
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Fatalf("attribute %s binds no value", name)
	}
	return ctx.EvalWithScope(usage.Value, sym.OwnerScope)
}

// TestCalcUsageReadThroughPartChain requires a feature chain through a part to
// read the outputs of a calc usage the part declares, in that part's context.
func TestCalcUsageReadThroughPartChain(t *testing.T) {
	for _, tt := range []struct {
		name string
		want float64
	}{
		{"throughPart", 200.0},
		{"throughAttribute", 200.0},
		{"throughNestedPart", 3.0},
	} {
		got, err := evalNamedAttribute(t, partChainModel, tt.name)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got.Const.Real != tt.want {
			t.Errorf("%s = %s, want %v", tt.name, FormatTraceValue(got), tt.want)
		}
	}
}

// TestPartChainReadsTheObjectInHand requires a chain evaluated against an object
// to read that object's own part, redefinitions included, rather than a separate
// occurrence of the part's declaration.
func TestPartChainReadsTheObjectInHand(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Wheel { attribute radius : Real = 1.0; }
			part def Car {
				part wheel : Wheel { attribute :>> radius = 2.0; }
				attribute r : Real = wheel.radius;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	car := findSymbolByName(idx.DocumentRoot("<test>"), "Car", ast.DefPart)
	if car == nil {
		t.Fatal("part def Car not found")
	}
	inst, err := ctx.Instantiate(car)
	if err != nil {
		t.Fatalf("instantiate Car: %v", err)
	}
	wheel, err := inst.GetFeatureValue(ctx, "wheel")
	if err != nil {
		t.Fatalf("Car.wheel: %v", err)
	}
	this, ok := ctx.instances[wheel.Value.Instance]
	if !ok {
		t.Fatalf("Car.wheel = %s, want the object of this car's wheel", FormatTraceValue(wheel.Value))
	}
	if _, err := this.GetFeatureValue(ctx, "radius"); err != nil {
		t.Fatalf("Car.wheel.radius: %v", err)
	}
	this.FeatureValues["radius"].Value = constReal(5.0)

	fv, err := inst.GetFeatureValue(ctx, "r")
	if err != nil {
		t.Fatalf("Car.r: %v", err)
	}
	if fv.Value.Const.Real != 5.0 {
		t.Errorf("Car.r = %s, want 5 (the radius this car's wheel carries)", FormatTraceValue(fv.Value))
	}
}

// TestPartChainThroughSeveralOccurrences requires a chain through a namespace usage of
// several occurrences to read every object it denotes, once each: the four wheels a
// package declares are the run's, so their radii are four values, not one.
func TestPartChainThroughSeveralOccurrences(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Wheel { attribute radius : Real = 1.0; }
			part wheels : Wheel[4];
			attribute probe : Real = wheels.radius;
		}
	`
	got, err := evalNamedAttribute(t, src, "probe")
	if err != nil {
		t.Fatalf("wheels.radius: %v", err)
	}
	if got.Kind != ValSequence || got.Sequence().Size() != 4 {
		t.Fatalf("wheels.radius = %s, want the radius of each of the four wheels", FormatValue(got))
	}
	for i, radius := range got.Sequence().Elements() {
		if radius.Kind != ValConst || radius.Const.Real != 1.0 {
			t.Errorf("wheels.radius #%d = %s, want 1.0", i+1, FormatValue(radius))
		}
	}
}

// TestPartChainRejectsSeveralNestedOccurrences requires a chain through a usage of
// several occurrences nested in a definition, of which no object stands, to be left
// undetermined, of their count, rather than answered from one of them.
func TestPartChainRejectsSeveralNestedOccurrences(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Wheel { attribute radius : Real = 1.0; }
			part def Car {
				part wheels : Wheel[4];
				attribute probe : Real = wheels.radius;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	cars := idx.LookupQualified("test::Car")
	if len(cars) != 1 {
		t.Fatalf("test::Car: %d matching symbols, want 1", len(cars))
	}
	probe := resolveSymbol(t, cars[0].Scope, "probe")
	got, err := ctx.EvalWithScope(probe.Decl.(*ast.Usage).Value, probe.OwnerScope)
	if err != nil {
		t.Fatalf("wheels.radius: %v", err)
	}
	if got.Kind != ValUndetermined {
		t.Fatalf("wheels.radius = %s, want a chain through four occurrences left undetermined", FormatValue(got))
	}
	if n, ok := got.Undetermined().Count().Exactly(); !ok || n != 4 {
		t.Errorf("wheels.radius counts %s values, want exactly 4", got.Undetermined().Count().Text())
	}
}

// TestCalcUsageChainDiagnostics requires the chain to name what went wrong: a
// usage read without naming an output, and a name it declares no output for.
func TestCalcUsageChainDiagnostics(t *testing.T) {
	tests := []struct {
		expr string
		want []string
	}{
		{"lander.mass", []string{"mDry", "mWet", "read one of them"}},
		{"lander.mass.nope", []string{"nope", "mDry"}},
		{"lander.nope", []string{"nope"}},
	}
	for _, tt := range tests {
		src := strings.Replace(
			partChainModel,
			"attribute throughPart : Real = lander.mass.mDry;",
			"attribute probe : Real = "+tt.expr+";",
			1,
		)
		_, err := evalNamedAttribute(t, src, "probe")
		if err == nil {
			t.Errorf("%s: want a diagnostic, got a value", tt.expr)
			continue
		}
		for _, want := range tt.want {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error = %v, want it to mention %q", tt.expr, err, want)
			}
		}
	}
}
