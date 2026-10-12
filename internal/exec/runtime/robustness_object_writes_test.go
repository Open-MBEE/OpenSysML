package runtime

import (
	"errors"
	"testing"
)

func TestRuntimeRobustnessObjectWrites(t *testing.T) {
	t.Run("including_at_out_of_range", func(t *testing.T) {
		_, err := evalCollectionExpr(t, "SequenceFunctions::includingAt((), 9, 2)")
		if !errors.Is(err, ErrIndexOutOfRange) {
			t.Errorf("includingAt = %v, want %v", err, ErrIndexOutOfRange)
		}
	})

	t.Run("destroy_empty_target", func(t *testing.T) {
		got, err := evalCollectionExpr(t, "OccurrenceFunctions::destroy(())")
		if err != nil {
			t.Fatalf("destroy(()) = %v", err)
		}
		if got.Kind != ValNull {
			t.Errorf("destroy(()) = %s, want null", FormatValue(got))
		}
	})
}
