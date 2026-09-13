package edit

import (
	"sort"
	"strings"

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
	// FailureReferencedElsewhere is a delete or rename of a declaration referred
	// to from a document the edit may not rewrite — one the model hands out no
	// source for, such as a document not open in the editor or a bundled library
	// file — so the reference could not follow. A reference from a document the
	// edit may rewrite is followed instead. A move respells references in its
	// own document only, so any other document's reference refuses it.
	FailureReferencedElsewhere
	// FailureOwnerInsideTarget is a move whose new owner is the target itself or
	// a declaration inside it.
	FailureOwnerInsideTarget
	// FailureMoveReferenced is a move leaving a reference no spelling can make
	// reach what it reached before.
	FailureMoveReferenced
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
	// Referrers is Referring with each name told apart from its document, for a
	// client that lists them by document; empty where Referring names no declaration.
	Referrers []Referrer
}

func (e *Error) Error() string { return e.Message }

// Referrer is one declaration referring to the target of a refused delete or rename.
type Referrer struct {
	// Name is the declaration as the notation names it, an anonymous one by its
	// heading or keyword within its namespace.
	Name string
	// Document is the document declaring it.
	Document string
}

// sortReferrers orders referrers by document, then by name.
func sortReferrers(referrers []Referrer) {
	sort.SliceStable(referrers, func(i, j int) bool {
		if referrers[i].Document != referrers[j].Document {
			return referrers[i].Document < referrers[j].Document
		}
		return referrers[i].Name < referrers[j].Name
	})
}

// referring spells referrers for a message, each qualified by its document when
// that is not own.
func referring(own string, referrers []Referrer) []string {
	out := make([]string, 0, len(referrers))
	for _, r := range referrers {
		if r.Document == own {
			out = append(out, r.Name)
		} else {
			out = append(out, r.Name+" ("+r.Document+")")
		}
	}
	return out
}

// referencedElsewhere refuses operation i on target, which the declarations
// referrers, in documents the edit may not rewrite, refer to.
func referencedElsewhere(i int, own, target string, referrers []Referrer) error {
	names := referring(own, referrers)
	return &Error{
		Failure:        FailureReferencedElsewhere,
		OperationIndex: i,
		Referring:      names,
		Referrers:      referrers,
		Message: target + " is referenced by " + strings.Join(names, ", ") +
			" in documents this edit cannot rewrite; change those references first",
	}
}
