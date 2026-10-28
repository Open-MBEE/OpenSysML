package queryexec

import (
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// evaluateNamed resolves each qualified name to the one element it denotes: no
// element is an unknown-element path error, several an ambiguous-element model error.
func (e *executor) evaluateNamed(expression queryplan.Expression) (sequence, error) {
	names, err := e.stringsArgument(expression, "qualifiedName")
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	for _, name := range names {
		found, dangling := e.denoted(name)
		elements := elementsOf(found)
		switch {
		case len(elements) > 1:
			return sequence{}, e.ambiguousElement(expression, name, found)
		case len(elements) == 0:
			return sequence{}, e.unknownElement(expression, name, dangling)
		}
		result.values = append(result.values, valueAt(ElementValue(elements[0]), expression.Origin()))
	}
	return result, nil
}

// denoted resolves name to the distinct elements it denotes, aliases followed
// and deduplicated; dangling reports a candidate alias that denotes nothing.
// denotation is a registration the name matched — a declaration or an alias — and
// the element it denotes; every registration is kept, elementsOf dedups the targets.
type denotation struct {
	registration *symbols.Symbol
	element      *symbols.Symbol
}

func (e *executor) denoted(name string) (found []denotation, dangling bool) {
	for _, candidate := range e.context.Index.LookupUniqueCandidates(name) {
		element, ok := e.context.Resolver.ResolveAliasTarget(candidate)
		if !ok || element == nil {
			dangling = true
			continue
		}
		found = append(found, denotation{registration: candidate, element: element})
	}
	return found, dangling
}

// elementsOf is the distinct elements the registrations denote.
func elementsOf(found []denotation) []*symbols.Symbol {
	out := make([]*symbols.Symbol, 0, len(found))
	for _, d := range found {
		if !slices.ContainsFunc(out, func(o *symbols.Symbol) bool { return symbols.SameElement(o, d.element) }) {
			out = append(out, d.element)
		}
	}
	return out
}

// ambiguousElement reports a name several elements carry, each by its
// fully-qualified name, in name order.
func (e *executor) ambiguousElement(expression queryplan.Expression, name string, candidates []denotation) error {
	fqns := make([]string, 0, len(candidates))
	for _, element := range elementsOf(candidates) {
		fqns = append(fqns, symbols.FQNOf(element))
	}
	sort.Strings(fqns)
	// One finding per namespace declaring the duplicate names (an alias counts where it
	// is declared, not where its target is), each listing every element denoted there.
	byOwner := map[string][]string{}
	for _, candidate := range candidates {
		owner := ownerOf(symbols.FQNOf(candidate.registration))
		byOwner[owner] = append(byOwner[owner], symbols.FQNOf(candidate.element))
	}
	for owner := range byOwner {
		sort.Strings(byOwner[owner])
	}
	step := Step{Kind: StepNamed, Argument: name}
	owners := make([]string, 0, len(byOwner))
	for owner := range byOwner {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	findings := make([]Finding, 0, len(owners))
	for _, owner := range owners {
		findings = append(findings, Finding{Step: step, Element: owner, Constraint: ConstraintAmbiguousName, Observed: declaredAs(byOwner[owner])})
	}
	return &Error{
		Kind:       ErrorAmbiguousElement,
		Query:      e.definition.Name(),
		Operation:  expression.Operation(),
		Actual:     name,
		Candidates: fqns,
		Origin:     expression.Origin(),
		Outcome:    &Outcome{Model: &ModelError{Findings: findings}},
	}
}

// ownerOf is the namespace a qualified name is declared in; a root name owns itself.
func ownerOf(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[:i]
	}
	return fqn
}

// unknownElement reports a name no element carries with its frontier: the longest
// resolving prefix, the segment it lacks, the nearest spellings; or the alias denoting nothing.
func (e *executor) unknownElement(expression queryplan.Expression, name string, alias bool) error {
	failure := Failure{Step: Step{Kind: StepNamed, Argument: name}, Expected: expectedNamed}
	err := &Error{
		Kind:      ErrorUnknownElement,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Actual:    name,
		Origin:    expression.Origin(),
		Outcome:   &Outcome{Path: &PathError{Failures: []Failure{failure}}},
	}
	record := func() {
		failure.Frontier = Frontier{At: err.Target, Members: e.members(err.Target), Nearest: first(err.Candidates)}
		err.Outcome.Path.Failures[0] = failure
	}
	if alias {
		err.Path = []string{name}
		failure.Expected = "an alias denoting an element"
		failure.Frontier = Frontier{At: name}
		err.Outcome.Path.Failures[0] = failure
		return err
	}
	segments := symbols.SegmentsOf(name)
	for i := len(segments) - 1; i >= 1; i-- {
		prefix := source.QualifiedNameOf(segments[:i])
		found, dangling := e.denoted(prefix)
		owners := elementsOf(found)
		switch len(owners) {
		case 0:
			if dangling {
				// An alias on the path denotes nothing: that alias is the frontier.
				err.Path = []string{prefix}
				failure.Expected = "an alias denoting an element"
				failure.Frontier = Frontier{At: prefix}
				err.Outcome.Path.Failures[0] = failure
				return err
			}
			continue
		case 1:
			err.Target = symbols.FQNOf(owners[0])
			err.Expected = segments[i]
			err.Candidates = suggest.Nearest(segments[i], e.memberNames(err.Target))
			record()
			return err
		default:
			// The first segment that does not resolve is one several elements carry:
			// the frontier is that ambiguity, a finding about the model.
			return e.ambiguousElement(expression, prefix, found)
		}
	}
	err.Expected = segments[0]
	err.Candidates = suggest.Nearest(segments[0], suggest.SimpleNames(e.context.Index))
	record()
	return err
}

// expectedNamed is what Named asserts of its argument, as the Lean side words it.
const expectedNamed = "exactly one element with that qualified name"

func first(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return xs[0]
}

// memberNames lists the simple names a qualified reference from outside reaches
// directly under fqn, aliases included, in name order; a name only a private import surfaced is not one.
func (e *executor) memberNames(fqn string) []string {
	names := make([]string, 0)
	e.eachDeclared(fqn, func(name string, _ *symbols.Symbol) { names = append(names, name) })
	sort.Strings(names)
	return slices.Compact(names)
}

// members lists fqn's owned members as distinct (name, metaclass) pairs (for "" the root namespaces);
// an alias is a Membership of its element (KerML 8.2.3.5.2), not an owned member, so it is skipped.
func (e *executor) members(fqn string) []Member {
	out := []Member{}
	e.eachDeclared(fqn, func(name string, target *symbols.Symbol) {
		if target.Kind == symbols.SymbolAlias {
			return
		}
		out = append(out, Member{Name: name, Metaclass: e.metaclassOf(target)})
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Metaclass < out[j].Metaclass
	})
	return slices.Compact(out)
}

// eachDeclared visits every symbol registered directly under fqn with its simple name.
func (e *executor) eachDeclared(fqn string, visit func(name string, target *symbols.Symbol)) {
	for _, registered := range e.context.Index.FQNs() {
		rest := registered
		if fqn != "" {
			var ok bool
			if rest, ok = strings.CutPrefix(registered, fqn+"::"); !ok {
				continue
			}
		}
		if strings.Contains(rest, "::") {
			continue
		}
		for _, target := range e.context.Index.LookupQualified(registered) {
			visit(rest, target)
		}
	}
}

// metaclassOf is the simple name of the element's metaclass, `PartUsage`.
func (e *executor) metaclassOf(sym *symbols.Symbol) string {
	if metaclass := e.context.Model.MetaclassOf(sym); metaclass != nil {
		return symbols.LastSegment(metaclass.Name)
	}
	return ""
}
