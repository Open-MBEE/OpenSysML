package runtime

import (
	"slices"
	"testing"
)

// A branch whose condition stands on the control it already runs under — an
// inner if reading what the outer if read — adds each choice point once, what
// a move stands on lists them sorted, and releasing the branch restores the
// controls the enclosing one ran under.
func TestUnderControlAddsEachChoicePointOnce(t *testing.T) {
	ctx := &Context{}
	outer := ctx.underControl([]int64{7})
	if got := ctx.controlling(); !slices.Equal(got, []int64{7}) {
		t.Fatalf("controls under the outer branch = %v, want [7]", got)
	}
	inner := ctx.underControl([]int64{3, 7})
	if got := ctx.controlling(); !slices.Equal(got, []int64{3, 7}) {
		t.Fatalf("controls under the inner branch = %v, want [3 7]", got)
	}
	if got := unionSerials(ctx.controlling(), []int64{5}); !slices.Equal(got, []int64{3, 5, 7}) {
		t.Fatalf("standing on the controls and a mark = %v, want [3 5 7]", got)
	}
	ctx.releaseControl(inner)
	if got := ctx.controlling(); !slices.Equal(got, []int64{7}) {
		t.Fatalf("controls after the inner branch = %v, want [7]", got)
	}
	ctx.releaseControl(outer)
	if got := ctx.controlling(); len(got) != 0 {
		t.Fatalf("controls after the outer branch = %v, want none", got)
	}
}
