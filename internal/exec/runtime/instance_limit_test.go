package runtime

import (
	"errors"
	"testing"
)

const instanceLimitSrc = `
	part def Wheel;
	part def Engine;
	part def Car {
		part engine : Engine;
		part wheels : Wheel[2];
	}
	part car : Car;
`

// TestInstanceLimit_CountsNestedObjects: the bound counts the objects a read
// materializes under a root, and a read that would pass it fails leaving none
// of them, the ones that fit included.
func TestInstanceLimit_CountsNestedObjects(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, instanceLimitSrc)
	ctx := NewContext(typedModel(model, resolver), 1000)
	ctx.SetMaxInstances(3)
	if got := ctx.MaxInstances(); got != 3 {
		t.Fatalf("MaxInstances = %d, want 3", got)
	}
	carSym := resolveSymbol(t, root, "car")

	car, err := ctx.Instantiate(carSym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := car.GetFeatureValue(ctx, "engine"); err != nil {
		t.Fatalf("engine: %v", err)
	}
	if got := ctx.InstanceCount(); got != 2 {
		t.Fatalf("InstanceCount after engine = %d, want 2", got)
	}

	// The first wheel fits, the second does not: neither stays.
	_, err = car.GetFeatureValue(ctx, "wheels")
	if !errors.Is(err, ErrInstanceLimitExceeded) {
		t.Fatalf("wheels past the bound: err = %v, want ErrInstanceLimitExceeded", err)
	}
	if got := ctx.InstanceCount(); got != 2 {
		t.Errorf("InstanceCount after the refused read = %d, want 2: the failed read left objects behind", got)
	}
	if _, live := ctx.Instance(3); live {
		t.Error("the wheel that fit is still registered after the read failed")
	}
	if fv := car.FeatureValues["wheels"]; fv != nil && fv.Materialized {
		t.Error("wheels reads as materialized after the refused read")
	}
}

// TestInstanceLimit_RootFailsWhole: a creation that passes the bound in a
// nested object leaves neither the root nor the objects under it.
func TestInstanceLimit_RootFailsWhole(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, instanceLimitSrc)
	ctx := NewContext(typedModel(model, resolver), 1000)
	ctx.SetMaxInstances(5)
	carSym := resolveSymbol(t, root, "car")

	readAll := func(inst *Instance) error {
		for _, name := range []string{"engine", "wheels"} {
			if _, err := inst.GetFeatureValue(ctx, name); err != nil {
				return err
			}
		}
		return nil
	}
	first, err := ctx.InstantiateRead(carSym, readAll)
	if err != nil {
		t.Fatalf("first car: %v", err)
	}
	if got := ctx.InstanceCount(); got != 4 {
		t.Fatalf("InstanceCount after the first car = %d, want 4", got)
	}

	_, err = ctx.InstantiateRead(carSym, readAll)
	if !errors.Is(err, ErrInstanceLimitExceeded) {
		t.Fatalf("second car: err = %v, want ErrInstanceLimitExceeded", err)
	}
	if got := ctx.InstanceCount(); got != 4 {
		t.Errorf("InstanceCount after the refused car = %d, want 4", got)
	}
	if _, live := ctx.Instance(first.ID + 4); live {
		t.Error("the refused car's root is still registered")
	}
	if ids := ctx.occurrences[carSym]; len(ids) != 1 || ids[0] != first.ID {
		t.Errorf("car denotes %v, want the first car #%d alone", ids, first.ID)
	}

	// Room for one more object, and no more, remains; lifting the bound frees it.
	wheelSym := resolveSymbol(t, root, "Wheel")
	if _, err := ctx.Instantiate(wheelSym); err != nil {
		t.Fatalf("fifth object: %v", err)
	}
	if _, err := ctx.Instantiate(wheelSym); !errors.Is(err, ErrInstanceLimitExceeded) {
		t.Fatalf("sixth object: err = %v, want ErrInstanceLimitExceeded", err)
	}
	ctx.SetMaxInstances(0)
	if _, err := ctx.Instantiate(wheelSym); err != nil {
		t.Fatalf("unbounded Instantiate: %v", err)
	}
	if got := ctx.InstanceCount(); got != 6 {
		t.Errorf("InstanceCount = %d, want 6", got)
	}
}
