package export_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// commentBodies are the bodies KerML 1.1 §8.2.3.3.2 lets a REGULAR_COMMENT
// state: every text without "*/", white space at either end included.
var commentBodies = []string{
	"", " ", "Sliced.", " lead", "trail ", "\tboth\t", "a\n  indented\nb",
	"\nafter a blank line", "before a blank line\n", "* a bullet\n* another", "ends in a star*",
}

// apiJSONBodies answers the Comment::body of every comment and documentation
// the notation declares, as the API JSON reports them, sorted.
func apiJSONBodies(t *testing.T, src string) []string {
	t.Helper()
	out, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v\n%s", err, src)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatalf("api-json: %v", err)
	}
	var bodies []string
	for _, element := range elements {
		if kind := element["@type"]; kind == "Comment" || kind == "Documentation" {
			body, ok := element["body"].(string)
			if !ok {
				t.Fatalf("%v has no string body", element)
			}
			bodies = append(bodies, body)
		}
	}
	slices.Sort(bodies)
	return bodies
}

// commentModel declares one comment and one documentation stating body.
func commentModel(t *testing.T, body string) string {
	t.Helper()
	comment, ok := source.CommentText(body, "    ")
	if !ok {
		t.Fatalf("CommentText(%q) refused", body)
	}
	return "package P {\n    comment c " + comment + "\n    part def A {\n        doc " +
		strings.ReplaceAll(comment, "\n", "\n    ") + "\n    }\n}\n"
}

// TestCommentBodiesRoundTrip carries each body through the API JSON and
// through Turtle with the source text stripped, so the structural sysml:body
// alone must restate it.
func TestCommentBodiesRoundTrip(t *testing.T) {
	for _, body := range commentBodies {
		src := commentModel(t, body)
		want := []string{body, body}
		if got := apiJSONBodies(t, src); !slices.Equal(got, want) {
			t.Errorf("api-json bodies of\n%s= %q, want %q", src, got, want)
			continue
		}
		turtle := idTurtle(t, src)
		structural := withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail")
		back := toNotation(t, structural)
		if got := apiJSONBodies(t, back); !slices.Equal(got, want) {
			t.Errorf("bodies after the stripped turtle trip of %q = %q, rebuilt as\n%s", body, got, back)
		}
		if again := withoutTriples(t, withoutTriples(t, idTurtle(t, back), "sysx:sourceText"), "sysx:sourceTail"); string(again) != string(structural) {
			t.Errorf("the rebuilt notation of %q does not state the graph:\n--- want ---\n%s--- got ---\n%s", body, structural, again)
		}

		apiJSON, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
		if err != nil {
			t.Fatalf("to api-json: %v", err)
		}
		fromJSON, err := convert.Convert("m.json", apiJSON, convert.FormatAPIJSON, convert.FormatSysML)
		if err != nil {
			t.Fatalf("api-json back to notation: %v", err)
		}
		if got := apiJSONBodies(t, string(fromJSON)); !slices.Equal(got, want) {
			t.Errorf("bodies after the api-json trip of %q = %q, rebuilt as\n%s", body, got, fromJSON)
		}
	}
}

// A body holding "*/" or a carriage return has no comment that states it, so
// a graph carrying one is refused rather than written as something else.
func TestUnrepresentableCommentBodyIsRefused(t *testing.T) {
	turtle := withoutTriples(t, idTurtle(t, "package P {\n    comment c /* placeholder*/\n}\n"), "sysx:sourceText")
	for _, body := range []string{`ends */ early`, `carriage\rreturn`} {
		escaped := strings.ReplaceAll(body, "\r", `\r`)
		edited := editTurtle(t, turtle, `sysml:body "placeholder"`, `sysml:body "`+escaped+`"`)
		_, err := convert.Convert("m.ttl", edited, convert.FormatTurtle, convert.FormatSysML)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Errorf("body %q converted with err = %v, want UnsupportedError", body, err)
		}
	}
}

// TestCommentLineTerminatorsReadAsLineBreaks reads a comment written with
// `\r\n` or lone `\r` line terminators (KerML 1.1 §8.2.2.1) as the same body
// its `\n` spelling states, in the API JSON and in the structural Turtle.
func TestCommentLineTerminatorsReadAsLineBreaks(t *testing.T) {
	const body = "a\n  indented\nb"
	src := commentModel(t, body)
	want := []string{body, body}
	structural := withoutTriples(t, withoutTriples(t, idTurtle(t, src), "sysx:sourceText"), "sysx:sourceTail")
	for _, terminator := range []string{"\r\n", "\r"} {
		spelled := strings.ReplaceAll(src, "\n", terminator)
		if got := apiJSONBodies(t, spelled); !slices.Equal(got, want) {
			t.Errorf("api-json bodies with %q line terminators = %q, want %q", terminator, got, want)
		}
		got := withoutTriples(t, withoutTriples(t, idTurtle(t, spelled), "sysx:sourceText"), "sysx:sourceTail")
		if string(got) != string(structural) {
			t.Errorf("turtle with %q line terminators:\n--- want ---\n%s--- got ---\n%s", terminator, structural, got)
		}
	}
}
