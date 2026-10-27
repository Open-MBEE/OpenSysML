package queryexec

import (
	"strings"
	"testing"
)

const namedBody = `
package Vehicle {
	package Config {
		part def Wheel;
		individual part def FrontLeft :> Wheel;
		individual part def Spare :> Wheel;
	}
	package 'Spare Parts' {
		part def Pad;
	}
	alias Wheels for Config;
	alias Rim for Config::Wheel;
}
package Other {
	package Config;
	alias Rim for Vehicle::Config::Wheel;
	private import Vehicle::'Spare Parts'::*;
}
package Aliases {
	alias Broken for Missing;
	alias Loop1 for Loop2;
	alias Loop2 for Loop1;
}
calc def ByName :> Query {
	in qualifiedName : String[1..*] ordered;
	Named(qualifiedName = qualifiedName)
}
calc def ConfigWheels :> Query {
	WhereFeature(
		source = WhereType(source = Descendants(source = Named(qualifiedName = "Vehicle::Config")), type = "Vehicle::Config::Wheel"),
		'feature' = "isIndividual", operator = "=", value = "true"
	)
}
`

func TestExecuteNamedResolvesQualifiedNamesInOrder(t *testing.T) {
	fixture := loadExecutionFixture(t, namedBody)
	run := func(names ...string) *RowSet {
		t.Helper()
		values := make([]Value, len(names))
		for i, name := range names {
			values[i] = StringValue(name)
		}
		result, err := fixture.execute(t, "ByName", Bindings{"qualifiedName": values}, Options{})
		if err != nil {
			t.Fatalf("ByName(%v): %v", names, err)
		}
		return result
	}
	// Argument order is row order, a quoted-in-notation name is spelled raw, a
	// full or unique partial qualified name both resolve, and a name given twice
	// yields its element once per mention.
	got := rowNames(run("Observatory::Vehicle::Spare Parts::Pad", "Vehicle::Config::Wheel", "Vehicle::Config::Wheel"))
	want := "Observatory::Vehicle::Spare Parts::Pad,Observatory::Vehicle::Config::Wheel,Observatory::Vehicle::Config::Wheel"
	if strings.Join(got, ",") != want {
		t.Fatalf("Named rows = %v, want %v", got, want)
	}
	// An alias names its target.
	if got := rowNames(run("Vehicle::Wheels")); strings.Join(got, ",") != "Observatory::Vehicle::Config" {
		t.Fatalf("Named alias = %v", got)
	}
	// A unique simple name resolves as it does for WhereType.
	if got := rowNames(run("Pad")); strings.Join(got, ",") != "Observatory::Vehicle::Spare Parts::Pad" {
		t.Fatalf("Named simple = %v", got)
	}
	for _, row := range run("Vehicle::Config").Rows() {
		if !row.Origin().Located() {
			t.Fatalf("row %v without provenance", row.Element())
		}
	}
	// The named element roots a traversal.
	wheels, err := fixture.execute(t, "ConfigWheels", nil, Options{})
	if err != nil {
		t.Fatalf("ConfigWheels: %v", err)
	}
	if got := rowNames(wheels); strings.Join(got, ",") != "Observatory::Vehicle::Config::FrontLeft,Observatory::Vehicle::Config::Spare" {
		t.Fatalf("Descendants(Named) = %v", got)
	}
}

// A name no element carries is an unknown-element error that says where the
// path stopped resolving and what is spelled nearest; an alias that denotes no
// element (dangling or cyclic) says so; never a row.
func TestExecuteNamedRejectsUnknownNames(t *testing.T) {
	fixture := loadExecutionFixture(t, namedBody)
	cases := map[string]string{
		"Vehicle::Missing": "names no element Vehicle::Missing: Observatory::Vehicle has no member Missing",
		"Vehicle::Confg":   "names no element Vehicle::Confg: Observatory::Vehicle has no member Confg — did you mean Config?",
		"Nowhere::Config":  "names no element Nowhere::Config: nothing is named Nowhere",
		// Other privately imports Pad, so Pad is not offered for a typo under Other.
		"Other::Pda":      "names no element Other::Pda: Observatory::Other has no member Pda",
		"Aliases::Broken": "names no element Aliases::Broken: the alias Aliases::Broken denotes no element",
		"Aliases::Loop1":  "names no element Aliases::Loop1: the alias Aliases::Loop1 denotes no element",
		"Aliases::Loop2":  "names no element Aliases::Loop2: the alias Aliases::Loop2 denotes no element",
		// A path through a dangling or cyclic alias stops at the alias, not at a
		// member its namespace lacks.
		"Aliases::Broken::foo": "names no element Aliases::Broken::foo: the alias Aliases::Broken denotes no element",
		"Aliases::Loop1::foo":  "names no element Aliases::Loop1::foo: the alias Aliases::Loop1 denotes no element",
	}
	for name, want := range cases {
		_, err := fixture.execute(t, "ByName", Bindings{"qualifiedName": {StringValue(name)}}, Options{})
		unknown := executionError(t, err, ErrorUnknownElement)
		if unknown.Actual != name || unknown.Operation != "named" {
			t.Fatalf("Named(%q) error = %v", name, unknown)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Named(%q) message = %v, want it to contain %q", name, err, want)
		}
		if name == "Other::Pda" && strings.Contains(err.Error(), "did you mean") {
			t.Fatalf("Named(%q) offered a privately imported name: %v", name, err)
		}
	}
}

// A qualified name reads as the notation spells it, a segment no basic name
// spells quoted; it names the element, and a frontier is told in that spelling.
func TestExecuteNamedReadsQuotedSegments(t *testing.T) {
	fixture := loadExecutionFixture(t, namedBody)
	for _, name := range []string{"Vehicle::'Spare Parts'::Pad", "Observatory::Vehicle::'Spare Parts'::Pad", "'Spare Parts'::Pad"} {
		result, err := fixture.execute(t, "ByName", Bindings{"qualifiedName": {StringValue(name)}}, Options{})
		if err != nil {
			t.Fatalf("Named(%s): %v", name, err)
		}
		if got := strings.Join(rowNames(result), ","); got != "Observatory::Vehicle::Spare Parts::Pad" {
			t.Fatalf("Named(%s) rows = %v", name, got)
		}
	}
	cases := map[string]string{
		"Vehicle::'Spare Parts'::Pda": "names no element Vehicle::'Spare Parts'::Pda: Observatory::Vehicle::'Spare Parts' has no member Pda — did you mean Pad?",
		"Vehicle::'Spare Partz'::Pad": "names no element Vehicle::'Spare Partz'::Pad: Observatory::Vehicle has no member 'Spare Partz' — did you mean 'Spare Parts'?",
	}
	for name, want := range cases {
		_, err := fixture.execute(t, "ByName", Bindings{"qualifiedName": {StringValue(name)}}, Options{})
		unknown := executionError(t, err, ErrorUnknownElement)
		if unknown.Actual != name || !strings.Contains(err.Error(), want) {
			t.Fatalf("Named(%s) = %v, want it to contain %q", name, err, want)
		}
	}
}

// Two aliases of one element denote one element, not two.
func TestExecuteNamedResolvesAliasesToTheirElement(t *testing.T) {
	fixture := loadExecutionFixture(t, namedBody)
	result, err := fixture.execute(t, "ByName", Bindings{"qualifiedName": {StringValue("Rim")}}, Options{})
	if err != nil {
		t.Fatalf("Named(Rim): %v", err)
	}
	if got := strings.Join(rowNames(result), ","); got != "Observatory::Vehicle::Config::Wheel" {
		t.Fatalf("Named(Rim) rows = %v", got)
	}
}

// A name several elements carry is an ambiguous-element error listing them: a
// finding about the model, not an empty result and not an unknown name.
func TestExecuteNamedRejectsAmbiguousNames(t *testing.T) {
	fixture := loadExecutionFixture(t, namedBody)
	_, err := fixture.execute(t, "ByName", Bindings{"qualifiedName": {StringValue("Config")}}, Options{})
	ambiguous := executionError(t, err, ErrorAmbiguousElement)
	if ambiguous.Actual != "Config" || ambiguous.Operation != "named" {
		t.Fatalf("Named(Config) error = %v", ambiguous)
	}
	if got := strings.Join(ambiguous.Candidates, ","); got != "Observatory::Other::Config,Observatory::Vehicle::Config" {
		t.Fatalf("Named(Config) candidates = %v", ambiguous.Candidates)
	}
	if want := "names 2 elements Config: Observatory::Other::Config, Observatory::Vehicle::Config"; !strings.Contains(err.Error(), want) {
		t.Fatalf("Named(Config) message = %v, want it to contain %q", err, want)
	}
	// Two members declared under one name are that name once, with its count.
	twice := loadExecutionFixture(t, `
package Twice {
	part def P;
	part a : P;
	part a : P;
}
calc def ByName :> Query {
	in qualifiedName : String[1..*] ordered;
	Named(qualifiedName = qualifiedName)
}
`)
	_, err = twice.execute(t, "ByName", Bindings{"qualifiedName": {StringValue("Twice::a")}}, Options{})
	ambiguous = executionError(t, err, ErrorAmbiguousElement)
	if len(ambiguous.Candidates) != 2 {
		t.Fatalf("Named(Twice::a) candidates = %v", ambiguous.Candidates)
	}
	if want := "names 2 elements Twice::a: Observatory::Twice::a (declared 2 times)"; !strings.Contains(err.Error(), want) {
		t.Fatalf("Named(Twice::a) message = %v, want it to contain %q", err, want)
	}
	// A path through an ambiguous prefix is reported as that ambiguity, not as a
	// member the prefix lacks.
	_, err = twice.execute(t, "ByName", Bindings{"qualifiedName": {StringValue("Twice::a::missing")}}, Options{})
	ambiguous = executionError(t, err, ErrorAmbiguousElement)
	if ambiguous.Actual != "Twice::a" {
		t.Fatalf("Named(Twice::a::missing) error = %v", ambiguous)
	}
}
