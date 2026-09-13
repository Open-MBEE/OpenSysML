package edit

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Failure is why edits were refused. Every refusal carries one: an edit is
// never silently dropped.
type Failure int

const (
	// FailureNone is no failure.
	FailureNone Failure = iota
	// FailureNoOperations is a request naming no edit.
	FailureNoOperations
	// FailureUnknownTarget is a target this model declares nothing under.
	FailureUnknownTarget
	// FailureAmbiguousTarget is a target several declarations answer to.
	FailureAmbiguousTarget
	// FailureNotValued is a target that can carry no value.
	FailureNotValued
	// FailureInvalidValue is a new value that does not parse as an expression.
	FailureInvalidValue
	// FailureInvalidName is a new name that does not lex as an identifier, or a
	// connection end that is not written as a feature reference.
	FailureInvalidName
	// FailureNotNamed is a target declaring no name to rewrite.
	FailureNotNamed
	// FailureRenameReferenced is a rename that references to the element would
	// not survive.
	FailureRenameReferenced
	// FailureOverlappingEdits is two edits covering the same bytes.
	FailureOverlappingEdits
	// FailureResultInvalid is edited notation carrying errors the original had
	// not.
	FailureResultInvalid
	// FailureOwnerUnknown is an add-member owner that is not declared.
	FailureOwnerUnknown
	// FailureOwnerNotNamespace is an element that cannot contain members.
	FailureOwnerNotNamespace
	// FailureIllegalKind is a declaration kind invalid for the document language.
	FailureIllegalKind
	// FailureMemberNameTaken is a member name already declared by the owner.
	FailureMemberNameTaken
	// FailureDeleteReferenced is a non-cascade delete with live references.
	FailureDeleteReferenced
	// FailureReferencedElsewhere is a delete or rename of a declaration that
	// another document refers to, which an edit of this one cannot follow.
	FailureReferencedElsewhere
	// FailureOwnerInsideTarget is a move whose new owner is the target itself or
	// a declaration inside it.
	FailureOwnerInsideTarget
	// FailureMoveReferenced is a move leaving a reference no spelling can make
	// reach what it reached before.
	FailureMoveReferenced
	// FailureNotAView is a layout view, or a Canvas target, that is no view.
	FailureNotAView
	// FailureNotExposed is a view-local layout of an element the view does not
	// expose.
	FailureNotExposed
	// FailureNotDrawn is a layout of an element no rendering draws as the node
	// or edge the annotation positions.
	FailureNotDrawn
	// FailureNotAnnotated is a clearing of an annotation that is not there.
	FailureNotAnnotated
)

var failureNames = map[Failure]string{
	FailureNone:                "none",
	FailureNoOperations:        "no-operations",
	FailureUnknownTarget:       "unknown-target",
	FailureAmbiguousTarget:     "ambiguous-target",
	FailureNotValued:           "not-valued",
	FailureInvalidValue:        "invalid-value",
	FailureInvalidName:         "invalid-name",
	FailureNotNamed:            "not-named",
	FailureRenameReferenced:    "rename-referenced",
	FailureOverlappingEdits:    "overlapping-edits",
	FailureResultInvalid:       "result-invalid",
	FailureOwnerUnknown:        "owner-unknown",
	FailureOwnerNotNamespace:   "owner-not-namespace",
	FailureIllegalKind:         "illegal-kind",
	FailureMemberNameTaken:     "member-name-taken",
	FailureDeleteReferenced:    "delete-referenced",
	FailureReferencedElsewhere: "referenced-elsewhere",
	FailureOwnerInsideTarget:   "owner-inside-target",
	FailureMoveReferenced:      "move-referenced",
	FailureNotAView:            "not-a-view",
	FailureNotExposed:          "not-exposed",
	FailureNotDrawn:            "not-drawn",
	FailureNotAnnotated:        "not-annotated",
}

// String returns the lowercase name of the failure, or "unknown".
func (f Failure) String() string {
	if name, ok := failureNames[f]; ok {
		return name
	}
	return "unknown"
}

// Error is a refused edit: which kind of refusal it is, and the evidence for it.
type Error struct {
	Failure Failure
	Message string
	// OperationIndex is the operation refused, or -1 for a whole request.
	OperationIndex int
	// Diagnostics are the errors behind a refusal: those of an unreadable new
	// value, or those the edited notation was found to have.
	Diagnostics []passes.Diagnostic
	// Diagnosed is the source the Diagnostics' spans are offsets into: the new
	// value's text, or the edited notation. A refusal still returns no model.
	Diagnosed *source.SourceFile
	// Referring names the declarations referring to a target whose delete,
	// rename or move was refused, each qualified by its document when that is another.
	Referring []string
}

func (e *Error) Error() string { return e.Message }
