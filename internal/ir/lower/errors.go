package lower

import "errors"

var (
	ErrCyclicSpecialization      = errors.New("action specializes itself")
	ErrRecursiveActionTyping     = errors.New("typed action body performs a type it is nested in")
	ErrRedefinedStepMissing      = errors.New("redefined action step not found")
	ErrIncompatibleRedefinedStep = errors.New("redefining feature is not an action step")
	ErrAmbiguousInheritedStep    = errors.New("inherited succession reaches more than one redefining step")
)
