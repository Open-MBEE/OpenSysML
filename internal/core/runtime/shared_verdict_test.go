package runtime

import (
	"strings"
	"testing"
)

// sparseSides instantiates and validates the named part with sharing on and off,
// returning both readings and how much the sharing side shared.
func sparseSides(t *testing.T, src, part string) (sharing, materializing string, shared int) {
	t.Helper()
	ctx, idx := contextForSource(t, src)
	ctx.SetSharedDefaults(true)
	sym := lookupOne(t, idx, part)
	root := idx.DocumentRoot("<test>")
	sharing, shared = sparseReading(ctx, sym, root)
	off, _ := contextForSource(t, src)
	off.SetSharedDefaults(false)
	materializing, _ = sparseReading(off, sym, root)
	if sharing != materializing {
		t.Errorf("sharing reads differently from materializing\n--- sharing\n%s\n--- materializing\n%s", sharing, materializing)
	}
	return sharing, materializing, shared
}

// verdictLines are the verdict lines of a reading, in report order.
func verdictLines(reading string) []string {
	var out []string
	for _, line := range strings.Split(reading, "\n") {
		if strings.HasPrefix(line, "constraint ") || strings.HasPrefix(line, "requirement ") || strings.HasPrefix(line, "satisfaction ") {
			out = append(out, line)
		}
	}
	return out
}

const mixedVerdictSrc = `package test {
	part def Sat {
		attribute a : ScalarValues::Integer = 2;
		attribute b : ScalarValues::Integer = a * 3;
		assert constraint light { b <= 10 }
		requirement fits { require constraint { b < 20 } }
	}
	part def Fleet {
		part sats : Sat[5];
		part heavy :> sats {
			attribute :>> a = 5;
		}
		part huge :> sats {
			attribute :>> a = 9;
		}
	}
	part fleet : Fleet;
}`

// Verdicts over occurrences of one shape are decided once and fanned out; an
// occurrence with its own value is decided on its own, in the same report order.
func TestSharedVerdictsOverMixedShapes(t *testing.T) {
	reading, _, shared := sparseSides(t, mixedVerdictSrc, "test::fleet")
	lines := verdictLines(reading)
	want := []string{
		`constraint "assert constraint light" on "sats[1]": violated (constraint light: assertion evaluated to false: b <= 10)`,
		`requirement "requirement fits" on "sats[1]": holds`,
		`constraint "assert constraint light" on "sats[2]": violated (constraint light: assertion evaluated to false: b <= 10)`,
		`requirement "requirement fits" on "sats[2]": violated (requirement fits: require condition evaluated to false: b < 20)`,
		`constraint "assert constraint light" on "sats[3]": holds`,
		`requirement "requirement fits" on "sats[3]": holds`,
		`constraint "assert constraint light" on "sats[4]": holds`,
		`requirement "requirement fits" on "sats[4]": holds`,
		`constraint "assert constraint light" on "sats[5]": holds`,
		`requirement "requirement fits" on "sats[5]": holds`,
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("verdicts:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	if shared == 0 {
		t.Error("no default or verdict shared over five occurrences")
	}
}

// A satisfaction assertion about occurrences named by subsetting members shares
// its verdict between those reading only declared values.
func TestSharedSatisfactionVerdicts(t *testing.T) {
	const src = `package test {
	requirement def MassLimit {
		subject s : Sat;
		attribute limit : ScalarValues::Integer = 10;
		require constraint { s.b <= limit }
	}
	part def Sat {
		attribute a : ScalarValues::Integer = 2;
		attribute b : ScalarValues::Integer = a * 3;
	}
	part def Fleet {
		part sats : Sat[4];
		part unit1 :> sats;
		part unit2 :> sats;
		part unit3 :> sats {
			attribute :>> a = 5;
		}
	}
	part fleet : Fleet {
		satisfy MassLimit by unit1;
		satisfy MassLimit by unit2;
		satisfy MassLimit by unit3;
	}
}`
	reading, _, _ := sparseSides(t, src, "test::fleet")
	lines := verdictLines(reading)
	want := []string{
		`satisfaction "satisfy MassLimit by unit1" on "sats[1]": holds`,
		`satisfaction "satisfy MassLimit by unit2" on "sats[2]": holds`,
		`satisfaction "satisfy MassLimit by unit3" on "sats[3]": violated (satisfaction satisfy MassLimit by unit3: require condition evaluated to false: s.b <= limit)`,
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("verdicts:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

// Within a span a check is decided once per distinct input: occurrences as declared
// take one verdict, occurrences written the same value another; none outside the span.
func TestSharedVerdictsOncePerDistinctInput(t *testing.T) {
	ctx, fleet, idx := sharedFixture(t, mixedVerdictSrc, "test::fleet")
	light := lookupOne(t, idx, "test::Sat::light")
	scope := lookupOne(t, idx, "test::Sat").Scope
	done := ctx.ShareVerdicts()
	defer done()
	check := func(occurrence string, holds bool) {
		t.Helper()
		result, err := ctx.CheckConstraintOn(light, scope, at(t, ctx, fleet, occurrence))
		if result.Holds != holds || (err == nil) != holds {
			t.Fatalf("%s light = %v, %v; want holds %v", occurrence, result.Holds, err, holds)
		}
	}
	expectTaken := func(want int) {
		t.Helper()
		if got := ctx.SharedVerdictsTaken(); got != want {
			t.Errorf("verdicts taken = %d, want %d", got, want)
		}
	}
	// Declared occurrences: decided once, taken by the second.
	check("sats[3]", true)
	check("sats[4]", true)
	expectTaken(1)
	// An occurrence with its own value is decided on its own inputs …
	write(t, ctx, fleet, "sats[5]", "a", 4)
	check("sats[5]", false)
	expectTaken(1)
	// … and another reading the same value takes that verdict; a third input is decided again.
	write(t, ctx, fleet, "sats[4]", "a", 4)
	check("sats[4]", false)
	expectTaken(2)
	write(t, ctx, fleet, "sats[3]", "a", 3)
	check("sats[3]", true)
	expectTaken(2)
	write(t, ctx, fleet, "sats[5]", "a", 3)
	check("sats[5]", true)
	expectTaken(3)
	done()
	expectTaken(0)
}
