package view

import (
	"slices"
	"strings"
	"testing"
)

// A kind's words split at spaces outside a quoted name, so a quoted
// user-defined keyword stays one modifier and the keyword after it names the
// compartment.
func TestKindWordsKeepQuotedNamesWhole(t *testing.T) {
	kind := "abstract #'Safety Level' #$::M::Tag part"
	if got, want := kindWords(kind), []string{"abstract", "#'Safety Level'", "#$::M::Tag", "part"}; !slices.Equal(got, want) {
		t.Errorf("kindWords(%q) = %q, want %q", kind, got, want)
	}
	escaped := `#'Owner\'s Level' part`
	if got, want := kindWords(escaped), []string{`#'Owner\'s Level'`, "part"}; !slices.Equal(got, want) {
		t.Errorf("kindWords(%q) = %q, want %q", escaped, got, want)
	}
	if got := compartmentName(escaped); got != "parts" {
		t.Errorf("compartmentName(%q) = %q, want parts", escaped, got)
	}
	if prefix, keyword := splitRowPrefix(kind); prefix != "abstract #'Safety Level' #$::M::Tag" || keyword != "part" {
		t.Errorf("splitRowPrefix(%q) = %q, %q", kind, prefix, keyword)
	}
	if got := compartmentName(kind); got != "parts" {
		t.Errorf("compartmentName(%q) = %q, want parts", kind, got)
	}
	if got := rowCompartments("end attribute"); !slices.Equal(got, []string{"ends", "attributes"}) {
		t.Errorf("rowCompartments(end attribute) = %q", got)
	}
}

// A quoted or global metadata name heads its declaration as written.
func TestExtendedKindWritesQuotedAndGlobalNames(t *testing.T) {
	rendering := renderSource(t, "M::V::hub", `package M {
	metadata def 'Safety Level';
	metadata def Tag;
	part def Sensor;
	part def Hub {
		#'Safety Level' part s : Sensor;
		#$::M::Tag part t : Sensor;
	}
	package V {
		private import Views::*;
		view hub { expose M::Hub; expose M::Sensor; render asTreeDiagram; }
	}
}`)
	out, err := rendering.Write(FormText)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#'Safety Level' part s\n", "#$::M::Tag part t\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("text rendering lacks %q:\n%s", want, out)
		}
	}
	dot, err := rendering.WriteWith(FormDot, Options{Compartments: CompartmentsMost})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dot), "Level' parts") || !strings.Contains(string(dot), ">parts<") {
		t.Errorf("the quoted keyword's rows are not listed under parts:\n%s", dot)
	}
}
