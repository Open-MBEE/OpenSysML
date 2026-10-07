package runtime

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestRuntimeRobustnessStringNotation checks that a String prints as a String
// literal the grammar admits and that reads back to the same String, whatever
// characters it holds.
func TestRuntimeRobustnessStringNotation(t *testing.T) {
	t.Run("every_character_reads_back", testStringNotationReadsBack)
	t.Run("invalid_notation_names_the_literal", testInvalidNotationNamesTheLiteral)
}

// stringNotationSamples holds every ASCII character and the Unicode characters
// a Go-quoted String would escape: format, separator, private-use and
// noncharacter code points.
func stringNotationSamples() []string {
	samples := []string{"", `"'\`, "\\n", "\u200b\u200d\u2028\u2029\ufeff", "\u0085\u00a0\u00ad", "\ue000\U000E0001\U0010FFFF\ufffe", "\U0001F600"}
	for c := rune(0); c < utf8.RuneSelf; c++ {
		samples = append(samples, string(c), "a"+string(c)+"b")
	}
	return samples
}

func testStringNotationReadsBack(t *testing.T) {
	for _, s := range stringNotationSamples() {
		printed := FormatValue(NewStringValue(s))
		if bad := lexer.InvalidEscapes(source.Span{}, printed); len(bad) > 0 {
			t.Errorf("%q prints as %s, with an escape no String literal admits", s, printed)
		}
		if got := source.StringValue(printed); got != s {
			t.Errorf("%q prints as %s, which reads back as %q", s, printed, got)
		}
		if strings.ContainsAny(printed[1:len(printed)-1], "\b\t\n\f\r") {
			t.Errorf("%q prints as %s, holding a control character the grammar names an escape for", s, printed)
		}
	}
}

func testInvalidNotationNamesTheLiteral(t *testing.T) {
	err := invalidNotation("RealFunctions::ToReal", "1\u200b", "Real")
	if !errors.Is(err, ErrInvalidNotation) || !strings.Contains(err.Error(), "\"1\u200b\" is not a Real") {
		t.Errorf("invalidNotation = %v, want ErrInvalidNotation naming the String literal \"1\u200b\"", err)
	}
}
