package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
)

// libraryGraphWithoutSourceText converts a bundled library file to Turtle and
// strips the source text, leaving the graph to carry the notation back alone.
func libraryGraphWithoutSourceText(t *testing.T, name string) (turtle, stripped []byte) {
	t.Helper()
	src, err := libs.EmbeddedSource().Read(name)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err = export.Convert(name, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	return turtle, withoutSourceText(t, turtle)
}

// A library graph read back from the mapping alone is read in the library
// document's place, so a chain segment the library implies — a transition's
// accepter, which Actions::TransitionAction declares and every transition
// inherits — resolves where it is written, and the whole file converts.
func TestLibraryGraphSpellsImpliedChainSegmentWithoutSourceText(t *testing.T) {
	const name = "Systems Library/Actions.sysml"
	turtle, stripped := libraryGraphWithoutSourceText(t, name)
	back := toNotation(t, stripped)
	if !strings.Contains(back, "aState.aTransition.accepter.acceptedMessage") {
		t.Errorf("the chain through the implied accepter was not spelled as the library writes it:\n%s", back)
	}
	keepsIDsWithoutSourceText(t, name, turtle)
}

// A KerML library graph whose roots record no grammar is read in the library
// document's grammar, not as SysML, so its KerML-only notation parses back.
func TestKerMLLibraryGraphReadsAsKerMLWithoutSourceText(t *testing.T) {
	const name = "Kernel Libraries/Kernel Semantic Library/Clocks.kerml"
	turtle, stripped := libraryGraphWithoutSourceText(t, name)
	back := toNotation(t, stripped)
	if !strings.Contains(back, "private struct UniversalClockLife[1] subsets Clock, Life {") {
		t.Errorf("the library was not written in KerML:\n%s", back)
	}
	keepsIDsWithoutSourceText(t, name, turtle)
}
