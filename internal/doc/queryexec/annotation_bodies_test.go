package queryexec

import (
	"errors"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
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

// featureNamedBody declares features that share their names with the
// annotation properties, on a definition an object can be made of.
const featureNamedBody = `
part def Message {
	attribute body : String default "Hello";
	attribute locale : String default "fr";
	attribute language : String default "en";
}
part msg : Message;
part blank;
`

// A feature named body, locale or language is still read on an element that is no
// annotation; one lacking it is the missing-feature error, whether or not another row has it.
func TestExecuteAnnotationPropertiesDoNotShadowFeatures(t *testing.T) {
	fixture := loadExecutionFixture(t, featureNamedBody+`
calc def Q :> Query {
	Project(source = Named(qualifiedName = "Observatory::msg"), properties = ("body", "locale", "language"))
}
calc def Missing :> Query {
	Project(
		source = WhereType(source = Descendants(source = Named(qualifiedName = "Observatory"), maxDepth = 1), type = "PartUsage"),
		properties = ("body")
	)
}
calc def NoRowHasIt :> Query {
	Project(source = Named(qualifiedName = "Observatory::blank"), properties = ("body"))
}`)
	result, err := fixture.execute(t, "Q", Bindings{}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for column, want := range map[string]string{"body": "Hello", "locale": "fr", "language": "en"} {
		if got := cellStrings(t, result, column); !slices.EqualFunc(got, [][]string{{want}}, slices.Equal) {
			t.Errorf("%s cells = %q, want [[%q]]", column, got, want)
		}
	}
	_, err = fixture.execute(t, "Missing", Bindings{}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorMissingFeature || executionError.Target != "Observatory::blank" {
		t.Fatalf("body of a part without one: err = %v, want ErrorMissingFeature at Observatory::blank", err)
	}
	_, err = fixture.execute(t, "NoRowHasIt", Bindings{}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorMissingFeature || executionError.Target != "Observatory::blank" {
		t.Fatalf("body no row has: err = %v, want ErrorMissingFeature at Observatory::blank", err)
	}
}

// TestExecuteObjectFeaturesNamedLikeAnnotationProperties verifies an object row
// reads its own body, locale and language features: an object is no annotation.
func TestExecuteObjectFeaturesNamedLikeAnnotationProperties(t *testing.T) {
	fixture := loadExecutionFixture(t, featureNamedBody+`
calc def Q :> Query {
	in root : Element;
	Project(source = root, properties = ("name", "body", "locale", "language"))
}`)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	message, err := ctx.Instantiate(fixture.symbol(t, "msg"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	result, err := Execute(fixture.program(t, "Q"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
			Roots: []Root{{Label: "msg", Object: message}}},
		Bindings{"root": {ObjectValue(message, "msg")}}, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for column, want := range map[string]string{"body": "Hello", "locale": "fr", "language": "en"} {
		if got := cellStrings(t, result, column); !slices.EqualFunc(got, [][]string{{want}}, slices.Equal) {
			t.Errorf("%s cells = %q, want [[%q]]", column, got, want)
		}
	}
}

// TestExecuteProjectsAnEmptyAnnotationBody verifies a blank `/* */` is the
// empty string, not an empty cell.
func TestExecuteProjectsAnEmptyAnnotationBody(t *testing.T) {
	result := annotatedRows(t, `
part empty {
	doc /* */
}
calc def Q :> Query {
	Project(
		source = WhereType(source = Descendants(source = Named(qualifiedName = "Observatory::empty"), maxDepth = 1), type = "Documentation"),
		properties = ("body")
	)
}`)
	if got := cellStrings(t, result, "body"); !slices.EqualFunc(got, [][]string{{""}}, slices.Equal) {
		t.Fatalf("body cells = %q, want [[\"\"]]", got)
	}
}
