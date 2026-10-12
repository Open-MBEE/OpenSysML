package solve

import (
	"fmt"
	"math"
	"math/big"
)

// RoundingSound returns a new query over the same variables and sorts plus one
// fresh Real variable per rounding site, whose models over-approximate every
// evaluation the runtime evaluator's binary64 arithmetic performs, as replay.go
// models it; Integer and Rational arithmetic is exact and needs none. An unsatisfiable rounding-sound query therefore proves the exact
// query's unsat holds under the evaluator's arithmetic too; the query itself is
// never replayed — only its verdict is used.
//
// A rounding site is a `+`, `-`, `*` or `/` a Real takes part in, or an exact
// operand of one, which the evaluator rounds once to binary64; an exact literal
// operand is replaced by the exact rational of its nearest float64. Negation is
// exact and is not a site. Identical rewritten subterms share one site.
//
// Each site variable r is bounded by the doubles the exact value v lies
// between: for every double z the axioms state `(v <= z) => r <= z` and
// `(v >= z) => r >= z`, where z is every Real variable, the value 0 and
// every rewritten real literal (all always doubles), and every other site r'
// whose guard is the condition under which the evaluator computes it. No guard
// bounds a site by itself.
//
// Soundness: assign each evaluated site its actual float64 result and each
// unevaluated site its exact value v. Round-to-nearest is monotone and fixes
// doubles, so the float64 result lies between the same doubles v does, and z is
// a double whenever its guard holds at that point of the evaluation. Every
// assignment the evaluator can produce is then a model, so an unsat verdict
// refutes float64 arithmetic as surely as exact arithmetic. Relative-error
// bounds are deliberately omitted: fewer axioms is still sound, since
// over-approximating evaluation is the sound direction.
//
// No case is needed for a non-finite site: the evaluator refuses one
// (constArithmetic via RealResult/MagnitudeArith; replayArithmetic mirrors it),
// so an overflowing assignment yields no verdict and every verdict computes
// finite sites. NaN cannot arise, and underflow to 0 is covered because 0 is a
// bound.
func (q *Query) RoundingSound() *Query {
	rw := &roundingRewrite{sites: map[string]*roundingSite{}}
	assertions := make([]Assertion, 0, len(q.Assertions))
	var earlier *Term
	for _, a := range q.Assertions {
		e := earlier
		if !conditionRole(a.From.Role) {
			e = BoolTerm(false)
		}
		term := rw.rewrite(a.Term, e)
		assertions = append(assertions, Assertion{Term: term, From: a.From})
		if conditionRole(a.From.Role) {
			earlier = and(earlier, term)
		}
	}

	doubles := rw.doubles(q)
	for _, site := range rw.order {
		axioms := site.axioms(doubles, q)
		assertions = append(assertions, axioms...)
	}

	vars := make([]*Var, 0, len(q.Vars)+len(rw.order))
	vars = append(vars, q.Vars...)
	for _, site := range rw.order {
		vars = append(vars, site.variable)
	}
	sortVars(vars)

	sorts := append([]Sort(nil), q.Sorts...)
	sortSorts(sorts)
	return &Query{
		Kind:            q.Kind,
		Element:         q.Element,
		Negated:         q.Negated,
		Violation:       q.Violation,
		Sorts:           sorts,
		Vars:            vars,
		Assertions:      assertions,
		Nonlinear:       q.Nonlinear,
		IntegerDivision: q.IntegerDivision,
		Pinned:          q.Pinned,
		Unread:          q.Unread,
	}
}

// conditionRole reports whether the role is a condition the evaluator checks in
// order — required, assumed, denied or violated — as opposed to an assertion
// standing beside them: a domain, a pin, a definedness guard or an exclusion.
func conditionRole(role Role) bool {
	switch role {
	case RoleRequired, RoleAssumed, RoleDenied, RoleViolated:
		return true
	}
	return false
}

// roundingSite is one place the evaluator's arithmetic may round: the fresh
// variable approximating its result, the exact value the operation computes
// over the rewritten operands, and the conditions under which it is evaluated.
type roundingSite struct {
	// key is the rewritten term's S-expression, so identical subterms share one
	// site and one variable.
	key string

	// variable is the Real-sorted variable standing for the rounded result.
	variable *Var

	// value is the exact value the site computes over the rewritten operands.
	value *Term

	// evals are the conditions under which the evaluator reaches this site, one
	// term per occurrence; their disjunction is the site's evaluation guard.
	evals []*Term
}

// axioms bounds the site between the doubles its exact value lies between, for
// every double in Z but the site itself: `(G(z) ∧ v <= z) => r <= z` and
// `(G(z) ∧ v >= z) => r >= z`, where a site double is guarded by its own
// evaluation condition and an unconditional double needs none.
func (s *roundingSite) axioms(doubles []roundingDouble, q *Query) []Assertion {
	r := VarTerm(s.variable)
	var out []Assertion
	for _, z := range doubles {
		if z.site == s {
			continue
		}
		for _, bound := range []struct{ op, arg Op }{{OpLe, OpLe}, {OpGe, OpGe}} {
			condition := Binary(bound.op, Bool, s.value, z.term)
			conclusion := Binary(bound.arg, Bool, r, z.term)
			term := Binary(OpImplies, Bool, and(z.guard, condition), conclusion)
			out = append(out, Assertion{
				Term: term,
				From: Provenance{Kind: q.Kind, Element: q.Element, Condition: "rounding", Role: RoleDefined},
			})
		}
	}
	return out
}

// roundingDouble is a value that is always a float64 in the evaluator: a Real
// variable, the value 0, a rewritten literal, or another site — which is one
// only under its own evaluation guard.
type roundingDouble struct {
	term  *Term
	guard *Term // nil for an unconditional double
	site  *roundingSite
}

// doubles is the set Z the site's bounds are taken over: every Real variable, the value 0, every rewritten real literal, and every site guarded
// by its evaluation condition, each once.
func (r *roundingRewrite) doubles(q *Query) []roundingDouble {
	var out []roundingDouble
	seen := map[string]bool{}
	add := func(term *Term, guard *Term, site *roundingSite) {
		key := writeTerm(term)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, roundingDouble{term: term, guard: guard, site: site})
	}
	add(RealTerm(new(big.Rat)), nil, nil)
	for _, v := range q.Vars {
		if v.Binary64 {
			add(VarTerm(v), nil, nil)
		}
	}
	for _, lit := range r.literals {
		add(lit, nil, nil)
	}
	for _, site := range r.order {
		add(VarTerm(site.variable), site.guard(), site)
	}
	return out
}

// guard is the site's evaluation condition: nil — unconditional — when any
// occurrence lies on the unconstrained path of the first condition, else the
// disjunction of its occurrences' conditions.
func (s *roundingSite) guard() *Term {
	var evals []*Term
	for _, e := range s.evals {
		if e == nil {
			return nil
		}
		evals = append(evals, e)
	}
	return Or(evals...)
}

// roundingRewrite rewrites a query's terms into the rounding-sound form,
// collecting the sites and literals met.
type roundingRewrite struct {
	sites    map[string]*roundingSite
	order    []*roundingSite
	literals []*Term
}

// rewrite returns the term's rounding-sound form under evaluation condition e:
// sites replaced by their variables, exact literals binary64 arithmetic meets by
// their float64 values. A nil e
// is the unconstrained path of the first condition, and BoolTerm(false) the
// path of an assertion the evaluator does not check in order.
func (r *roundingRewrite) rewrite(t, e *Term) *Term {
	switch t.Op {
	case OpReal:
		// A literal binary64 holds exactly is a double the sites are bounded by.
		if _, exact := t.Real.Float64(); exact {
			r.literals = append(r.literals, t)
		}
		return t
	case OpNeg:
		return Unary(OpNeg, t.Sort, r.rewrite(t.Args[0], e))
	case OpAnd, OpOr:
		args := make([]*Term, len(t.Args))
		prefix := e
		for i, arg := range t.Args {
			args[i] = r.rewrite(arg, prefix)
			if t.Op == OpAnd {
				prefix = and(prefix, args[i])
			} else {
				prefix = and(prefix, Not(args[i]))
			}
		}
		if t.Op == OpAnd {
			return And(args...)
		}
		return Or(args...)
	case OpImplies:
		left := r.rewrite(t.Args[0], e)
		right := r.rewrite(t.Args[1], and(e, left))
		return Binary(OpImplies, t.Sort, left, right)
	case OpIte:
		cond := r.rewrite(t.Args[0], e)
		then := r.rewrite(t.Args[1], and(e, cond))
		otherwise := r.rewrite(t.Args[2], and(e, Not(cond)))
		return Ite(cond, then, otherwise)
	}
	if (t.Op == OpAdd || t.Op == OpSub || t.Op == OpMul || t.Op == OpDiv) && t.Sort.Kind == SortReal && t.Binary64() {
		args := make([]*Term, len(t.Args))
		for i, arg := range t.Args {
			args[i] = r.double(arg, e)
		}
		return r.site(&Term{Op: t.Op, Sort: Real, Args: args}, e)
	}
	return r.rebuild(t, e)
}

// double is an operand of binary64 arithmetic as the evaluator holds it: a
// binary64 value as it is, an exact literal as its nearest float64, and any
// other exact value rounded once at a site of its own.
func (r *roundingRewrite) double(t, e *Term) *Term {
	if t.Binary64() {
		return r.rewrite(t, e)
	}
	switch {
	case t.Op == OpReal:
		return r.literal(t)
	case t.Op == OpToReal && t.Args[0].Op == OpInt:
		return r.literal(RealTerm(new(big.Rat).SetInt(t.Args[0].IntBig())))
	}
	return r.site(r.rewrite(t, e), e)
}

// literal is an exact literal as binary64 arithmetic rounds it: the exact
// rational of its nearest float64, which is what replay computes. A literal float64 cannot hold is
// kept exact, since the evaluator could not evaluate it anyway.
func (r *roundingRewrite) literal(t *Term) *Term {
	f, _ := t.Real.Float64()
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return RealTerm(t.Real)
	}
	lit := RealTerm(new(big.Rat).SetFloat64(f))
	r.literals = append(r.literals, lit)
	return lit
}

// args rewrites a term's operands under the same evaluation condition.
func (r *roundingRewrite) args(t, e *Term) []*Term {
	args := make([]*Term, len(t.Args))
	for i, arg := range t.Args {
		args[i] = r.rewrite(arg, e)
	}
	return args
}

// site returns the variable standing for the rewritten term's rounded result,
// recording the exact value and the evaluation condition the site occurred
// under; an already-seen subterm shares its site.
func (r *roundingRewrite) site(term, e *Term) *Term {
	key := writeTerm(term)
	s, ok := r.sites[key]
	if !ok {
		s = &roundingSite{
			key:      key,
			variable: &Var{Name: fmt.Sprintf("rounded!%d", len(r.order)), Sort: Real},
			value:    term,
		}
		r.sites[key] = s
		r.order = append(r.order, s)
	}
	s.evals = append(s.evals, e)
	return VarTerm(s.variable)
}

// rebuild copies a term that is no site and rewrites nothing else, its operands
// rewritten under the same evaluation condition.
func (r *roundingRewrite) rebuild(t, e *Term) *Term {
	if len(t.Args) == 0 {
		return t
	}
	out := *t
	out.Args = r.args(t, e)
	return &out
}

// and is the conjunction of e with term, where a nil e is no constraint and a
// nil term keeps e.
func and(e, term *Term) *Term {
	if e == nil {
		return term
	}
	if term == nil {
		return e
	}
	return And(e, term)
}
