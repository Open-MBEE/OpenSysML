package export_test

import (
	"path/filepath"
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

// The library files whose graphs carry a cross feature the norm ids, a
// declaration inside an expression body, or an invocation of a named function
// come back from the mapping alone: every element keeps its id, and the notation
// written from the graph converts to a graph that writes the same notation again.
func TestLibraryFilesComeBackFromTheGraphAlone(t *testing.T) {
	for _, name := range []string{
		"Systems Library/Items.sysml",
		"Kernel Libraries/Kernel Semantic Library/Links.kerml",
		"Kernel Libraries/Kernel Semantic Library/Occurrences.kerml",
		"Kernel Libraries/Kernel Semantic Library/TransitionPerformances.kerml",
		"Domain Libraries/Cause and Effect/CausationConnections.sysml",
		"Domain Libraries/Analysis/TradeStudies.sysml",
		"Kernel Libraries/Kernel Semantic Library/Observation.kerml",
	} {
		t.Run(filepath.Base(name), func(t *testing.T) {
			turtle, stripped := libraryGraphWithoutSourceText(t, name)
			back := toNotation(t, stripped)
			if strings.Contains(back, "sysx:") {
				t.Errorf("the notation leaks graph vocabulary:\n%s", back)
			}
			keepsIDsWithoutSourceText(t, name, turtle)
			copyName := "copy" + filepath.Ext(name)
			second, err := export.Convert(copyName, []byte(back), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("rebuilt notation to turtle: %v", err)
			}
			again := toNotation(t, withoutSourceText(t, second))
			if again != back {
				t.Errorf("the graph of the rebuilt notation writes different notation:\n%s", firstLineDifference([]byte(back), []byte(again)))
			}
			third, err := export.Convert(copyName, []byte(again), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("second rebuilt notation to turtle: %v", err)
			}
			if string(third) != string(second) {
				t.Errorf("the second source-free hop is not idempotent:\n%s", firstLineDifference(second, third))
			}
		})
	}
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
