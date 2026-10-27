package queryexec

import (
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// evaluateNamed resolves each qualified name, in argument order, to the one
// element it denotes, as WhereType resolves a type name. A name no element
// carries is an unknown-element error that says where the path stopped
// resolving; a name several elements carry is an ambiguous-element error that
// lists them. Neither is ever an empty result.
func (e *executor) evaluateNamed(expression queryplan.Expression) (sequence, error) {
	names, err := e.stringsArgument(expression, "qualifiedName")
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	for _, name := range names {
		candidates := e.context.Index.LookupUniqueCandidates(name)
		if len(candidates) > 1 {
			return sequence{}, e.ambiguousElement(expression, name, candidates)
		}
		var element *symbols.Symbol
		ok := false
		if len(candidates) == 1 {
			element, ok = e.context.Resolver.ResolveAliasTarget(candidates[0])
		}
		if !ok || element == nil {
			return sequence{}, e.unknownElement(expression, name, len(candidates) == 1)
		}
		result.values = append(result.values, valueAt(ElementValue(element), expression.Origin()))
	}
	return result, nil
}

// ambiguousElement reports a name several elements carry, each by its
// fully-qualified name, in name order.
func (e *executor) ambiguousElement(expression queryplan.Expression, name string, candidates []*symbols.Symbol) error {
	fqns := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		fqns = append(fqns, symbols.FQNOf(candidate))
	}
	sort.Strings(fqns)
	return &Error{
		Kind:       ErrorAmbiguousElement,
		Query:      e.definition.Name(),
		Operation:  expression.Operation(),
		Actual:     name,
		Candidates: fqns,
		Origin:     expression.Origin(),
	}
}

// unknownElement reports a name no element carries with the frontier the model
// offers: the longest prefix of the name that resolves (Target), the first
// segment it has no member for (Expected), and the members spelled nearest it
// (Candidates); with no resolving prefix, the simple names nearest the first
// segment. An alias that denotes no element (dangling, cyclic) says so instead.
func (e *executor) unknownElement(expression queryplan.Expression, name string, alias bool) error {
	err := &Error{
		Kind:      ErrorUnknownElement,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Actual:    name,
		Origin:    expression.Origin(),
	}
	if alias {
		err.Path = []string{name}
		return err
	}
	segments := strings.Split(name, "::")
	for i := len(segments) - 1; i >= 1; i-- {
		owner := e.context.Index.LookupUnique(strings.Join(segments[:i], "::"))
		if owner == nil {
			continue
		}
		err.Target = symbols.FQNOf(owner)
		err.Expected = segments[i]
		err.Candidates = suggest.Nearest(segments[i], e.memberNames(err.Target))
		return err
	}
	err.Expected = segments[0]
	err.Candidates = suggest.Nearest(segments[0], suggest.SimpleNames(e.context.Index))
	return err
}

// memberNames lists the simple names registered directly under the namespace
// fqn, in name order.
func (e *executor) memberNames(fqn string) []string {
	var out []string
	for _, registered := range e.context.Index.FQNs() {
		rest, ok := strings.CutPrefix(registered, fqn+"::")
		if ok && !strings.Contains(rest, "::") {
			out = append(out, rest)
		}
	}
	sort.Strings(out)
	return slices.Compact(out)
}
