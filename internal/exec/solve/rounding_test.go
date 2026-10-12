package solve

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// realVars are named variables declared Real, held in binary64, for building
// queries by hand.
func realVars(names ...string) []*Var {
	vars := make([]*Var, len(names))
	for i, name := range names {
		vars[i] = &Var{Name: name, Sort: Real, Binary64: true}
	}
	return vars
}

// siteVars are the rounding-site variables a rounding-sound query declares, in
// declaration order.
func siteVars(q *Query) []*Var {
	var out []*Var
	for _, v := range q.Vars {
		if strings.HasPrefix(v.Name, "rounded!") {
			out = append(out, v)
		}
	}
	return out
}

// axioms are a rounding-sound query's axioms, the assertions it adds in role
// RoleDefined with condition "rounding".
func axioms(q *Query) []Assertion {
	var out []Assertion
	for _, a := range q.Assertions {
		if a.From.Role == RoleDefined && a.From.Condition == "rounding" {
			out = append(out, a)
		}
	}
	return out
}

// TestRoundingSoundRealOperationSites: each real-sorted arithmetic operation is
// a site, replaced by a variable bounded by the doubles the exact value lies
// between.
func TestRoundingSoundRealOperationSites(t *testing.T) {
	vars := realVars("x")
	site := Binary(OpAdd, Real, VarTerm(vars[0]), RealTerm(big.NewRat(1, 1)))
	q := &Query{Kind: "constraint", Element: "C", Vars: vars, Assertions: []Assertion{
		{Term: Binary(OpGt, Bool, site, RealTerm(big.NewRat(2, 1))), From: Provenance{Role: RoleRequired}},
	}}
	sound := q.RoundingSound()
	if got := writeTerm(sound.Assertions[0].Term); got != "(> |rounded!!0| 2.0)" {
		t.Fatalf("rewritten assertion is %s", got)
	}
	sites := siteVars(sound)
	if len(sites) != 1 {
		t.Fatalf("declared %d site variables, want 1: %+v", len(sites), sound.Vars)
	}
	// Z is {0, x, 1.0, 2.0}: two bounds per double, the site not bounding itself.
	if got := axioms(sound); len(got) != 8 {
		t.Fatalf("stated %d axioms, want 8:\n%s", len(got), Script(sound))
	}
	for _, a := range axioms(sound) {
		if !strings.HasPrefix(writeTerm(a.Term), "(=> ") {
			t.Errorf("axiom %s is not an implication", writeTerm(a.Term))
		}
	}
}

// TestRoundingSoundSharedSites: identical rewritten subterms share one site and
// one variable, so an operation written twice rounds once.
func TestRoundingSoundSharedSites(t *testing.T) {
	vars := realVars("x")
	site := Binary(OpAdd, Real, VarTerm(vars[0]), RealTerm(big.NewRat(1, 1)))
	q := &Query{Kind: "constraint", Element: "C", Vars: vars, Assertions: []Assertion{
		{Term: And(Binary(OpGt, Bool, site, RealTerm(new(big.Rat))), Binary(OpLt, Bool, site, RealTerm(big.NewRat(10, 1)))), From: Provenance{Role: RoleRequired}},
	}}
	sound := q.RoundingSound()
	if n := len(siteVars(sound)); n != 1 {
		t.Fatalf("declared %d site variables for one shared subterm, want 1", n)
	}
	if got := writeTerm(sound.Assertions[0].Term); got != "(and (> |rounded!!0| 0.0) (< |rounded!!0| 10.0))" {
		t.Errorf("rewritten assertion is %s", got)
	}
}

// TestRoundingSoundRatioIsExact: a whole-number quotient is the exact Rational,
// so it is no site at all.
func TestRoundingSoundRatioIsExact(t *testing.T) {
	a := &Var{Name: "a", Sort: Int}
	b := &Var{Name: "b", Sort: Int}
	q := &Query{Kind: "constraint", Element: "C", Vars: []*Var{a, b}, Assertions: []Assertion{
		{Term: Binary(OpGt, Bool, RatioDiv(VarTerm(a), VarTerm(b)), RealTerm(big.NewRat(1, 1))), From: Provenance{Role: RoleRequired}},
	}}
	sound := q.RoundingSound()
	if n := len(siteVars(sound)); n != 0 {
		t.Fatalf("declared %d site variables, want none: %v", n, sound.Vars)
	}
	if got := writeTerm(sound.Assertions[0].Term); got != "(> (/ (to_real a) (to_real b)) 1.0)" {
		t.Errorf("rewritten assertion is %s", got)
	}
}

// TestRoundingSoundNegationIsExact: negation is not a site — the float64
// negation is exact — while an Integer operand of Real arithmetic is one,
// rounding beyond 2^53, as is the sum.
func TestRoundingSoundNegationIsExact(t *testing.T) {
	vars := realVars("x")
	i := &Var{Name: "i", Sort: Int}
	q := &Query{Kind: "constraint", Element: "C", Vars: append(vars, i), Assertions: []Assertion{
		{Term: Binary(OpGt, Bool, Binary(OpAdd, Real, Unary(OpNeg, Real, VarTerm(vars[0])), ToReal(VarTerm(i))), RealTerm(new(big.Rat))), From: Provenance{Role: RoleRequired}},
	}}
	sound := q.RoundingSound()
	sites := siteVars(sound)
	if len(sites) != 2 {
		t.Fatalf("declared %d site variables, want the widened integer's and the sum's", len(sites))
	}
	if got := writeTerm(sound.Assertions[0].Term); got != "(> |rounded!!1| 0.0)" {
		t.Errorf("rewritten assertion is %s", got)
	}
}

// TestRoundingSoundLiteralIsTheFloat64: an exact literal binary64 arithmetic
// meets is the exact rational of its nearest float64 — 0.1 + x rounds 1/10.
func TestRoundingSoundLiteralIsTheFloat64(t *testing.T) {
	vars := realVars("x")
	tenth, _ := new(big.Rat).SetString("0.1")
	q := &Query{Kind: "constraint", Element: "C", Vars: vars, Assertions: []Assertion{
		{Term: Binary(OpGt, Bool, Binary(OpAdd, Real, VarTerm(vars[0]), RealTerm(tenth)), RealTerm(new(big.Rat))), From: Provenance{Role: RoleRequired}},
	}}
	sound := q.RoundingSound()
	var lit *Term
	for _, a := range axioms(sound) {
		walkTerms(a.Term, func(t *Term) {
			if t.Op == OpAdd && t.Args[1].Op == OpReal {
				lit = t.Args[1]
			}
		})
	}
	if lit == nil {
		t.Fatalf("no rewritten sum in:\n%s", Script(sound))
	}
	f, _ := tenth.Float64()
	exact := new(big.Rat).SetFloat64(f)
	if lit.Real.Cmp(exact) != 0 {
		t.Errorf("literal is %s, want the exact rational of the float64 %v", lit.Real, f)
	}
}

// TestRoundingSoundEvaluationGuards: a site another site bounds is guarded by
// the conditions under which the evaluator computes it — earlier conditions for
// the assertion order, and the operands deciding an `and`, `or`, `implies` or
// `ite` within a term.
func TestRoundingSoundEvaluationGuards(t *testing.T) {
	x := &Var{Name: "x", Sort: Real, Binary64: true}
	y := &Var{Name: "y", Sort: Real, Binary64: true}
	b := &Var{Name: "b", Sort: Bool}
	site0 := Binary(OpAdd, Real, VarTerm(x), RealTerm(big.NewRat(1, 1)))
	site1 := Binary(OpMul, Real, VarTerm(y), RealTerm(big.NewRat(2, 1)))
	// b; (x + 1.0 > 0.0) and (b implies y * 2.0 > 0.0):
	// site0 is computed when b holds, site1 when b ∧ x+1.0>0 ∧ b holds.
	q := &Query{Kind: "constraint", Element: "C", Vars: []*Var{x, y, b}, Assertions: []Assertion{
		{Term: VarTerm(b), From: Provenance{Role: RoleRequired, Condition: "b"}},
		{Term: And(
			Binary(OpGt, Bool, site0, RealTerm(new(big.Rat))),
			Binary(OpImplies, Bool, VarTerm(b), Binary(OpGt, Bool, site1, RealTerm(new(big.Rat)))),
		), From: Provenance{Role: RoleRequired}},
	}}
	sound := q.RoundingSound()
	if n := len(siteVars(sound)); n != 2 {
		t.Fatalf("declared %d site variables, want 2", n)
	}
	// site1's evaluation guard is `b ∧ x+1.0>0 ∧ b`; an axiom of site0 bounding
	// it is guarded by that, while an unconditional double needs no guard.
	var guarded, unguarded bool
	for _, a := range axioms(sound) {
		text := writeTerm(a.Term)
		if strings.Contains(text, "(=> (and (and (and b (> |rounded!!0| 0.0)) b) ") {
			guarded = true
		}
		if strings.HasPrefix(text, "(=> (<= ") || strings.HasPrefix(text, "(=> (>= ") {
			unguarded = true
		}
	}
	if !guarded {
		t.Error("no axiom guards a site by another site's evaluation condition:\n" + Script(sound))
	}
	if !unguarded {
		t.Error("no axiom bounds a site by an unconditional double:\n" + Script(sound))
	}
}

// TestRoundingSoundLemmaIsProved: the design's worked example — unsat over
// exact reals, proved again over the evaluator's float64 arithmetic.
func TestRoundingSoundLemmaIsProved(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "<test>", `
		package P {
			private import ScalarValues::*;
			part hg { attribute efficiency : Real; attribute power : Real; }
			attribute d : Real;
			assert constraint lemma {
				(hg.efficiency >= 0.0 and hg.efficiency <= 1.0 and hg.power >= 0.0 and d >= 0.0)
				implies (hg.power * d * hg.efficiency) <= hg.power * d
			}
		}
	`)
	q, err := ConstraintViolation(ctx, symbolNamed(t, idx, "P::lemma"), nil, nil)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusUnsat || !result.RoundingProved {
		t.Fatalf("result is %s proved=%v, want unsat proved", result.Status, result.RoundingProved)
	}
}

// TestRoundingSoundOverflowYieldsNoVerdict: an assignment whose evaluation
// overflows is no counterexample — the evaluator refuses the non-finite result,
// so neither does it confirm the solver's witness — and no false proof either:
// the same lemma still proves, and pinning the overflow values leaves it proved
// since the exact claim is true where the evaluator only errors.
func TestRoundingSoundOverflowYieldsNoVerdict(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "<test>", `
		package P {
			private import ScalarValues::*;
			part hg { attribute efficiency : Real; attribute power : Real; }
			attribute d : Real;
			assert constraint lemma {
				(hg.efficiency >= 0.0 and hg.efficiency <= 1.0 and hg.power >= 0.0 and d >= 0.0)
				implies (hg.power * d * hg.efficiency) <= hg.power * d
			}
		}
	`)
	q, err := ConstraintViolation(ctx, symbolNamed(t, idx, "P::lemma"), nil, nil)
	if err != nil {
		t.Fatalf("translate lemma: %v", err)
	}
	varNamed := func(tail string) string {
		for _, v := range q.Vars {
			if v.Name == tail || strings.HasSuffix(v.Name, "."+tail) || strings.HasSuffix(v.Name, "::"+tail) {
				return v.Name
			}
		}
		t.Fatalf("no variable names %s: %+v", tail, q.Vars)
		return ""
	}
	overflow := ModelValue{Kind: SortReal, Number: new(big.Rat).SetFloat64(1e200)}
	ok, why := q.Confirm(map[string]ModelValue{
		varNamed("power"):      overflow,
		varNamed("d"):          overflow,
		varNamed("efficiency"): {Kind: SortReal, Number: new(big.Rat)},
	})
	if ok {
		t.Fatal("the evaluator confirmed a witness whose arithmetic overflows")
	}
	if !strings.Contains(why, "not a finite Real") {
		t.Errorf("the witness was refused for %q, want the non-finite result named", why)
	}
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve lemma: %v", err)
	}
	if result.Status != StatusUnsat || !result.RoundingProved {
		t.Fatalf("lemma's violation is %s proved=%v, want unsat proved", result.Status, result.RoundingProved)
	}

	ctx, idx = fixture(t, "<test>", `
		package P {
			private import ScalarValues::*;
			part def Big {
				attribute power : Real = 1.0e200;
				attribute d : Real = 1.0e200;
				attribute efficiency : Real = 0.0;
				assert constraint lemma {
					(efficiency >= 0.0 and efficiency <= 1.0 and power >= 0.0 and d >= 0.0)
					implies (power * d * efficiency) <= power * d
				}
			}
			part big : Big;
		}
	`)
	big := symbolNamed(t, idx, "P::Big")
	lemma := symbolNamed(t, idx, "P::Big::lemma")
	inst, err := ctx.Instantiate(big)
	if err != nil {
		t.Fatalf("instantiate P::big: %v", err)
	}
	pins, _ := FixedFor(ctx, Fixing{Element: lemma, Owner: big, Object: inst, ObjectType: big})
	pinned, err := ConstraintViolation(ctx, lemma, lemma.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate pinned lemma: %v", err)
	}
	result, err = solver.Solve(context.Background(), pinned)
	if err != nil {
		t.Fatalf("solve pinned lemma: %v", err)
	}
	if result.Status != StatusUnsat || !result.RoundingProved {
		t.Fatalf("the pinned lemma's violation is %s proved=%v, want unsat proved — the exact claim is true where the evaluator only errors", result.Status, result.RoundingProved)
	}
}

// TestRoundingSoundKeepsACounterexample: `x + 1.0 > x` holds over exact reals
// but not over float64, so its violation's unsat must not be proved.
func TestRoundingSoundKeepsACounterexample(t *testing.T) {
	solver := requireSolver(t)
	q := violationQuery(t, constraintSource(`
		in x : Real;
		assert constraint { x + 1.0 > x }
	`), "test::C")
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusUnsat {
		t.Fatalf("violation is %s, want unsat over exact reals", result.Status)
	}
	if result.RoundingProved {
		t.Error("`x + 1.0 > x` was proved, but float64 has counterexamples — an unsound proof")
	}
}

// TestRoundingSoundDivergenceNotProved: associativity holds over exact reals
// and diverges in float64, so its violation's unsat must not be proved either.
func TestRoundingSoundDivergenceNotProved(t *testing.T) {
	solver := requireSolver(t)
	q := violationQuery(t, constraintSource(`
		in a : Real;
		in b : Real;
		in c : Real;
		assert constraint { (a + b) + c == a + (b + c) }
	`), "test::C")
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusUnsat {
		t.Fatalf("violation is %s, want unsat over exact reals", result.Status)
	}
	if result.RoundingProved {
		t.Error("float64 addition associativity was proved — an unsound proof")
	}
}

// TestRoundingSoundSharedSiteOnTheOpenPath: a site occurring twice in the first
// condition — once on the unconstrained path — is guarded by nothing: its
// evaluation condition is unconditional, and no term in the rewritten query
// carries a nil argument.
func TestRoundingSoundSharedSiteOnTheOpenPath(t *testing.T) {
	x := &Var{Name: "x", Sort: Real, Binary64: true}
	y := &Var{Name: "y", Sort: Real, Binary64: true}
	sum := Binary(OpAdd, Real, VarTerm(x), VarTerm(y))
	rw := &roundingRewrite{sites: map[string]*roundingSite{}}
	term := rw.rewrite(Or(
		Binary(OpGe, Bool, sum, RealTerm(big.NewRat(1, 1))),
		Binary(OpLt, Bool, sum, RealTerm(big.NewRat(1, 1))),
	), nil)
	if term.Op != OpOr || len(term.Args) != 2 {
		t.Fatalf("rewritten term is %s, want the disjunction", writeTerm(term))
	}
	if n := len(rw.order); n != 1 {
		t.Fatalf("the shared sum made %d sites, want 1", n)
	}
	if guard := rw.order[0].guard(); guard != nil {
		t.Errorf("the shared site's guard is %s, want unconditional", writeTerm(guard))
	}

	// With another site present the unconditional guard must still build: the
	// other site's axioms bound it and must contain no nil argument.
	second := Binary(OpMul, Real, VarTerm(x), VarTerm(y))
	q := &Query{Kind: "constraint", Element: "C", Vars: []*Var{x, y}, Assertions: []Assertion{
		{Term: Or(
			Binary(OpGe, Bool, sum, RealTerm(big.NewRat(1, 1))),
			And(Binary(OpLt, Bool, sum, RealTerm(big.NewRat(1, 1))), Binary(OpGt, Bool, second, RealTerm(new(big.Rat)))),
		), From: Provenance{Role: RoleRequired}},
	}}
	sound := q.RoundingSound()
	var checkNil func(term *Term, path string)
	checkNil = func(term *Term, path string) {
		if term == nil {
			t.Errorf("nil term at %s", path)
			return
		}
		for i, arg := range term.Args {
			checkNil(arg, path+"/"+string(rune('a'+i)))
		}
	}
	for i, a := range sound.Assertions {
		checkNil(a.Term, fmt.Sprintf("assertion %d", i))
	}
}

// TestRoundingSoundSharedTautologyProves: `x + y >= 1.0 or x + y < 1.0` holds
// over every assignment — the shared site makes it a tautology of the rounded
// value, so the holds question proves it.
func TestRoundingSoundSharedTautologyProves(t *testing.T) {
	solver := requireSolver(t)
	q := violationQuery(t, constraintSource(`
		in x : Real;
		in y : Real;
		assert constraint c { x + y >= 1.0 or x + y < 1.0 }
	`), "test::C")
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusUnsat || !result.RoundingProved {
		t.Fatalf("result is %s proved=%v, want unsat proved", result.Status, result.RoundingProved)
	}
}

// walkTerms visits a term and every term beneath it.
func walkTerms(t *Term, visit func(*Term)) {
	visit(t)
	for _, arg := range t.Args {
		walkTerms(arg, visit)
	}
}
