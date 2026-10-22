package runtime

import "testing"

// A sequence of one value written to a unique feature repeats nothing, so the check
// answers without building the set it judges repeats by: a calculation's result held
// in its one-element shape pays nothing to be read into a unique feature.
func TestUniquenessOfOneValueAllocatesNothing(t *testing.T) {
	ctx := emptyRuntimeContext()
	one := sequenceOf([]Value{integerValue(7)})
	if msg := ctx.uniquenessRefusal(true, false, &one); msg != "" {
		t.Fatalf("uniquenessRefusal(one value) = %q, want no refusal", msg)
	}
	if allocs := testing.AllocsPerRun(20, func() { ctx.uniquenessRefusal(true, false, &one) }); allocs != 0 {
		t.Errorf("uniquenessRefusal(one value) allocates %.0f times, want 0", allocs)
	}
	repeated := sequenceOf([]Value{integerValue(7), integerValue(8), integerValue(7)})
	if msg := ctx.uniquenessRefusal(true, false, &repeated); msg == "" {
		t.Fatalf("uniquenessRefusal(7, 8, 7) = no refusal, want the repeat named")
	}
}
