package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// MaterializationErrors reads the feature values a caller would otherwise leave lazy, so
// an object created without reading it still reports what its defaults are.
func TestMaterializationErrorsReadsEveryFeatureValue(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Sub {
				attribute volume : Real = 2.0;
				attribute wrong : Real[3] = 1.0;
			}
			part def Holder {
				part sub : Sub;
				attribute two : Real = (1.0, 2.0);
			}
		}
	`)

	errs, bounded := ctx.MaterializationErrors(inst)
	if bounded {
		t.Error("bounded = true, want a small object read in full")
	}
	if len(errs) != 2 {
		t.Fatalf("MaterializationErrors = %v, want the object's own and its nested object's", errs)
	}
	for _, want := range []string{"Holder.two", "Sub.wrong"} {
		if !containsError(errs, want) {
			t.Errorf("no error naming %s:\n%v", want, errs)
		}
	}
	for _, err := range errs {
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("err = %v, want ErrMultiplicityViolation", err)
		}
	}
}

// An object whose feature values all materialize reports nothing, and one holding itself
// terminates rather than descending forever.
func TestMaterializationErrorsOfAConformingObject(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Node {
				attribute mass : Real = 1.0;
				part child : Node;
			}
			part def Holder {
				part root : Node;
				attribute total : Real[0..*] = root.mass;
			}
		}
	`)

	errs, bounded := ctx.MaterializationErrors(inst)
	if len(errs) != 0 {
		t.Errorf("MaterializationErrors = %v, want none", errs)
	}
	// The object holds its own kind, which the walk elides rather than expanding
	// forever, so what it holds below that is unchecked rather than clean.
	if !bounded {
		t.Error("bounded = false, want the elided recursion reported as unchecked")
	}
	if errs, _ := ctx.MaterializationErrors(nil); errs != nil {
		t.Errorf("MaterializationErrors(nil) = %v, want none", errs)
	}
}

// A redefinition names the redefined feature again and the two names read one
// feature value, so one faulty feature value is reported once.
func TestMaterializationErrorsReportsARedefinedFeatureValueOnce(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Base { attribute mass : Real; }
			part def Holder :> Base {
				attribute grossMass :>> mass = (1.0, 2.0);
			}
		}
	`)

	errs, _ := ctx.MaterializationErrors(inst)
	if len(errs) != 1 {
		t.Fatalf("MaterializationErrors = %v, want the shared feature value reported once", errs)
	}
	if !errors.Is(errs[0], ErrMultiplicityViolation) {
		t.Errorf("err = %v, want ErrMultiplicityViolation", errs[0])
	}
}

// An unset quantity attribute reads as unset, so the walk does not evaluate what its
// value type derives from the value it lacks; a default that genuinely fails is still reported.
func TestMaterializationErrorsPassOverAnUnsetQuantity(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ISQ::*;
			private import ScalarValues::Real;
			part def Engine {
				attribute mass :> ISQ::mass;
				attribute wrong : Real[3] = 1.0;
			}
			part def Holder { part engine : Engine; }
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
	if sym == nil {
		t.Fatal("Holder part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	errs, bounded := ctx.MaterializationErrors(inst)
	if bounded {
		t.Error("bounded = true, want a small object read in full")
	}
	if len(errs) != 1 || !errors.Is(errs[0], ErrMultiplicityViolation) || !containsError(errs, "Engine.wrong") {
		t.Fatalf("MaterializationErrors = %v, want only the multiplicity violation of Engine.wrong", errs)
	}
	engines := heldInstances(ctx, inst.FeatureValues["engine"])
	if len(engines) != 1 {
		t.Fatalf("engine holds %d objects, want 1", len(engines))
	}
	fv, err := engines[0].GetFeatureValue(ctx, "mass")
	if err != nil {
		t.Fatalf("engine.mass: %v", err)
	}
	if !ctx.HoldsNoValue(fv.HeldValue()) {
		t.Errorf("engine.mass holds %v, want %s", fv.HeldValue(), UnsetText)
	}
}

// A destroyed object holds no feature values to materialize, so the walk passes
// it over rather than reporting each of its features as unreadable.
func TestMaterializationErrorsSkipsDestroyedObjects(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Sub { attribute wrong : Real[3] = 1.0; }
			part def Holder { part sub : Sub; }
		}
	`)
	fv, err := inst.GetFeatureValue(ctx, "sub")
	if err != nil {
		t.Fatalf("GetFeatureValue(sub): %v", err)
	}
	id, _ := fv.HeldValue().Object()
	sub, _ := ctx.Instance(id)
	if err := ctx.destroy(sub); err != nil {
		t.Fatalf("destroy: %v", err)
	}

	errs, bounded := ctx.MaterializationErrors(inst)
	if len(errs) != 0 || bounded {
		t.Errorf("MaterializationErrors = %v, %v; want none after the part was destroyed", errs, bounded)
	}
	if errs, _ := ctx.MaterializationErrors(sub); len(errs) != 0 {
		t.Errorf("MaterializationErrors(destroyed) = %v, want none", errs)
	}
}

// Nesting multiplies and reading a feature value materializes the objects it holds, so a
// wide model stops at the walk's budget rather than allocating without end.
func TestMaterializationErrorsBoundsAWideModel(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Leaf { attribute v : Real = 1.0; }
			part def L3 { part leaves : Leaf[100]; }
			part def L2 { part inner : L3[100]; }
			part def L1 { part inner : L2[100]; }
			part def Holder { part inner : L1[100]; }
		}
	`)

	bounded := make(chan bool, 1)
	go func() {
		_, hit := ctx.MaterializationErrors(inst)
		bounded <- hit
	}()
	select {
	case hit := <-bounded:
		if !hit {
			t.Error("bounded = false, want the walk to report it stopped at its budget")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("MaterializationErrors did not return, want a bounded walk")
	}
}

func containsError(errs []error, substr string) bool {
	for _, err := range errs {
		if strings.Contains(err.Error(), substr) {
			return true
		}
	}
	return false
}
