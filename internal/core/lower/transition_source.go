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

// transitionSourceLeavesFormat opens every report of the shorthand whose nearest
// preceding member is not a state it may leave; %s is that member.
const transitionSourceLeavesFormat = "transition without a source leaves %s, the member declared before it, "

// TransitionSourceNotVertexFormat reports the shorthand whose nearest preceding
// member is not a state, shared the same way.
const TransitionSourceNotVertexFormat = transitionSourceLeavesFormat +
	"which is not a state of this state machine (SysML v2 7.18.3)"

// TransitionSourceRegionFormat reports the shorthand whose nearest preceding
// member is an orthogonal region of a parallel state, which no transition leaves.
const TransitionSourceRegionFormat = transitionSourceLeavesFormat +
	"which is an orthogonal region of a parallel state rather than a state of this state machine (SysML v2 7.18.3)"

// TransitionSourcePseudostateFormat reports the shorthand whose nearest preceding
// member is a pseudostate, which only a transition naming it as `first` leaves.
const TransitionSourcePseudostateFormat = transitionSourceLeavesFormat +
	"which is a pseudostate rather than a state (SysML v2 7.18.3): write `transition first %s … then …;` to leave it"

// TransitionSourceMarkerFormat reports the shorthand whose nearest preceding
// member is a `first start then s;` succession or a `done;` marker, not a state.
const TransitionSourceMarkerFormat = transitionSourceLeavesFormat +
	"which is not a state of this state machine (SysML v2 7.18.3): write `transition first <state> … then …;` to leave one"

// ErrNoTransitionSource marks a transition written without a source that no
// member precedes in its body.
var ErrNoTransitionSource = errors.New(NoTransitionSourceMessage)

// TransitionSourceError marks a sourceless transition whose preceding member, Source,
// is not a state of the machine; Region records that it is a region of a parallel state.
type TransitionSourceError struct {
	Source ast.Node
	Region bool
}

func (e *TransitionSourceError) Error() string {
	switch source := e.Source.(type) {
	case *ast.PseudostateNode:
		return fmt.Sprintf(TransitionSourcePseudostateFormat, DescribeMember(source), source.Name)
	case *ast.InitialNode, *ast.FinalNode:
		return fmt.Sprintf(TransitionSourceMarkerFormat, DescribeMember(source))
	}
	if e.Region {
		return fmt.Sprintf(TransitionSourceRegionFormat, DescribeMember(e.Source))
	}
	return fmt.Sprintf(TransitionSourceNotVertexFormat, DescribeMember(e.Source))
}

// IsStateSource reports whether a vertex the shorthand found before it is one it may
// leave: SysML v2 7.18.3 names the previous state usage, which a pseudostate, a
// `first start then s;` succession and a `done;` marker are not.
func IsStateSource(source ast.Node) bool {
	switch source.(type) {
	case *ast.PseudostateNode, *ast.InitialNode, *ast.FinalNode:
		return false
	}
	return true
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
