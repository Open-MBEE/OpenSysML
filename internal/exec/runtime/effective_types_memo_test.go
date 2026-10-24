package runtime

import "testing"

// A feature's effective types are computed once per model: a second query for the
// same feature allocates nothing, instantiation after instantiation.
func TestEffectiveTypesAreComputedOncePerFeature(t *testing.T) {
	ctx, idx := libraryShapeContext(t, holdingFeatureTypesModel)
	instantiateQualified(t, ctx, idx, "test::rig")
	front := lookupOne(t, idx, "test::Rig::front")
	if types := ctx.effectiveTypes(front); len(types) != 2 {
		t.Fatalf("effectiveTypes(front) = %v, want Panel and StructuredSurface", types)
	}
	if got := testing.AllocsPerRun(20, func() { ctx.effectiveTypes(front) }); got != 0 {
		t.Fatalf("a repeated effectiveTypes query allocates %.0f times, want 0", got)
	}
}
