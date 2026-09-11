package solve

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Expression is one translated expression: the term it yields, and the side
// conditions under which the evaluator computes that term rather than reporting
// an error — a computed divisor being non-zero. A consumer encoding a step of
// execution asserts Term where Defined all hold and the evaluator's error where
// one does not, since SMT-LIB's division is total and the evaluator's is not.
type Expression struct {
	// Term is the value the expression yields.
	Term *Term

	// Defined are the conditions the evaluator needs to compute Term, each a
	// boolean term; empty for an expression it always computes.
	Defined []*Term
}

// Translator translates the expressions one element writes — the guards,
// assignments and conditions of a behavior — into terms over one set of
// variables, for a consumer assembling a Query of its own from them. Each
// feature read is one variable, named and sorted as a condition's translation
// names it, so a consumer substituting variables of its own for them (a copy per
// step, say) keys the substitution by the feature.
type Translator struct {
	t *translator
}

// NewTranslator starts translating the expressions subject writes. Names resolve
// as the evaluator resolves them: first among the features subject declares or
// inherits, then in the scope each expression is written in.
func NewTranslator(ctx *runtime.Context, subject Subject) (*Translator, error) {
	if ctx == nil {
		return nil, fmt.Errorf("solve: no runtime context")
	}
	t := newTranslator(ctx, subject)
	if subject.Symbol != nil {
		t.condFile = subject.Symbol.DocName
	}
	return &Translator{t: t}, nil
}

// Expression translates one expression written in scope. within names the
// statement or guard the expression belongs to, which a refusal reports as the
// condition it appeared in. A construct outside the subset refuses with
// ErrNotTranslatable; nothing is degraded.
func (x *Translator) Expression(node ast.Node, scope *symbols.Scope, within string) (*Expression, error) {
	t := x.t
	t.condLabel = within
	if node == nil {
		return nil, t.refuse(nil, "empty expression", "it states no expression")
	}
	// Definedness is per expression here: what a guard hoisted over one
	// expression says nothing about another, and a divisor read twice in two
	// expressions guards both.
	guards, guarded := t.guards, t.guarded
	t.guards, t.guarded = nil, map[string]bool{}
	defer func() { t.guards, t.guarded = guards, guarded }()
	term, err := t.expr(node, scope)
	if err != nil {
		return nil, err
	}
	expr := &Expression{Term: term}
	for _, guard := range t.guards {
		expr.Defined = append(expr.Defined, guard.Term)
	}
	return expr, nil
}

// Boolean translates an expression that must yield a boolean — a guard, a loop
// or branch condition — refusing one that yields another sort, as the executor
// reports a condition that is not Boolean.
func (x *Translator) Boolean(node ast.Node, scope *symbols.Scope, within string) (*Expression, error) {
	expr, err := x.Expression(node, scope, within)
	if err != nil {
		return nil, err
	}
	if expr.Term.Sort.Kind != SortBool {
		return nil, x.t.refuse(node, within, "it yields "+expr.Term.Sort.Name+" rather than a boolean")
	}
	return expr, nil
}

// Variable is the variable standing for the feature name resolves to where it is
// written in scope — the one a reference to the name in an expression reads — for
// a consumer encoding a write to it. within names the statement writing it.
func (x *Translator) Variable(name string, scope *symbols.Scope, within string) (*Var, error) {
	t := x.t
	t.condLabel = within
	segments := []string{name}
	chain, err := t.resolvePath(nil, scope, segments)
	if err != nil {
		return nil, err
	}
	if _, _, literal := t.valueOf(chain[len(chain)-1]); literal {
		return nil, t.refuse(nil, "feature `"+name+"`", "it names a literal rather than a feature")
	}
	term, err := t.variableOf(nil, scope, chain, segments)
	if err != nil {
		return nil, err
	}
	return term.Var, nil
}

// Vars are the variables the expressions translated so far read, ordered by
// name; each stands for one feature.
func (x *Translator) Vars() []*Var {
	vars := make([]*Var, 0, len(x.t.vars))
	for _, v := range x.t.vars {
		if v.Unit == "" {
			v.Unit = x.t.baseUnits[v.Dimension]
		}
		vars = append(vars, v)
	}
	sortVars(vars)
	return vars
}

// Sorts are the finite datatype sorts the expressions translated so far range
// over, ordered by name.
func (x *Translator) Sorts() []Sort {
	sorts := make([]Sort, 0, len(x.t.sorts))
	for _, s := range x.t.sorts {
		sorts = append(sorts, s)
	}
	sortSorts(sorts)
	return sorts
}

// Domains are the bounds the declarations of the variables read put on their
// values (a Natural is not negative), in a deterministic order. A consumer
// asserts each one of every copy of the variable it declares.
func (x *Translator) Domains() []Assertion {
	domains := append([]Assertion(nil), x.t.domains...)
	sortAssertions(domains)
	return domains
}

// Literal renders a value the evaluator holds for v as a term of v's sort, as a
// pinned value is rendered. A value the term language has no literal for
// refuses with a PinError naming the feature.
func (x *Translator) Literal(v *Var, value runtime.Value) (*Term, error) {
	if v == nil {
		return nil, fmt.Errorf("solve: a literal of no variable")
	}
	term, _, err := x.t.pinTerm(Pin{Feature: v.Symbol, Name: v.Name, Value: value, Source: PinHeld}, v)
	return term, err
}

// Substitute returns t with every variable replaced by what replace returns for
// it, sharing the subterms that read no variable. A replacement must yield the
// variable's sort; replace returning nil keeps the variable.
func Substitute(t *Term, replace func(*Var) *Term) *Term {
	if t == nil {
		return nil
	}
	if t.Op == OpVar {
		if replaced := replace(t.Var); replaced != nil {
			return replaced
		}
		return t
	}
	if len(t.Args) == 0 {
		return t
	}
	args := make([]*Term, len(t.Args))
	changed := false
	for i, arg := range t.Args {
		args[i] = Substitute(arg, replace)
		if args[i] != arg {
			changed = true
		}
	}
	if !changed {
		return t
	}
	copied := *t
	copied.Args = args
	return &copied
}
