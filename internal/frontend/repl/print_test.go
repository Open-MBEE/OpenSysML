package repl

import (
	"slices"
	"strings"
	"testing"
)

// %print with no name shows the whole session, every declaration of it, through
// the notation writer — so it is indented as a save would write it.
func TestPrintWholeSession(t *testing.T) {
	s := NewSession()
	s.Submit("package First { part def A; }")
	s.Submit("package Second {\npart def B;\n}")

	got := run(t, s, "%print")
	wants(t, got, "package First {", "part def A;", "package Second {", "part def B;")
	if !strings.HasPrefix(got, "package First {") {
		t.Errorf("%%print led with something other than the model:\n%s", got)
	}
	if strings.Contains(got, "\tpart def A;") {
		t.Errorf("%%print did not indent through the writer:\n%s", got)
	}
}

// %print <name> shows one element and its body, not the rest of the buffer.
func TestPrintElement(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")

	got := run(t, s, "%print Demo::Vehicle")
	wants(t, got, "part def Vehicle {", "attribute mass = 1500.0;", "part engine : Engine;")
	if !strings.HasPrefix(got, "part def Vehicle {") {
		t.Errorf("%%print of an element led with something else:\n%s", got)
	}
	for _, unwanted := range []string{"part def Engine {", "calc add", "package Demo"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("%%print of one element also printed %q:\n%s", unwanted, got)
		}
	}
}

// A simple name is found where every other command finds one: through the whole
// scope tree, without qualification.
func TestPrintElementBySimpleName(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%print Engine"), "part def Engine {", "attribute power = 300.0;")
}

// The names %print takes are spelled as the notation spells them: a quoted
// segment, and a qualified name reaching through packages.
func TestPrintQuotedAndQualifiedNames(t *testing.T) {
	s := NewSession()
	s.Submit("package 'My Pkg' {\npart def Car {\nattribute wheels = 4;\n}\n}")
	s.Submit("package Top {\npackage 'Inner Pkg' {\npart def Bike;\n}\n}")

	for _, name := range []string{"'My Pkg'::Car", "Car"} {
		got := run(t, s, "%print "+name)
		wants(t, got, "part def Car {", "attribute wheels = 4;")
		if strings.Contains(got, "package") {
			t.Errorf("%%print %s printed the enclosing package too:\n%s", name, got)
		}
	}
	wants(t, run(t, s, "%print Top::'Inner Pkg'::Bike"), "part def Bike;")
	// A quoted package prints its own body.
	wants(t, run(t, s, "%print Top::'Inner Pkg'"), "package 'Inner Pkg' {", "part def Bike;")
}

// Notes written above a declaration belong to it, and a print keeps them: that
// is why printing goes through the source-preserving writer.
func TestPrintKeepsComments(t *testing.T) {
	s := NewSession()
	s.Submit("// the car of interest\npackage P {\n// how many wheels\npart def Car {\nattribute wheels = 4;\n}\n}")

	wants(t, run(t, s, "%print P::Car"), "// how many wheels", "part def Car {")
	wants(t, run(t, s, "%print"), "// the car of interest", "// how many wheels")
}

// A declaration's span reaches the token after it, so the note written for what
// follows must not be printed with the element before it — nor twice.
func TestPrintStopsBeforeTheNextElementsComment(t *testing.T) {
	s := NewSession()
	s.Submit("package P {\npart def Engine { attribute power = 300.0; }\n\n// how heavy it is\npart def Car {\nattribute mass = 1500.0;\n}\n}")

	engine := run(t, s, "%print P::Engine")
	if strings.Contains(engine, "how heavy it is") {
		t.Errorf("printing an element printed the next element's note:\n%s", engine)
	}
	car := run(t, s, "%print P::Car")
	if !strings.Contains(car, "// how heavy it is") {
		t.Errorf("printing an element dropped its own note:\n%s", car)
	}
}

// What a print writes is a model: submitting it into a fresh session declares
// the same elements and prints back identically.
func TestPrintRoundTripsThroughSubmit(t *testing.T) {
	s := NewSession()
	s.Submit("package Demo {\npart def Engine { attribute power = 300.0; }\npart def Vehicle {\nattribute mass = 1500.0;\npart engine : Engine;\n}\n}")
	printed := run(t, s, "%print")

	again := NewSession()
	if res := again.Submit(printed); len(res.Diagnostics) > 0 {
		t.Fatalf("printed notation does not parse back: %v", res.Diagnostics)
	}
	if reprinted := run(t, again, "%print"); reprinted != printed {
		t.Errorf("resubmitting a print changed the model:\n--- first ---\n%s\n--- again ---\n%s", printed, reprinted)
	}
	first, second := s.declaredSymbolNames(), again.declaredSymbolNames()
	slices.Sort(first)
	slices.Sort(second)
	if !slices.Equal(first, second) {
		t.Errorf("resubmitting a print declared %v, want %v", second, first)
	}
	// The model still runs the same way, so the round trip carried the bodies.
	wants(t, run(t, again, "%instantiate Demo::Vehicle"), "Demo::Vehicle")
	wants(t, run(t, again, "%features Demo::Vehicle"), "mass = 1500", "engine")
}

// An empty session says so rather than printing nothing at all.
func TestPrintEmptySession(t *testing.T) {
	s := NewSession()
	if got := run(t, s, "%print"); got != "nothing to print: the session is empty" {
		t.Errorf("%%print of an empty session said %q", got)
	}
}

// A name nothing declares is reported in one line, the way every name-taking
// command reports it.
func TestPrintUnresolvableName(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	got := run(t, s, "%print Demo::Nope")
	if len(strings.Split(got, "\n")) != 1 {
		t.Errorf("expected one line for an unresolved name, got:\n%s", got)
	}
	wants(t, got, "Demo::Nope")
	if strings.Contains(got, "part def") {
		t.Errorf("an unresolved name printed notation anyway:\n%s", got)
	}
}

// A library symbol resolves, but this session holds no source declaring it, so
// the answer explains that instead of printing an empty body.
func TestPrintSymbolWithoutNotation(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	got := run(t, s, "%print ISQ")
	if len(strings.Split(got, "\n")) != 1 {
		t.Errorf("expected one line for a symbol with no notation, got:\n%s", got)
	}
	wants(t, got, "no notation to print", "ISQ")
}

// Printing is notation only: the RDF experimental notice belongs to the .ttl
// path of %save and must not follow a print.
func TestPrintSaysNothingAboutRDF(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	for _, line := range []string{"%print", "%print Demo::Vehicle"} {
		got := run(t, s, line)
		for _, unwanted := range []string{"RDF", "rdf", "Turtle", "experimental"} {
			if strings.Contains(got, unwanted) {
				t.Errorf("%s mentioned %q:\n%s", line, unwanted, got)
			}
		}
	}
}

// A print is a read: the objects the session materialized, its buffer and its
// declarations are the same afterwards.
func TestPrintLeavesInstancesAndBufferUntouched(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Demo::Vehicle")
	before, instancesBefore, listBefore := s.text(), run(t, s, "%instances"), run(t, s, "%list")

	run(t, s, "%print")
	run(t, s, "%print Demo::Vehicle")
	run(t, s, "%print Demo::Nope")

	if s.text() != before {
		t.Errorf("%%print changed the buffer:\n%s", s.text())
	}
	if got := run(t, s, "%instances"); got != instancesBefore {
		t.Errorf("%%print changed the objects: %q, want %q", got, instancesBefore)
	}
	if got := run(t, s, "%list"); got != listBefore {
		t.Errorf("%%print changed the declarations: %q, want %q", got, listBefore)
	}
	wants(t, run(t, s, "%features Demo::Vehicle"), "ID: 1", "mass = 1500")
}

// A print does not end an action debugging session, and the session steps on
// from where it was.
func TestPrintLeavesActionDebuggerRunning(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	wants(t, run(t, s, "%action tally"), "Started action executor")
	tokens := run(t, s, "%tokens")

	run(t, s, "%print")
	run(t, s, "%print Debug::tally")

	if got := run(t, s, "%tokens"); got != tokens {
		t.Errorf("%%print moved the debugging session: %q, want %q", got, tokens)
	}
	stepped := run(t, s, "%step")
	if strings.Contains(stepped, "no active") || strings.Contains(stepped, "ended") {
		t.Errorf("%%print ended the action debugging session:\n%s", stepped)
	}
}

// The same for a state machine session.
func TestPrintLeavesStateDebuggerRunning(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")
	wants(t, run(t, s, "%state Cycle"), "Started state machine executor")
	current := run(t, s, "%current")

	run(t, s, "%print")
	run(t, s, "%print Debug::Cycle")

	if got := run(t, s, "%current"); got != current {
		t.Errorf("%%print moved the state debugging session: %q, want %q", got, current)
	}
}

// Tab completion offers the command, and names after it, as it does for the
// other name-taking commands.
func TestPrintCompletion(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")

	if got := s.Complete("%pri", 4); !slices.Contains(got.Candidates, "%print") {
		t.Errorf("completing %%pri offered %v", got.Candidates)
	}
	got := s.Complete("%print Demo::Veh", len("%print Demo::Veh"))
	if !slices.Contains(got.Candidates, "Demo::Vehicle") {
		t.Errorf("completing a name after %%print offered %v", got.Candidates)
	}
	if got.Prefix != "Demo::Veh" {
		t.Errorf("completion prefix after %%print is %q, want the typed name", got.Prefix)
	}
}

// A session holding text the parser could not read is still printed, as typed,
// with the syntax errors reported as warnings — the reason %save tolerates them.
func TestPrintOfUnparsableSessionWarns(t *testing.T) {
	s := NewSession()
	s.Submit("package Broken { part def Car { ??? } }")

	got := run(t, s, "%print")
	wants(t, got, "warning:", "package Broken")
}
