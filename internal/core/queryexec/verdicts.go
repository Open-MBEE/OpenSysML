package queryexec

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Verdict rows report the assertions checked about an object: a session's
// object as it stands, or a model element's object as declared.

// verdictKinds are the values the kind argument of Verdicts accepts.
var verdictKinds = map[string]runtime.AssertionKind{
	"all":          "",
	"constraint":   runtime.AssertionConstraint,
	"requirement":  runtime.AssertionRequirement,
	"satisfaction": runtime.AssertionSatisfaction,
	"verification": AssertionVerification,
}

// verdictFQN declares the verdict properties a column expression may reference.
const verdictFQN = "DocumentQueries::Verdict"

// Verdict properties, beside the metadata every row answers.
const (
	propertyAssertion    = "assertion"
	propertyKind         = "kind"
	propertyCarrier      = "carrier"
	propertyPath         = "path"
	propertyVerdict      = "verdict"
	propertyCondition    = "condition"
	propertyReason       = "reason"
	propertyVerification = "verification"
)

// evaluateVerdicts checks the assertions about each source row's object and the
// objects it holds, one row per verdict, in the order the check reached them.
func (e *executor) evaluateVerdicts(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	kindName := "all"
	if hasArgument(expression, "kind") {
		if kindName, err = e.stringArgument(expression, "kind"); err != nil {
			return sequence{}, err
		}
	}
	wanted, ok := verdictKinds[kindName]
	if !ok {
		return sequence{}, e.invalidArgument(expression, "kind", kindName)
	}
	var result sequence
	for _, row := range source.values {
		verdicts, err := e.verdictsOf(expression, row)
		if err != nil {
			return sequence{}, err
		}
		for _, verdict := range verdicts {
			if !e.consumeVisit() {
				return sequence{}, e.budgetError(expression)
			}
			if wanted == "" || verdict.kind == wanted {
				result.values = append(result.values, VerdictValue(verdict))
			}
		}
	}
	return result, nil
}

// verdictsOf checks one row's object: a session object in the session, an
// element's declared object in the behavior-free declared context.
func (e *executor) verdictsOf(expression queryplan.Expression, row Value) ([]Verdict, error) {
	scopes := e.assertionScopes()
	if inst, label, isObject := row.Object(); isObject {
		report, err := e.context.Runtime.ValidateObject(inst, scopes)
		if err != nil {
			return nil, e.incompleteValidation(expression, row, err)
		}
		return e.reportVerdicts(expression, row, report, label, true, e.context.Runtime, scopes)
	}
	if verdict, isVerdict := row.Verdict(); isVerdict {
		return nil, e.verdictRowError(expression, "source", verdict)
	}
	sym, _ := row.Element()
	reader := e.derived.get(e.context)
	report, err := reader.Validate(sym, scopes)
	if err != nil {
		if errors.Is(err, runtime.ErrNotAnObject) {
			return nil, &Error{
				Kind:      ErrorNotAnObject,
				Query:     e.definition.Name(),
				Operation: expression.Operation(),
				Parameter: "source",
				Target:    symbols.FQNOf(sym),
				Origin:    expression.Origin(),
				Cause:     err,
			}
		}
		return nil, e.incompleteValidation(expression, row, err)
	}
	return e.reportVerdicts(expression, row, report, symbols.FQNOf(sym), false, reader, scopes)
}

// reportVerdicts converts a validation report to verdict rows about the row it
// was asked of; a verification row follows each requirement it verifies, once
// per requirement. A walk that left objects unreached is a typed error.
func (e *executor) reportVerdicts(
	expression queryplan.Expression,
	row Value,
	report runtime.ValidationReport,
	label string,
	held bool,
	verifier verifier,
	scopes []*symbols.Scope,
) ([]Verdict, error) {
	if len(report.Unread) > 0 {
		return nil, e.incompleteValidation(expression, row, report.Unread[0])
	}
	if report.Bounded {
		return nil, e.incompleteValidation(expression, row, errUnreachedObjects)
	}
	out := make([]Verdict, 0, len(report.Verdicts))
	verified := make(map[*symbols.Symbol]bool)
	for _, v := range report.Verdicts {
		var cases []runtime.VerificationVerdict
		if v.Requirement != nil {
			cases = e.derived.verifications(verifier, scopes, v.Requirement)
		}
		verdict := assertionVerdict(v, label, held, verificationKinds(cases))
		out = append(out, verdict)
		if v.Requirement == nil || verified[v.Requirement] {
			continue
		}
		verified[v.Requirement] = true
		for _, c := range cases {
			out = append(out, verificationVerdict(verdict, c))
		}
	}
	return out, nil
}

// incompleteValidation reports a row whose assertions could not all be checked.
func (e *executor) incompleteValidation(expression queryplan.Expression, row Value, cause error) error {
	return &Error{
		Kind:      ErrorIncompleteValidation,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Parameter: "source",
		Target:    rowTarget(row),
		Origin:    expression.Origin(),
		Cause:     cause,
	}
}

// errUnreachedObjects reports a validation walk that stopped short of the
// objects the root holds, whose assertions therefore went unchecked.
var errUnreachedObjects = errors.New("objects nested beyond the depth the validation descends were not checked")

// assertionScopes are the document scopes satisfactions and verification cases
// are looked for in: the workspace's documents, libraries aside.
func (e *executor) assertionScopes() []*symbols.Scope {
	names := e.context.Index.WorkspaceDocuments()
	out := make([]*symbols.Scope, 0, len(names))
	for _, name := range names {
		if root := e.context.Index.DocumentRoot(name); root != nil {
			out = append(out, root)
		}
	}
	return out
}

// verificationKinds lists the kinds of a requirement's verification verdicts.
func verificationKinds(cases []runtime.VerificationVerdict) []runtime.VerdictKind {
	if len(cases) == 0 {
		return nil
	}
	out := make([]runtime.VerdictKind, 0, len(cases))
	for _, c := range cases {
		out = append(out, c.Kind)
	}
	return out
}

// verifier runs the verification cases verifying a requirement: a session's
// runtime context, or the declared reader's behavior-free one.
type verifier interface {
	VerificationVerdictsIn(scopes []*symbols.Scope, req *symbols.Symbol) []runtime.VerificationVerdict
}

// verdictPropertyValues reads a property of a verdict row: the verdict's own
// properties first, then the metadata of the assertion it is about.
func (e *executor) verdictPropertyValues(row Value, property string) ([]Value, bool, error) {
	verdict, _ := row.Verdict()
	origin := row.Origin()
	text := func(values ...string) []Value {
		out := make([]Value, 0, len(values))
		for _, value := range values {
			out = append(out, valueAt(StringValue(value), origin))
		}
		return out
	}
	switch property {
	case propertyAssertion:
		if verdict.assertion == nil {
			return nil, true, nil
		}
		return []Value{valueAt(ElementValue(verdict.assertion), origin)}, true, nil
	case propertyKind:
		return text(string(verdict.kind)), true, nil
	case propertyCarrier:
		if verdict.carrier == nil {
			return nil, true, nil
		}
		if verdict.held {
			return []Value{valueAt(ObjectValue(verdict.carrier, verdict.path), origin)}, true, nil
		}
		if verdict.carrierDecl == nil {
			return nil, true, nil
		}
		return []Value{valueAt(ElementValue(verdict.carrierDecl), origin)}, true, nil
	case propertyPath:
		return text(verdict.path), true, nil
	case propertyVerdict:
		return text(verdict.status.String()), true, nil
	case propertyCondition:
		if verdict.condition == "" {
			return nil, true, nil
		}
		return text(verdict.condition), true, nil
	case propertyReason:
		if verdict.reason == "" {
			return nil, true, nil
		}
		return text(verdict.reason), true, nil
	case propertyVerification:
		kinds := verdict.verification
		out := make([]string, 0, len(kinds))
		for _, kind := range kinds {
			out = append(out, string(kind))
		}
		return text(out...), true, nil
	}
	if verdict.assertion == nil {
		return nil, false, nil
	}
	return e.propertyValues(ElementValue(verdict.assertion), property)
}
