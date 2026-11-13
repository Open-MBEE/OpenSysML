package export_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// A `/* */` comment written where a member may appear is a Comment element
// with neither name nor `about`, owned by the namespace it is written in
// (KerML.xtext Comment, SysML.xtext AnnotatingMember), so it reaches the API
// JSON and the graph like a `comment /* */` does and comes back from either.
func TestRegularCommentsAreCommentElements(t *testing.T) {
	t.Parallel()
	const src = `/* at the top of the file */
package P {
    /* before a member */
    part def A {
        /* in a body */
        attribute x : Real;
        /* at the close of a body */
    }
    comment /* stated as a comment */
    doc /* package doc */
}
/* at the end of the file */
`
	want := []string{"at the close of a body ", "at the end of the file ", "at the top of the file ", "before a member ",
		"in a body ", "package doc ", "stated as a comment "}
	if got := apiJSONBodies(t, src); !slices.Equal(got, want) {
		t.Fatalf("api-json bodies = %q, want %q", got, want)
	}

	apiJSON, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(apiJSON, &elements); err != nil {
		t.Fatalf("api-json: %v", err)
	}
	for _, element := range elements {
		if element["@type"] != "Comment" || element["body"] != "in a body " {
			continue
		}
		if owner, _ := element["owner"].(map[string]any); owner["@id"] != "P__A" {
			t.Errorf("the comment in A's body is owned by %v, want P::A", element["owner"])
		}
		if element["declaredName"] != nil {
			t.Errorf("the comment is named %v, want no name", element["declaredName"])
		}
	}

	fromJSON, err := convert.Convert("m.json", apiJSON, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("api-json back to notation: %v", err)
	}
	if got := apiJSONBodies(t, string(fromJSON)); !slices.Equal(got, want) {
		t.Errorf("bodies after the api-json trip = %q, want %q, rebuilt as\n%s", got, want, fromJSON)
	}

	structural := withoutTriples(t, withoutTriples(t, idTurtle(t, src), "sysx:sourceText"), "sysx:sourceTail")
	back := toNotation(t, structural)
	if got := apiJSONBodies(t, back); !slices.Equal(got, want) {
		t.Errorf("bodies after the stripped turtle trip = %q, want %q, rebuilt as\n%s", got, want, back)
	}
	if again := withoutTriples(t, withoutTriples(t, idTurtle(t, back), "sysx:sourceText"), "sysx:sourceTail"); string(again) != string(structural) {
		t.Errorf("the rebuilt notation does not state the graph:\n--- want ---\n%s--- got ---\n%s", structural, again)
	}
}

// A comment inside a declaration or an expression is not a member position,
// so it is no element; a line note never is.
func TestCommentsOutsideMemberPositionsAreNotElements(t *testing.T) {
	t.Parallel()
	src := strings.Join([]string{
		"package P { // a note",
		"    part def A { attribute x /* in a declaration */ : Real = 1 /* in an expression */ + 2; }",
		"}",
		"",
	}, "\n")
	if got := apiJSONBodies(t, src); len(got) != 0 {
		t.Errorf("api-json bodies = %q, want none", got)
	}
}
