package queryexec

import (
	"fmt"
	"strings"
	"testing"
)

// matchesFixture declares parts named sensor0..sensor<n-1> and a query keeping
// those whose name ends in 7.
func matchesFixture(t testing.TB, parts int) executionFixture {
	t.Helper()
	var body strings.Builder
	body.WriteString("part root {\n")
	for i := range parts {
		fmt.Fprintf(&body, "\tpart sensor%d;\n", i)
	}
	body.WriteString(`}
calc def Sevens :> Query {
	in source : Element;
	WhereName(source = OwnedElements(source = source), operator = "matches", value = "^sensor[0-9]*7$")
}
`)
	return loadExecutionFixture(t, body.String())
}

// TestWhereNameMatchesEveryRow checks one pattern is applied to each row's name.
func TestWhereNameMatchesEveryRow(t *testing.T) {
	fixture := matchesFixture(t, 30)
	result, err := fixture.execute(t, "Sevens", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var names []string
	for _, row := range result.Rows() {
		sym, _ := row.Element().Element()
		names = append(names, sym.Name)
	}
	if got, want := strings.Join(names, " "), "sensor7 sensor17 sensor27"; got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

// TestComparisonOperators checks which operators text and feature comparisons take
// and that a pattern that does not compile is told apart from an unknown operator.
func TestComparisonOperators(t *testing.T) {
	if _, err := newTextComparison("matches", "["); err == nil || err == errComparison {
		t.Errorf("text matches \"[\": err = %v, want a pattern error", err)
	}
	if _, err := newTextComparison("<", "1"); err != errComparison {
		t.Errorf("text <: err = %v, want errComparison", err)
	}
	if _, err := newFeatureComparison("<", "x"); err == nil || err == errComparison {
		t.Errorf("feature < \"x\": err = %v, want a number error", err)
	}
	ordering, err := newFeatureComparison("<", "1")
	if err != nil {
		t.Fatalf("feature < 1: %v", err)
	}
	if _, err := ordering.text("0"); err != errComparison {
		t.Errorf("ordering of text: err = %v, want errComparison", err)
	}
	pattern, err := newFeatureComparison("matches", "^a+$")
	if err != nil {
		t.Fatalf("feature matches: %v", err)
	}
	for text, want := range map[string]bool{"aaa": true, "ab": false} {
		if got, err := pattern.text(text); err != nil || got != want {
			t.Errorf("%q matches ^a+$ = %v, %v; want %v", text, got, err, want)
		}
	}
}

// BenchmarkWhereNameMatches filters 2,000 rows by a pattern.
func BenchmarkWhereNameMatches(b *testing.B) {
	fixture := matchesFixture(b, 2000)
	program := fixture.program(b, "Sevens")
	context := Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}
	bindings := Bindings{"source": {ElementValue(fixture.symbol(b, "root"))}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Execute(program, context, bindings, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}
