package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// Every declared element carries its range in its document, 1-based lines and
// columns with an exclusive end, as a diagnostic's span gives them: the two `x`
// share their text but not their place, and an `x` sharing a line with its
// owner, which has no sysx:sourceText of its own, is still placed (#654).
func TestEachDeclarationCarriesItsSourceRange(t *testing.T) {
	const src = `package W {
    part a { attribute x; }
    part b { attribute x; }
}
`
	out, err := convert.Convert("w.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	got := map[string][4]float64{}
	for _, e := range elements {
		name, _ := e["qualifiedName"].(string)
		if line, ok := e["sysx:sourceLine"].(float64); ok {
			got[name] = [4]float64{line, e["sysx:sourceColumn"].(float64), e["sysx:sourceEndLine"].(float64), e["sysx:sourceEndColumn"].(float64)}
		}
	}
	for name, want := range map[string][4]float64{
		"W":       {1, 1, 4, 2},
		"W::a":    {2, 5, 2, 28},
		"W::a::x": {2, 14, 2, 26},
		"W::b":    {3, 5, 3, 28},
		"W::b::x": {3, 14, 3, 26},
	} {
		if got[name] != want {
			t.Errorf("%s: range %v, want %v", name, got[name], want)
		}
	}
	// The range reads nothing back: the graph without its text writes the same model.
	turtle, err := convert.Convert("w.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(turtle), "sysx:sourceLine ") {
		t.Fatalf("the Turtle form states no sysx:sourceLine:\n%s", turtle)
	}
	back, err := convert.Convert("w.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("a graph carrying source ranges did not read back: %v", err)
	}
	if string(back) != src {
		t.Errorf("the notation read back changed:\n%s", back)
	}
}

// A range ends at the declaration's last token: a note or comment after it
// belongs to no declaration, while a comment that is a declaration's body
// (`doc /* … */`) is part of it.
func TestSourceRangeStopsBeforeTrailingComments(t *testing.T) {
	const src = `package C {
    part a; /* note */ part b; // after
    part d { doc /* body */ }
}
`
	got := sourceRanges(t, src)
	for name, want := range map[string][4]float64{
		"C::a":     {2, 5, 2, 12},
		"C::b":     {2, 24, 2, 31},
		"C::d":     {3, 5, 3, 30},
		"C::d::@0": {3, 14, 3, 28},
	} {
		if got[name] != want {
			t.Errorf("%s: range %v, want %v", name, got[name], want)
		}
	}
}

// sourceRanges converts src to the API's JSON form and reads each element's
// range by qualified name.
func sourceRanges(t *testing.T, src string) map[string][4]float64 {
	t.Helper()
	out, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	got := map[string][4]float64{}
	for _, e := range elements {
		name, _ := e["qualifiedName"].(string)
		if line, ok := e["sysx:sourceLine"].(float64); ok {
			got[name] = [4]float64{line, e["sysx:sourceColumn"].(float64), e["sysx:sourceEndLine"].(float64), e["sysx:sourceEndColumn"].(float64)}
		}
	}
	return got
}

// A declaration's body is the `/* … */` comment the parser attached to it, even
// after a note: `doc // note` followed by its body on the next line ends at
// the body, as does a `comment` or a textual representation written that way.
func TestSourceRangeIncludesBodyAfterNote(t *testing.T) {
	const src = `package C {
    doc // note
    /* body */
    part p {
        comment // n
        /* about */
        rep inline language "text" // n
        /* text */
    }
}
`
	got := sourceRanges(t, src)
	for name, want := range map[string][4]float64{
		"C::@0":        {2, 5, 3, 15},
		"C::p":         {4, 5, 9, 6},
		"C::p::@0":     {5, 9, 6, 20},
		"C::p::inline": {7, 9, 8, 19},
	} {
		if got[name] != want {
			t.Errorf("%s: range %v, want %v", name, got[name], want)
		}
	}
}
