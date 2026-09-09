package repl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// VerdictStatus is what a checked condition answered.
type VerdictStatus int

const (
	// VerdictHolds is a condition the model satisfies.
	VerdictHolds VerdictStatus = iota
	// VerdictFails is a condition the model answered false.
	VerdictFails
	// VerdictUnresolved is a check that was never decided — an unknown name, a
	// session with no declarations, an element stating no assertion, an
	// evaluation that could not be carried out — which is a question about the
	// model, not an answer from it.
	VerdictUnresolved
)

// String names the status for a report a machine reads.
func (s VerdictStatus) String() string {
	switch s {
	case VerdictHolds:
		return "holds"
	case VerdictFails:
		return "fails"
	default:
		return "unresolved"
	}
}

// Verdict is the outcome of one checked constraint, requirement or satisfaction
// assertion. It carries both the status a caller decides on (an exit code, a
// report) and the lines the REPL prints, so the prompt and a non-interactive
// caller always report the same verdict from the same evaluation.
type Verdict struct {
	// Subject is what was checked, spelled as the caller named it.
	Subject string
	Status  VerdictStatus
	Lines   []string
	// Values are what a run produced, for a caller reporting more than a status.
	Values []NamedValue
	// Verifications are the verdicts the bodies of the verification cases
	// verifying the checked requirement answered, reported beside its status
	// rather than deciding it.
	Verifications []VerificationVerdict
	// Rows are the runs a sweep made, one per row of its table; no other kind of
	// verdict has any.
	Rows []VerdictRow
	// Outcomes are the distinct outcomes an exploration reached, in canonical
	// order, and Exploration how it ended; only an explored run has them.
	Outcomes    []VerdictOutcome
	Exploration *VerdictExploration
}

// VerdictRow is one run of a sweep: what it was given, what it produced and
// decided, how long it took, and what stopped it when it failed.
type VerdictRow struct {
	Inputs   []NamedValue
	Outputs  []NamedValue
	Verdicts []NamedValue
	Millis   float64
	Error    string
}

// Holds reports whether the checked condition is satisfied.
func (v Verdict) Holds() bool { return v.Status == VerdictHolds }

// WorstStatus is the status a set of verdicts should be judged by: unresolved
// outranks a failure, since a check that was never made says nothing about the
// model. An empty set holds.
func WorstStatus(verdicts []Verdict) VerdictStatus {
	worst := VerdictHolds
	for _, v := range verdicts {
		if v.Status > worst {
			worst = v.Status
		}
	}
	return worst
}

// failedStatus is the status of a condition that did not hold: the model
// answering false, or, for any other evaluation error, a check that was never
// decided — an unbound subject or feature is not evidence against the model.
func failedStatus(err error) VerdictStatus {
	var violation *runtime.ViolationError
	if err == nil || errors.As(err, &violation) || errors.Is(err, runtime.ErrViolated) {
		return VerdictFails
	}
	return VerdictUnresolved
}

// unresolvedVerdict reports a check that could not be made, phrased as the
// prompt reports any command it cannot carry out.
func unresolvedVerdict(subject, msg string) Verdict {
	return Verdict{Subject: subject, Status: VerdictUnresolved, Lines: []string{"error: " + msg}}
}

// unevaluable reports whether an evaluation error left a check undecided, as
// against the model itself answering the condition false.
func unevaluable(err error) bool {
	return err != nil && failedStatus(err) == VerdictUnresolved
}

// unevaluableVerdict reports a condition whose evaluation could not be carried
// out, saying so and naming why: it decided nothing, so its text must not read
// as a verdict against the model. what names the condition, e.g. "Constraint C".
func unevaluableVerdict(subject, what string, err error, inst *runtime.Instance, owner string) Verdict {
	return Verdict{Subject: subject, Status: VerdictUnresolved, Lines: []string{
		fmt.Sprintf("? %s could not be evaluated%s", what, onInstance(inst, owner)),
		fmt.Sprintf("  Error: %v", err),
	}}
}

// withTrace prefixes a verdict with the execution trace its evaluation produced,
// so a caller outside the prompt reports what `%trace on` reports there.
func (s *Session) withTrace(v Verdict) Verdict {
	if trace := s.drainTrace(); len(trace) > 0 {
		v.Lines = append(trace, v.Lines...)
	}
	return v
}

// CheckConstraint evaluates a constraint definition, against the object that
// carries it when one has been created, so the verdict is about concrete values.
func (s *Session) CheckConstraint(name string) Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.withTrace(s.checkConstraint(name))
}

func (s *Session) checkConstraint(name string) Verdict {
	target, bad := s.resolveCheckTarget(name)
	if bad != nil {
		return *bad
	}

	// A name that declares something else is a wrong argument, not a verdict, and
	// is answered so before a subject is chosen for it.
	if err := runtime.RequireConstraint(target.sym); err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	inst, owner, bad := s.checkSubject(name, target)
	if bad != nil {
		return *bad
	}
	result, err := target.ctx.CheckConstraintOn(target.sym, target.scope, inst)
	inst, owner = s.reportedSubject(result, inst, owner)
	if unevaluable(err) {
		return unevaluableVerdict(name, "Constraint "+name, err, inst, owner)
	}
	if err != nil || !result.Holds {
		return Verdict{Subject: name, Status: VerdictFails, Lines: []string{
			fmt.Sprintf("✗ Constraint %s failed%s", name, onInstance(inst, owner)),
			"  " + verdictDetail("Assertion", err),
		}}
	}
	return Verdict{Subject: name, Status: VerdictHolds, Lines: []string{
		fmt.Sprintf("✓ Constraint %s passed%s", name, onInstance(inst, owner)),
	}}
}

// CheckRequirement evaluates a requirement definition, against the object that
// carries it when one has been created.
func (s *Session) CheckRequirement(name string) Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.withTrace(s.checkRequirement(name))
}

func (s *Session) checkRequirement(name string) Verdict {
	target, bad := s.resolveCheckTarget(name)
	if bad != nil {
		return *bad
	}

	// As for a constraint: a name of another kind is a wrong argument, settled
	// before a subject is chosen.
	if err := runtime.RequireRequirement(target.sym); err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	inst, owner, bad := s.checkSubject(name, target)
	if bad != nil {
		return *bad
	}
	result, err := target.ctx.CheckRequirementOn(target.sym, target.scope, inst)
	inst, owner = s.reportedSubject(result, inst, owner)
	if unevaluable(err) {
		return s.withVerifications(unevaluableVerdict(name, "Requirement "+name, err, inst, owner), target.ctx, target.sym)
	}
	if err != nil || !result.Holds {
		return s.withVerifications(Verdict{Subject: name, Status: VerdictFails, Lines: []string{
			fmt.Sprintf("✗ Requirement %s failed%s", name, onInstance(inst, owner)),
			"  " + verdictDetail("Required condition", err),
		}}, target.ctx, target.sym)
	}
	return s.withVerifications(Verdict{Subject: name, Status: VerdictHolds, Lines: []string{
		fmt.Sprintf("✓ Requirement %s satisfied%s", name, onInstance(inst, owner)),
	}}, target.ctx, target.sym)
}

// CheckSatisfy evaluates satisfaction assertions: every one the model states
// when name is empty, or, given a name, the ones the named element states — or
// that element itself, when it is a named satisfaction assertion. The usual
// `assert satisfy r by p;` is anonymous, so the element stating it is how a
// caller reaches it.
func (s *Session) CheckSatisfy(name string) []Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.satisfyVerdicts(name)
}

// satisfyVerdicts is CheckSatisfy with the session already held.
func (s *Session) satisfyVerdicts(name string) []Verdict {
	verdicts := s.checkSatisfy(name)
	if len(verdicts) > 0 {
		// The trace of every assertion belongs to the report as a whole, which is
		// how the prompt reports it too.
		verdicts[0] = s.withTrace(verdicts[0])
	}
	return verdicts
}

func (s *Session) checkSatisfy(name string) []Verdict {
	docScopes := s.docScopes()
	if len(docScopes) == 0 {
		return []Verdict{unresolvedVerdict(name, "no declarations loaded")}
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []Verdict{unresolvedVerdict(name, err.Error())}
	}

	scopes := docScopes
	where := "the session"
	if name != "" {
		sym, fqn, lerr := s.lookupSymbol(name)
		if lerr != nil {
			return []Verdict{unresolvedVerdict(name, lerr.Error())}
		}
		if a, aerr := ctx.SatisfyAssertionOf(sym); aerr == nil {
			return []Verdict{s.satisfyVerdict(ctx, a)}
		}
		if sym.Scope == nil {
			return []Verdict{unresolvedVerdict(name, fmt.Sprintf("%s states no satisfaction assertion", name))}
		}
		scopes, where = []*symbols.Scope{sym.Scope}, fqn
	}

	var assertions []*runtime.SatisfyAssertion
	for _, scope := range scopes {
		assertions = append(assertions, ctx.SatisfyAssertionsIn(scope)...)
	}
	if len(assertions) == 0 {
		// Nothing was checked, so nothing is claimed about the model.
		return []Verdict{unresolvedVerdict(name, fmt.Sprintf("no satisfaction assertion in %s", where))}
	}
	verdicts := make([]Verdict, 0, len(assertions))
	for _, a := range assertions {
		verdicts = append(verdicts, s.satisfyVerdict(ctx, a))
	}
	return verdicts
}

// checkTarget is a resolved element to evaluate the conditions of: the symbol,
// the name instances of it are keyed under, the scope its conditions were
// written in, and the runtime to evaluate them.
type checkTarget struct {
	sym   *symbols.Symbol
	fqn   string
	scope *symbols.Scope
	ctx   *runtime.Context
}

// checkSubject is the object a check is about, as `%eval` answers about the same
// one. A subject that is a question yields the third result.
func (s *Session) checkSubject(name string, target checkTarget) (*runtime.Instance, string, *Verdict) {
	inst, owner, err := s.subjectFor(name, target.fqn, target.sym)
	if err != nil {
		bad := unresolvedVerdict(name, err.Error())
		return nil, "", &bad
	}
	return inst, owner, nil
}

// reportedSubject is the object a verdict is about and the name to report it
// under: the runtime may answer about an object nested in the one supplied, or
// in another the session holds, which is then named by the object it was reached
// from and the features walked to it.
func (s *Session) reportedSubject(result runtime.CheckResult, inst *runtime.Instance, owner string) (*runtime.Instance, string) {
	if result.Subject == nil || result.Subject == inst {
		return inst, owner
	}
	root := owner
	if result.SubjectRoot != inst || root == "" {
		root = s.instanceName(result.SubjectRoot)
	}
	if root == "" {
		// An object under no name this session can report is not named at all,
		// rather than reported under the wrong one.
		return inst, owner
	}
	return result.Subject, root + featurePath(result.SubjectPath)
}

// featurePath spells walked feature names as a label's tail, each after a `.`
// and quoted where its spelling needs it: `engine`, `mount` → `.engine.mount`.
func featurePath(names []string) string {
	var out strings.Builder
	for _, name := range names {
		out.WriteString(".")
		out.WriteString(lexer.NameText(name))
	}
	return out.String()
}

// instanceName is the label the session holds inst under — its name as the
// notation writes it, the first in name order when several do, `#<id>` for one
// displaced from its name — and empty for an object it did not create.
func (s *Session) instanceName(inst *runtime.Instance) string {
	if inst == nil {
		return ""
	}
	name := ""
	for held, obj := range s.instances {
		if obj == inst && (name == "" || held < name) {
			name = held
		}
	}
	if name != "" {
		return s.declaredName(name)
	}
	for _, u := range s.unnamed {
		if u.obj == inst {
			return fmt.Sprintf("#%d", inst.ID)
		}
	}
	return ""
}

// resolveCheckTarget resolves the element a constraint/requirement check names.
// The second result is non-nil for a check that cannot be made at all.
func (s *Session) resolveCheckTarget(name string) (checkTarget, *Verdict) {
	docScopes := s.docScopes()
	if len(docScopes) == 0 {
		bad := unresolvedVerdict(name, "no declarations loaded")
		return checkTarget{}, &bad
	}
	sym, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		bad := unresolvedVerdict(name, lerr.Error())
		return checkTarget{}, &bad
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		bad := unresolvedVerdict(name, err.Error())
		return checkTarget{}, &bad
	}
	scope := declaringScope(sym, docScopes[0])
	if sym.OwnerScope == nil {
		// The root fallback names the document declaring the symbol.
		for _, root := range docScopes {
			if scopeSymbolFor(root, sym.Decl) != nil {
				scope = root
				break
			}
		}
	}
	return checkTarget{sym: sym, fqn: fqn, scope: scope, ctx: ctx}, nil
}

// InstantiateNamed creates an object of the named definition and keeps it, so a
// later check about that name is a check about this object. It returns the lines
// `%instantiate` prints, and an error for a name the session cannot resolve or
// an instantiation the runtime rejected.
func (s *Session) InstantiateNamed(name string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.instantiateLines(name)
}

// instantiateLines is InstantiateNamed with the session already held.
func (s *Session) instantiateLines(name string) ([]string, error) {
	lines, err := s.instantiateNamed(name)
	if err != nil {
		return nil, err
	}
	return append(s.drainTrace(), lines...), nil
}

// behaviorsOf names what the object being unnamed is running, so a machine only
// its id now reaches is not overlooked.
func behaviorsOf(inst *runtime.Instance) string {
	if n := len(inst.Behaviors()); n > 0 {
		return fmt.Sprintf(", with %s", countOf(n, "behavior of its own", "behaviors of its own"))
	}
	return ""
}

func (s *Session) instantiateNamed(name string) ([]string, error) {
	// Resolved before the runtime is built, so a misspelling is reported as one
	// even when the session has nothing to instantiate from.
	sym, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		return nil, lerr
	}

	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("runtime init: %w", err)
	}

	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return nil, fmt.Errorf("instantiation failed: %w", err)
	}

	// Keyed by the resolved name, so %features finds the instance whichever
	// spelling of the name created it.
	previous, again := s.instances[fqn]
	displaced := again && previous != nil && previous.ID != inst.ID
	var relabelled []string
	if displaced {
		relabelled = s.releaseDebuggedName(fqn)
	}
	s.instances[fqn] = inst
	s.lost = instanceLoss{}
	out := []string{
		fmt.Sprintf("✓ Created instance of %s", s.declaredName(fqn)),
		fmt.Sprintf("  ID: %d", inst.ID),
	}
	// A second instantiation is a second object, so say which one the name now
	// denotes rather than let the earlier one look like it was reused.
	if displaced {
		s.unnamed = append(s.unnamed, unnamedObject{fqn: fqn, obj: previous})
		out = append(out, fmt.Sprintf("  note: %s now denotes this object; object #%d is displaced from that name%s and stays reachable as #%d",
			s.declaredName(fqn), previous.ID, behaviorsOf(previous), previous.ID))
		for _, notice := range relabelled {
			out = append(out, "  "+notice)
		}
	}
	return append(out, fmt.Sprintf("  Use %%features %s to inspect", name)), nil
}
