package main

import (
	"strings"
	"testing"
)

// selfBindingModel binds a declaration to itself by qualified name, directly and
// through a two-element cycle spelled with dot notation.
const selfBindingModel = `package test {
    private import ScalarValues::Real;
    part self {
        attribute a : Real = test::self::a;
        attribute b : Real = test::self.c;
        attribute c : Real = test::self::b;
    }
}`

// TestEvalSelfReferentialQualifiedNameReportsCycle checks that an expression reading
// a declaration bound to its own qualified name exits 2 with the cycle named on
// stderr — named as the declaration the cycle closes on, by qualified name when
// it closes on a qualified-name read and as the chain when it closes on a
// dot-notation one — from the model and from an object, rather than crashing.
func TestEvalSelfReferentialQualifiedNameReportsCycle(t *testing.T) {
	binary := buildCLI(t)

	for expr, cycle := range map[string]string{
		"test::self::a": "cyclic feature value dependency: test::self::a",
		"test::self.a":  "cyclic feature value dependency: test::self::a",
		"test::self::b": "cyclic feature value dependency: self.c",
		"test::self.c":  "cyclic feature value dependency: self.c",
	} {
		got := check(t, binary, selfBindingModel, "-e", expr)
		wantReport(t, got, 2, "evaluation failed", cycle)
		if strings.Contains(got.output(), "stack overflow") || strings.Contains(got.output(), "goroutine") {
			t.Errorf("-e %s crashed the process:\n%s", expr, got.output())
		}
	}
	got := check(t, binary, selfBindingModel, "-instantiate", "test::self", "-e", "test::self.a")
	wantReport(t, got, 2, "Created instance of test::self", "evaluation failed", "cyclic feature value dependency: test::self::a")
}
