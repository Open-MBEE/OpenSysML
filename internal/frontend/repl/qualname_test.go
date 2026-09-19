package repl

import (
	"strings"
	"testing"
)

// The notation quotes a name that is not an identifier, so the prompt has to read
// the whole quoted form as one name — including one holding a space.
func TestPlainNameReadsQuotedNames(t *testing.T) {
	for _, tc := range []struct {
		text string
		want string
	}{
		{"Demo", "Demo"},
		{"Demo::Vehicle::mass", "Demo::Vehicle::mass"},
		{"'My Pkg'", "My Pkg"},
		{"'My Pkg'::Car", "My Pkg::Car"},
		{"'My Pkg'::Car::m", "My Pkg::Car::m"},
		{"Top::'My Pkg'::Car::m", "Top::My Pkg::Car::m"},
		{"'part'::Widget", "part::Widget"},
		{"'a-b.c'::Gadget", "a-b.c::Gadget"},
	} {
		got, ok := plainName(tc.text)
		if !ok || got != tc.want {
			t.Errorf("plainName(%q) = %q, %v; want %q, true", tc.text, got, ok, tc.want)
		}
	}
}

// A name is what the notation reads the whole of the text as: an expression, a
// partial name or an empty argument is not one.
func TestPlainNameRejectsWhatIsNotAName(t *testing.T) {
	for _, text := range []string{"", "  ", "mass * 2", "'My", "My Pkg", "Demo::", "f(1)", "1"} {
		if got, ok := plainName(text); ok {
			t.Errorf("plainName(%q) = %q, true; want false", text, got)
		}
	}
}

// A resolved name is reported back in the spelling that can be typed into the
// next command, so quoting is restored on the segments that need it.
func TestNotationNameQuotesWhatNeedsQuoting(t *testing.T) {
	for _, tc := range []struct{ fqn, want string }{
		{"", ""},
		{"Demo::Vehicle", "Demo::Vehicle"},
		{"My Pkg::Car", "'My Pkg'::Car"},
		{"Top::My Pkg::Car::m", "Top::'My Pkg'::Car::m"},
		{"part::Widget", "'part'::Widget"},
		{"a-b.c::Gadget", "'a-b.c'::Gadget"},
	} {
		if got := notationName(tc.fqn); got != tc.want {
			t.Errorf("notationName(%q) = %q, want %q", tc.fqn, got, tc.want)
		}
	}
}

// Round trip: what a command prints as a name is a name the same command reads.
func TestNotationNameRoundTrips(t *testing.T) {
	for _, fqn := range []string{"Demo::Vehicle", "My Pkg::Car::curb mass", "Top::My Pkg::Car", "part::Widget"} {
		got, ok := plainName(notationName(fqn))
		if !ok || got != fqn {
			t.Errorf("plainName(notationName(%q)) = %q, %v; want %q, true", fqn, got, ok, fqn)
		}
	}
}

// The argument parsing must keep a quoted name holding a space as one argument,
// while a "…" string keeps behaving as it did.
func TestParseArgsKeepsQuotedNamesWhole(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{"%instantiate 'My Pkg'::Car", []string{"%instantiate", "'My Pkg'::Car"}},
		{"%features Top::'My Pkg'::Car", []string{"%features", "Top::'My Pkg'::Car"}},
		{"%calc 'My Pkg'::'add up' 2 3", []string{"%calc", "'My Pkg'::'add up'", "2", "3"}},
		{"%action 'My Pkg'::'run it' 'My Pkg'::Car", []string{"%action", "'My Pkg'::'run it'", "'My Pkg'::Car"}},
		{`%load "dir with space/model.sysml"`, []string{"%load", "dir with space/model.sysml"}},
		{"%load 'dir with space/model.sysml'", []string{"%load", "'dir with space/model.sysml'"}},
		// An apostrophe in ordinary text opens nothing, so the arguments after it
		// are still separate arguments.
		{"%load ~/o'brien/a.sysml b.sysml", []string{"%load", "~/o'brien/a.sysml", "b.sysml"}},
		{"%load a.sysml ~/o'brien/b.sysml", []string{"%load", "a.sysml", "~/o'brien/b.sysml"}},
		// Nor does a quote the line never closes.
		{"%instantiate 'My Pkg::Car other", []string{"%instantiate", "'My", "Pkg::Car", "other"}},
	} {
		got := parseArgs(tc.line)
		if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
			t.Errorf("parseArgs(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

// A `-action`/`-state` value splits as `%action` does: the quoted name is one
// word, the performer the words after it.
func TestSplitBehaviorKeepsQuotedNamesWhole(t *testing.T) {
	for _, tc := range []struct {
		spec      string
		name      string
		performer []string
	}{
		{"Demo::Drive", "Demo::Drive", nil},
		{"Demo::Drive rover1", "Demo::Drive", []string{"rover1"}},
		{"'My Pkg'::'Drive Home' 'My Pkg'::rover", "'My Pkg'::'Drive Home'", []string{"'My Pkg'::rover"}},
		{"", "", nil},
	} {
		name, performer := SplitBehavior(tc.spec)
		if name != tc.name || strings.Join(performer, "\x00") != strings.Join(tc.performer, "\x00") {
			t.Errorf("SplitBehavior(%q) = %q, %q; want %q, %q", tc.spec, name, performer, tc.name, tc.performer)
		}
	}
}

// Every command that takes a name accepts the quoted spelling the model author
// writes — the defect was that the argument was split on the space inside it.
func TestQuotedNamesAcceptedByNameTakingCommands(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{"%instances", []string{"'My Pkg'::Car", "Top::'My Pkg'::Car"}},
		{"%features 'My Pkg'::Car", []string{"Instance: 'My Pkg'::Car", "m = 5"}},
		{"%eval 'My Pkg'::Car::m", []string{"= 5"}},
		{"%eval 'My Pkg'::Car::'curb mass'", []string{"= 1200"}},
		{"%eval in 'My Pkg'::Car : m * 2", []string{"= 10"}},
		{"%calc 'My Pkg'::'add up' 2 3", []string{"= 5"}},
		{"%constraint 'My Pkg'::'light enough'", []string{"✓ Constraint 'My Pkg'::'light enough' passed"}},
		{"%requirement 'My Pkg'::'Safe Mass'", []string{"✓ Requirement 'My Pkg'::'Safe Mass' satisfied"}},
		{"%action 'My Pkg'::'run it'", []string{"run it"}},
		{"%state 'My Pkg'::'spin up'", []string{"spin up"}},
		{"%instantiate Top::'My Pkg'::Car", []string{"✓ Created instance of Top::'My Pkg'::Car"}},
		{"%features Top::'My Pkg'::Car", []string{"Instance: Top::'My Pkg'::Car", "m = 7"}},
		{"%instantiate 'part'::Widget", []string{"✓ Created instance of 'part'::Widget"}},
		{"%instantiate 'a-b.c'::Gadget", []string{"✓ Created instance of 'a-b.c'::Gadget"}},
		{"%satisfy 'Sat Pkg'::'the analysis'", []string{"holds (on 'Sat Pkg'::'slow lander'"}},
		// The object a behavior is performed by is a name too.
		{"%action 'My Pkg'::'run it' 'My Pkg'::Car", []string{"run it"}},
		{"%state 'My Pkg'::'spin up' 'My Pkg'::Car", []string{"spin up"}},
		{"%search 'My Pkg'", []string{"'My Pkg'::Car"}},
	} {
		s := loadFixture(t, "testdata/quoted_names.sysml")
		// A command about an object needs one, which %instantiate is under test for too.
		run(t, s, "%instantiate 'My Pkg'::Car")
		run(t, s, "%instantiate Top::'My Pkg'::Car")
		got := run(t, s, tc.line)
		wants(t, got, tc.want...)
		rejects(t, got, "unresolved reference", "symbol not found")
	}
}

// An object materialized from a quoted name is the one every later command about
// that name is about, whichever quoted segment of the chain needed the quoting.
func TestQuotedNameNamesTheSameObjectThroughout(t *testing.T) {
	s := loadFixture(t, "testdata/quoted_names.sysml")
	wants(t, run(t, s, "%instantiate 'My Pkg'::Car"), "✓ Created instance of 'My Pkg'::Car", "ID: 1")
	wants(t, run(t, s, "%features 'My Pkg'::Car"), "ID: 1")
	wants(t, run(t, s, "%eval 'My Pkg'::Car::m"), "(on 'My Pkg'::Car ID: 1)")
	wants(t, run(t, s, "%eval in 'My Pkg'::Car : m"), "(on 'My Pkg'::Car ID: 1)")
}

// A command taking a name never panics on a quoted argument it cannot make sense
// of: an unterminated quote, a quote in the middle of a word, an empty name.
func TestQuotedNameFailuresAreReportedNotPanics(t *testing.T) {
	s := loadFixture(t, "testdata/quoted_names.sysml")
	for _, line := range []string{
		"%instantiate 'My",
		"%instantiate 'My Pkg",
		"%instantiate ''",
		"%instantiate 'My Pkg'::",
		"%features 'My Pkg'Car",
		"%calc 'My Pkg'::'add up",
		"%eval in 'My Pkg' : ",
		"%constraint '",
	} {
		out, quit, err := s.RunMeta(line)
		if err != nil || quit {
			t.Fatalf("%s: err = %v, quit = %v", line, err, quit)
		}
		if len(out) == 0 {
			t.Errorf("%s: reported nothing", line)
		}
		// The failure echoes what was typed, not the same text quoted again.
		for _, l := range out {
			if strings.Contains(l, "''") && !strings.Contains(line, "''") {
				t.Errorf("%s: reported %q, which quotes the argument twice", line, l)
			}
		}
	}
}
