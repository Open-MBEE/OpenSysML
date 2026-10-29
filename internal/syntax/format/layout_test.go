package format

import (
	"strings"
	"testing"
)

func layout(t *testing.T, src string) string {
	t.Helper()
	out, err := Source("t.sysml", []byte(src), ConvertOptions)
	if err != nil {
		t.Fatalf("Source() error = %v", err)
	}
	twice, err := Source("t.sysml", out, ConvertOptions)
	if err != nil {
		t.Fatalf("Source() second pass error = %v", err)
	}
	if string(twice) != string(out) {
		t.Fatalf("not idempotent:\n%s\n--- second pass ---\n%s", out, twice)
	}
	return string(out)
}

// A body written on one line comes out one statement per line, nested bodies
// included, and the closing braces each take a line of their own.
func TestBreakStatementsLaysOutOneLiners(t *testing.T) {
	got := layout(t, "package P { part def Wheel { attribute diameter : Real = 16.0; attribute width : Real; } part car : Car { part redefines wheels[4] { attribute redefines diameter = 17.0; } } }\n")
	want := strings.Join([]string{
		"package P {",
		"    part def Wheel {",
		"        attribute diameter : Real = 16.0;",
		"        attribute width : Real;",
		"    }",
		"    part car : Car {",
		"        part redefines wheels[4] {",
		"            attribute redefines diameter = 17.0;",
		"        }",
		"    }",
		"}",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Top-level statements break too, and a line the author already broke is
// never joined, blank lines included.
func TestBreakStatementsKeepsAuthorBreaks(t *testing.T) {
	got := layout(t, "package A; package B;\npackage C {\n\n    part def D;\n}\n")
	want := "package A;\npackage B;\npackage C {\n\n    part def D;\n}\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// The body of an expression is laid out as written: a `->forAll { in x; ... }`,
// the `{ in x; ... }` an operator takes, and anything inside brackets.
func TestBreakStatementsLeavesExpressionBodies(t *testing.T) {
	cases := []struct{ src, want string }{
		{"package Q { attribute p = (1..size(seq))->selectOne{in i; seq#(i) == value}; }\n",
			"package Q {\n    attribute p = (1..size(seq))->selectOne{in i; seq#(i) == value};\n}\n"},
		{"package Q { constraint c { element.feature->forAll { in f : KerML::Feature; not f.isComposite } } }\n",
			"package Q {\n    constraint c { element.feature->forAll { in f : KerML::Feature; not f.isComposite } }\n}\n"},
		{"package Q { attribute v = {in x; x + 1}; }\n",
			"package Q {\n    attribute v = {in x; x + 1};\n}\n"},
		{"package Q { attribute w = f(x->select{in y; y > 0}); }\n",
			"package Q {\n    attribute w = f(x->select{in y; y > 0});\n}\n"},
		{"package Q { return r = xs->ControlFunctions::collect {in x; x + 1}; }\n",
			"package Q {\n    return r = xs->ControlFunctions::collect {in x; x + 1};\n}\n"},
		{"package Q { attribute total = {in x; x} + 1; attribute s = xs->reduce {in a; in b; a + b} + 1; }\n",
			"package Q {\n    attribute total = {in x; x} + 1;\n    attribute s = xs->reduce {in a; in b; a + b} + 1;\n}\n"},
		{"calc def C { in x; {in y; y} + x }\n",
			"calc def C {\n    in x;\n    {in y; y} + x\n}\n"},
		{"calc def N { in x; -x }\n", "calc def N {\n    in x;\n    -x\n}\n"},
		{"calc def P { in x; +x }\n", "calc def P {\n    in x;\n    +x\n}\n"},
	}
	for _, tc := range cases {
		if got := layout(t, tc.src); got != tc.want {
			t.Errorf("layout(%q):\ngot:  %q\nwant: %q", tc.src, got, tc.want)
		}
	}
}

// A statement that goes on after its body keeps the continuation on the `}`
// line: `} then B;`, `} else {`, `} until done;`.
func TestBreakStatementsJoinsContinuations(t *testing.T) {
	got := layout(t, "action def A { if x { action a1; } else { action a2; } loop { action l; } until done; transition first A accept s do { action d; } then B; }\n")
	want := strings.Join([]string{
		"action def A {",
		"    if x {",
		"        action a1;",
		"    } else {",
		"        action a2;",
		"    }",
		"    loop {",
		"        action l;",
		"    } until done;",
		"    transition first A accept s do {",
		"        action d;",
		"    } then B;",
		"}",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Empty braces stay on the declaration's line, as written.
func TestBreakStatementsKeepsEmptyBodies(t *testing.T) {
	got := layout(t, "package P { part def E {} part def F { } }\n")
	want := "package P {\n    part def E {}\n    part def F { }\n}\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// A line comment the author wrote after a statement stays on its line, and a
// `doc` or `comment` body ends the statement it belongs to.
func TestBreakStatementsAndComments(t *testing.T) {
	got := layout(t, "package P { part x : X; // why\n doc /* about P */ part def Y; /* bare */ part def Z; }\n")
	want := "package P {\n    part x : X; // why\n    doc /* about P */\n    part def Y;\n    /* bare */\n    part def Z;\n}\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// The default layout is unchanged: it still keeps a one-liner.
func TestDefaultLayoutKeepsOneLiners(t *testing.T) {
	src := "package P { part def Car; }\n"
	if got := format(t, src); got != src {
		t.Fatalf("default layout rewrapped: %q", got)
	}
}

func TestStatementsGolden(t *testing.T) {
	checkGolden(t, "testdata/statements.sysml", ConvertOptions)
}

// A body holding only a comment element stays on its line, as an elided body
// is written; a comment element beside statements gets a line of its own.
func TestBreakStatementsAndCommentOnlyBodies(t *testing.T) {
	cases := []struct{ src, want string }{
		{"action a { /* ... */ }\n", "action a { /* ... */ }\n"},
		{"package P { /* about P */ part x : X; }\n",
			"package P {\n    /* about P */\n    part x : X;\n}\n"},
		{"package P { part x : X; comment /* about x */ }\n",
			"package P {\n    part x : X;\n    comment /* about x */\n}\n"},
	}
	for _, tc := range cases {
		if got := layout(t, tc.src); got != tc.want {
			t.Errorf("layout(%q):\ngot:  %q\nwant: %q", tc.src, got, tc.want)
		}
	}
}

// An operator-led line after a finished statement, `:>> x = 1;`, sits at the
// body's level under both layouts.
func TestRedefinitionLinesAreNotContinuations(t *testing.T) {
	want := "part p : P {\n    :>> x = 1;\n    :> y;\n    attribute z = a\n        + b;\n}\n"
	src := "part p : P {\n        :>> x = 1;\n        :> y;\n    attribute z = a\n    + b;\n}\n"
	for _, opts := range []Options{DefaultOptions, ConvertOptions} {
		got, err := Source("t.sysml", []byte(src), opts)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%+v:\ngot:  %q\nwant: %q", opts, got, want)
		}
	}
	if got := layout(t, "part p : P { :>> x = 1; :> y; }\n"); got != "part p : P {\n    :>> x = 1;\n    :> y;\n}\n" {
		t.Errorf("one-liner: got %q", got)
	}
}
