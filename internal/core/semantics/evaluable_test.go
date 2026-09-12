package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// evaluableIn parses expr as an expression written at the root of src and
// reports whether the model can evaluate it.
func evaluableIn(t *testing.T, src, expr string) bool {
	t.Helper()
	m, root := buildModel(t, src)
	p := parser.New(source.New("<expr>", []byte(expr)))
	node := p.ParseExpression()
	if node == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("failed to parse %q: %v", expr, p.Diagnostics)
	}
	return m.ModelLevelEvaluable(root, node)
}

const evaluableModel = `part def A { attribute y; attribute z = 3; }
enum def E { enum e; }
attribute k = 2;
attribute unset;
attribute fromUnset = unset;
attribute bad = ~3;
attribute c1 = c2;
attribute c2 = c1;
part a : A;
metadata def M { attribute q = 4; attribute q2; }
calc def f { in x; return : ScalarValues::Integer = 1; }
`

// A metadata feature value and a filter condition must be decidable from the
// model alone (KerML §7.4.9, Expression::isModelLevelEvaluable). A feature read
// is decided by FeatureReferenceExpression::modelLevelEvaluable: a feature of a
// type other than a metaclass is not, an unfeatured one is as its value is. An
// extent `all T` never is, whatever T (§8.2.5.8.1 Table 5): a run decides it.
func TestModelLevelEvaluable(t *testing.T) {
	for expr, want := range map[string]bool{
		"1":                       true,
		"1.5":                     true,
		`"e"`:                     true,
		"true":                    true,
		"null":                    true,
		"*":                       true,
		"1 + 2":                   true,
		"true and (1 < 2)":        true,
		"not (2 == 2)":            true,
		"(1, 2, 3)":               true,
		"E::e":                    true,
		"k":                       true,
		"k + 1":                   true,
		"unset":                   true,
		"fromUnset":               true,
		"a":                       true,
		"a.y":                     true,
		"M::q":                    true,
		"M::q2":                   true,
		"new A(null, 1, \"\")":    true,
		"~3":                      false,
		"bad":                     false,
		"c1":                      false,
		"A::y":                    false,
		"A::z":                    false,
		"(as A).y":                false,
		"f((as A).y)":             false,
		"f(1)":                    false,
		"(as A).y->Base::size()":  false,
		"new A(null, (as A).y)":   false,
		"nowhere":                 false,
		"(1, (as A).y)":           false,
		"true and ((as A).y > 1)": false,
		"all A":                   false,
		"all E":                   false,
		"all a":                   false,
		"all M":                   false,
		"(all A)->Base::size()":   false,
		"(1, all E)":              false,
	} {
		if got := evaluableIn(t, evaluableModel, expr); got != want {
			t.Errorf("ModelLevelEvaluable(%q) = %v, want %v", expr, got, want)
		}
	}
}

// Only the Kernel Function Library functions the model itself evaluates may be
// called: being a library function is not enough, and a local one never is.
func TestModelLevelEvaluableCallsOnlyModelLevelFunctions(t *testing.T) {
	const lib = `standard library package ControlFunctions { abstract function 'if' { in c; in t; in f; } }
standard library package RealFunctions { function sqrt { in x; } }`
	p := parser.New(source.New("kfl.sysml", []byte(lib)))
	libRoot := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("library parse diagnostics: %v", p.Diagnostics)
	}
	q := parser.New(source.New("t.sysml", []byte("calc def once { in x; return : ScalarValues::Integer = 1; }")))
	root := q.ParseFile()
	if len(q.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", q.Diagnostics)
	}

	idx := symbols.NewIndex()
	idx.AddDocument("kfl.sysml", libRoot)
	idx.AddDocument("t.sysml", root)
	idx.MarkLibrary("kfl.sysml")
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.ResolveDocument("kfl.sysml", libRoot)
	r.ResolveDocument("t.sysml", root)
	scope := idx.DocumentRoot("t.sysml")

	for expr, want := range map[string]bool{
		"ControlFunctions::'if'(true, 1, 2)": true,
		"RealFunctions::sqrt(4.0)":           false,
		"once(2)":                            false,
	} {
		e := parser.New(source.New("<expr>", []byte(expr))).ParseExpression()
		if e == nil {
			t.Fatalf("failed to parse %q", expr)
		}
		if got := m.ModelLevelEvaluable(scope, e); got != want {
			t.Errorf("ModelLevelEvaluable(%q) = %v, want %v", expr, got, want)
		}
	}
}
