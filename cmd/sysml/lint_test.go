package main

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
)

const lintTypo = `package P {
	attribute def Ping;
	state def M { entry; then a; state a; state b; transition first a when Pnig then b; }
	action s { send new Ping() to self; }
}`

// The lints warn, even strictly, and -disable-lint silences one by its code.
func TestDisableLintSilencesALint(t *testing.T) {
	binary := buildCLI(t)
	wantReport(t, check(t, binary, lintTypo, "-validate"), 0, "`when Pnig` names no declaration", "did you mean Ping?")
	wantReport(t, check(t, binary, lintTypo, "-strict", "-validate"), 0, "warning: `when Pnig`")
	got := check(t, binary, lintTypo, "-disable-lint", passes.CodeUndeclaredSignal+","+passes.CodePortTypeMismatch, "-validate")
	wantReport(t, got, 0)
	if strings.Contains(got.output(), "Pnig") {
		t.Fatalf("a disabled lint still reported:\n%s", got.output())
	}
	wantReport(t, check(t, binary, lintTypo, "-disable-lint", "no-such-lint", "-validate"), 2, "no-such-lint")
}

func TestLintListAcceptsRepeatsAndCommas(t *testing.T) {
	var l lintList
	if err := l.Set(passes.CodeUndeclaredSignal + ", "); err != nil {
		t.Fatal(err)
	}
	if err := l.Set(passes.CodePortTypeMismatch); err != nil {
		t.Fatal(err)
	}
	if got := l.String(); got != passes.CodeUndeclaredSignal+","+passes.CodePortTypeMismatch {
		t.Fatalf("lintList = %q", got)
	}
	if err := l.Set("bogus"); err == nil {
		t.Fatal("an unknown code was accepted")
	}
}

const roundedLiteral = `package P { private import ScalarValues::*; attribute x : Real = 0.1; }`

// An opt-in lint is silent until -enable-lint names it, and -disable-lint wins.
func TestEnableLintReportsAnOptInLint(t *testing.T) {
	binary := buildCLI(t)
	quiet := check(t, binary, roundedLiteral, "-validate")
	wantReport(t, quiet, 0)
	if strings.Contains(quiet.output(), "rounded") {
		t.Fatalf("an opt-in lint reported by default:\n%s", quiet.output())
	}
	wantReport(t, check(t, binary, roundedLiteral, "-enable-lint", passes.CodeRoundedRealLiteral, "-validate"), 0,
		"warning: 0.1 is rounded to the nearest Real, 0.10000000000000001")
	both := check(t, binary, roundedLiteral, "-enable-lint", passes.CodeRoundedRealLiteral,
		"-disable-lint", passes.CodeRoundedRealLiteral, "-validate")
	wantReport(t, both, 0)
	if strings.Contains(both.output(), "rounded") {
		t.Fatalf("a disabled lint reported:\n%s", both.output())
	}
	wantReport(t, check(t, binary, roundedLiteral, "-enable-lint", "no-such-lint", "-validate"), 2, "no-such-lint")
}
