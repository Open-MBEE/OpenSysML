package solve

import (
	"testing"
)

// enumSort is a datatype sort standing for an enumeration definition's literals.
var enumSort = Sort{Kind: SortDatatype, Name: "Gears", Values: []string{"low", "high|est"}, Origin: "Gears"}

func TestRenderedAssignments(t *testing.T) {
	cases := []struct {
		name  string
		v     *Var
		reply string
		want  string
	}{
		{"integer", &Var{Name: "P::i", Sort: Int}, "4", "4"},
		{"negative integer", &Var{Name: "P::i", Sort: Int}, "(- 4)", "-4"},
		{"real", &Var{Name: "P::x", Sort: Real}, "1.5", "1.5"},
		{"real numeral", &Var{Name: "P::x", Sort: Real}, "2", "2.0"},
		{"widened real", &Var{Name: "P::x", Sort: Real}, "(to_real 3)", "3.0"},
		{"rational real", &Var{Name: "P::x", Sort: Real}, "(/ 1.0 3.0)", "1/3"},
		{"negative real", &Var{Name: "P::x", Sort: Real}, "(- (/ 3.0 2.0))", "-1.5"},
		{"quantity in its base units", &Var{Name: "P::m", Sort: Real, Dimension: "M", Unit: "gram"}, "1500.0", "1500.0 [gram]"},
		{"quantity whose base units are unnamed", &Var{Name: "P::m", Sort: Real, Dimension: "M"}, "1500.0", "1500.0 (in the base units of M)"},
		{"boolean", &Var{Name: "P::b", Sort: Bool}, "true", "true"},
		{"string", &Var{Name: "P::s", Sort: String}, `"ok"`, `"ok"`},
		{"enumeration literal", &Var{Name: "P::g", Sort: enumSort}, "low", "low"},
		{"escaped enumeration literal", &Var{Name: "P::g", Sort: enumSort}, "high!pest", "high|est"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := assign(c.v, parse(t, c.reply))
			if !got.Rendered {
				t.Fatalf("value %s was not rendered: %q", c.reply, got.Value)
			}
			if got.Value != c.want {
				t.Errorf("rendered %s as %q, want %q", c.reply, got.Value, c.want)
			}
			if got.Raw == "" {
				t.Error("the solver's own reply was not kept")
			}
		})
	}
}

// A value the notation has no literal for is reported as the solver wrote it,
// marked as unrendered rather than mistaken for an OpenSysML value.
func TestUnrenderedAssignments(t *testing.T) {
	cases := []struct {
		name  string
		v     *Var
		reply string
	}{
		{"algebraic number", &Var{Name: "P::x", Sort: Real}, "(root-obj (+ (^ x 2) (- 2)) 1)"},
		{"a real where an integer was declared", &Var{Name: "P::i", Sort: Int}, "1.5"},
		{"an undeclared enumeration value", &Var{Name: "P::g", Sort: enumSort}, "reverse"},
		{"a non-boolean where a boolean was declared", &Var{Name: "P::b", Sort: Bool}, "1"},
		{"an exponent form", &Var{Name: "P::x", Sort: Real}, "1e5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := assign(c.v, parse(t, c.reply))
			if got.Rendered {
				t.Fatalf("value %s was rendered as %q", c.reply, got.Value)
			}
			if got.Value != got.Raw || got.Raw == "" {
				t.Errorf("unrendered value is %q, want the solver's own %q", got.Value, got.Raw)
			}
		})
	}
}

// smtName is the inverse of the escaping a script's symbols are written with, so
// a model names the feature the query declared.
func TestSMTNameInvertsSMTSymbol(t *testing.T) {
	for _, name := range []string{
		"Check::C::i", "a b", "with|bar", `with\backslash`, "with!bang", "!", "|", "",
	} {
		symbol := smtSymbol(name)
		if got := smtName(trimBars(symbol)); got != name {
			t.Errorf("smtName(%q) = %q, want %q", symbol, got, name)
		}
	}
}

// trimBars strips the `|…|` a quoted symbol is written in, as the reply reader
// does before a name is recovered.
func trimBars(symbol string) string {
	if len(symbol) >= 2 && symbol[0] == '|' && symbol[len(symbol)-1] == '|' {
		return symbol[1 : len(symbol)-1]
	}
	return symbol
}
