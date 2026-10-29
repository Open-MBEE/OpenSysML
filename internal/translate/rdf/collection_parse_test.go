package rdf

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Comparing written collections against the graph parses every annotation, so
// a member in the writer's own spelling costs a fixed few allocations.
func TestParseCollectionJSONAllocationsPerMember(t *testing.T) {
	spellings := map[string]func(int) string{
		"reference": func(i int) string { return fmt.Sprintf(`{"@id":"e%d"}`, i) },
		"string":    func(i int) string { return fmt.Sprintf(`"s%d"`, i) },
		"decimal":   func(i int) string { return fmt.Sprintf(`%d.5`, i) },
		"boolean":   func(i int) string { return fmt.Sprint(i%2 == 0) },
	}
	for name, spell := range spellings {
		allocs := func(n int) float64 {
			items := make([]string, n)
			for i := range items {
				items[i] = spell(i)
			}
			text := "[" + strings.Join(items, ",") + "]"
			return testing.AllocsPerRun(5, func() {
				if _, err := ParseCollectionJSON(text); err != nil {
					t.Fatal(err)
				}
			})
		}
		if perMember := (allocs(400) - allocs(200)) / 200; perMember > 3 {
			t.Errorf("a %s member costs %.1f allocations; want at most 3", name, perMember)
		}
	}
}

func TestParsePlainMemberAgreesWithDecoder(t *testing.T) {
	for _, item := range []string{
		`{"@id":"e1"}`, `{"@id":""}`, `{"@id":"a\"b"}`, `{"@id":"a\u0041"}`, `{"@id":"x","y":1}`,
		`{ "@id":"e1"}`, `{"@id":"é"}`, `"plain"`, `""`, `"a\nb"`, `"\u00e9"`, `"é"`, "\"\xff\"",
		`true`, `false`, `null`, `0`, `-0`, `12`, `-3.25`, `1e5`, `1.5E-3`, `[1]`, `{}`,
	} {
		want, wantErr := decodeMember(json.RawMessage(item))
		got, ok := parsePlainMember(json.RawMessage(item))
		if !ok {
			continue
		}
		if wantErr != nil || got != want {
			t.Errorf("%s: plain read %+v, decoder %+v (err %v)", item, got, want, wantErr)
		}
	}
}
