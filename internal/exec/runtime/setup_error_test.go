package runtime

import (
	"errors"
	"fmt"
	"testing"
)

func TestWrapSetupErrorPreservesInputAndPerformerErrors(t *testing.T) {
	cause := errors.New("input binding failed")
	for _, err := range []error{
		inputBindingError{Err: cause},
		fmt.Errorf("performer failed: %w", ErrOccurrenceLifetime),
	} {
		wrapped := wrapSetupError(err)
		var setup *SetupError
		if errors.As(wrapped, &setup) {
			t.Errorf("wrapSetupError(%v) = %v; want the run error unchanged", err, wrapped)
		}
	}

	cause = errors.New("executor initialization failed")
	wrapped := wrapSetupError(cause)
	var setup *SetupError
	if !errors.As(wrapped, &setup) || !errors.Is(wrapped, cause) || wrapped.Error() != cause.Error() {
		t.Errorf("wrapSetupError(%v) = %v; want SetupError preserving the message and cause", cause, wrapped)
	}
}
