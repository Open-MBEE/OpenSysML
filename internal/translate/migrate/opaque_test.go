package migrate

import (
	"strconv"
	"strings"
	"testing"
)

// fakeScope answers the translator's names from a table, as a block with
// these features would; a path through `this` reads the same table.
type fakeScope map[string]opaqueRef

func (s fakeScope) feature(path []string, write bool) (opaqueRef, *refusal) {
	name := strings.Join(path, ".")
	if path[0] == "this" && len(path) > 1 {
		name = strings.Join(path[1:], ".")
	}
	if name == "clock" && write {
		return opaqueRef{}, &refusal{kind: refusedConstruct, token: name, why: "the simulation clock is read, never assigned"}
	}
	ref, ok := s[name]
	if !ok {
		return opaqueRef{}, &refusal{kind: refusedName, token: name, why: "nothing is called " + name}
	}
	return ref, nil
}

var testScope = fakeScope{
	"i":                {expr: "this.i", scalar: "Integer"},
	"Retries":          {expr: "this.Retries", scalar: "Integer"},
	"GS_Found":         {expr: "this.GS_Found", scalar: "Boolean"},
	"t":                {expr: "this.t", scalar: "Real"},
	"t0":               {expr: "this.t0", scalar: "Real"},
	"clock":            {expr: clockRead, scalar: "Real"},
	"name":             {expr: "this.name", scalar: "String"},
	"xs":               {expr: "this.xs", scalar: "Real", plural: true},
	"ns":               {expr: "this.ns", scalar: "Integer", plural: true},
	"mode":             {expr: "this.mode"},
	"OFF":              {expr: "Modes::OFF"},
	"state":            {expr: "this.state", object: []string{"State"}},
	"ON":               {expr: "States::ON", object: []string{"State"}},
	"tank":             {expr: "this.tank", object: []string{"Tank"}},
	"tanks":            {expr: "this.tanks", object: []string{"Tank"}, plural: true},
	"drum":             {expr: "this.drum", object: []string{"Drum", "Tank"}},
	"vat":              {expr: "this.vat", object: []string{"Vat", "Tank"}},
	"tcs.i":            {expr: "this.tcs.i", scalar: "Integer"},
	"Guide Star Found": {expr: "this.'Guide Star Found'", scalar: "Boolean"},
	"Guide Star":       {expr: "this.'Guide Star'", object: []string{"Star"}},
	"Stage 2 Ready":    {expr: "this.'Stage 2 Ready'", scalar: "Boolean"},
	"Retry Count":      {expr: "this.'Retry Count'", scalar: "Integer"},
}

func TestTranslateExpr(t *testing.T) {
	cases := []struct {
		lang, body, want, expr, scalar string
	}{
		{"JavaScript", "i >= Retries", "Boolean", "this.i >= this.Retries", "Boolean"},
		{"JavaScript", "GS_Found", "Boolean", "this.GS_Found", "Boolean"},
		{"JavaScript", "GS_Found;", "Boolean", "this.GS_Found", "Boolean"},
		{"JavaScript", "!GS_Found", "", "not this.GS_Found", "Boolean"},
		{"JavaScript", "i < Retries && !GS_Found", "", "this.i < this.Retries and not this.GS_Found", "Boolean"},
		{"JavaScript", "i == 1 || i === 2", "", "this.i == 1 or this.i == 2", "Boolean"},
		{"JavaScript", "i != Retries", "", "this.i != this.Retries", "Boolean"},
		{"JavaScript", "(t - t0) * 2", "", "(this.t - this.t0) * 2", "Real"},
		{"JavaScript", "t / 2 + i % 3", "", "this.t / 2 + this.i % 3", "Real"},
		{"JavaScript", "t - (t0 - 1)", "", "this.t - (this.t0 - 1)", "Real"},
		{"JavaScript", "(t + t0) * 2", "", "(this.t + this.t0) * 2", "Real"},
		{"JavaScript", "-(t + 1)", "", "-(this.t + 1)", "Real"},
		{"JavaScript", "!(GS_Found || i > 0)", "", "not (this.GS_Found or this.i > 0)", "Boolean"},
		{"JavaScript", "(i > 0 || GS_Found) && i < 3", "", "(this.i > 0 or this.GS_Found) and this.i < 3", "Boolean"},
		{"JavaScript", "-t", "", "-this.t", "Real"},
		{"JavaScript", "1.5e3", "", "1500.0", "Real"},
		{"JavaScript", "'a\"b'", "", `"a\"b"`, "String"},
		{"JavaScript", `"a\nb\tc\rd\be\ff"`, "", `"a\nb\tc\rd\be\ff"`, "String"},
		{"JavaScript", `"\x41\u0042\u{1F600}\\\'\d"`, "", "\"AB\U0001F600\\\\'d\"", "String"},
		{"JavaScript", "\"a\\\nb\"", "", `"ab"`, "String"},
		{"JavaScript", "Math.round(t)", "", "RealFunctions::floor(this.t + 0.5)", "Integer"},
		{"JavaScript", "state == ON", "", "this.state == States::ON", "Boolean"},
		{"JavaScript", "state != ON", "Boolean", "this.state != States::ON", "Boolean"},
		{"JavaScript", "state == mode", "", "this.state == this.mode", "Boolean"},
		{"JavaScript", "tank == drum", "", "this.tank == this.drum", "Boolean"},
		{"JavaScript", "drum != tank", "", "this.drum != this.tank", "Boolean"},
		{"JavaScript", "drum == vat", "", "this.drum == this.vat", "Boolean"},
		{"JavaScript", "GS_Found ? drum : tank", "", "if this.GS_Found ? this.drum else this.tank", ""},
		{"JavaScript", "GS_Found ? state : ON", "", "if this.GS_Found ? this.state else States::ON", ""},
		{"JavaScript", "9007199254740991", "", "9007199254740991", "Integer"},
		{"Java", "9007199254740993", "", "9007199254740993", "Integer"},
		{"Java", "9223372036854775807", "", "9223372036854775807", "Integer"},
		{"JavaScript", "9007199254740993.0", "", "9007199254740992.0", "Real"},
		{"JavaScript", `"\uD83D\uDE00"`, "", "\"\U0001F600\"", "String"},
		{"JavaScript", `"a\uD83D\uDE00b\u{1F600}"`, "", "\"a\U0001F600b\U0001F600\"", "String"},
		{"JavaScript", `"\u{D83D}\u{DE00}"`, "", "\"\U0001F600\"", "String"},
		{"JavaScript", "Math.round(t - t0)", "", "RealFunctions::floor(this.t - this.t0 + 0.5)", "Integer"},
		{"JavaScript", "i > 0 ? t : 0.0", "", "if (this.i > 0) ? this.t else 0.0", "Real"},
		{"JavaScript", "Math.max(t, t0, 1.0)", "", "RealFunctions::max(RealFunctions::max(this.t, this.t0), 1.0)", "Real"},
		{"JavaScript", "Math.min(i, 3)", "", "IntegerFunctions::min(this.i, 3)", "Integer"},
		{"Java", "Math.max(t, t0)", "", "RealFunctions::max(this.t, this.t0)", "Real"},
		{"JavaScript", "Math.abs(t - t0)", "", "RealFunctions::abs(this.t - this.t0)", "Real"},
		{"JavaScript", "Math.abs(i)", "", "IntegerFunctions::abs(this.i)", "Integer"},
		{"JavaScript", "Math.floor(t)", "", "RealFunctions::floor(this.t)", "Integer"},
		{"JavaScript", "Math.ceil(t)", "", "OpenSysMLMathFunctions::ceiling(this.t)", "Integer"},
		{"JavaScript", "Math.sqrt(t)", "", "RealFunctions::sqrt(this.t)", "Real"},
		{"JavaScript", "Math.pow(t, 2)", "", "this.t ** 2", "Real"},
		{"JavaScript", "(-t) ** 2", "", "(-this.t) ** 2", "Real"},
		{"JavaScript", "-(t ** 2)", "", "-(this.t ** 2)", "Real"},
		{"JavaScript", "2 ** -i", "", "2 ** -this.i", "Real"},
		{"JavaScript", "1", "Natural", "1", "Integer"},
		{"Java", "java.util.Collections.max(xs)", "", "this.xs->ControlFunctions::reduce { in x; in y; RealFunctions::max(x, y) }", "Real"},
		{"Java", "i / Retries", "", "OpenSysMLMathFunctions::quotient(this.i, this.Retries)", "Integer"},
		{"Java", "(i + 1) / 2 * 3", "", "OpenSysMLMathFunctions::quotient(this.i + 1, 2) * 3", "Integer"},
		{"Java", "t / 2", "", "this.t / 2", "Real"},
		{"Java", "i / 2.0", "", "this.i / 2.0", "Real"},
		{"Java", "Math.floor(t)", "", "RealFunctions::floor(this.t)", "Real"},
		{"Java", "Math.ceil(t)", "", "OpenSysMLMathFunctions::ceiling(this.t)", "Real"},
		{"Java", "Math.round(t)", "", "RealFunctions::floor(this.t + 0.5)", "Integer"},
		{"Java", "Math.floor(t) / 2", "", "RealFunctions::floor(this.t) / 2", "Real"},
		{"Java", "Math.ceil(t) / i", "", "OpenSysMLMathFunctions::ceiling(this.t) / this.i", "Real"},
		{"Java", "Math.round(t) / 2", "", "OpenSysMLMathFunctions::quotient(RealFunctions::floor(this.t + 0.5), 2)", "Integer"},
		{"Java", "Math.floor(t) + 1", "", "RealFunctions::floor(this.t) + 1", "Real"},
		{"JavaScript", "Math.floor(t) + 1", "", "RealFunctions::floor(this.t) + 1", "Integer"},
		{"JavaScript", "Math.ceil(t) / 2", "", "OpenSysMLMathFunctions::ceiling(this.t) / 2", "Real"},
		{"JavaScript", `name == "ready"`, "", `this.name == "ready"`, "Boolean"},
		{"JavaScript", `name !== "ready"`, "", `this.name != "ready"`, "Boolean"},
		{"Java", `name.equals("ready")`, "", `this.name == "ready"`, "Boolean"},
		{"Java", `"ready".equals(name)`, "", `"ready" == this.name`, "Boolean"},
		{"Java", `!name.equals("ready")`, "", `not (this.name == "ready")`, "Boolean"},
		{"Java", `mode.equals(name)`, "", `this.mode == this.name`, "Boolean"},
		{"Java", "i == Retries", "", "this.i == this.Retries", "Boolean"},
		{"Java", "mode == OFF", "", "this.mode == Modes::OFF", "Boolean"},
		{"Java 8", "i / 2", "", "OpenSysMLMathFunctions::quotient(this.i, 2)", "Integer"},
		{"java17", "i / 2", "", "OpenSysMLMathFunctions::quotient(this.i, 2)", "Integer"},
		{"Java 1.8.0_202", "i / 2", "", "OpenSysMLMathFunctions::quotient(this.i, 2)", "Integer"},
		{"Java 17.0.2+8", "i / 2", "", "OpenSysMLMathFunctions::quotient(this.i, 2)", "Integer"},
		{"Java 11-ea", "i / 2", "", "OpenSysMLMathFunctions::quotient(this.i, 2)", "Integer"},
		{"JavaScript", "i / Retries", "", "this.i / this.Retries", "Real"},
		{"JavaScript", "Collections.min(ns)", "", "this.ns->ControlFunctions::reduce { in x; in y; IntegerFunctions::min(x, y) }", "Integer"},
		{"JavaScript", "this.i + 1", "", "this.i + 1", "Integer"},
		{"JavaScript", "tcs.i == 2", "", "this.tcs.i == 2", "Boolean"},
		{"JavaScript", "mode == OFF", "", "this.mode == Modes::OFF", "Boolean"},
		{"JavaScript", "clock - t0", "", clockRead + " - this.t0", "Real"},
		{"", "i = Retries", "Boolean", "this.i == this.Retries", "Boolean"},
		{"English", "TRUE", "Boolean", "true", "Boolean"},
		{"English", "false", "", "false", "Boolean"},
		{"English", "GS_Found", "Boolean", "this.GS_Found", "Boolean"},
		{"English", "not GS_Found", "", "not this.GS_Found", "Boolean"},
		{"English", "GS_Found and i = 1", "", "this.GS_Found and this.i == 1", "Boolean"},
		{"English", "GS_Found or i == Retries", "", "this.GS_Found or this.i == this.Retries", "Boolean"},
		{"English", "i >= Retries", "", "this.i >= this.Retries", "Boolean"},
		{"English", "Guide Star Found", "Boolean", "this.'Guide Star Found'", "Boolean"},
		{"English", "not Guide Star Found", "Boolean", "not this.'Guide Star Found'", "Boolean"},
		{"English", "Guide Star Found and not Stage 2 Ready", "Boolean", "this.'Guide Star Found' and not this.'Stage 2 Ready'", "Boolean"},
		{"English", "(Guide Star Found or GS_Found) and Retry Count >= Retries", "Boolean", "(this.'Guide Star Found' or this.GS_Found) and this.'Retry Count' >= this.Retries", "Boolean"},
		{"English", "Retry Count = 1", "Boolean", "this.'Retry Count' == 1", "Boolean"},
		{"English", "NOT Guide Star Found OR Stage 2 Ready", "Boolean", "not this.'Guide Star Found' or this.'Stage 2 Ready'", "Boolean"},
		{"English", "Mean = 0.0", "", "this.t == 0.0", "Boolean"},
	}
	for _, c := range cases {
		body := c.body
		if strings.HasPrefix(body, "Mean") {
			body = strings.Replace(body, "Mean", "t", 1)
		}
		got, err := translateExpr(body, c.lang, testScope, oneOf(c.want))
		if err != nil {
			t.Errorf("%s %q: refused: %s", c.lang, c.body, err.note())
			continue
		}
		if got.expr != c.expr || got.scalar != c.scalar {
			t.Errorf("%s %q: got %q (%s), want %q (%s)", c.lang, c.body, got.expr, got.scalar, c.expr, c.scalar)
		}
		if _, ok := parseExpr(got.expr); !ok {
			t.Errorf("%s %q: translation %q does not parse as a v2 expression", c.lang, c.body, got.expr)
		}
	}
}

// A value wanted as a non-scalar type is held by it or a type specializing it,
// and a collection is taken where several values are held.
func TestTranslateExprWanted(t *testing.T) {
	tank := wanted{object: []string{"Tank"}, single: true}
	cases := []struct {
		body string
		want wanted
		expr string
	}{
		{"drum", tank, "this.drum"},
		{"GS_Found ? drum : vat", tank, "if this.GS_Found ? this.drum else this.vat"},
		{"tank", wanted{object: []string{"Drum", "Tank"}}, ""},
		{"tanks", wanted{object: []string{"Tank"}}, "this.tanks"},
		{"xs", wanted{scalar: "Real"}, "this.xs"},
		{"tanks", tank, ""},
		{"i", tank, ""},
	}
	for _, c := range cases {
		got, err := translateExpr(c.body, "JavaScript", testScope, c.want)
		switch {
		case c.expr == "" && err == nil:
			t.Errorf("%q wanted as %+v: translated %q, want a refusal", c.body, c.want, got.expr)
		case c.expr != "" && err != nil:
			t.Errorf("%q wanted as %+v: refused: %s", c.body, c.want, err.note())
		case err == nil && got.expr != c.expr:
			t.Errorf("%q wanted as %+v: got %q, want %q", c.body, c.want, got.expr, c.expr)
		}
	}
}

func TestTranslateStatements(t *testing.T) {
	cases := []struct {
		lang, body string
		lines      []string
	}{
		{"JavaScript", "i = 1", []string{"assign this.i := 1;"}},
		{"JavaScript", "i += 1;", []string{"assign this.i := this.i + 1;"}},
		{"JavaScript", "i++", []string{"assign this.i := this.i + 1;"}},
		{"JavaScript", "--i;", []string{"assign this.i := this.i - 1;"}},
		{"JavaScript", "t -= 2; t *= 3;\nt /= 4", []string{
			"assign this.t := this.t - 2;", "assign this.t := this.t * 3;", "assign this.t := this.t / 4;"}},
		{"JavaScript", "GS_Found = true", []string{"assign this.GS_Found := true;"}},
		{"JavaScript", "i = 2.0", []string{"assign this.i := 2;"}},
		{"JavaScript", "state = ON", []string{"assign this.state := States::ON;"}},
		{"JavaScript", "state = mode", []string{"assign this.state := this.mode;"}},
		{"JavaScript", "tank = drum", []string{"assign this.tank := this.drum;"}},
		{"JavaScript", "tank = GS_Found ? drum : vat", []string{"assign this.tank := if this.GS_Found ? this.drum else this.vat;"}},
		{"JavaScript", "i = 9007199254740991", []string{"assign this.i := 9007199254740991;"}},
		{"Java", "i = 9007199254740993", []string{"assign this.i := 9007199254740993;"}},
		{"JavaScript", `name = "\uD83D\uDE00"`, []string{"assign this.name := \"\U0001F600\";"}},
		{"Java", "i /= 2", []string{"assign this.i := OpenSysMLMathFunctions::quotient(this.i, 2);"}},
		{"Java", "t /= 2", []string{"assign this.t := this.t / 2;"}},
		{"Java", "t = Math.floor(t) / 2", []string{"assign this.t := RealFunctions::floor(this.t) / 2;"}},
		{"Java", "i = Math.round(t) / 2", []string{"assign this.i := OpenSysMLMathFunctions::quotient(RealFunctions::floor(this.t + 0.5), 2);"}},
		{"JavaScript", "i = Math.floor(t)", []string{"assign this.i := RealFunctions::floor(this.t);"}},
		{"JavaScript", "t = 2", []string{"assign this.t := 2;"}},
		{"JavaScript", "t = clock\nt0 = clock - t;", []string{
			"assign this.t := " + clockRead + ";", "assign this.t0 := " + clockRead + " - this.t;"}},
		{"JavaScript", "// start\nt = 0.0; /* reset */ i = 0", []string{"assign this.t := 0.0;", "assign this.i := 0;"}},
		{"JavaScript", "i = 1 /* explanation\n */ t = 2", []string{"assign this.i := 1;", "assign this.t := 2;"}},
		{"JavaScript", "i = 1\r\nt = 2\ri = 2\u2028t = 3", []string{
			"assign this.i := 1;", "assign this.t := 2;", "assign this.i := 2;", "assign this.t := 3;"}},
		{"JavaScript", "i = 1 // one\rt = 2", []string{"assign this.i := 1;", "assign this.t := 2;"}},
		{"JavaScript", "i = 1 /* one */ + 2", []string{"assign this.i := 1 + 2;"}},
		{"JavaScript", "var n = i + 1; i = n * 2", []string{
			"attribute n : ScalarValues::Integer;", "assign n := this.i + 1;", "assign this.i := n * 2;"}},
		{"JavaScript", "this.tcs.i = Math.max(i, 0)", []string{"assign this.tcs.i := IntegerFunctions::max(this.i, 0);"}},
		{"JavaScript", "const n = 2; i = i * n", []string{
			"attribute n : ScalarValues::Integer;", "assign n := 2;", "assign this.i := this.i * n;"}},
		{"", "GS_Found = i >= Retries", []string{"assign this.GS_Found := this.i >= this.Retries;"}},
	}
	for _, c := range cases {
		got, err := translateStatements(c.body, c.lang, testScope)
		if err != nil {
			t.Errorf("%s %q: refused: %s", c.lang, c.body, err.note())
			continue
		}
		if strings.Join(got, "\n") != strings.Join(c.lines, "\n") {
			t.Errorf("%s %q:\n got  %q\n want %q", c.lang, c.body, got, c.lines)
		}
		for _, line := range got {
			if !parseStatement(line) {
				t.Errorf("%s %q: line %q does not parse in an action body", c.lang, c.body, line)
			}
		}
	}
}

func TestTranslateRefusals(t *testing.T) {
	cases := []struct {
		lang, body string
		statements bool
		kind       refusalKind
		token      string
	}{
		{"OCL", "i > 1", false, refusedLanguage, "OCL"},
		{"English", "i = 1", true, refusedLanguage, "English"},
		{"JavaCC", "i / 2", false, refusedLanguage, "JavaCC"},
		{"Java Expression Language", "i / 2", false, refusedLanguage, "Java Expression Language"},
		{"JavaFX Script", "i = i / 2", true, refusedLanguage, "JavaFX Script"},
		{"Java SE", "i / 2", false, refusedLanguage, "Java SE"},
		{"Java 8 SE", "i / 2", false, refusedLanguage, "Java 8 SE"},
		{"JavaScript", "for (i = 0; i < 3; i++) t = 1", true, refusedConstruct, "for"},
		{"JavaScript", "i = 1 /* one */ t = 2", true, refusedSyntax, "t"},
		{"Java", "mode / 2", false, refusedType, "/"},
		{"Java", "i / mode", false, refusedType, "/"},
		{"Java", "i = Math.floor(t)", true, refusedType, "i ="},
		{"Java", "i = Math.ceil(t)", true, refusedType, "i ="},
		{"Java", `name == "ready"`, false, refusedConstruct, "=="},
		{"Java", `name != "ready"`, false, refusedConstruct, "!="},
		{"Java", `"ready" == mode`, false, refusedConstruct, "=="},
		{"Java", "GS_Found = name == name", true, refusedConstruct, "=="},
		{"Java", "name.equals(i)", false, refusedType, "name.equals"},
		{"Java", "i.equals(1)", false, refusedType, "i.equals"},
		{"Java", "mode.equals(OFF)", false, refusedType, "mode.equals"},
		{"Java", "xs.equals(name)", false, refusedType, "xs.equals"},
		{"Java", `name.equals("a", "b")`, false, refusedCall, "name.equals"},
		{"Java", `"a".equals()`, false, refusedCall, `"a".equals`},
		{"Java", `"a".trim()`, false, refusedCall, `"a".trim`},
		{"Java", `"a".length`, false, refusedConstruct, `"a".length`},
		{"JavaScript", `name.equals("ready")`, false, refusedCall, "name.equals"},
		{"JavaScript", "while (GS_Found) i = 1", true, refusedConstruct, "while"},
		{"JavaScript", "if (GS_Found) i = 1", true, refusedConstruct, "if"},
		{"JavaScript", "var a = 1, b = 2", true, refusedConstruct, "var a"},
		{"JavaScript", "var a", true, refusedConstruct, "var a"},
		{"JavaScript", "var i = 1; i += 1", true, refusedConstruct, "var i"},
		{"JavaScript", "let t = 1; t = t0", true, refusedConstruct, "let t"},
		{"JavaScript", "var n = 1; var n = 2", true, refusedConstruct, "var n"},
		{"JavaScript", "var n = 1; const n = 2", true, refusedConstruct, "const n"},
		{"JavaScript", "var done = 1", true, refusedConstruct, "var done"},
		{"JavaScript", "var start = 0; i = start", true, refusedConstruct, "var start"},
		{"JavaScript", "print(\"done\")", true, refusedCall, "print"},
		{"JavaScript", "t = clock; print(\"t: \" + t);", true, refusedCall, "print"},
		{"JavaScript", "i = Math.random()", true, refusedCall, "Math.random"},
		{"JavaScript", "i = Math.max(1)", true, refusedCall, "Math.max"},
		{"Java", "t = Math.max(t, t0, 1.0)", true, refusedCall, "Math.max"},
		{"Java", "i = Math.min(i)", true, refusedCall, "Math.min"},
		{"JavaScript", "i = new Date()", true, refusedConstruct, "new"},
		{"JavaScript", `name = "\v"`, true, refusedConstruct, `\v`},
		{"JavaScript", `name = "\0"`, true, refusedConstruct, `\0`},
		{"JavaScript", `name = "\101"`, true, refusedConstruct, `\1`},
		{"JavaScript", `name = "\01"`, true, refusedConstruct, `\01`},
		{"JavaScript", `name = "\x1b[0m"`, true, refusedConstruct, `\x1b`},
		{"JavaScript", `name = "\u001F"`, true, refusedConstruct, `\u001F`},
		{"JavaScript", `name = "\uD83D"`, true, refusedConstruct, `\uD83D`},
		{"JavaScript", `name = "\uDE00"`, true, refusedConstruct, `\uDE00`},
		{"JavaScript", `name = "\uD83D\u0041"`, true, refusedConstruct, `\uD83D`},
		{"JavaScript", `name = "\uD83D\uD83D"`, true, refusedConstruct, `\uD83D`},
		{"JavaScript", `name = "\uDE00\uD83D"`, true, refusedConstruct, `\uDE00`},
		{"JavaScript", `name = "\uD83Dx"`, true, refusedConstruct, `\uD83D`},
		{"JavaScript", `name = "\u{D83D}"`, true, refusedConstruct, `\u{D83D}`},
		{"JavaScript", "i = 9007199254740993", true, refusedConstruct, "9007199254740993"},
		{"JavaScript", "i = 9223372036854775808", true, refusedConstruct, "9223372036854775808"},
		{"Java", "i = 9223372036854775808", true, refusedConstruct, "9223372036854775808"},
		{"Java", "i = -9223372036854775808", true, refusedConstruct, "9223372036854775808"},
		{"JavaScript", "i = 010", true, refusedConstruct, "010"},
		{"JavaScript", "state + 1", false, refusedType, "+"},
		{"JavaScript", "1 - state", false, refusedType, "-"},
		{"JavaScript", "-state", false, refusedType, "-"},
		{"JavaScript", "!state", false, refusedType, "!"},
		{"JavaScript", "state && GS_Found", false, refusedType, "&&"},
		{"JavaScript", "state", false, refusedType, "state"},
		{"JavaScript", "state ? 1 : 2", false, refusedType, "?"},
		{"JavaScript", "GS_Found ? state : 1", false, refusedType, "?"},
		{"JavaScript", "i < state", false, refusedType, "<"},
		{"JavaScript", "state == 1", false, refusedType, "=="},
		{"JavaScript", "state == tank", false, refusedType, "=="},
		{"JavaScript", "drum == state", false, refusedType, "=="},
		{"JavaScript", "GS_Found ? drum : state", false, refusedType, "?"},
		{"JavaScript", "drum = GS_Found ? drum : vat", true, refusedType, "drum ="},
		{"JavaScript", "drum = GS_Found ? drum : tank", true, refusedType, "drum ="},
		{"JavaScript", "Math.abs(state)", false, refusedType, "Math.abs"},
		{"JavaScript", "Math.max(state, i)", false, refusedType, "Math.max"},
		{"JavaScript", "java.util.Collections.max(tanks)", false, refusedType, "java.util.Collections.max"},
		{"JavaScript", "i = state", true, refusedType, "i ="},
		{"JavaScript", "state = 1", true, refusedType, "state ="},
		{"JavaScript", "state = tank", true, refusedType, "state ="},
		{"JavaScript", "drum = tank", true, refusedType, "drum ="},
		{"JavaScript", "drum = vat", true, refusedType, "drum ="},
		{"JavaScript", "state += 1", true, refusedType, "state +="},
		{"JavaScript", "i += state", true, refusedType, "i +="},
		{"JavaScript", "state++", true, refusedType, "state++"},
		{"JavaScript", "var s = state", true, refusedType, "var s"},
		{"JavaScript", `name = "\xZZ"`, true, refusedSyntax, `\x`},
		{"JavaScript", `name = "\u12"`, true, refusedSyntax, `\u12`},
		{"JavaScript", `name = "\u{}"`, true, refusedSyntax, `\u{}`},
		{"JavaScript", `name = "\u{110000}"`, true, refusedSyntax, `\u{110000}`},
		{"JavaScript", `name = "\u{12`, true, refusedSyntax, `\u{12`},
		{"JavaScript", `name = "abc`, true, refusedSyntax, `"`},
		{"JavaScript", "name.length", false, refusedName, "name.length"},
		{"JavaScript", "name.trim()", false, refusedCall, "name.trim"},
		{"JavaScript", "name + \"!\"", false, refusedConstruct, "+"},
		{"JavaScript", "/ab+/.test(name)", false, refusedConstruct, "/"},
		{"JavaScript", "xs[0]", false, refusedConstruct, "["},
		{"JavaScript", "{a: 1}", false, refusedConstruct, "{"},
		{"JavaScript", "this.nothing = 1", true, refusedName, "nothing"},
		{"JavaScript", "unknown", false, refusedName, "unknown"},
		{"JavaScript", "i = true", true, refusedType, "i ="},
		{"JavaScript", "GS_Found = 1", true, refusedType, "GS_Found ="},
		{"JavaScript", "i = 1.5", true, refusedType, "i ="},
		{"JavaScript", "GS_Found + 1", false, refusedType, "+"},
		{"JavaScript", "i < GS_Found", false, refusedType, "<"},
		{"JavaScript", "GS_Found && t", false, refusedType, "&&"},
		{"JavaScript", "!t", false, refusedType, "!"},
		{"JavaScript", "GS_Found == 1", false, refusedType, "=="},
		{"JavaScript", "clock = 1", true, refusedConstruct, "clock"},
		{"JavaScript", "java.util.Collections.max(t)", false, refusedType, "java.util.Collections.max"},
		{"JavaScript", "GS_Found ? 1 : 'a'", false, refusedType, "?"},
		{"JavaScript", "i +", false, refusedSyntax, ""},
		{"JavaScript", "(i", false, refusedSyntax, "("},
		{"JavaScript", "i > 1 j", false, refusedSyntax, "j"},
		{"JavaScript", "GS_Found; i > 1", false, refusedSyntax, "i"},
		{"JavaScript", "GS_Found;;", false, refusedSyntax, ";"},
		{"JavaScript", "GS_Found; false", false, refusedSyntax, "false"},
		{"JavaScript", "const n = 1; n = 2", true, refusedConstruct, "n"},
		{"JavaScript", "const n = 1; n += 2", true, refusedConstruct, "n"},
		{"JavaScript", "const n = 1; n++", true, refusedConstruct, "n"},
		{"JavaScript", "const n = 1; --n", true, refusedConstruct, "n"},
		{"JavaScript", "i = 1 j = 2", true, refusedSyntax, "j"},
		{"JavaScript", "i++ + 1", false, refusedConstruct, "++"},
		{"JavaScript", "i = 0x1F", true, refusedConstruct, "0x1F"},
		{"JavaScript", "Retries", false, refusedType, "Retries"},
		{"JavaScript", "GS_Found = i", true, refusedType, "GS_Found ="},
		{"JavaScript", "", true, refusedSyntax, ""},
		{"JavaScript", "TRUE", false, refusedName, "TRUE"},
		{"English", "Guide Star Lost", false, refusedName, "Guide Star Lost"},
		{"English", "not Ready To Go", false, refusedName, "Ready To Go"},
		{"English", "GS_Found and Guide Star Lost", false, refusedName, "Guide Star Lost"},
		{"English", "Guide Star and GS_Found", false, refusedType, "and"},
		{"JavaScript", "Guide Star Found", false, refusedSyntax, "Star"},
		{"JavaScript", "-t ** 2", false, refusedConstruct, "-t **"},
		{"JavaScript", "-2 ** 2", false, refusedConstruct, "-2 **"},
		{"JavaScript", "+t ** 2", false, refusedConstruct, "+t **"},
		{"JavaScript", "!GS_Found ** 2", false, refusedConstruct, "not GS_Found **"},
		{"JavaScript", "2 * -t ** 2", false, refusedConstruct, "-t **"},
		{"Java", "t ** 2", false, refusedConstruct, "**"},
		{"JavaScript", "Math.sqrt(t)", false, refusedType, "Math.sqrt(t)"},
		{"JavaScript", "-1", false, refusedType, "-1"},
		{"JavaScript", "GS_Found ? drum : vat", false, refusedType, "GS_Found ? drum : vat"},
		{"JavaScript", "i", false, refusedType, "i"},
		{"JavaScript", "tank", false, refusedType, "tank"},
		{"JavaScript", "xs", false, refusedType, "xs"},
		{"JavaScript", "tanks", false, refusedType, "tanks"},
	}
	for _, c := range cases {
		var err *refusal
		want := wanted{}
		switch c.body {
		case "Retries", "state", "Math.sqrt(t)":
			want = oneOf("Boolean")
		case "-1":
			want = oneOf("Natural")
		case "GS_Found ? drum : vat", "i", "tank":
			want = wanted{object: []string{"Drum", "Tank"}, single: true}
		case "xs", "tanks":
			want = oneOf("")
		}
		if c.statements {
			_, err = translateStatements(c.body, c.lang, testScope)
		} else {
			_, err = translateExpr(c.body, c.lang, testScope, want)
		}
		if err == nil {
			t.Errorf("%s %q: translated, want a refusal at %q", c.lang, c.body, c.token)
			continue
		}
		if err.kind != c.kind || err.token != c.token {
			t.Errorf("%s %q: refused with kind %d at %q (%s), want kind %d at %q", c.lang, c.body, err.kind, err.token, err.note(), c.kind, c.token)
		}
		if c.token != "" && !strings.Contains(err.note(), strconv.Quote(c.token)) {
			t.Errorf("%s %q: note %q does not carry the token %q", c.lang, c.body, err.note(), c.token)
		}
	}
}
