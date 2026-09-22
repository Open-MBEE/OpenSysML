package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A {"@ref": <name>} member — sysml-toolkit's spelling of an unresolved target
// — reads as the name literal the mapping writes for a name-valued reference,
// whether the member stands alone or inside a collection.
func TestReadAPIJSONRefName(t *testing.T) {
	document := `[
		{"@type": "Package", "@id": "P", "ownedRelationship": [{"@id": "M"}]},
		{"@type": "OwningMembership", "@id": "M",
			"memberElement": {"@ref": "kg"},
			"ownedRelatedElement": [{"@ref": "kg"}],
			"owningRelatedElement": {"@id": "P"}}
	]`
	graph, err := ReadAPIJSON([]byte(document))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	membership := rdf.ElementIRIForID("M")
	for _, predicate := range []string{"memberElement", "ownedRelatedElement"} {
		if !graph.Has(rdf.Triple{Subject: membership, Predicate: rdf.SysMLTerm(predicate), Object: rdf.String("kg")}) {
			t.Errorf("missing %s literal triple:\n%s", predicate, rdf.WriteTurtle(graph))
		}
	}
}

// An object that is neither {"@id": <id>} nor {"@ref": <name>} is refused, and
// the message names both spellings.
func TestReadAPIJSONRefusesOtherObjects(t *testing.T) {
	for name, value := range map[string]string{
		"unknown key":    `{"@foo": "x"}`,
		"@id and @ref":   `{"@id": "a", "@ref": "b"}`,
		"empty @id":      `{"@id": ""}`,
		"empty @ref":     `{"@ref": ""}`,
		"non-string @id": `{"@id": 5}`,
	} {
		document := `[{"@type": "Package", "@id": "X", "ownedMember": ` + value + `}]`
		_, err := ReadAPIJSON([]byte(document))
		if err == nil {
			t.Errorf("%s: ReadAPIJSON succeeded, want an error", name)
			continue
		}
		want := `an object value is a reference {"@id": <id>} or {"@ref": <name>}`
		if got := err.Error(); !strings.Contains(got, want) {
			t.Errorf("%s: error %q, want one containing %q", name, got, want)
		}
	}
}
