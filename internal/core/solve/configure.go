package solve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ErrNoVariations is returned for a query whose conditions read no variation
// point, so there is no configuration to check or enumerate: the element may
// still be satisfiable, which is what solving it plainly answers.
var ErrNoVariations = errors.New("no variation point is read")

// MaxConfigurationsEnv names the environment variable that overrides how many
// configurations one enumeration reports.
const MaxConfigurationsEnv = "OPENSYSML_SMT_MAX_CONFIGURATIONS"

// DefaultMaxConfigurations bounds how many configurations one enumeration
// reports. The combinations grow as the product of the variants of every
// variation point read, so the bound is what stops a runaway enumeration; results
// stopped by it are reported as truncated rather than as all there are.
const DefaultMaxConfigurations = 32

// NoVariationsError says which element reads no variation point. It unwraps to
// ErrNoVariations.
type NoVariationsError struct {
	// Kind and Element name the element asked about.
	Kind    string
	Element string
}

// Error reports that the element's conditions read no variation point.
func (e *NoVariationsError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Kind, e.Element, ErrNoVariations)
}

// Unwrap returns ErrNoVariations.
func (e *NoVariationsError) Unwrap() error { return ErrNoVariations }

// Variations are the variables standing for the variation points the query's
// conditions read, in declaration order. They are the variables a configuration
// assigns.
func (q *Query) Variations() []*Var {
	if q == nil {
		return nil
	}
	out := make([]*Var, 0, len(q.Vars))
	for _, v := range q.Vars {
		if v.Sort.Kind == SortDatatype && v.Sort.Variation {
			out = append(out, v)
		}
	}
	return out
}

// FixValue fixes a variable of a finite sort to one of that sort's values,
// naming a variant or an enumeration literal by its qualified name, as a caller
// choosing a configuration to check does. The assertion is appended, so the
// positions the query's other assertions hold — and any core naming them — do not
// move.
func (q *Query) FixValue(v *Var, value string, src PinSource) error {
	if q == nil || v == nil {
		return &PinError{Value: value, Reason: "no variable to fix", Source: src}
	}
	refuse := func(reason string) error {
		return &PinError{
			Feature: v.Name, Value: value, Reason: reason, Source: src,
			File: v.File, Span: v.Span, Location: v.Location,
		}
	}
	if !q.declares(v) {
		return refuse("the query does not read it")
	}
	if v.Sort.Kind != SortDatatype {
		return refuse("the feature's values are " + v.Sort.Name + ", which no name of a value denotes")
	}
	if !contains(v.Sort.Values, value) {
		return refuse("it is not one of the values of " + v.Sort.Name)
	}
	q.Assertions = append(q.Assertions, Assertion{
		Term: Binary(OpEq, Bool, VarTerm(v), ValueTerm(v.Sort, value)),
		From: Provenance{
			Kind:      "feature",
			Element:   v.Name,
			Condition: v.Name + " == " + value + ", " + src.phrase(),
			Role:      RolePinned,
			Declared:  v.Symbol,
			File:      v.File,
			Span:      v.Span,
			Location:  v.Location,
		},
	})
	q.Pinned = append(q.Pinned, PinnedValue{Var: v, Value: value, Source: src, Index: len(q.Assertions) - 1})
	return nil
}

// declares reports whether the variable is one the query reads.
func (q *Query) declares(v *Var) bool {
	for _, declared := range q.Vars {
		if declared == v {
			return true
		}
	}
	return false
}

// contains reports whether values holds name.
func contains(values []string, name string) bool {
	for _, value := range values {
		if value == name {
			return true
		}
	}
	return false
}

// MaxConfigurationsFromEnv is the bound the environment asks enumeration to use,
// or DefaultMaxConfigurations when it names none or names an unusable one: a
// bound is what stops a runaway, so an unreadable override never removes it.
func MaxConfigurationsFromEnv() int {
	text := strings.TrimSpace(os.Getenv(MaxConfigurationsEnv))
	if text == "" {
		return DefaultMaxConfigurations
	}
	n, err := strconv.Atoi(text)
	if err != nil || n <= 0 {
		return DefaultMaxConfigurations
	}
	return n
}

// Configurations enumerates the combinations of variants that satisfy the query,
// asking for at most limit of them; a limit of zero or less uses the bound the
// environment names. Each combination is found by its own `check-sat`, with the
// combinations already reported denied, and Result.Truncated says the enumeration
// stopped at its bound rather than having shown there is no other.
func Configurations(ctx context.Context, q *Query, limit int) (*Result, error) {
	solver, err := Discover()
	if err != nil {
		return nil, err
	}
	return solver.Configurations(ctx, q, limit)
}

// Configurations enumerates satisfying combinations of variants with this solver.
func (s *Solver) Configurations(ctx context.Context, q *Query, limit int) (*Result, error) {
	if q == nil {
		return nil, fmt.Errorf("solve: no query to configure")
	}
	vars := q.Variations()
	if len(vars) == 0 {
		return nil, &NoVariationsError{Kind: q.Kind, Element: q.Element}
	}
	if limit <= 0 {
		limit = MaxConfigurationsFromEnv()
	}
	if err := s.require(ctx, q, "enumerating configurations", CapModels, CapIncremental); err != nil {
		return nil, err
	}
	return s.solve(ctx, q, func(sess *session) (*Result, error) { return sess.enumerate(q, vars, limit) })
}

// ErrNotDeclared is returned for an enumeration over a variable the query does
// not declare, which no model of it assigns.
var ErrNotDeclared = errors.New("the query does not declare it")

// NotDeclaredError names the variable an enumeration was asked over that the
// query does not declare. It unwraps to ErrNotDeclared.
type NotDeclaredError struct {
	// Kind and Element name the query asked about.
	Kind    string
	Element string

	// Variable is the name of the variable the query lacks.
	Variable string
}

// Error reports which variable the query lacks.
func (e *NotDeclaredError) Error() string {
	return fmt.Sprintf("%s %s: enumerating %s: %s", e.Kind, e.Element, e.Variable, ErrNotDeclared)
}

// Unwrap returns ErrNotDeclared.
func (e *NotDeclaredError) Unwrap() error { return ErrNotDeclared }

// Enumerate enumerates the distinct assignments to vars the query admits, asking
// for at most limit of them, each found by its own `check-sat` with the ones
// already reported denied; a limit of zero or less asks for one. It is
// Configurations over variables the caller names rather than over the variation
// points read, for a query whose distinct models are told apart by chosen
// variables — the moves of a bounded run, say — rather than by every variable.
// Result.Truncated says the enumeration stopped at its bound rather than having
// shown there is no other assignment.
func (s *Solver) Enumerate(ctx context.Context, q *Query, vars []*Var, limit int) (*Result, error) {
	if q == nil {
		return nil, fmt.Errorf("solve: no query to enumerate")
	}
	if len(vars) == 0 {
		return nil, fmt.Errorf("%s %s: no variable to enumerate", q.Kind, q.Element)
	}
	for _, v := range vars {
		if !q.declares(v) {
			return nil, &NotDeclaredError{Kind: q.Kind, Element: q.Element, Variable: v.Name}
		}
	}
	if limit <= 0 {
		limit = 1
	}
	if err := s.require(ctx, q, "enumerating assignments", CapModels, CapIncremental); err != nil {
		return nil, err
	}
	return s.solve(ctx, q, func(sess *session) (*Result, error) { return sess.enumerate(q, vars, limit) })
}

// enumerate holds the dialogue that reports one configuration per `check-sat`,
// denying each one it reported before asking again, until the solver answers
// unsat — which is when the reported ones are all there are — or the bound is
// reached.
func (s *session) enumerate(q *Query, vars []*Var, limit int) (*Result, error) {
	if err := s.send("(set-option :produce-models true)\n" + Script(q)); err != nil {
		return nil, err
	}
	result := &Result{}
	// A failure past the first configuration carries the ones already reported,
	// so a deadline mid-enumeration does not discard them.
	fail := func(err error) (*Result, error) {
		if len(result.Solutions) == 0 {
			return nil, err
		}
		result.Status, result.Truncated, result.Undecided = StatusSat, true, true
		return nil, &partialResult{result: result, err: err}
	}
	for {
		status, err := s.verdict()
		if err != nil {
			return fail(err)
		}
		switch status {
		case StatusUnsat:
			if len(result.Solutions) == 0 {
				result.Status = StatusUnsat
				return result, nil
			}
			// Every configuration was reported: the denials left nothing.
			result.Status = StatusSat
			return result, nil
		case StatusUnknown:
			result.Reason = s.reasonUnknown()
			if len(result.Solutions) == 0 {
				result.Status = StatusUnknown
				return result, nil
			}
			// The configurations found stand; whether others exist is undecided.
			result.Status, result.Truncated, result.Undecided = StatusSat, true, true
			return result, nil
		}
		values, err := s.values(vars)
		if err != nil {
			return fail(err)
		}
		result.Solutions = append(result.Solutions, values)
		if len(result.Solutions) >= limit {
			result.Status, result.Truncated, result.AtBound = StatusSat, true, true
			return result, nil
		}
		if err := s.deny(values); err != nil {
			return fail(err)
		}
	}
}

// partialResult carries the answers a dialogue had already reported when it
// failed, so a deadline is answered with them rather than with nothing.
type partialResult struct {
	result *Result
	err    error
}

// Error reports the failure that ended the dialogue.
func (e *partialResult) Error() string { return e.err.Error() }

// Unwrap returns that failure, so a caller not reading the partial answers sees
// the error it always did.
func (e *partialResult) Unwrap() error { return e.err }

// foundBeforeDeadline is the partial answers a failed dialogue reported, or nil
// when it reported none.
func foundBeforeDeadline(err error) *Result {
	var partial *partialResult
	if errors.As(err, &partial) {
		return partial.result
	}
	return nil
}

// deny excludes the configuration just reported and asks again, so the next
// `check-sat` answers about a different combination.
func (s *session) deny(values []Assignment) error {
	clause, err := s.blocking(values)
	if err != nil {
		return err
	}
	return s.send("; " + Role(RoleExcluded).String() + ": a configuration already reported\n" +
		"(assert (not " + clause + "))\n(check-sat)\n")
}

// blocking builds the conjunction that holds exactly of the assignment reported,
// from the values decoded into the term language rather than from the solver's
// own text, so what is denied is a combination of values the sorts declare.
func (s *session) blocking(values []Assignment) (string, error) {
	parts := make([]string, 0, len(values))
	for _, a := range values {
		value, err := DecodeValue(a)
		if err != nil {
			return "", s.solver.processError("get-value",
				"its model gave "+a.Raw+" for "+a.Var.Name+", which is no value of "+a.Var.Sort.Name+": "+err.Error(),
				s.stderrText(), nil)
		}
		literal, err := value.literal(a.Var.Sort)
		if err != nil {
			return "", s.solver.processError("get-value",
				"its model gave "+a.Raw+" for "+a.Var.Name+", which the term language cannot deny: "+err.Error(),
				s.stderrText(), nil)
		}
		parts = append(parts, writeTerm(Binary(OpEq, Bool, VarTerm(a.Var), literal)))
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return "(and " + strings.Join(parts, " ") + ")", nil
}
