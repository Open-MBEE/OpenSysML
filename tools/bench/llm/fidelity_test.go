package llm

import (
	"context"
	"math"
	"strings"
	"testing"
)

func TestExtractStructure(t *testing.T) {
	apiJSON := []byte(`[
	 {"@type":"Package","@id":"p","qualifiedName":"P"},
	 {"@type":"PartDefinition","@id":"a","qualifiedName":"P::A","owner":{"@id":"p"}},
	 {"@type":"PartUsage","@id":"u","qualifiedName":"P::A::b","owner":{"@id":"a"}},
	 {"@type":"PartUsage","@id":"anon","owner":{"@id":"a"}},
	 {"@type":"FeatureTyping","@id":"ft","source":[{"@id":"u"}],"target":[{"@id":"a"}]},
	 {"@type":"OwningMembership","@id":"om","source":[{"@id":"p"}],"target":[{"@id":"a"}]},
	 {"@type":"Specialization","@id":"sp","qualifiedName":"P::A::spec","source":[{"@id":"u"}],"target":[{"@id":"a"}]}
	]`)
	s, err := ExtractStructure(apiJSON)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for f := range s.Facts() {
		lines = append(lines, f)
	}
	facts := strings.Join(lines, "\n")
	for _, want := range []string{"PartDefinition P::A", "PartUsage P::A::b", "FeatureTyping P::A::b -> P::A", "Specialization P::A::spec", "Specialization P::A::b -> P::A"} {
		if !strings.Contains(facts, want) {
			t.Errorf("facts lack %q:\n%s", want, facts)
		}
	}
	if strings.Contains(facts, "OwningMembership") || strings.Contains(facts, "anon") {
		t.Errorf("facts keep memberships or anonymous elements:\n%s", facts)
	}
	if got := Jaccard(s, s); got != 1 {
		t.Errorf("Jaccard(s, s) = %v", got)
	}
	empty := Structure{}
	if got := Jaccard(empty, empty); got != 1 {
		t.Errorf("Jaccard(empty, empty) = %v", got)
	}
	if got := Jaccard(s, empty); got != 0 {
		t.Errorf("Jaccard(s, empty) = %v", got)
	}
}

// TestFidelityOrdersRealModels checks the score over the real export: a gold
// scores 1 against itself, a model missing one part scores strictly less and a
// model about something else scores near 0.
func TestFidelityOrdersRealModels(t *testing.T) {
	tc := testToolchain(t)
	corpus, err := LoadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	structure := func(src string) Structure {
		t.Helper()
		data, err := tc.Export(ctx, src)
		if err != nil {
			t.Fatal(err)
		}
		s, err := ExtractStructure(data)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	tasks, err := corpus.Select([]string{"S01", "B01"})
	if err != nil {
		t.Fatal(err)
	}
	s01, _ := corpus.GoldSource(tasks[0])
	b01, _ := corpus.GoldSource(tasks[1])
	gold := structure(s01)
	if len(gold.Elements) < 5 || len(gold.Relationships) < 1 {
		t.Fatalf("S01 export yields %d elements and %d relationships", len(gold.Elements), len(gold.Relationships))
	}
	trimmed := structure(strings.Replace(s01, "part saddle : Saddle;", "", 1))
	other := structure(b01)
	self, partial, unrelated := Jaccard(gold, gold), Jaccard(gold, trimmed), Jaccard(gold, other)
	if self != 1 || !(partial < 1 && partial > 0.5) || unrelated > 0.2 || math.IsNaN(partial) {
		t.Fatalf("self=%v partial=%v unrelated=%v", self, partial, unrelated)
	}
}
