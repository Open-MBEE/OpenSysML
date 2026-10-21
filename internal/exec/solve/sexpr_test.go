package solve

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

// parse reads one S-expression from text, as the driver reads a reply.
func parse(t *testing.T, text string) sexpr {
	t.Helper()
	got, err := readSexpr(bufio.NewReader(strings.NewReader(text)))
	if err != nil {
		t.Fatalf("read %q: %v", text, err)
	}
	return got
}

func TestReadSexpr(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"atom", "sat\n", "sat"},
		{"atom at end of output", "sat", "sat"},
		{"empty list", "()\n", "()"},
		{"nested list", "((x 1) (y (- 2)))\n", "((x 1) (y (- 2)))"},
		{"leading comment", "; thinking\nunsat\n", "unsat"},
		{"quoted symbol", "|Check::C::a b|\n", "|Check::C::a b|"},
		{"string literal", `"a ""quoted"" reply"` + "\n", `"a ""quoted"" reply"`},
		{"comment inside a list", "(x ; note\n 1)\n", "(x 1)"},
		{"decimal", "(/ 1.0 3.0)\n", "(/ 1.0 3.0)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parse(t, c.in).String(); got != c.want {
				t.Errorf("read %q as %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// A reply written back and read again is the same reply: a `|quoted|` symbol
// keeps its bars, whatever a SysML name carries, so a value kept as the solver's
// text decodes to the atom the solver gave rather than to its first word.
func TestSexprStringReadsBackToTheSameAtom(t *testing.T) {
	names := []string{
		"Cfg Space::fam::engine::e4",
		"7b-Variant Configurations::VariabilityModel::'engine-transmission selection constraint'",
		"Fahrzeug::Größe::große",
		"Vehicle::engine's::v8",
		"with|bar", `with\backslash`, "with!bang", "a::b::c", "(parens)", `"quotes"`, "",
	}
	for _, name := range names {
		symbol := smtSymbol(name)
		for _, text := range []string{symbol, "(x " + symbol + ")", "((|P::v| " + symbol + "))"} {
			first := parse(t, text)
			again := parse(t, first.String())
			if again.String() != first.String() {
				t.Errorf("%q read as %q, which reads as %q", text, first.String(), again.String())
			}
		}
		atom := parse(t, symbol)
		if atom.IsList || atom.Quoted || smtName(atom.Atom) != name {
			t.Errorf("%q read as %+v, want the atom %q", symbol, atom, name)
		}
		if atom.Symbol != strings.HasPrefix(symbol, "|") {
			t.Errorf("%q read with Symbol=%v, want %v", symbol, atom.Symbol, !atom.Symbol)
		}
	}
}

func TestReadSexprReportsAnEndedOutputAsEOF(t *testing.T) {
	for _, in := range []string{"", "   \n", "; only a comment\n"} {
		_, err := readSexpr(bufio.NewReader(strings.NewReader(in)))
		if !errors.Is(err, errEOF) {
			t.Errorf("read %q gave %v, want errEOF", in, err)
		}
	}
}

func TestReadSexprRejectsMalformedReplies(t *testing.T) {
	for _, in := range []string{")\n", "(x\n", "|unterminated\n"} {
		_, err := readSexpr(bufio.NewReader(strings.NewReader(in)))
		if err == nil {
			t.Errorf("read %q gave no error", in)
		}
	}
}

func TestSexprError(t *testing.T) {
	msg, ok := parse(t, `(error "line 3: unsupported")`).isError()
	if !ok || msg != "line 3: unsupported" {
		t.Errorf("isError gave %q, %v", msg, ok)
	}
	if _, ok := parse(t, "unsat").isError(); ok {
		t.Error("a verdict was read as an error reply")
	}
}
