package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A subrequirement's object answers what its container's check counts: the container's `result` is its
// check's verdict, and it is true exactly when every value of `constraints` is, under true assumptions.
func TestSubrequirementObjectReadsAgreeWithContainerCheck(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "conformance", "subrequirement_object_reads.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, idx := libraryShapeContext(t, string(src))
	root := idx.DocumentRoot("<test>")
	box := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::box"))

	for _, usage := range []string{"rr", "rrOk", "unnamed"} {
		sym := requirementUsage(t, idx, "test::box::"+usage)
		result, err := ctx.CheckRequirementOn(sym, box.Type.Scope, box)
		if err != nil && !violationErr(err) {
			t.Fatalf("CheckRequirementOn(%s): %v", usage, err)
		}
		read := mustEvalOn(t, ctx, box, usage+".result")
		if read.Kind != ValConst || read.Const.Bool != result.Holds {
			t.Errorf("%s.result = %s, want the requirement check's %v", usage, FormatValue(read), result.Holds)
		}
		all := mustEvalIn(t, ctx, root, "ControlFunctions::allTrue(test::box."+usage+".constraints.result)")
		if all.Kind != ValConst || all.Const.Bool != result.Holds {
			t.Errorf("allTrue(%s.constraints.result) = %s, want the requirement check's %v", usage, FormatValue(all), result.Holds)
		}
	}
	// Each subrequirement is one value of `subrequirements`, in declaration order, and of `constraints`.
	subs := mustEvalOn(t, ctx, box, "rr.subrequirements")
	want := []string{"light", "inl", "pow", "deep", "guarded", "worry"}
	held := instancesOf(ctx, elementsOf(subs))
	if len(held) != len(want) {
		t.Fatalf("rr.subrequirements holds %d objects, want %d", len(held), len(want))
	}
	for i, inst := range held {
		if _, feature := inst.Owner(); feature != want[i] {
			t.Errorf("rr.subrequirements[%d] is held as %q, want %q", i, feature, want[i])
		}
	}
}

// A collection of objects a package-level requirement usage would hold is read as undetermined of the
// count the model fixes, and the read leaves no requirement object standing.
func TestPackageRequirementCollectionReadKeepsItsCount(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "conformance", "subrequirement_package_usage_reads.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, idx := libraryShapeContext(t, string(src))
	root := idx.DocumentRoot("<test>")
	for src, count := range map[string]string{
		"test::top.subrequirements":     "[5]",
		"test::top.constraints":         "[6]",
		"test::top.two":                 "[2]",
		"test::top.light":               "[1]",
		"test::unbound.subrequirements": "[5]",
	} {
		val, err := evalIn(t, ctx, root, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{
		"SequenceFunctions::size(test::top.subrequirements)": "5",
		"test::top.light.result":                             "false",
		"test::topOk.light.result":                           "true",
	} {
		if got := FormatValue(mustEvalIn(t, ctx, root, src)); got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
	for _, inst := range ctx.instances {
		if usage, ok := inst.Type.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageRequirement {
			t.Errorf("requirement %s materialized by a model-level chain read", inst.Type.Name)
		}
		if def, ok := inst.Type.Decl.(*ast.Definition); ok && def.Kind == ast.DefRequirement {
			t.Errorf("an object of requirement def %s stands after a model-level chain read", inst.Type.Name)
		}
	}
}

// A subrequirement declaring no subject reads its container's over the value its definition binds, as
// the container's check binds it: the read, the subject and the check agree.
func TestSubrequirementSubjectOverridesItsDefinitionsBinding(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `
		package test {
			private import ScalarValues::*;
			part def Veh { attribute mass : Real; }
			part veh : Veh { attribute :>> mass = 20.0; }
			part feather : Veh { attribute :>> mass = 5.0; }
			requirement def DefBound { subject v : Veh = feather; require constraint { v.mass < 10.0 } }
			requirement def Rebinding { subject v : Veh; requirement db : DefBound; }
			part box { requirement rebinding : Rebinding { subject v = veh; } }
		}
	`)
	box := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::box"))
	result, err := ctx.CheckRequirementOn(requirementUsage(t, idx, "test::box::rebinding"), box.Type.Scope, box)
	if err != nil && !violationErr(err) {
		t.Fatal(err)
	}
	for src, want := range map[string]string{
		"rebinding.result": "false", "rebinding.db.result": "false", "rebinding.db.v.mass": "20.0",
	} {
		if got := FormatValue(mustEvalOn(t, ctx, box, src)); got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
	if result.Holds {
		t.Error("the container's check holds although its subrequirement judges a vehicle of mass 20.0")
	}
}
