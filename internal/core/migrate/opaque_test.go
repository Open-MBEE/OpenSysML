package migrate

import (
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
	"tcs.i":            {expr: "this.tcs.i", scalar: "Integer"},
	"Guide Star Found": {expr: "this.'Guide Star Found'", scalar: "Boolean"},
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
		{"JavaScript", "Math.round(t - t0)", "", "RealFunctions::floor(this.t - this.t0 + 0.5)", "Integer"},
		{"JavaScript", "i > 0 ? t : 0.0", "", "if (this.i > 0) ? this.t else 0.0", "Real"},
		{"JavaScript", "Math.max(t, t0, 1.0)", "", "RealFunctions::max(RealFunctions::max(this.t, this.t0), 1.0)", "Real"},
		{"JavaScript", "Math.min(i, 3)", "", "IntegerFunctions::min(this.i, 3)", "Integer"},
		{"JavaScript", "Math.abs(t - t0)", "", "RealFunctions::abs(this.t - this.t0)", "Real"},
		{"JavaScript", "Math.abs(i)", "", "IntegerFunctions::abs(this.i)", "Integer"},
		{"JavaScript", "Math.floor(t)", "", "RealFunctions::floor(this.t)", "Integer"},
		{"JavaScript", "Math.ceil(t)", "", "-RealFunctions::floor(-this.t)", "Integer"},
		{"JavaScript", "Math.sqrt(t)", "", "RealFunctions::sqrt(this.t)", "Real"},
		{"JavaScript", "Math.pow(t, 2)", "", "this.t ** 2", "Real"},
		{"Java", "java.util.Collections.max(xs)", "", "this.xs->ControlFunctions::reduce { in x; in y; RealFunctions::max(x, y) }", "Real"},
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
		{"English", "Mean = 0.0", "", "this.t == 0.0", "Boolean"},
	}
	for _, c := range cases {
		body := c.body
		if strings.HasPrefix(body, "Mean") {
			body = strings.Replace(body, "Mean", "t", 1)
		}
		got, err := translateExpr(body, c.lang, testScope, c.want)
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
		{"JavaScript", "t = 2", []string{"assign this.t := 2;"}},
		{"JavaScript", "t = clock\nt0 = clock - t;", []string{
			"assign this.t := " + clockRead + ";", "assign this.t0 := " + clockRead + " - this.t;"}},
		{"JavaScript", "// start\nt = 0.0; /* reset */ i = 0", []string{"assign this.t := 0.0;", "assign this.i := 0;"}},
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
		{"JavaScript", "for (i = 0; i < 3; i++) t = 1", true, refusedConstruct, "for"},
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
		{"JavaScript", "i = new Date()", true, refusedConstruct, "new"},
		{"JavaScript", `name = "\v"`, true, refusedConstruct, `\v`},
		{"JavaScript", `name = "\0"`, true, refusedConstruct, `\0`},
		{"JavaScript", `name = "\101"`, true, refusedConstruct, `\1`},
		{"JavaScript", `name = "\01"`, true, refusedConstruct, `\01`},
		{"JavaScript", `name = "\x1b[0m"`, true, refusedConstruct, `\x1b`},
		{"JavaScript", `name = "\u001F"`, true, refusedConstruct, `\u001F`},
		{"JavaScript", `name = "\uD83D"`, true, refusedConstruct, `\uD83D`},
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
	}
	for _, c := range cases {
		var err *refusal
		want := ""
		if c.body == "Retries" {
			want = "Boolean"
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
		if c.token != "" && !strings.Contains(err.note(), c.token) {
			t.Errorf("%s %q: note %q does not carry the token %q", c.lang, c.body, err.note(), c.token)
		}
	}
}
