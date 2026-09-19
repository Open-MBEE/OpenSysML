package solve

import (
	"math/big"
	"strings"
	"testing"
)

// TestWriteRat: a real literal is written exactly — a decimal when the magnitude
// has a terminating one, else the quotient itself, so no rounding enters a script.
func TestWriteRat(t *testing.T) {
	cases := []struct {
		num, den int64
		want     string
	}{
		{0, 1, "0.0"},
		{3, 1, "3.0"},
		{3, 2, "1.5"},
		{5, 4, "1.25"},
		{1, 20, "0.05"},
		{-3, 2, "(- 1.5)"},
		{1, 3, "(/ 1.0 3.0)"},
		{-1, 3, "(- (/ 1.0 3.0))"},
		{25, 1000, "0.025"},
	}
	for _, c := range cases {
		got := writeRat(big.NewRat(c.num, c.den))
		if got != c.want {
			t.Errorf("%d/%d writes as %s, want %s", c.num, c.den, got, c.want)
		}
	}
}

// TestWriteInt: SMT-LIB numerals are non-negative, so a negative one is written
// as a negation.
func TestWriteInt(t *testing.T) {
	if got := writeInt(-7); got != "(- 7)" {
		t.Errorf("-7 writes as %s", got)
	}
	if got := writeInt(7); got != "7" {
		t.Errorf("7 writes as %s", got)
	}
}

// TestSMTSymbol quotes what is not a simple symbol and escapes reversibly, so two
// names never collapse into one symbol.
func TestSMTSymbol(t *testing.T) {
	cases := map[string]string{
		"pressure":     "pressure",
		"test::C::x":   "|test::C::x|",
		"a b":          "|a b|",
		"":             "||",
		"pipe|name":    "|pipe!pname|",
		`back\slash`:   "|back!bslash|",
		"bang!name":    "|bang!!name|",
		"bang!name x":  "|bang!!name x|",
		"pipe!pname":   "|pipe!!pname|",
		"pipe!pname x": "|pipe!!pname x|",
	}
	for name, want := range cases {
		if got := smtSymbol(name); got != want {
			t.Errorf("%q writes as %s, want %s", name, got, want)
		}
	}
	// Quoting is only lexical, so an escaped name must not be left unquoted:
	// |pipe!pname| and pipe!pname would be one symbol.
	for _, pair := range [][2]string{
		{"pipe|name x", "pipe!pname x"},
		{"pipe|name", "pipe!pname"},
		{`back\slash`, "back!bslash"},
	} {
		if smtSymbol(pair[0]) == smtSymbol(pair[1]) {
			t.Errorf("%q and %q write as the same symbol", pair[0], pair[1])
		}
	}
}

// TestWriteTermOperators writes every compound operator, so the script speaks
// SMT-LIB rather than the source notation.
func TestWriteTermOperators(t *testing.T) {
	x := VarTerm(&Var{Name: "x", Sort: Int})
	b := VarTerm(&Var{Name: "b", Sort: Bool})
	cases := []struct {
		term *Term
		want string
	}{
		{Not(b), "(not b)"},
		{And(b, BoolTerm(true)), "(and b true)"},
		{Or(b, BoolTerm(false)), "(or b false)"},
		{Binary(OpXor, Bool, b, b), "(xor b b)"},
		{Binary(OpImplies, Bool, b, b), "(=> b b)"},
		{Binary(OpEq, Bool, x, IntTerm(1)), "(= x 1)"},
		{Binary(OpNe, Bool, x, IntTerm(1)), "(distinct x 1)"},
		{Binary(OpLt, Bool, x, IntTerm(1)), "(< x 1)"},
		{Binary(OpLe, Bool, x, IntTerm(1)), "(<= x 1)"},
		{Binary(OpGt, Bool, x, IntTerm(1)), "(> x 1)"},
		{Binary(OpGe, Bool, x, IntTerm(1)), "(>= x 1)"},
		{Binary(OpAdd, Int, x, IntTerm(2)), "(+ x 2)"},
		{Binary(OpSub, Int, x, IntTerm(2)), "(- x 2)"},
		{Binary(OpMul, Int, x, IntTerm(2)), "(* x 2)"},
		{Binary(OpDiv, Real, ToReal(x), RealTerm(big.NewRat(1, 2))), "(/ (to_real x) 0.5)"},
		{Unary(OpNeg, Int, x), "(- x)"},
		{Ite(b, x, IntTerm(0)), "(ite b x 0)"},
		{StringTerm(`say "hi"`), `"say ""hi"""`},
		{ValueTerm(Sort{Kind: SortDatatype, Name: "test::Finish"}, "test::Finish::matte"), "|test::Finish::matte|"},
	}
	for _, c := range cases {
		if got := writeTerm(c.term); got != c.want {
			t.Errorf("term writes as %s, want %s", got, c.want)
		}
	}
}

// TestTermConstructorsFold: a double negation and a singleton junction fold, and
// widening a real or an integer literal needs no conversion term.
func TestTermConstructorsFold(t *testing.T) {
	b := VarTerm(&Var{Name: "b", Sort: Bool})
	if Not(Not(b)) != b {
		t.Error("a double negation does not fold")
	}
	if And(b) != b {
		t.Error("a one-term conjunction does not fold")
	}
	if got := And(); got.Op != OpBool || !got.Bool {
		t.Error("an empty conjunction is not true")
	}
	if got := Or(); got.Op != OpBool || got.Bool {
		t.Error("an empty disjunction is not false")
	}
	realTerm := RealTerm(big.NewRat(1, 2))
	if ToReal(realTerm) != realTerm {
		t.Error("widening a real does not leave it alone")
	}
	if got := ToReal(IntTerm(3)); got.Op != OpReal || got.Real.Cmp(big.NewRat(3, 1)) != 0 {
		t.Error("widening an integer literal does not fold to a real literal")
	}
}

// TestLogicSelection names the narrowest standard logic the sorts and operators
// used need, and falls back to a non-standard name only for what the SMT-LIB
// logic list has no logic for.
func TestLogicSelection(t *testing.T) {
	query := func(vars []*Var, nonlinear bool, terms ...*Term) *Query {
		q := &Query{Vars: vars, Nonlinear: nonlinear}
		for _, term := range terms {
			q.Assertions = append(q.Assertions, Assertion{Term: term})
		}
		return q
	}
	boolVar := &Var{Name: "b", Sort: Bool}
	intVar := &Var{Name: "i", Sort: Int}
	intVar2 := &Var{Name: "j", Sort: Int}
	realVar := &Var{Name: "r", Sort: Real}
	strVar := &Var{Name: "s", Sort: String}
	finish := Sort{Kind: SortDatatype, Name: "test::Finish", Values: []string{"test::Finish::matte"}}
	dtVar := &Var{Name: "d", Sort: finish}

	cases := []struct {
		name     string
		query    *Query
		want     string
		standard bool
	}{
		{"booleans only", query([]*Var{boolVar}, false, VarTerm(boolVar)), "QF_UF", true},
		{"linear integers", query([]*Var{intVar}, false, Binary(OpGe, Bool, VarTerm(intVar), IntTerm(0))), "QF_LIA", true},
		{"linear reals", query([]*Var{realVar}, false, Binary(OpGe, Bool, VarTerm(realVar), RealTerm(big.NewRat(1, 2)))), "QF_LRA", true},
		{"nonlinear integers", query([]*Var{intVar, intVar2}, false,
			Binary(OpEq, Bool, Binary(OpMul, Int, VarTerm(intVar), VarTerm(intVar2)), IntTerm(6))), "QF_NIA", true},
		{"nonlinear reals", query([]*Var{realVar}, false,
			Binary(OpEq, Bool, Binary(OpMul, Real, VarTerm(realVar), VarTerm(realVar)), RealTerm(big.NewRat(2, 1)))), "QF_NRA", true},
		// Reals_Ints has no quantifier-free logic in the list, so mixed arithmetic
		// takes the narrowest logic that has it, not a non-standard name.
		{"mixed linear arithmetic", query([]*Var{intVar, realVar}, false,
			Binary(OpLe, Bool, ToReal(VarTerm(intVar)), VarTerm(realVar))), "AUFLIRA", true},
		{"mixed nonlinear arithmetic", query([]*Var{intVar, realVar}, false,
			Binary(OpLe, Bool, Binary(OpMul, Real, ToReal(VarTerm(intVar)), VarTerm(realVar)), RealTerm(big.NewRat(1, 1)))),
			"AUFNIRA", true},
		// Truncating integer division by a literal stays inside the linear logic,
		// where it used to widen the script to ALL.
		{"integer division by a literal", query([]*Var{intVar}, false,
			Binary(OpEq, Bool, Binary(OpIntDiv, Int, VarTerm(intVar), IntTerm(2)), IntTerm(3))), "QF_LIA", true},
		{"integer division by a term", query([]*Var{intVar, intVar2}, false,
			Binary(OpEq, Bool, Binary(OpIntDiv, Int, VarTerm(intVar), VarTerm(intVar2)), IntTerm(3))), "QF_NIA", true},
		{"strings", query([]*Var{strVar}, false, Binary(OpEq, Bool, VarTerm(strVar), StringTerm("descent"))), "ALL", false},
		{"datatypes", &Query{Sorts: []Sort{finish}, Vars: []*Var{dtVar},
			Assertions: []Assertion{{Term: Binary(OpEq, Bool, VarTerm(dtVar), ValueTerm(finish, "test::Finish::matte"))}}},
			"ALL", false},
		{"a declared nonlinear flag is honoured", query([]*Var{intVar}, true, VarTerm(intVar)), "QF_NIA", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			choice := c.query.LogicChoice()
			if choice.Name != c.want {
				t.Errorf("logic is %s, want %s", choice.Name, c.want)
			}
			if choice.Name != c.query.Logic() {
				t.Errorf("Logic() is %s, want %s", c.query.Logic(), choice.Name)
			}
			if choice.Standard != c.standard {
				t.Errorf("logic %s standard is %t, want %t", choice.Name, choice.Standard, c.standard)
			}
			if choice.Why == "" {
				t.Errorf("logic %s says nothing about why it was chosen", choice.Name)
			}
			if !choice.Standard && !strings.Contains(Script(c.query), "; no SMT-LIB logic covers ") {
				t.Errorf("a script setting the non-standard %s does not say why", choice.Name)
			}
		})
	}
}

// TestNonStandardLogicIsExplained: a script that must set a non-standard logic
// says which feature no SMT-LIB logic covers, and needs that of a backend.
func TestNonStandardLogicIsExplained(t *testing.T) {
	strVar := &Var{Name: "s", Sort: String}
	q := &Query{Vars: []*Var{strVar}, Assertions: []Assertion{
		{Term: Binary(OpEq, Bool, VarTerm(strVar), StringTerm("descent"))},
	}}
	script := Script(q)
	for _, want := range []string{"the strings theory", "which the SMT-LIB logic list does not define", "(set-logic ALL)"} {
		if !strings.Contains(script, want) {
			t.Errorf("script does not mention %q:\n%s", want, script)
		}
	}
	if !hasCapability(q.Requires(), CapNonStandardLogic) || !hasCapability(q.Requires(), CapStrings) {
		t.Errorf("a strings query requires %v, want the strings and non-standard-logic capabilities", q.Requires())
	}
}

// hasCapability reports whether the capability is in the list.
func hasCapability(caps []Capability, want Capability) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}

// TestScriptShape: a script sets its logic, declares each datatype and variable
// once, comments every assertion, and ends by asking for satisfiability.
func TestScriptShape(t *testing.T) {
	finish := Sort{
		Kind: SortDatatype, Name: "test::Finish", Origin: "test::Finish",
		Values: []string{"test::Finish::matte", "test::Finish::polished"},
	}
	choice := &Var{Name: "test::ring::finish", Sort: finish, Location: "ring.sysml:4:3"}
	level := &Var{Name: "test::ring::level", Sort: Real, Dimension: "m", Location: "ring.sysml:5:3"}
	q := &Query{
		Kind:    "constraint",
		Element: "Polished",
		Sorts:   []Sort{finish},
		Vars:    []*Var{choice, level},
		Assertions: []Assertion{{
			Term: Binary(OpEq, Bool, VarTerm(choice), ValueTerm(finish, "test::Finish::matte")),
			From: Provenance{Kind: "constraint", Element: "Polished", Condition: "finish == Finish::matte", Role: RoleRequired, Location: "ring.sysml:7:3"},
		}},
	}
	want := strings.Join([]string{
		"; OpenSysML SMT-LIB2 translation of constraint Polished",
		"; the runtime evaluator remains normative; solving is an optional extension",
		"; no SMT-LIB logic covers algebraic datatypes (declare-datatypes), so the logic set below is ALL, which the SMT-LIB logic list does not define",
		"(set-logic ALL)",
		"; |test::Finish| of test::Finish",
		"(declare-datatypes ((|test::Finish| 0)) (((|test::Finish::matte|) (|test::Finish::polished|))))",
		"; test::ring::finish, declared at ring.sysml:4:3",
		"(declare-const |test::ring::finish| |test::Finish|)",
		"; test::ring::level in base units of m, declared at ring.sysml:5:3",
		"(declare-const |test::ring::level| Real)",
		"; required condition: finish == Finish::matte — constraint Polished, at ring.sysml:7:3",
		"(assert (= |test::ring::finish| |test::Finish::matte|))",
		"(check-sat)",
		"",
	}, "\n")
	if got := Script(q); got != want {
		t.Errorf("script is:\n%s\nwant:\n%s", got, want)
	}
}

// TestScriptCommentsStayOnOneLine: a condition written across lines still
// comments one line, so the script stays parsable.
func TestScriptCommentsStayOnOneLine(t *testing.T) {
	q := &Query{
		Kind:    "constraint",
		Element: "Multi",
		Assertions: []Assertion{{
			Term: BoolTerm(true),
			From: Provenance{Kind: "constraint", Element: "Multi", Condition: "a\n&& b", Role: RoleRequired},
		}},
	}
	for _, line := range strings.Split(strings.TrimSuffix(Script(q), "\n"), "\n") {
		if strings.HasPrefix(line, ";") && strings.Contains(line, "\n") {
			t.Errorf("comment spans lines: %q", line)
		}
		if !strings.HasPrefix(line, ";") && strings.Contains(line, "&& b") {
			t.Errorf("condition text escaped its comment: %q", line)
		}
	}
	if !strings.Contains(Script(q), "; required condition: a && b") {
		t.Errorf("condition is not commented on one line:\n%s", Script(q))
	}
}
