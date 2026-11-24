package jupyter

import (
	"strings"
	"testing"
)

func TestMetadataIsListedInACell(t *testing.T) {
	e := newEngine(t)
	mustRun(t, e, `package Demo {
    private import ScalarValues::*;
    metadata def Tag { attribute who : String default = "nobody"; }
    part def Wheel { @Tag; }
    part w : Wheel { @Tag { who = "w"; } }
}`)
	if got := mustRun(t, e, "%metadata Demo::Wheel").text(); !strings.Contains(got, "Element: Demo::Wheel (partDef)") ||
		!strings.Contains(got, `who = "nobody" (default)`) {
		t.Errorf("%%metadata of a definition = %q", got)
	}
	got := mustRun(t, e, "%instantiate Demo::w\n%metadata w").text()
	for _, want := range []string{
		"Annotations of usage Demo::w, which the object is an instance of:",
		`who = "w"`,
		"Annotations of definition Demo::Wheel, which Demo::w specializes:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%%metadata of an object lacks %q:\n%s", want, got)
		}
	}
	// An unresolved target fails the cell; an unannotated element does not.
	if failure := mustFail(t, e, "%metadata Demo::nope"); !strings.Contains(failure.Error(), "unresolved reference") {
		t.Errorf("%%metadata of an unknown name = %v", failure)
	}
	if got := mustRun(t, e, "%metadata Demo").text(); !strings.Contains(got, "(no annotations)") {
		t.Errorf("%%metadata of an unannotated element = %q", got)
	}
}
