package queryexec

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
)

// LocateInline wraps a planning or execution error whose origin is in an
// inline query's text as a queryplan.InlineError positioned within the text.
func LocateInline(inline *queryplan.Inline, err error) error {
	var planErr *queryplan.Error
	if errors.As(err, &planErr) {
		return inline.Locate(err, planErr.Origin)
	}
	var execErr *Error
	if errors.As(err, &execErr) {
		return inline.Locate(err, execErr.Origin)
	}
	return err
}
