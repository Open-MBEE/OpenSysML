package main

import (
	"flag"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// testFlags mirrors the shapes the real command line has: flags that take a
// value, boolean flags, and a flag whose value is optional.
func testFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("sysml", flag.ContinueOnError)
	fs.String("convert", "", "")
	fs.String("o", "", "")
	fs.Bool("trace", false, "")
	fs.Var(&optionalValue{}, "satisfy", "")
	return fs
}

// optionalValue is a flag whose value may be omitted, so the argument after it
// is a file rather than its value.
type optionalValue struct{}

func (v *optionalValue) String() string   { return "" }
func (v *optionalValue) IsBoolFlag() bool { return true }
func (v *optionalValue) Set(string) error { return nil }

func TestPermuteArgs(t *testing.T) {
	cases := map[string]struct {
		args []string
		want []string
	}{
		"already ordered":     {[]string{"-convert", "ttl", "m.sysml"}, []string{"-convert", "ttl", "--", "m.sysml"}},
		"model first":         {[]string{"m.sysml", "-convert", "ttl"}, []string{"-convert", "ttl", "--", "m.sysml"}},
		"model between flags": {[]string{"-trace", "m.sysml", "-o", "out.ttl"}, []string{"-trace", "-o", "out.ttl", "--", "m.sysml"}},
		"value looks like a file": {
			[]string{"m.sysml", "-o", "-weird.ttl"},
			[]string{"-o", "-weird.ttl", "--", "m.sysml"},
		},
		"joined value":       {[]string{"m.sysml", "-convert=ttl"}, []string{"-convert=ttl", "--", "m.sysml"}},
		"optional value":     {[]string{"-satisfy", "m.sysml"}, []string{"-satisfy", "--", "m.sysml"}},
		"double dash":        {[]string{"-trace", "--", "-m.sysml", "-o"}, []string{"-trace", "--", "-m.sysml", "-o"}},
		"unknown flag stays": {[]string{"m.sysml", "-nope", "x"}, []string{"-nope", "--", "m.sysml", "x"}},
		"dash is a file":     {[]string{"-convert", "ttl", "-"}, []string{"-convert", "ttl", "--", "-"}},
		"missing value":      {[]string{"m.sysml", "-convert"}, []string{"-convert"}},
		"no arguments":       {nil, []string{}},
		"only flags":         {[]string{"-trace"}, []string{"-trace"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := permuteArgs(testFlags(), tc.args)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("permuteArgs(%q) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

// TestPermuteArgsParses checks the reordering against the parser it feeds: the
// flags are set and the files are left as positional arguments.
func TestPermuteArgsParses(t *testing.T) {
	fs := testFlags()
	if err := fs.Parse(permuteArgs(fs, []string{"a.sysml", "-convert", "ttl", "b.sysml", "-trace"})); err != nil {
		t.Fatal(err)
	}
	if got := fs.Lookup("convert").Value.String(); got != "ttl" {
		t.Errorf("-convert = %q, want %q", got, "ttl")
	}
	if got := fs.Lookup("trace").Value.String(); got != "true" {
		t.Errorf("-trace = %q, want true", got)
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"a.sysml", "b.sysml"}) {
		t.Errorf("files = %q, want [a.sysml b.sysml]", got)
	}
}

// TestPermuteArgsReportsMissingValue checks that a value forgotten at the end of
// the command line is reported, rather than read from the rest of the arguments.
func TestPermuteArgsReportsMissingValue(t *testing.T) {
	fs := testFlags()
	fs.SetOutput(io.Discard)
	err := fs.Parse(permuteArgs(fs, []string{"m.sysml", "-convert", "ttl", "-o"}))
	if err == nil || !strings.Contains(err.Error(), "flag needs an argument: -o") {
		t.Fatalf("parse error = %v, want a missing value for -o", err)
	}
}

// TestPermuteArgsKeepsProtectedFiles checks that a file named like a flag and
// protected by an end-of-options marker survives the reordering as a file.
func TestPermuteArgsKeepsProtectedFiles(t *testing.T) {
	fs := testFlags()
	if err := fs.Parse(permuteArgs(fs, []string{"-trace", "--", "-weird.sysml", "-o"})); err != nil {
		t.Fatal(err)
	}
	if got := fs.Lookup("o").Value.String(); got != "" {
		t.Errorf("-o = %q, want unset", got)
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"-weird.sysml", "-o"}) {
		t.Errorf("files = %q, want [-weird.sysml -o]", got)
	}
}

// TestPermuteArgsJoinsAnOptionalValue checks that the word after a bare
// -explain becomes its value by position, flags and files staying apart.
func TestPermuteArgsJoinsAnOptionalValue(t *testing.T) {
	fs := flag.NewFlagSet("sysml", flag.ContinueOnError)
	var explain codeNames
	fs.Var(&explain, "explain", "")
	fs.Bool("validate", false, "")
	cases := []struct{ in, want []string }{
		{[]string{"-explain", "unresolved"}, []string{"-explain=unresolved"}},
		{[]string{"-explain", "syntax", "model.sysml"}, []string{"-explain=syntax", "--", "model.sysml"}},
		{[]string{"-explain", "-validate"}, []string{"-explain=", "-validate"}},
		{[]string{"-explain"}, []string{"-explain="}},
		{[]string{"-explain", "--", "syntax"}, []string{"-explain=", "--", "syntax"}},
		{[]string{"-explain", "true"}, []string{"-explain=true"}},
		{[]string{"-explain=syntax", "unresolved"}, []string{"-explain=syntax", "--", "unresolved"}},
	}
	for _, c := range cases {
		if got := permuteArgs(fs, c.in); !slices.Equal(got, c.want) {
			t.Errorf("permuteArgs(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
