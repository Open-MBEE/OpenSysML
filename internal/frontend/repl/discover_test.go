package repl

import (
	"strings"
	"testing"
)

// TestSearchCommand covers what %search answers over the bundled library and
// the session's own declarations.
func TestSearchCommand(t *testing.T) {
	tests := []struct {
		name     string
		declare  string
		line     string
		wants    []string
		rejects  []string
		maxLines int
	}{
		{
			name:  "library declaration with its kind",
			line:  "%search Integer",
			wants: []string{"ScalarValues::Integer  datatype"},
		},
		{
			name: "re-export aliases are not listed as declarations",
			line: "%search Integer",
			rejects: []string{
				"\nSysML::Integer",
				"\nKerML::Integer",
			},
		},
		{
			name:  "case insensitive",
			line:  "%search scalarvalues",
			wants: []string{"ScalarValues"},
		},
		{
			name:    "session declarations are searched",
			declare: "part def Wheel { attribute diameter = 1.0; }",
			line:    "%search Wheel",
			wants:   []string{"Wheel  part def"},
		},
		{
			name:    "a chained binding end declares no symbol",
			declare: "package B { part R { part inner { attribute hhh; } } part ccc; binding bind R.inner.hhh = ccc; }",
			line:    "%search hhh",
			wants:   []string{"B::R::inner::hhh  attribute"},
			rejects: []string{"\nB::hhh", "unknown"},
		},
		{
			name:    "a named control node is listed",
			declare: "package B { action def F { first start; fork Jump; action S; first start then Jump; first Jump then S; } }",
			line:    "%search Jump",
			wants:   []string{"B::F::Jump  fork"},
		},
		{
			name:  "no match",
			line:  "%search zzzznotasymbol",
			wants: []string{`no symbol matches "zzzznotasymbol"`},
		},
		{
			name:     "listing is bounded",
			line:     "%search a",
			wants:    []string{"more; narrow the search"},
			maxLines: searchLimit + 1,
		},
		{
			name:  "usage without an argument",
			line:  "%search",
			wants: []string{"usage: %search <substring>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			if tt.declare != "" {
				if res := s.Submit(tt.declare); len(res.Diagnostics) > 0 {
					t.Fatalf("declaration has diagnostics: %v", res.Diagnostics)
				}
			}
			out, _, err := s.RunMeta(tt.line)
			if err != nil {
				t.Fatalf("%s: %v", tt.line, err)
			}
			got := strings.Join(out, "\n")
			wants(t, got, tt.wants...)
			rejects(t, got, tt.rejects...)
			if tt.maxLines > 0 && len(out) > tt.maxLines {
				t.Errorf("got %d lines, want at most %d", len(out), tt.maxLines)
			}
		})
	}
}

// TestBuiltinsCommand checks %builtins reports the functions the runtime
// implements, in the form each is called in, each with the import a call by its
// unqualified name needs.
func TestBuiltinsCommand(t *testing.T) {
	out := run(t, NewSession(), "%builtins")
	wants(t, out,
		"only where the model imports",
		"sqrt(x)  RealFunctions::sqrt  (import RealFunctions::*;)",
		"abs(x)  RealFunctions::abs  (import RealFunctions::*;)",
		"abs(x)  IntegerFunctions::abs  (import IntegerFunctions::*;)",
		"max(x, y)  NumericalFunctions::max  (import NumericalFunctions::*;)",
		"floor(x)  RealFunctions::floor  (import RealFunctions::*;)",
		"x->isEmpty()  SequenceFunctions::isEmpty  (import SequenceFunctions::*;)",
		"x->size()  SequenceFunctions::size  (import SequenceFunctions::*;)",
		"x->size()  CollectionFunctions::size  (import CollectionFunctions::*;)",
		"x->sum()  NumericalFunctions::sum  (import NumericalFunctions::*;)",
	)
	rejects(t, out, "callable whatever", "without an import")
}

// An extension function is implemented, so it is listed with its import like
// every other rather than left out as if unsupported.
func TestBuiltinsListsAnExtensionFunctionWithItsImport(t *testing.T) {
	out := run(t, NewSession(), "%builtins")
	wants(t, out,
		"exp(x)  OpenSysMLMathFunctions::exp  (import OpenSysMLMathFunctions::*;)",
		"atan2(y, x)  OpenSysMLMathFunctions::atan2  (import OpenSysMLMathFunctions::*;)",
	)
}

// TestQualifiedNameSuggestion covers the hint an unresolved bare library name
// gets: the qualified name the symbol index already knows it under.
func TestQualifiedNameSuggestion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wants   []string
		rejects []string
	}{
		{
			name:  "bare library name",
			input: "part def A { attribute x : Integer = 1; }",
			wants: []string{"unresolved reference: Integer — did you mean ScalarValues::Integer?"},
		},
		{
			name:    "qualified name gets no unqualified hint",
			input:   "part def B { attribute x : NoSuchLib::Integer = 1; }",
			rejects: []string{"did you mean"},
		},
		{
			name:    "unknown name gets no hint",
			input:   "part def C { attribute x : Zzzznotatype = 1; }",
			rejects: []string{"did you mean"},
		},
		{
			name:    "resolvable name has no diagnostic",
			input:   "part def D { attribute x : ScalarValues::Integer = 1; }",
			rejects: []string{"unresolved reference"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			res := s.Submit(tt.input)
			var msgs []string
			for _, d := range res.Diagnostics {
				msgs = append(msgs, d.Message)
			}
			got := strings.Join(msgs, "\n")
			wants(t, got, tt.wants...)
			rejects(t, got, tt.rejects...)
		})
	}
}

// TestUnknownCommandSuggestion covers the edit-distance hint a misspelled meta
// command gets, and the fallback when nothing is close.
func TestUnknownCommandSuggestion(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{cmd: "%evl", want: `unknown command "%evl" — did you mean %eval?`},
		{cmd: "%hepl", want: `unknown command "%hepl" — did you mean %help?`},
		{cmd: "%isntances", want: `unknown command "%isntances" — did you mean %instances?`},
		{cmd: "%zzzzqqqq", want: `unknown command "%zzzzqqqq" (try %help)`},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := unknownCommandLine(tt.cmd); got != tt.want {
				t.Errorf("unknownCommandLine(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

// TestSymbolSuggestion covers the hints a name no declaration answers to gets.
func TestSymbolSuggestion(t *testing.T) {
	tests := []struct {
		name    string
		declare string
		line    string
		wants   []string
	}{
		{
			name:    "misspelled declaration",
			declare: "part def Wheel { attribute diameter = 1.0; }",
			line:    "%instantiate Whel",
			wants:   []string{"unresolved reference: Whel", "did you mean Wheel?"},
		},
		{
			name:  "bare library name is offered qualified",
			line:  "%instantiate Integer",
			wants: []string{"unresolved reference: Integer", "did you mean ScalarValues::Integer?"},
		},
		{
			name:    "nothing close",
			declare: "part def Wheel { attribute diameter = 1.0; }",
			line:    "%instantiate Zzzzqqqqwwww",
			wants:   []string{"unresolved reference: Zzzzqqqqwwww"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			if tt.declare != "" {
				if res := s.Submit(tt.declare); len(res.Diagnostics) > 0 {
					t.Fatalf("declaration has diagnostics: %v", res.Diagnostics)
				}
			}
			wants(t, run(t, s, tt.line), tt.wants...)
		})
	}
}

// TestQualifiedMissSuggestion covers the hints a qualified name no declaration
// answers to gets: members of its own qualifier, of a kind the command can act
// on, and nothing when the qualifier itself resolves nowhere.
func TestQualifiedMissSuggestion(t *testing.T) {
	const declare = "package Demo { action def Go; action runFast; part def Wheel; } package 'My Pkg' { part def Wheel; }"
	tests := []struct {
		name    string
		line    string
		wants   []string
		rejects []string
	}{
		{
			name:  "misspelled member of a resolved qualifier",
			line:  "%instantiate Demo::Whel",
			wants: []string{"unresolved reference: Demo::Whel", "did you mean Demo::Wheel?"},
		},
		{
			name:  "an action command is offered actions",
			line:  "%action Demo::runFst",
			wants: []string{"unresolved reference: Demo::runFst", "did you mean Demo::runFast?"},
		},
		{
			name:    "an action command is not offered a part",
			line:    "%action Demo::Whel",
			wants:   []string{"unresolved reference: Demo::Whel"},
			rejects: []string{"did you mean"},
		},
		{
			name:  "a hint under a quoted qualifier is spelled typable",
			line:  "%instantiate 'My Pkg'::Whel",
			wants: []string{"did you mean 'My Pkg'::Wheel?"},
		},
		{
			name:    "an unresolved qualifier offers nothing",
			line:    "%action Nope::Rescue",
			wants:   []string{"unresolved reference: Nope::Rescue"},
			rejects: []string{"did you mean"},
		},
		{
			name:    "nothing close under the qualifier offers nothing",
			line:    "%instantiate Demo::Zzzzqqqq",
			wants:   []string{"unresolved reference: Demo::Zzzzqqqq"},
			rejects: []string{"did you mean"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			if res := s.Submit(declare); len(errorDiagnostics(res.Diagnostics)) > 0 {
				t.Fatalf("declaration has diagnostics: %v", res.Diagnostics)
			}
			got := run(t, s, tt.line)
			wants(t, got, tt.wants...)
			rejects(t, got, tt.rejects...)
		})
	}
}

// TestLibraryReachableFromEmptySession checks library symbols answer browsing,
// lookup and instantiation before the session declares anything.
func TestLibraryReachableFromEmptySession(t *testing.T) {
	tests := []struct {
		line    string
		wants   []string
		rejects []string
	}{
		{line: "%search mass", wants: []string{"ISQBase::mass"}},
		{line: "%eval ISQ::mass", rejects: []string{"no declarations loaded"}},
		{line: "%instantiate ISQ::mass", wants: []string{"✓ Created instance of ISQBase::mass"}},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := run(t, NewSession(), tt.line)
			wants(t, got, tt.wants...)
			rejects(t, got, tt.rejects...)
		})
	}
}
