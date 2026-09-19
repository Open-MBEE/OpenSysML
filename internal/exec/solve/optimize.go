package solve

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// OptimumStatus is what came of asking for one objective's optimum. The cases
// stay distinct: a bound no assignment attains is never reported as a value, and
// an answer that did not survive verification is never reported as an optimum.
type OptimumStatus int

const (
	// OptimumAttained means the value reported is the optimum and an assignment
	// attains it, both checked here rather than taken on the solver's word.
	OptimumAttained OptimumStatus = iota

	// OptimumBounded means the objective approaches a bound no assignment
	// attains, which is what a solver reports as an infinitesimal or an interval.
	OptimumBounded

	// OptimumUnbounded means the conditions permit arbitrarily better values, so
	// there is no optimum to report.
	OptimumUnbounded

	// OptimumUnverified means the solver reported an optimum that verification
	// refuted: a better value is feasible. Only the feasible value found stands.
	OptimumUnverified

	// OptimumUndecided means verification itself was not decided, so whether the
	// value reported is the optimum is unknown.
	OptimumUndecided
)

// String names the outcome in the terms a report uses.
func (s OptimumStatus) String() string {
	switch s {
	case OptimumAttained:
		return "attained"
	case OptimumBounded:
		return "bounded"
	case OptimumUnbounded:
		return "unbounded"
	case OptimumUnverified:
		return "unverified"
	default:
		return "undecided"
	}
}

// Optimum is what a solver answered about one objective.
type Optimum struct {
	// Objective is the objective asked about.
	Objective Objective

	// Status says what came of asking.
	Status OptimumStatus

	// Value is the optimum in the notation's own terms, with the units its
	// magnitude is expressed in; empty unless Status is OptimumAttained.
	Value string

	// Bound is the value the objective approaches without attaining it, in the
	// notation's own terms; empty unless Status is OptimumBounded.
	Bound string

	// Feasible is a value the reported model attains, in the notation's own
	// terms: a witness the conditions permit, not an optimum.
	Feasible string

	// Raw is the solver's own expression for the optimum.
	Raw string

	// Detail says why an optimum was not reported, empty when one was.
	Detail string
}

// Optimize discovers a solver and asks it for the query's optima.
func Optimize(ctx context.Context, q *Query) (*Result, error) {
	solver, err := Discover()
	if err != nil {
		return nil, err
	}
	return solver.Optimize(ctx, q)
}

// Optimize asks the solver for the optimum of each objective, in the
// lexicographic order the query declares them.
//
// Optimization is a z3 extension rather than SMT-LIB, so a backend without it is
// reported rather than degraded to a plain satisfiability check, and every
// optimum a backend does report is verified here: that an assignment attains it,
// and that no assignment does lexicographically better. A solver whose answer
// fails either check reports no optimum, only the feasible value it found.
func (s *Solver) Optimize(ctx context.Context, q *Query) (*Result, error) {
	if q == nil {
		return nil, fmt.Errorf("solve: no query to optimize")
	}
	if !q.Optimizes() {
		return nil, &NoObjectiveError{Element: q.Element}
	}
	if err := s.requireOptimization(ctx, q); err != nil {
		return nil, err
	}
	return s.solve(ctx, q, func(sess *session) (*Result, error) { return sess.optimize(q) })
}

// requireOptimization settles from the capability model, before a query is sent,
// whether the backend implements the optimization commands the script emits, so
// an unsupported backend is never asked and its answer never misread.
func (s *Solver) requireOptimization(ctx context.Context, q *Query) error {
	err := s.require(ctx, q, "optimizing", CapOptimization, CapOptimizationPriority)
	var unsupported *UnsupportedCapabilityError
	// A capability the query needs for another reason keeps its own report: only an
	// optimization extension makes this a solver that does not optimize.
	if errors.As(err, &unsupported) && optimizationCapability(unsupported.Missing[0]) {
		// A backend refusing optimization is reported as the extension it lacks,
		// which says which solver to run instead.
		return &NoOptimizationError{
			Solver: unsupported.Solver,
			Detail: "lacks " + unsupported.Missing[0].Feature() + ": " + unsupported.Detail,
			Cause:  err,
		}
	}
	return err
}

// optimizationCapability reports whether the capability is one of the optimization
// extensions rather than something the query needs anyway.
func optimizationCapability(c Capability) bool {
	return c == CapOptimization || c == CapOptimizationPriority
}

// optimize holds the optimizing dialogue: the script with its objectives, the
// optima the solver reports, the model attaining them, and the checks that say
// whether that model is optimal.
func (s *session) optimize(q *Query) (*Result, error) {
	if err := s.send("(set-option :produce-models true)\n" + Script(q)); err != nil {
		return nil, err
	}
	status, err := s.verdict()
	if err != nil {
		return nil, err
	}
	result := &Result{Status: status}
	switch status {
	case StatusUnsat:
		return result, nil
	case StatusUnknown:
		result.Reason = s.reasonUnknown()
		return result, nil
	}
	reported, err := s.reportedOptima(q)
	if err != nil {
		return nil, err
	}
	// The model must be read before any further check-sat, as a solver keeps
	// only the model of its latest one.
	model, err := s.values(q.Vars)
	if err != nil {
		return nil, err
	}
	result.Model = model
	attained, err := s.attained(q)
	if err != nil {
		return nil, err
	}
	verified, err := s.verifyOptimal(q, attained)
	if err != nil {
		return nil, err
	}
	result.Optima = classifyOptima(q, reported, attained, verified)
	return result, nil
}

// msgAnswered opens a report of what the solver said in place of an answer.
const msgAnswered = "answered "

// reportedOptima asks for the optima, one entry per objective in the order the
// script emitted them. It is asked only of a satisfiable query, so the dialogue
// carries no reply an unsatisfiable one would leave unread.
func (s *session) reportedOptima(q *Query) ([]sexpr, error) {
	if err := s.send("(get-objectives)\n"); err != nil {
		return nil, err
	}
	reply, err := s.read("get-objectives")
	if err != nil {
		return nil, err
	}
	if msg, ok := reply.isError(); ok {
		return nil, &NoOptimizationError{Solver: s.solver.Name, Detail: "rejected (get-objectives): " + msg}
	}
	// SMT-LIB's own answer for a command a backend does not implement.
	if reply.Atom == "unsupported" {
		return nil, &NoOptimizationError{Solver: s.solver.Name,
			Detail: msgAnswered + quoteReply(reply) + " to (get-objectives)"}
	}
	if !reply.IsList || len(reply.List) == 0 || reply.List[0].Atom != "objectives" {
		return nil, s.optimumError("", msgAnswered+quoteReply(reply)+" rather than its objectives")
	}
	entries := reply.List[1:]
	if len(entries) != len(q.Objectives) {
		return nil, s.optimumError("", fmt.Sprintf("reported %d optima for %d objectives",
			len(entries), len(q.Objectives)))
	}
	out := make([]sexpr, 0, len(entries))
	for i, entry := range entries {
		// Each entry is `(<term> <value>)`, the term as the solver prints it.
		if !entry.IsList || len(entry.List) != 2 {
			return nil, s.optimumError(objectiveName(q.Objectives[i]),
				"reported "+quoteReply(entry)+" rather than an objective and its optimum")
		}
		out = append(out, entry.List[1])
	}
	return out, nil
}

// attained reads what each objective's term evaluates to in the model reported,
// which is what says whether the model attains the optimum.
func (s *session) attained(q *Query) ([]*big.Rat, error) {
	out := make([]*big.Rat, 0, len(q.Objectives))
	for _, obj := range q.Objectives {
		if err := s.send("(get-value (" + writeTerm(obj.Term) + "))\n"); err != nil {
			return nil, err
		}
		reply, err := s.read("get-value")
		if err != nil {
			return nil, err
		}
		if msg, ok := reply.isError(); ok {
			return nil, s.solver.processError("get-value",
				"it would not evaluate the objective: "+msg, s.stderrText(), nil)
		}
		if !reply.IsList || len(reply.List) != 1 || !reply.List[0].IsList || len(reply.List[0].List) != 2 {
			return nil, s.optimumError(objectiveName(obj),
				msgAnswered+quoteReply(reply)+" rather than the value its model gives the objective")
		}
		rat, ok := ratOfSexpr(reply.List[0].List[1])
		if !ok {
			return nil, s.optimumError(objectiveName(obj),
				"gave the objective the value "+quoteReply(reply.List[0].List[1])+", which is no number")
		}
		out = append(out, rat)
	}
	return out, nil
}

// verifyOptimal checks the values the model attains for being the lexicographic
// optimum, by asking whether any assignment does better: better in an earlier
// objective, or equal there and better in a later one. Unsat is what makes them
// optimal; the solver's own claim is not taken for it. The verdict is returned as
// it came, so an undecided check is not read as a refutation.
func (s *session) verifyOptimal(q *Query, attained []*big.Rat) (Status, error) {
	clauses := make([]*Term, 0, len(q.Objectives))
	for i, obj := range q.Objectives {
		parts := make([]*Term, 0, i+1)
		for j := 0; j < i; j++ {
			parts = append(parts, Binary(OpEq, Bool, q.Objectives[j].Term, ratTerm(q.Objectives[j].Term.Sort, attained[j])))
		}
		parts = append(parts, better(obj, ratTerm(obj.Term.Sort, attained[i])))
		clauses = append(clauses, And(parts...))
	}
	if err := s.send("(push 1)\n(assert " + writeTerm(Or(clauses...)) + ")\n(check-sat)\n"); err != nil {
		return StatusUnknown, err
	}
	status, err := s.verdict()
	if err != nil {
		return StatusUnknown, err
	}
	if err := s.send("(pop 1)\n"); err != nil {
		return StatusUnknown, err
	}
	return status, nil
}

// better is the term saying the objective takes a strictly better value than the
// one given, which is what its direction means.
func better(obj Objective, value *Term) *Term {
	op := OpLt
	if obj.Direction == Maximize {
		op = OpGt
	}
	return Binary(op, Bool, obj.Term, value)
}

// ratTerm is a numeric literal of the sort given, an integer objective's values
// being integers.
func ratTerm(sort Sort, rat *big.Rat) *Term {
	if sort.Kind == SortInt && rat.IsInt() {
		return IntTerm(rat.Num().Int64())
	}
	return RealTerm(new(big.Rat).Set(rat))
}

// classifyOptima says, per objective, what the solver's answer and the checks
// together establish.
func classifyOptima(q *Query, reported []sexpr, attained []*big.Rat, verified Status) []Optimum {
	out := make([]Optimum, 0, len(q.Objectives))
	for i, obj := range q.Objectives {
		out = append(out, classifyOptimum(obj, reported[i], attained[i], verified))
	}
	return out
}

// classifyOptimum reads one reported optimum and settles what may be said about
// it. Nothing is reported as an optimum that verification did not confirm.
func classifyOptimum(obj Objective, reported sexpr, attained *big.Rat, verified Status) Optimum {
	out := Optimum{
		Objective: obj,
		Raw:       reported.String(),
		Feasible:  renderMagnitude(obj, attained),
	}
	value := parseOptimum(reported, obj.Direction)
	switch value.kind {
	case optimumInfinite:
		out.Status = OptimumUnbounded
		out.Detail = "the conditions permit arbitrarily " + betterWord(obj.Direction) + " values"
		return out
	case optimumUnreadable:
		out.Status = OptimumUndecided
		out.Detail = "the solver reported the optimum as " + quoteReply(reported) + ", which is no number"
		return out
	case optimumBound:
		out.Status = OptimumBounded
		out.Bound = renderMagnitude(obj, value.rat)
		out.Detail = "the objective approaches this bound without any assignment attaining it"
		return out
	}
	switch {
	case verified == StatusSat:
		out.Status = OptimumUnverified
		out.Detail = "the solver reported " + renderMagnitude(obj, value.rat) +
			", but a strictly " + betterWord(obj.Direction) + " value is feasible, so it is no optimum"
	case verified == StatusUnknown:
		out.Status = OptimumUndecided
		out.Detail = "the solver reported " + renderMagnitude(obj, value.rat) +
			", but did not decide whether a " + betterWord(obj.Direction) + " value is feasible"
	case value.rat.Cmp(attained) != 0:
		out.Status = OptimumBounded
		out.Bound = renderMagnitude(obj, value.rat)
		out.Detail = "no assignment reported attains this value"
	default:
		out.Status = OptimumAttained
		out.Value = renderMagnitude(obj, attained)
	}
	return out
}

// betterWord names which way a direction improves a value.
func betterWord(d Direction) string {
	if d == Maximize {
		return "greater"
	}
	return "smaller"
}

// renderMagnitude writes an objective's value as the notation does, with the base
// units its magnitude is expressed in.
func renderMagnitude(obj Objective, rat *big.Rat) string {
	if obj.Term.Sort.Kind == SortInt && rat.IsInt() {
		return withUnit(rat.Num().String(), obj.Unit, obj.Dimension)
	}
	return withUnit(renderRat(rat), obj.Unit, obj.Dimension)
}

// objectiveName names an objective for a message about it.
func objectiveName(obj Objective) string {
	if obj.Name != "" {
		return "objective " + obj.Name
	}
	return "the objective " + obj.Expression
}

// optimumError builds a failure of a solver that answered sat but would not
// report a readable optimum.
func (s *session) optimumError(objective, detail string) error {
	return &OptimumError{
		Solver:    s.solver.Name,
		Objective: objective,
		Detail:    detail,
		Stderr:    s.stderrText(),
	}
}

// optimumKind is the form a solver reported an optimum in.
type optimumKind int

const (
	// optimumFinite is an exact value.
	optimumFinite optimumKind = iota
	// optimumInfinite is an unbounded objective, which z3 writes as `oo`.
	optimumInfinite
	// optimumBound is a value approached but not attained, which z3 writes with
	// an infinitesimal or as an interval.
	optimumBound
	// optimumUnreadable is anything else.
	optimumUnreadable
)

// reportedOptimum is a parsed optimum: its form and, where it has one, the exact
// value or bound.
type reportedOptimum struct {
	kind optimumKind
	rat  *big.Rat
}

// parseOptimum reads the forms a solver reports an optimum in: an exact value, an
// infinity, a value offset by an infinitesimal, or an interval the optimum lies
// in, whose bound in the direction of improvement is what it approaches.
func parseOptimum(value sexpr, dir Direction) reportedOptimum {
	if sign, ok := infinity(value); ok {
		// An infinity in the direction of improvement is what unbounded means; one
		// against it is no answer about this objective.
		if (dir == Maximize) == (sign > 0) {
			return reportedOptimum{kind: optimumInfinite}
		}
		return reportedOptimum{kind: optimumUnreadable}
	}
	if value.IsList && len(value.List) == 3 && value.List[0].Atom == "interval" {
		side := value.List[1]
		if dir == Maximize {
			side = value.List[2]
		}
		bound := parseOptimum(side, dir)
		if bound.kind == optimumFinite {
			// An interval is a range the optimum lies in rather than a value.
			return reportedOptimum{kind: optimumBound, rat: bound.rat}
		}
		return bound
	}
	if hasEpsilon(value) {
		if rat, ok := ratIgnoringEpsilon(value); ok {
			return reportedOptimum{kind: optimumBound, rat: rat}
		}
		return reportedOptimum{kind: optimumUnreadable}
	}
	if rat, ok := ratOfSexpr(value); ok {
		return reportedOptimum{kind: optimumFinite, rat: rat}
	}
	return reportedOptimum{kind: optimumUnreadable}
}

// infinity reads `oo` and `(- oo)`, with the sign of the infinity.
func infinity(value sexpr) (int, bool) {
	if !value.IsList {
		if strings.EqualFold(value.Atom, "oo") {
			return 1, true
		}
		return 0, false
	}
	if len(value.List) == 2 && value.List[0].Atom == "-" {
		if _, ok := infinity(value.List[1]); ok {
			return -1, true
		}
	}
	return 0, false
}

// hasEpsilon reports whether an infinitesimal appears in the expression, which is
// how a solver says a bound is approached rather than attained.
func hasEpsilon(value sexpr) bool {
	if !value.IsList {
		return value.Atom == "epsilon" && !value.Quoted
	}
	for _, arg := range value.List {
		if hasEpsilon(arg) {
			return true
		}
	}
	return false
}

// ratIgnoringEpsilon reads the value a bound expression carries beside its
// infinitesimal part: `(+ 10.5 (* (- 1.0) epsilon))` bounds the objective at 10.5.
func ratIgnoringEpsilon(value sexpr) (*big.Rat, bool) {
	if !hasEpsilon(value) {
		return ratOfSexpr(value)
	}
	if !value.IsList {
		// A bare infinitesimal offsets zero.
		return new(big.Rat), true
	}
	switch {
	case len(value.List) >= 2 && value.List[0].Atom == "+":
		sum := new(big.Rat)
		for _, arg := range value.List[1:] {
			if hasEpsilon(arg) {
				continue
			}
			rat, ok := ratOfSexpr(arg)
			if !ok {
				return nil, false
			}
			sum.Add(sum, rat)
		}
		return sum, true
	case len(value.List) == 2 && value.List[0].Atom == "-":
		rat, ok := ratIgnoringEpsilon(value.List[1])
		if !ok {
			return nil, false
		}
		return rat.Neg(rat), true
	case len(value.List) >= 2 && value.List[0].Atom == "*":
		// A product with an infinitesimal is itself infinitesimal, so it offsets
		// zero.
		return new(big.Rat), true
	}
	return nil, false
}
