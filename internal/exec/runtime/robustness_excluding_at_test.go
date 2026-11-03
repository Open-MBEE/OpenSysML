package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessExcludingAt: excludingAt follows its library body
// `(seq->subsequence(1, startIndex - 1), seq->subsequence(endIndex + 1))`, so
// only a position the first subsequence has to read past the end, or an index
// that is no Positive, is an error; the rest is answered, never a panic.
func TestRuntimeRobustnessExcludingAt(t *testing.T) {
	t.Run("start past size plus one reaches seq#(size+1)", func(t *testing.T) {
		for _, expr := range []string{"xs->excludingAt(5)", "xs->excludingAt(5, 2)", "()->excludingAt(2)"} {
			got, err := evalCollectionExpr(t, expr)
			if !errors.Is(err, ErrIndexOutOfRange) {
				t.Errorf("%s = (%v, %v), want %v", expr, got, err, ErrIndexOutOfRange)
			}
		}
	})
	t.Run("an index that is no Positive is refused", func(t *testing.T) {
		for _, tt := range []struct {
			expr string
			want error
		}{
			{"xs->excludingAt(0)", ErrIndexOutOfRange},
			{"xs->excludingAt(0 - 1)", ErrIndexOutOfRange},
			{"xs->excludingAt(1, 0)", ErrIndexOutOfRange},
			{"xs->excludingAt(1.5)", ErrTypeMismatch},
			{"xs->excludingAt(ys)", ErrTypeMismatch},
		} {
			got, err := evalCollectionExpr(t, tt.expr)
			if !errors.Is(err, tt.want) {
				t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
			}
		}
	})
	t.Run("past the end and inverted ranges are the body's answer", func(t *testing.T) {
		for _, tt := range []struct {
			expr string
			want []int64
		}{
			{"xs->excludingAt(4)", []int64{1, 2, 3}},
			{"xs->excludingAt(4, 9)", []int64{1, 2, 3}},
			{"xs->excludingAt(2, 9)", []int64{1}},
			{"xs->excludingAt(2, 9223372036854775807)", []int64{1}},
			{"xs->excludingAt(3, 2)", []int64{1, 2, 3}},
			{"xs->excludingAt(3, 1)", []int64{1, 2, 2, 3}},
			{"()->excludingAt(1)", nil},
		} {
			got, err := evalCollectionExpr(t, tt.expr)
			if err != nil {
				t.Errorf("%s: %v", tt.expr, err)
				continue
			}
			if ints := intsOf(t, got); !equalInts(ints, tt.want) {
				t.Errorf("%s = %v, want %v", tt.expr, ints, tt.want)
			}
		}
	})
}
