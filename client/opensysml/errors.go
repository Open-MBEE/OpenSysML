package opensysml

import (
	"errors"
	"fmt"
	"strings"
)

// Code is a canonical gRPC status code, the spelling in which the service
// refuses a call. A Code is itself an error, so a refusal can be tested with
// errors.Is(err, opensysml.CodeNotFound) whichever implementation answered.
type Code uint32

// The status codes a call can be refused with, mirroring the canonical gRPC
// codes. CodeOK never reaches a caller: a call that succeeds returns nil.
const (
	CodeOK                 Code = 0
	CodeCanceled           Code = 1
	CodeUnknown            Code = 2
	CodeInvalidArgument    Code = 3
	CodeDeadlineExceeded   Code = 4
	CodeNotFound           Code = 5
	CodeAlreadyExists      Code = 6
	CodePermissionDenied   Code = 7
	CodeResourceExhausted  Code = 8
	CodeFailedPrecondition Code = 9
	CodeAborted            Code = 10
	CodeOutOfRange         Code = 11
	CodeUnimplemented      Code = 12
	CodeInternal           Code = 13
	CodeUnavailable        Code = 14
	CodeDataLoss           Code = 15
	CodeUnauthenticated    Code = 16
)

// codeNames are the canonical gRPC spellings, the ones conformance scenarios
// and other languages' clients use.
var codeNames = map[Code]string{
	CodeOK:                 "OK",
	CodeCanceled:           "CANCELLED",
	CodeUnknown:            "UNKNOWN",
	CodeInvalidArgument:    "INVALID_ARGUMENT",
	CodeDeadlineExceeded:   "DEADLINE_EXCEEDED",
	CodeNotFound:           "NOT_FOUND",
	CodeAlreadyExists:      "ALREADY_EXISTS",
	CodePermissionDenied:   "PERMISSION_DENIED",
	CodeResourceExhausted:  "RESOURCE_EXHAUSTED",
	CodeFailedPrecondition: "FAILED_PRECONDITION",
	CodeAborted:            "ABORTED",
	CodeOutOfRange:         "OUT_OF_RANGE",
	CodeUnimplemented:      "UNIMPLEMENTED",
	CodeInternal:           "INTERNAL",
	CodeUnavailable:        "UNAVAILABLE",
	CodeDataLoss:           "DATA_LOSS",
	CodeUnauthenticated:    "UNAUTHENTICATED",
}

// String is the canonical gRPC spelling of the code ("NOT_FOUND").
func (c Code) String() string {
	if name, ok := codeNames[c]; ok {
		return name
	}
	return fmt.Sprintf("CODE(%d)", uint32(c))
}

// Error makes a Code usable as an errors.Is target.
func (c Code) Error() string {
	return "opensysml: " + strings.ToLower(c.String())
}

// StatusError is a call the service refused: the transport-level failure a
// remote caller would see as a gRPC status. errors.Is(err, opensysml.CodeX)
// matches the code, and errors.As recovers the message.
type StatusError struct {
	// Code says why the call was refused.
	Code Code
	// Message is the service's own wording.
	Message string
}

// Error renders the refusal with its canonical code name. Code.String is
// spelled out because a Code is itself an error, which fmt would render with
// Error instead.
func (e *StatusError) Error() string {
	return fmt.Sprintf("opensysml: %s: %s", e.Code.String(), e.Message)
}

// Unwrap exposes the Code, so errors.Is(err, opensysml.CodeNotFound) matches.
func (e *StatusError) Unwrap() error {
	return e.Code
}

// ErrFailure is the errors.Is target for every FailureError: a call the
// service answered, whose answer reports a failure.
var ErrFailure = errors.New("opensysml: the call was answered and the answer reports a failure")

// FailureError is a failure the service reported in a successful answer — an
// unparsable expression, an unknown symbol, a failed instantiation. It is
// deliberately distinct from StatusError: on the wire the call succeeds with
// status OK and the response's error field set.
type FailureError struct {
	// Op is the Client method that was answered ("Evaluate", "LookupSymbol").
	Op string
	// Message is the response's error field, verbatim.
	Message string
	// Diagnostics the response carried alongside the failure, if any.
	Diagnostics []Diagnostic
}

// Error renders the failure the service reported.
func (e *FailureError) Error() string {
	return fmt.Sprintf("opensysml: %s: %s", e.Op, e.Message)
}

// Is matches ErrFailure, so errors.Is(err, opensysml.ErrFailure) is true for
// every in-band failure.
func (e *FailureError) Is(target error) bool {
	return target == ErrFailure
}

// VerifyError is a verification or calculation the service could not answer at
// all, classified: an undecided verdict about the model arrives as a Verdict
// instead. It is a FailureError, so errors.Is(err, ErrFailure) matches.
type VerifyError struct {
	FailureError
	// Reason says what kind of failure this is, so a caller acts on the kind.
	Reason Reason
}

// EditError is a set of edits the service refused, classified by what it
// refused. Every refusal is one kind: no edit is silently dropped. It is a
// FailureError, so errors.Is(err, ErrFailure) matches.
type EditError struct {
	FailureError
	// Failure says which refusal this is.
	Failure EditFailure
	// Referring are the FQNs of the namespaces referring to a declaration whose
	// rename or deletion was refused, each suffixed with its document in
	// parentheses when that is not the edited one.
	Referring []string
	// Referrers is Referring with each document as a field of its own, in
	// document then name order.
	Referrers []Referrer
}

// Unwrap exposes the failure, so errors.As recovers a *FailureError from a
// classified one.
func (e *VerifyError) Unwrap() error { return &e.FailureError }

// Unwrap exposes the failure, so errors.As recovers a *FailureError from a
// refusal.
func (e *EditError) Unwrap() error { return &e.FailureError }
