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
	elements := sourceRangeDocument(t, src)
	root := elements[0]
	pkg := sourceRangeOwnedNamed(t, elements, root, "Package", "C")
	a := sourceRangeOwnedNamed(t, elements, pkg, "PartUsage", "a")
	b := sourceRangeOwnedNamed(t, elements, pkg, "PartUsage", "b")
	d := sourceRangeOwnedNamed(t, elements, pkg, "PartUsage", "d")
	body := sourceRangeOwnedUnnamedAt(t, elements, d, "Documentation", 0)
	for _, tc := range []struct {
		name    string
		element map[string]json.RawMessage
		want    [4]float64
	}{
		{"C::a", a, [4]float64{2, 5, 2, 12}},
		{"C::b", b, [4]float64{2, 24, 2, 31}},
		{"C::d", d, [4]float64{3, 5, 3, 30}},
		{"C::d::@0", body, [4]float64{3, 14, 3, 28}},
	} {
		if got := sourceRange(t, tc.element); got != tc.want {
			t.Errorf("%s: range %v, want %v", tc.name, got, tc.want)
		}
	}
}

func sourceRangeDocument(t *testing.T, src string) []map[string]json.RawMessage {
	t.Helper()
	out, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	return rootElements(t, out)
}

func sourceRangeOwnedElements(t *testing.T, elements []map[string]json.RawMessage, owner map[string]json.RawMessage) []map[string]json.RawMessage {
	t.Helper()
	var out []map[string]json.RawMessage
	for _, relationshipID := range rootRefs(t, owner["ownedRelationship"]) {
		relationship := apiJSONElement(t, elements, relationshipID)
		var targetIDs []string
		if related, ok := relationship["ownedRelatedElement"]; ok && string(related) != "null" {
			var refs []json.RawMessage
			if err := json.Unmarshal(related, &refs); err == nil {
				for _, ref := range refs {
					targetIDs = append(targetIDs, rootRef(t, ref))
				}
			}
		}
		if len(targetIDs) == 0 {
			if member, ok := relationship["memberElement"]; ok && string(member) != "null" {
				targetIDs = append(targetIDs, rootRef(t, member))
			}
		}
		for _, targetID := range targetIDs {
			out = append(out, apiJSONElement(t, elements, targetID))
		}
	}
	return out
}

func sourceRangeOwnedNamed(t *testing.T, elements []map[string]json.RawMessage, owner map[string]json.RawMessage, typ, name string) map[string]json.RawMessage {
	t.Helper()
	for _, child := range sourceRangeOwnedElements(t, elements, owner) {
		if rootString(t, child["@type"]) != typ {
			continue
		}
		declaredName, ok := child["declaredName"]
		if !ok || string(declaredName) == "null" || rootString(t, declaredName) != name {
			continue
		}
		return child
	}
	t.Fatalf("no owned %s named %q under %s", typ, name, owner["@id"])
	return nil
}

func sourceRangeOwnedUnnamedAt(t *testing.T, elements []map[string]json.RawMessage, owner map[string]json.RawMessage, typ string, index int) map[string]json.RawMessage {
	t.Helper()
	var unnamed []map[string]json.RawMessage
	for _, child := range sourceRangeOwnedElements(t, elements, owner) {
		if rootString(t, child["@type"]) != typ {
			continue
		}
		if name, ok := child["declaredName"]; ok && string(name) != "null" {
			continue
		}
		unnamed = append(unnamed, child)
	}
	if index < 0 || index >= len(unnamed) {
		t.Fatalf("owned unnamed %s at index %d not found under %s (found %d)", typ, index, owner["@id"], len(unnamed))
	}
	return unnamed[index]
}

func sourceRange(t *testing.T, element map[string]json.RawMessage) [4]float64 {
	t.Helper()
	var got [4]float64
	for i, key := range []string{"sysx:sourceLine", "sysx:sourceColumn", "sysx:sourceEndLine", "sysx:sourceEndColumn"} {
		if err := json.Unmarshal(element[key], &got[i]); err != nil {
			t.Fatalf("%s = %s: %v", key, element[key], err)
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
	elements := sourceRangeDocument(t, src)
	root := elements[0]
	pkg := sourceRangeOwnedNamed(t, elements, root, "Package", "C")
	documentation := sourceRangeOwnedUnnamedAt(t, elements, pkg, "Documentation", 0)
	p := sourceRangeOwnedNamed(t, elements, pkg, "PartUsage", "p")
	comment := sourceRangeOwnedUnnamedAt(t, elements, p, "Comment", 0)
	inline := sourceRangeOwnedNamed(t, elements, p, "TextualRepresentation", "inline")
	for _, tc := range []struct {
		name    string
		element map[string]json.RawMessage
		want    [4]float64
	}{
		{"C::@0", documentation, [4]float64{2, 5, 3, 15}},
		{"C::p", p, [4]float64{4, 5, 9, 6}},
		{"C::p::@0", comment, [4]float64{5, 9, 6, 20}},
		{"C::p::inline", inline, [4]float64{7, 9, 8, 19}},
	} {
		if got := sourceRange(t, tc.element); got != tc.want {
			t.Errorf("%s: range %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The accept action a transition's trigger declares is placed where the
// trigger is written, as every other declared element is.
func TestTransitionTriggerCarriesItsSourceRange(t *testing.T) {
	const src = `package T {
    attribute def Sig;
    state def S {
        state a;
        state b;
        transition t first a accept s : Sig then b;
    }
}
`
	elements := sourceRangeDocument(t, src)
	root := elements[0]
	pkg := sourceRangeOwnedNamed(t, elements, root, "Package", "T")
	state := sourceRangeOwnedNamed(t, elements, pkg, "StateDefinition", "S")
	transition := sourceRangeOwnedNamed(t, elements, state, "TransitionUsage", "t")
	trigger := sourceRangeOwnedUnnamedAt(t, elements, transition, "AcceptActionUsage", 0)
	if got, want := sourceRange(t, trigger), [4]float64{6, 30, 6, 44}; got != want {
		t.Errorf("the trigger's accept action: range %v, want %v", got, want)
	}
}
