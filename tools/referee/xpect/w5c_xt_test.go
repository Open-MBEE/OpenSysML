package xpect

import (
	"strings"
	"testing"
)

// variants covers every note shape that occurs in the pinned suites: `//` and
// `//*` notes, `-->`/`--->` arrows with and without spaces, a `//*` whose
// keyword is on the next line, and a `/*` comment that opens no note.
const variants = `//*
XPECT_SETUP org.omg.kerml.xpect.tests.testsuite.KerMLTest
	ResourceSet {
		ThisFile {}
		File {from ="/library/Base.kerml"}
		File "Dep.kerml" {}
	}
END_SETUP
*/

// XPECT noErrors --> ""
package test {
	// XPECT errors --> "first" at "class A;"
	class A;
	//XPECT errors ---> "second" at "class B;"
	class B;
	//* XPECT errors ---
	   "third" at "class C;"
	   "fourth" at "class C;"
	--- */
	class C;
	//* XPECT warnings --- "fifth" at "class D;" --- */
	class D;
	// XPECT linkedName at A --> test.A
	class E specializes A;
	//XPECT linkedName at B::b--> test.B.b
	class F specializes B::b;
	//* XPECT scope at A ---
	   A, B, C
	--- */
	//*
	XPECT exportedObjects ---
	sysml::Class: test::A
	--- */
	/* XPECT errors ---
	   "not run by Xpect" at "class G;"
	--- */
	class G;
}
`

func TestW5CParseVariants(t *testing.T) {
	f := parseXT("test.kerml.xt", "kerml", []byte(variants))
	if len(f.Problems) != 0 {
		t.Fatalf("problems: %v", f.Problems)
	}
	if f.SetupClass != "org.omg.kerml.xpect.tests.testsuite.KerMLTest" {
		t.Errorf("setup class = %q", f.SetupClass)
	}
	want := []resource{{ThisFile: true}, {From: "/library/Base.kerml"}, {From: "Dep.kerml"}}
	if len(f.Resources) != len(want) {
		t.Fatalf("resources = %+v, want %+v", f.Resources, want)
	}
	for i, r := range f.Resources {
		if r != want[i] {
			t.Errorf("resource %d = %+v, want %+v", i, r, want[i])
		}
	}

	type key struct {
		kind  string
		block bool
	}
	counts := map[key]int{}
	items := 0
	for _, a := range f.Assertions {
		counts[key{a.Kind, a.Block}]++
		items += len(a.Expect)
	}
	for k, n := range map[key]int{
		{kindNoErrors, false}:     1,
		{kindErrors, false}:       2,
		{kindErrors, true}:        1,
		{kindWarnings, true}:      1,
		{kindLinkedName, false}:   2,
		{kindScope, true}:         1,
		{"exportedObjects", true}: 1,
	} {
		if counts[k] != n {
			t.Errorf("%s (block=%v) count = %d, want %d", k.kind, k.block, counts[k], n)
		}
	}
	if len(counts) != 7 {
		t.Errorf("assertion kinds = %v", counts)
	}
	if items != 5 {
		t.Errorf("declared diagnostic items = %d, want 5", items)
	}

	// The `/*` note is XPECT-shaped but not an Xpect note: reported, not parsed.
	if len(f.Ignored) != 1 || !strings.Contains(f.Ignored[0], "opens no `//` or `//*` note") {
		t.Errorf("ignored = %v", f.Ignored)
	}
}

func TestW5CParseLinkedNameAndItems(t *testing.T) {
	f := parseXT("test.kerml.xt", "kerml", []byte(variants))
	var links []assertion
	for _, a := range f.Assertions {
		if a.Kind == kindLinkedName {
			links = append(links, a)
		}
	}
	if len(links) != 2 {
		t.Fatalf("linkedName assertions = %d", len(links))
	}
	if links[0].At != "A" || links[0].Expected != "test.A" {
		t.Errorf("first = %q --> %q", links[0].At, links[0].Expected)
	}
	// The arrow is written without a space before it in the suites.
	if links[1].At != "B::b" || links[1].Expected != "test.B.b" {
		t.Errorf("second = %q --> %q", links[1].At, links[1].Expected)
	}

	for _, a := range f.Assertions {
		if a.Kind != kindErrors || !a.Block {
			continue
		}
		if len(a.Expect) != 2 || a.Expect[0].Message != "third" || a.Expect[1].At != "class C;" {
			t.Errorf("block errors items = %+v", a.Expect)
		}
	}
}

func TestW5CParseProblemsAreReported(t *testing.T) {
	cases := map[string]string{
		"no XPECT_SETUP block":                             "package p {}\n",
		"XPECT_SETUP block is not terminated by END_SETUP": "//*\nXPECT_SETUP a.B\n\tResourceSet { ThisFile {} }\n",
		// A corrupted terminator does not terminate: it is not the token.
		"is not terminated by END_SETUP":       "//*\nXPECT_SETUP a.B\n\tResourceSet { ThisFile {} }\nEND_SETUPX\n*/\n",
		"no ResourceSet block in XPECT_SETUP":  "//*\nXPECT_SETUP a.B\nEND_SETUP\n*/\n",
		"is not terminated":                    "//*\nXPECT_SETUP a.B\n\tResourceSet { ThisFile {} }\nEND_SETUP\n*/\n//* XPECT errors ---\n\"x\" at \"y\"\n",
		"XPECT errors declares no expectation": "//*\nXPECT_SETUP a.B\n\tResourceSet { ThisFile {} }\nEND_SETUP\n*/\n// XPECT errors -->\nclass A;\n",
	}
	for want, content := range cases {
		f := parseXT("t.kerml.xt", "kerml", []byte(content))
		if len(f.Problems) == 0 {
			t.Errorf("%q: no problem reported", want)
			continue
		}
		if !strings.Contains(strings.Join(f.Problems, "; "), want) {
			t.Errorf("problems = %v, want one containing %q", f.Problems, want)
		}
	}
}

func TestW5CMaskNotesKeepsOffsets(t *testing.T) {
	content := []byte("package p {\n\t// XPECT errors --> \"m\" at \"class A;\"\n\tclass A;\n}\n")
	masked, _ := maskNotes(content)
	if len(masked) != len(content) {
		t.Fatalf("masked length = %d, want %d", len(masked), len(content))
	}
	if strings.Contains(string(masked), "XPECT") {
		t.Errorf("note text survived masking: %q", masked)
	}
	// The model source is untouched, at its original offset.
	at := strings.LastIndex(string(content), "class A;")
	if got := string(masked[at : at+len("class A;")]); got != "class A;" {
		t.Errorf("model source at %d = %q", at, got)
	}
}

func TestW5CSqueezeLocate(t *testing.T) {
	content := []byte("package p {\n\tattribute un: A::p;\n\tattribute unrelated;\n}\n")
	src := squeeze(content)

	// Declared targets are spaced freely; whitespace must not defeat the match.
	offset, end, ok := src.locate(0, `attribute un : A::p;`)
	if !ok {
		t.Fatal("declared target not located")
	}
	if want := strings.Index(string(content), "attribute un:"); offset != want {
		t.Errorf("offset = %d, want %d", offset, want)
	}
	if got := string(content[offset:end]); got != "attribute un: A::p;" {
		t.Errorf("span = %q", got)
	}

	// `un` occurs inside `unrelated`, which is not an occurrence of `un`.
	offset, _, ok = src.locate(0, "un")
	if !ok {
		t.Fatal("`un` not located")
	}
	if content[offset+2] != ':' {
		t.Errorf("located `un` inside a longer identifier at %d", offset)
	}

	// The search starts at the assertion's region, never before it.
	region := strings.Index(string(content), "attribute unrelated")
	offset, _, ok = src.locate(region, "attribute")
	if !ok || offset != region {
		t.Errorf("offset = %d, ok = %v, want %d", offset, ok, region)
	}

	if _, _, ok := src.locate(0, "no such text"); ok {
		t.Error("absent target reported as located")
	}
}
