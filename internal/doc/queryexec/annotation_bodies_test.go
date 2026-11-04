package queryexec

import (
	"slices"
	"testing"
)

// annotatedBody documents one part and gives another a textual representation
// and a localized comment, each annotation unnamed but the comment.
const annotatedBody = `
part <a> aaaa {
	doc /*Documentation*/
	port p;
}
part <b> bbb {
	rep language "rus"
		/*Textual representation*/
	rep language "markdown" /* # Title
	                         * body */
	comment c locale "fr" /* Un commentaire */
	port p;
}
`

func annotatedRows(t *testing.T, query string) *RowSet {
	t.Helper()
	fixture := loadExecutionFixture(t, annotatedBody+query)
	result, err := fixture.execute(t, "Q", Bindings{}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return result
}

// TestExecuteProjectsTextualRepresentationBodies verifies Project reads an
// element's textualRepresentation as the body of each rep it owns, in order and
// exact, the way it reads documentation; an element owning none has an empty cell.
func TestExecuteProjectsTextualRepresentationBodies(t *testing.T) {
	result := annotatedRows(t, `
calc def Q :> Query {
	Project(
		source = WhereType(source = Descendants(source = Named(qualifiedName = "Observatory"), maxDepth = 1), type = "PartUsage"),
		properties = ("name", "textualRepresentation", "documentation")
	)
}`)
	if got := cellStrings(t, result, "textualRepresentation"); !slices.EqualFunc(got,
		[][]string{{}, {"Textual representation", "# Title\nbody "}}, slices.Equal) {
		t.Fatalf("textualRepresentation cells = %q", got)
	}
	if got := cellStrings(t, result, "documentation"); !slices.EqualFunc(got,
		[][]string{{"Documentation"}, {}}, slices.Equal) {
		t.Fatalf("documentation cells = %q", got)
	}
}

// TestExecuteProjectsAnnotationBodyLanguageAndLocale verifies a doc, rep or
// comment row reports its own body exactly, with the language or locale it
// declares and an empty cell for the one it does not.
func TestExecuteProjectsAnnotationBodyLanguageAndLocale(t *testing.T) {
	result := annotatedRows(t, `
calc def Q :> Query {
	Project(
		source = WhereType(source = Descendants(source = Named(qualifiedName = "Observatory"), maxDepth = 2), type = "AnnotatingElement"),
		properties = ("body", "language", "locale")
	)
}`)
	want := map[string][][]string{
		"body":     {{"Documentation"}, {"Textual representation"}, {"# Title\nbody "}, {"Un commentaire "}},
		"language": {{}, {"rus"}, {"markdown"}, {}},
		"locale":   {{}, {}, {}, {"fr"}},
	}
	for column, cells := range want {
		if got := cellStrings(t, result, column); !slices.EqualFunc(got, cells, slices.Equal) {
			t.Errorf("%s cells = %q, want %q", column, got, cells)
		}
	}
}
