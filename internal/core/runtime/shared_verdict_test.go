package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

// A condition deciding on the identity of its bound subject or actor compares it
// with an object it reads outside the occurrence, which makes the check the
// occurrence's own: nothing is shared, and each occurrence gets its own verdict.
func TestSubjectIdentityIsNotShared(t *testing.T) {
	const src = `package test {
	requirement def IsLead {
		subject s : Sat;
		require constraint { s == fleet.lead }
	}
	requirement def IsLeadActor {
		subject s : Sat;
		actor chief : Sat = fleet.lead;
		require constraint { s == chief }
	}
	requirement def IsOwnTwin {
		subject s : Sat;
		require constraint { s == s.twin }
	}
	part def Sat {
		attribute a : ScalarValues::Integer = 2;
		ref part twin : Sat = fleet.lead;
	}
	part def Fleet {
		part sats : Sat[3];
		ref part lead : Sat = sats#(2);
	}
	part fleet : Fleet {
		satisfy IsLead by sats;
		satisfy IsLeadActor by sats;
		satisfy IsOwnTwin by sats;
	}
}`
	reading, _, shared := sparseSides(t, src, "test::fleet")
	if shared != 0 {
		t.Errorf("shared %d values or verdicts deciding on an object's identity", shared)
	}
	lines := verdictLines(reading)
	want := []string{
		`satisfaction "satisfy IsLead by sats" on "sats[1]": violated (satisfaction satisfy IsLead by sats: require condition evaluated to false: s == fleet.lead)`,
		`satisfaction "satisfy IsLeadActor by sats" on "sats[1]": violated (satisfaction satisfy IsLeadActor by sats: require condition evaluated to false: s == chief)`,
		`satisfaction "satisfy IsOwnTwin by sats" on "sats[1]": violated (satisfaction satisfy IsOwnTwin by sats: require condition evaluated to false: s == s.twin)`,
		`satisfaction "satisfy IsLead by sats" on "sats[1].twin": holds`,
		`satisfaction "satisfy IsLeadActor by sats" on "sats[1].twin": holds`,
		`satisfaction "satisfy IsOwnTwin by sats" on "sats[1].twin": holds`,
		`satisfaction "satisfy IsLead by sats" on "sats[3]": violated (satisfaction satisfy IsLead by sats: require condition evaluated to false: s == fleet.lead)`,
		`satisfaction "satisfy IsLeadActor by sats" on "sats[3]": violated (satisfaction satisfy IsLeadActor by sats: require condition evaluated to false: s == chief)`,
		`satisfaction "satisfy IsOwnTwin by sats" on "sats[3]": violated (satisfaction satisfy IsOwnTwin by sats: require condition evaluated to false: s == s.twin)`,
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

// A traced context shares nothing: the trace records every check and derivation as
// the materializing path makes them, and sharing resumes once the trace is detached.
func TestTracedContextSharesNothing(t *testing.T) {
	traced := func(share bool) (*TraceRecorder, string, int) {
		ctx, idx := contextForSource(t, mixedVerdictSrc)
		ctx.SetSharedDefaults(share)
		tr := NewTraceRecorder()
		ctx.SetTrace(tr)
		reading, shared := sparseReading(ctx, lookupOne(t, idx, "test::fleet"), idx.DocumentRoot("<test>"))
		return tr, reading, shared
	}
	sharingTrace, sharingReading, shared := traced(true)
	materializingTrace, materializingReading, _ := traced(false)
	if shared != 0 {
		t.Errorf("a traced context shared %d evaluations", shared)
	}
	if len(sharingTrace.Entries()) == 0 {
		t.Fatal("validating recorded no trace")
	}
	if sharingTrace.String() != materializingTrace.String() || sharingReading != materializingReading {
		t.Errorf("traced sharing context differs from the materializing one\n--- sharing\n%s%s\n--- materializing\n%s%s",
			sharingTrace, sharingReading, materializingTrace, materializingReading)
	}

	ctx, idx := contextForSource(t, mixedVerdictSrc)
	ctx.SetSharedDefaults(true)
	ctx.SetTrace(NewTraceRecorder())
	ctx.SetTrace(nil)
	if _, shared := sparseReading(ctx, lookupOne(t, idx, "test::fleet"), idx.DocumentRoot("<test>")); shared == 0 {
		t.Error("nothing shared once the trace was detached")
	}
}

// A condition deciding on a lifetime decides on the run's state of one occurrence:
// nothing is shared, and an occurrence whose part has ended gets its own verdict.
func TestLifetimeVerdictsAreNotShared(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, lifetimeFleetSrc))
	ctx.SetSharedDefaults(true)
	fleet, err := ctx.Instantiate(lookupOne(t, idx, "test::fleet"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if err := ctx.destroy(at(t, ctx, fleet, "sats[2].c1")); err != nil {
		t.Fatalf("destroy sats[2].c1: %v", err)
	}
	done := ctx.ShareVerdicts()
	defer done()
	report, err := ctx.ValidateObject(fleet, []*symbols.Scope{idx.DocumentRoot("<test>")})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	var lines []string
	for _, v := range report.Verdicts {
		if v.Kind == "satisfaction" {
			lines = append(lines, fmt.Sprintf("%s on %q: %s", v.Kind, strings.Join(v.Path, "."), v.Status))
		}
	}
	want := []string{
		`satisfaction on "sats[1]": holds`,
		`satisfaction on "sats[2]": violated`,
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("verdicts:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	if shared := int(ctx.SharedDefaultsTaken()) + ctx.SharedVerdictsTaken(); shared != 0 {
		t.Errorf("shared %d values or verdicts deciding on a lifetime", shared)
	}
}

// libraryVerdicts validates the fleet of a library-backed model, sharing or not,
// and answers its verdict lines with what the sharing saved.
func libraryVerdicts(t *testing.T, src string, on bool) ([]string, int) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.SetSharedDefaults(on)
	fleet, err := ctx.Instantiate(lookupOne(t, idx, "test::fleet"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	done := ctx.ShareVerdicts()
	defer done()
	report, err := ctx.ValidateObject(fleet, []*symbols.Scope{idx.DocumentRoot("<test>")})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	var lines []string
	for _, v := range report.Verdicts {
		lines = append(lines, fmt.Sprintf("%s on %q: %s", v.Kind, strings.Join(v.Path, "."), v.Status))
	}
	return lines, int(ctx.SharedDefaultsTaken()) + ctx.SharedVerdictsTaken()
}

// An extent reads through every object it may reach, so one enumerated after a
// verdict was taken counts the parts the taken check never materialized.
func TestExtentAfterSharedVerdictsCountsEveryOccurrence(t *testing.T) {
	const src = `package test {
		private import SequenceFunctions::size;
		part def Comp { attribute mass : ScalarValues::Integer = 3; }
		part def Sat {
			part comp : Comp;
			requirement light { require constraint { comp.mass < 10 } }
		}
		part def Tail {
			requirement complete { require constraint { size(all Comp) == 3 } }
		}
		part def Fleet {
			part sats : Sat[3];
			part tail : Tail;
		}
		part fleet : Fleet;
	}`
	sharing, shared := libraryVerdicts(t, src, true)
	materializing, _ := libraryVerdicts(t, src, false)
	want := []string{
		`requirement on "sats[1]": holds`,
		`requirement on "sats[2]": holds`,
		`requirement on "sats[3]": holds`,
		`requirement on "tail": holds`,
	}
	if strings.Join(sharing, "\n") != strings.Join(want, "\n") {
		t.Errorf("verdicts:\n%s\nwant:\n%s", strings.Join(sharing, "\n"), strings.Join(want, "\n"))
	}
	if strings.Join(sharing, "\n") != strings.Join(materializing, "\n") {
		t.Errorf("sharing decides differently from materializing:\n%s\nvs\n%s", strings.Join(sharing, "\n"), strings.Join(materializing, "\n"))
	}
	if shared != 2 {
		t.Errorf("shared %d verdicts over three occurrences, want 2", shared)
	}
}
