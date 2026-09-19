package export_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// libraryGraphWithoutSourceText converts a bundled library file to Turtle and
// strips the source text, leaving the graph to carry the notation back alone.
func libraryGraphWithoutSourceText(t *testing.T, name string) (turtle, stripped []byte) {
	t.Helper()
	src, err := libs.EmbeddedSource().Read(name)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err = convert.Convert(name, src, convert.FormatSysML, convert.FormatTurtle)
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

// Every bundled library file comes back from the mapping alone: the notation
// written from its source-free graph is the library rooted at its normative
// ids, so it states no id in an annotation and converts to the same graph
// but for the source text, and that graph writes the same notation again.
func TestLibraryFilesComeBackFromTheGraphAlone(t *testing.T) {
	names := libs.EmbeddedSource().List()
	if len(names) == 0 {
		t.Fatal("no bundled library files")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			turtle, stripped := libraryGraphWithoutSourceText(t, name)
			back := toNotation(t, stripped)
			if strings.Contains(back, "sysx:") {
				t.Errorf("the notation leaks graph vocabulary:\n%s", back)
			}
			if strings.Contains(back, "@IdentityMetadata::ElementId") {
				t.Errorf("the library's ids are the norm's, yet the notation states one:\n%s", back)
			}
			keepsIDsWithoutSourceText(t, name, turtle)
			copyName := "copy" + filepath.Ext(name)
			second, err := convert.Convert(copyName, []byte(back), convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("rebuilt notation to turtle: %v", err)
			}
			if got := withoutSourceText(t, second); string(got) != string(stripped) {
				t.Errorf("the first source-free hop changed the graph:\n%s", firstLineDifference(stripped, got))
			}
			again := toNotation(t, withoutSourceText(t, second))
			if again != back {
				t.Errorf("the graph of the rebuilt notation writes different notation:\n%s", firstLineDifference([]byte(back), []byte(again)))
			}
			third, err := convert.Convert(copyName, []byte(again), convert.FormatSysML, convert.FormatTurtle)
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
