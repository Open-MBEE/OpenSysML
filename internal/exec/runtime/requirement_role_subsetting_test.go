package runtime

import (
	"slices"
	"testing"
)

const requirementRoleSubsettingModel = `package test {
	part def Person;
	part op : Person;
	concern def Safety;
	requirement def R {
		subject s : Person;
		actor driver : Person;
		stakeholder owner : Person;
		frame concern sf : Safety;
	}
	requirement r : R { subject :>> s; actor passenger : Person; }
	verification def VD { subject s : Person; actor tester : Person; }
	verification v : VD { actor :>> tester = op; }
}`

// A requirement's actors, stakeholders and framed concern are the members of its `actors`, `stakeholders`
// and `concerns`; a usage's actor redefines its definition's positionally, so both names hold one member.
func TestRequirementRolesFillTheRequirementCheckCollections(t *testing.T) {
	ctx, idx := libraryShapeContext(t, requirementRoleSubsettingModel)
	op := instantiateQualified(t, ctx, idx, "test::op")
	req := instantiateQualified(t, ctx, idx, "test::r")
	typ := lookupOne(t, idx, "test::r")

	for path, want := range map[string]string{
		"test::r::passenger": "actors",
		"test::R::driver":    "actors",
		"test::R::owner":     "stakeholders",
		"test::R::sf":        "concerns",
	} {
		if got := ctx.subsettedNamesOf(req, lookupOne(t, idx, path)); !slices.Contains(got, want) {
			t.Errorf("subsettedNamesOf(%s) = %v, want %s among them", path, got, want)
		}
	}
	for collection, want := range map[string][]string{"actors": {"driver", "passenger"}, "stakeholders": {"owner"}, "concerns": {"sf"}} {
		got := featureNames(ctx.SubsettingFeatures(req, typ, collection))
		for _, name := range want {
			if !slices.Contains(got, name) {
				t.Errorf("SubsettingFeatures(%s) = %v, want %s among them", collection, got, name)
			}
		}
	}
	for collection, members := range map[string][]string{"actors": {"passenger"}, "stakeholders": {"owner"}, "concerns": {"sf"}} {
		fv, err := req.GetFeatureValue(ctx, collection)
		if err != nil {
			t.Fatalf("read %s: %v", collection, err)
		}
		held, err := ctx.HeldElements(fv.HeldValue())
		if err != nil {
			t.Fatalf("%s: %v", collection, err)
		}
		var want []Value
		for _, member := range members {
			mv, err := req.GetFeatureValue(ctx, member)
			if err != nil {
				t.Fatalf("read %s: %v", member, err)
			}
			want = append(want, mv.Value)
		}
		if !slices.Equal(held, want) {
			t.Errorf("%s = %v, want the objects of %v (%v)", collection, held, members, want)
		}
	}
	driver, _ := req.GetFeatureValue(ctx, "driver")
	passenger, _ := req.GetFeatureValue(ctx, "passenger")
	if driver == nil || passenger == nil || driver.Value != passenger.Value {
		t.Errorf("driver and passenger hold different objects: the usage's actor redefines the definition's")
	}

	// A case's bound actor is the member of its actors.
	vc := instantiateQualified(t, ctx, idx, "test::v")
	fv, err := vc.GetFeatureValue(ctx, "actors")
	if err != nil {
		t.Fatalf("read v.actors: %v", err)
	}
	held, err := ctx.HeldElements(fv.HeldValue())
	if err != nil {
		t.Fatal(err)
	}
	if want := []Value{{Kind: ValInstance, Instance: op.ID}}; !slices.Equal(held, want) {
		t.Errorf("v.actors = %v, want %v (the bound actor)", held, want)
	}
}

// A referential collection nothing subsets stays as it was: unread, holding no made-up value.
func TestReferentialCollectionWithoutSubsettersHoldsNothing(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		part def Person;
		requirement def Bare { subject s : Person; }
	}`)
	req := instantiateQualified(t, ctx, idx, "test::Bare")
	fv, err := req.GetFeatureValue(ctx, "actors")
	if err != nil {
		t.Fatalf("read actors: %v", err)
	}
	if fv.Materialized {
		t.Errorf("actors materialized as %s with no actor to hold", FormatValue(fv.HeldValue()))
	}
}
