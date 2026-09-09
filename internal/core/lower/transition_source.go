package lower

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// NoTransitionSourceMessage reports the sourceless shorthand with no member before
// it to leave, shared by the check reporting it and the lowering backstopping it.
const NoTransitionSourceMessage = "transition without a source has no member before it to leave: " +
	"it leaves the state declared before it in the same body (SysML v2 7.18.3), and none precedes it"

// TransitionSourceNotVertexFormat reports the shorthand whose nearest preceding
// member is not a state or pseudostate, shared the same way.
const TransitionSourceNotVertexFormat = "transition without a source leaves %s, the member declared before it, " +
	"which is not a state of this state machine (SysML v2 7.18.3)"

// TransitionSourceRegionFormat reports the shorthand whose nearest preceding
// member is an orthogonal region of a parallel state, which no transition leaves.
const TransitionSourceRegionFormat = "transition without a source leaves %s, the member declared before it, " +
	"which is an orthogonal region of a parallel state rather than a state of this state machine (SysML v2 7.18.3)"

// EntryTransitionUnsupportedMessage reports the guarded entry transition, which
// this lowering has no starting-state choice for yet.
const EntryTransitionUnsupportedMessage = "a guarded transition out of the entry action (`entry; if c then s;`, SysML v2 7.18.3) " +
	"chooses the starting state by its guard, which is not supported yet: write `entry; then s;` to start in one state"

// ErrNoTransitionSource marks a transition written without a source that no
// member precedes in its body.
var ErrNoTransitionSource = errors.New(NoTransitionSourceMessage)

// ErrEntryTransitionUnsupported marks a guarded transition out of the entry
// action, legal notation whose starting-state choice is not lowered yet.
var ErrEntryTransitionUnsupported = errors.New(EntryTransitionUnsupportedMessage)

// TransitionSourceError marks a sourceless transition whose preceding member, Source,
// is not a vertex of the machine; Region records that it is a region of a parallel state.
type TransitionSourceError struct {
	Source ast.Node
	Region bool
}

func (e *TransitionSourceError) Error() string {
	if e.Region {
		return fmt.Sprintf(TransitionSourceRegionFormat, DescribeMember(e.Source))
	}
	return fmt.Sprintf(TransitionSourceNotVertexFormat, DescribeMember(e.Source))
}

// ImplicitSource returns the body member a transition written without a source
// leaves, or ErrNoTransitionSource when no member precedes it in members.
func ImplicitSource(members []ast.Node, transition *ast.TransitionMember) (ast.Node, error) {
	source := ast.ImplicitTransitionSource(members, transition)
	if source == nil {
		return nil, ErrNoTransitionSource
	}
	return source, nil
}

// IsEntryTransition reports whether an untriggered sourceless transition leaves the entry
// action source: `entry; if c then s;`, the guarded entry transition of SysML v2 7.18.3.
func IsEntryTransition(source ast.Node, transition *ast.TransitionMember) bool {
	_, entry := source.(*ast.EntryMember)
	return entry && transition.Trigger == nil
}
