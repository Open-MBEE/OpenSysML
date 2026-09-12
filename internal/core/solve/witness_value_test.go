package solve

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// varsOf indexes a query's variables by name.
func varsOf(q *Query) map[string]*Var {
	out := make(map[string]*Var, len(q.Vars))
	for _, v := range q.Vars {
		out[v.Name] = v
	}
	return out
}

func intValue(i int64) runtime.ToolValue {
	return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValInt, Int: i}}
}

func realValue(f float64, unit string) runtime.ToolValue {
	return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValReal, Real: f}, Unit: unit}
}

func boolValue(b bool) runtime.ToolValue {
	return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValBool, Bool: b}}
}

func textValue(s string) runtime.ToolValue { return runtime.ToolValue{Text: s} }

// witnessQuery is a query over one variable of every sort a witness can state a value for.
func witnessQuery(t *testing.T) *Query {
	t.Helper()
	return constraintQuery(t, `
		package test {
			private import ScalarValues::*;
			public import SI::*;
			enum def Mode { enum idle; enum busy; }
			constraint def C {
				in n : Integer; in x : Real; in ok : Boolean; in mode : Mode;
				in mass : ISQ::MassValue;
				assert constraint { n > 2 }
				assert constraint { x * 2.0 == 5.0 }
				assert constraint { ok }
				assert constraint { mode == Mode::busy }
				assert constraint { mass <= 2.5 [kg] }
			}
		}`, "test::C")
}

// TestVarValueOfReadsEachSort: a witness value of the sort the variable holds is read
// into a model value; a magnitude must carry the base unit its variable is expressed in.
func TestVarValueOfReadsEachSort(t *testing.T) {
	vars := varsOf(witnessQuery(t))
	cases := []struct {
		name  string
		given runtime.ToolValue
		want  ModelValue
	}{
		{"test::C::n", intValue(3), ModelValue{Kind: SortInt, Number: big.NewRat(3, 1)}},
		{"test::C::x", realValue(2.5, ""), ModelValue{Kind: SortReal, Number: big.NewRat(5, 2)}},
		{"test::C::x", intValue(2), ModelValue{Kind: SortReal, Number: big.NewRat(2, 1)}},
		{"test::C::ok", boolValue(true), ModelValue{Kind: SortBool, Bool: true}},
		{"test::C::mode", textValue("test::Mode::busy"), ModelValue{Kind: SortDatatype, Text: "test::Mode::busy"}},
		{"test::C::mass", realValue(2000, "gram"), ModelValue{Kind: SortReal, Number: big.NewRat(2000, 1)}},
	}
	for _, tc := range cases {
		got, err := vars[tc.name].ValueOf(tc.given)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got.Kind != tc.want.Kind || got.Bool != tc.want.Bool || got.Text != tc.want.Text ||
			(got.Number == nil) != (tc.want.Number == nil) || (got.Number != nil && got.Number.Cmp(tc.want.Number) != 0) {
			t.Errorf("%s: read %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// TestVarValueOfRefusesWrongValues: a value of another sort, an undeclared enumeration
// literal, a unit on what has none, a bare magnitude and a wrong unit are each refused
// with a reason naming what the variable holds.
func TestVarValueOfRefusesWrongValues(t *testing.T) {
	vars := varsOf(witnessQuery(t))
	cases := []struct {
		name  string
		given runtime.ToolValue
		want  string
	}{
		{"test::C::n", realValue(2.5, ""), "is not a value of Int"},
		{"test::C::n", textValue("3"), "is a text"},
		{"test::C::n", intValue(3), ""},
		{"test::C::ok", intValue(1), "is not a value of Bool"},
		{"test::C::mode", textValue("busy"), `"busy" is not a value of test::Mode`},
		{"test::C::mode", textValue("test::Mode::off"), `"test::Mode::off" is not a value of`},
		{"test::C::mode", intValue(1), "is not a value of"},
		{"test::C::x", realValue(1, "m"), "is measured in m"},
		{"test::C::mass", realValue(2, ""), "is a bare number"},
		{"test::C::mass", realValue(2, "kg"), "is measured in kg"},
	}
	for _, tc := range cases {
		_, err := vars[tc.name].ValueOf(tc.given)
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%s given %+v: error %v, want one containing %q", tc.name, tc.given, err, tc.want)
		}
	}
}

// TestModelValueAssignRendersAsSolved: a model value assigned to its variable renders as
// the solver's own assignment of that value would, in the notation's terms.
func TestModelValueAssignRendersAsSolved(t *testing.T) {
	vars := varsOf(witnessQuery(t))
	cases := []struct {
		name  string
		value ModelValue
		want  string
	}{
		{"test::C::n", ModelValue{Kind: SortInt, Number: big.NewRat(3, 1)}, "3"},
		{"test::C::n", ModelValue{Kind: SortInt, Number: big.NewRat(-3, 1)}, "-3"},
		{"test::C::x", ModelValue{Kind: SortReal, Number: big.NewRat(5, 2)}, "2.5"},
		{"test::C::ok", ModelValue{Kind: SortBool, Bool: true}, "true"},
		{"test::C::mode", ModelValue{Kind: SortDatatype, Text: "test::Mode::busy"}, "test::Mode::busy"},
	}
	for _, tc := range cases {
		a, err := tc.value.Assign(vars[tc.name])
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if !a.Rendered || a.Var != vars[tc.name] {
			t.Errorf("%s: assignment %+v is not rendered for its variable", tc.name, a)
		}
		if a.Value != tc.want {
			t.Errorf("%s: rendered %q, want %q", tc.name, a.Value, tc.want)
		}
		back, err := DecodeValue(a)
		if err != nil || back.Kind != tc.value.Kind {
			t.Errorf("%s: decodes back as %+v, %v", tc.name, back, err)
		}
	}
	mass, err := (ModelValue{Kind: SortReal, Number: big.NewRat(2000, 1)}).Assign(vars["test::C::mass"])
	if err != nil || !strings.Contains(mass.Value, "2000") || !strings.Contains(mass.Value, "gram") {
		t.Errorf("a magnitude renders as %q, %v; want it in its base unit", mass.Value, err)
	}
	if _, err := (ModelValue{Kind: SortBool, Bool: true}).Assign(vars["test::C::n"]); err == nil {
		t.Error("a truth assigned to an Integer variable was accepted")
	}
}

// TestQueryConfirmChecksEveryVariableAndAssertion: an assignment is confirmed when every
// variable has a value of its sort and every assertion evaluates true; otherwise the
// reason names the first variable or assertion that fails.
func TestQueryConfirmChecksEveryVariableAndAssertion(t *testing.T) {
	q := witnessQuery(t)
	good := map[string]ModelValue{
		"test::C::n":    {Kind: SortInt, Number: big.NewRat(3, 1)},
		"test::C::x":    {Kind: SortReal, Number: big.NewRat(5, 2)},
		"test::C::ok":   {Kind: SortBool, Bool: true},
		"test::C::mode": {Kind: SortDatatype, Text: "test::Mode::busy"},
		"test::C::mass": {Kind: SortReal, Number: big.NewRat(2000, 1)},
	}
	if ok, why := q.Confirm(good); !ok {
		t.Fatalf("a satisfying assignment was refused: %s", why)
	}
	with := func(name string, v ModelValue) map[string]ModelValue {
		out := make(map[string]ModelValue, len(good))
		for k, val := range good {
			out[k] = val
		}
		out[name] = v
		return out
	}
	without := func(name string) map[string]ModelValue {
		out := with(name, ModelValue{})
		delete(out, name)
		return out
	}
	cases := []struct {
		name   string
		values map[string]ModelValue
		want   string
	}{
		{"missing", without("test::C::ok"), "gives test::C::ok no value"},
		{"wrong sort", with("test::C::n", ModelValue{Kind: SortBool, Bool: true}), "no value of sort Int"},
		{"undeclared literal", with("test::C::mode", ModelValue{Kind: SortDatatype, Text: "test::Mode::off"}), "test::Mode::off, not a value of"},
		{"false assertion", with("test::C::n", ModelValue{Kind: SortInt, Number: big.NewRat(1, 1)}), "n > 2"},
		{"false real assertion", with("test::C::x", ModelValue{Kind: SortReal, Number: big.NewRat(1, 1)}), "x * 2.0 == 5.0"},
		{"false enumeration", with("test::C::mode", ModelValue{Kind: SortDatatype, Text: "test::Mode::idle"}), "mode == Mode::busy"},
		{"false magnitude", with("test::C::mass", ModelValue{Kind: SortReal, Number: big.NewRat(3000, 1)}), "mass <= 2.5 [kg]"},
	}
	for _, tc := range cases {
		ok, why := q.Confirm(tc.values)
		if ok || !strings.Contains(why, tc.want) {
			t.Errorf("%s: confirmed=%v reason %q, want a refusal naming %q", tc.name, ok, why, tc.want)
		}
	}
}
