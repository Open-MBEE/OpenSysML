package smt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Input is one feature of the initial state as the encoding takes it: free in
// the domain its declared type narrows its sort to, or pinned at the value the
// model binds it to, in the order the performance declares them.
type Input struct {
	// Name is the feature as the performance holds it, as `-check-input` spells it.
	Name string
	// Type is the declared type as written, "" without one.
	Type string
	// Var is the variable every state carries for the feature.
	Var *solve.Var
	// Free reports whether state 0 ranges over Domain rather than pinning Value.
	Free bool
	// Released reports whether the model binds the feature and the question freed it.
	Released bool
	// Optional reports a free feature whose multiplicity admits no value: state 0
	// ranges over its absence too, which a witness spells `null`.
	Optional bool
	// Domain spells what narrows the sort (`>= 0`, `{Fast, Slow}`); "" when the sort alone is the domain.
	Domain string
	// Value is the value the performance holds at its start, ValInvalid when it holds none.
	Value runtime.Value

	def ast.Node // the default the model wrote, nil without one
}

// absentInput is how a witness spells an optional input the solver left without
// a value: the notation's `null`, which a replay fixes the feature at.
const absentInput = "null"

// frees decides whether a held feature is free in state 0: released by the
// question, or an input the performance holds no value for.
func (e *Encoding) frees(attr lower.Attribute) (free, released bool) {
	if e.released[attr.Name] {
		return true, true
	}
	return e.unbound[attr.Name], false
}

// noDomain is the refusal for a free input the encoding cannot range over, with
// the translator's reason when it was the one refusing.
func noDomain(attr lower.Attribute, err error) error {
	reason := err.Error()
	var refused *solve.NotTranslatableError
	if errors.As(err, &refused) {
		reason = refused.Reason
	}
	return &analysis.DomainError{Engine: EngineName, Feature: attr.Name, Type: attr.Type, Reason: reason}
}

// checkReleases refuses a released name that is no feature of the action, or one
// the action writes back rather than reads, before any query is built.
func (e *Encoding) checkReleases(action string) error {
	features := make(map[string]lower.Attribute, len(e.held.Features()))
	for _, attr := range e.held.Features() {
		features[attr.Name] = attr
	}
	for _, name := range e.releases {
		attr, ok := features[name]
		switch {
		case !ok:
			return &analysis.InputError{Engine: EngineName, Feature: name,
				Reason: fmt.Sprintf("action %s declares no such feature", action)}
		case attr.Output():
			return &analysis.InputError{Engine: EngineName, Feature: name,
				Reason: fmt.Sprintf("action %s writes it back rather than reading it", action)}
		}
	}
	return nil
}

// domainText spells the domain of v: a datatype's constructors by their own
// names, else the bound its declaration puts on it, else nothing.
func (e *Encoding) domainText(v *solve.Var) string {
	if v.Sort.Kind == solve.SortDatatype {
		names := make([]string, 0, len(v.Sort.Values))
		for _, value := range v.Sort.Values {
			names = append(names, strings.TrimPrefix(value, v.Sort.Origin+"::"))
		}
		return "{" + strings.Join(names, ", ") + "}"
	}
	d, ok := e.domains[v.Name]
	if !ok {
		return ""
	}
	return boundText(d)
}

// boundText spells a declaration's bound on a variable as `>= 0`: the comparison
// and its literal side, for a domain of that shape.
func boundText(d *solve.Term) string {
	if len(d.Args) != 2 || d.Args[0].Op != solve.OpVar || !d.Args[1].Literal() {
		return ""
	}
	var op string
	switch d.Op {
	case solve.OpLt:
		op = "<"
	case solve.OpLe:
		op = "<="
	case solve.OpGt:
		op = ">"
	case solve.OpGe:
		op = ">="
	case solve.OpEq:
		op = "="
	case solve.OpNe:
		op = "!="
	default:
		return ""
	}
	return op + " " + literalText(d.Args[1])
}

// literalText spells a literal term as the notation writes it.
func literalText(t *solve.Term) string {
	switch t.Op {
	case solve.OpBool:
		return fmt.Sprint(t.Bool)
	case solve.OpInt:
		return fmt.Sprint(t.Int)
	case solve.OpReal:
		return t.Real.RatString()
	case solve.OpString:
		return fmt.Sprintf("%q", t.Str)
	case solve.OpValue:
		return t.Str
	}
	return ""
}
