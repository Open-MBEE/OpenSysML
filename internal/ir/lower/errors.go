package lower

import "errors"

var (
	ErrCyclicSpecialization      = errors.New("action specializes itself")
	ErrRedefinedStepMissing      = errors.New("redefined action step not found")
	ErrIncompatibleRedefinedStep = errors.New("redefining feature is not an action step")
	ErrAmbiguousInheritedStep    = errors.New("inherited succession reaches more than one redefining step")
	// ErrSuccessionEndNotPerformed is a succession end chaining to an occurrence the flow does not perform.
	ErrSuccessionEndNotPerformed = errors.New("succession end reaches an occurrence that is not an action, which no step of the flow performs")
)
