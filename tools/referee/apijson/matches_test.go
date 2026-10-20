package apijson

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMatchMovementsNameEveryMovedKey checks that a key matched in only one run
// is listed once, under the right side, with its fate in the other run.
func TestMatchMovementsNameEveryMovedKey(t *testing.T) {
	before := &Matches{Roots: map[string]*RootMatches{"probe": {
		Matched:   map[string]string{"P::a": "PartUsage", "P::b": "PartUsage", "P::c": "ActionUsage"},
		OursOnly:  map[string]string{"P::@2": "PartUsage"},
		PilotOnly: map[string]string{"P::d": "ItemUsage"},
	}}}
	after := &Matches{Roots: map[string]*RootMatches{
		"probe": {
			Matched:   map[string]string{"P::a": "PartUsage", "P::d": "ItemUsage"},
			OursOnly:  map[string]string{"P::b": "PartUsage", "P::c": "ActionUsage"},
			PilotOnly: map[string]string{"P::c": "ActionUsage"},
		},
		"other": {Matched: map[string]string{"Q::x": "PartUsage"}},
	}}
	lost, gained := MatchMovements(before, after)
	want := []Movement{
		{Root: "probe", Key: "P::c", Metaclass: "ActionUsage", Fate: "both-unmatched"},
		{Root: "probe", Key: "P::b", Metaclass: "PartUsage", Fate: "ours-only"},
	}
	if got, want := jsonOf(t, lost), jsonOf(t, want); got != want {
		t.Errorf("lost = %s\nwant %s", got, want)
	}
	want = []Movement{{Root: "probe", Key: "P::d", Metaclass: "ItemUsage", Fate: "pilot-only"}}
	if got, want := jsonOf(t, gained), jsonOf(t, want); got != want {
		t.Errorf("gained = %s\nwant %s", got, want)
	}
	text := renderMovements(lost, gained)
	for _, line := range []string{"Matches lost: 2, gained: 1", "-- probe / PartUsage (1)", "ours-only       P::b", "pilot-only      P::d"} {
		if !strings.Contains(text, line) {
			t.Errorf("rendering lacks %q:\n%s", line, text)
		}
	}
	if (&RootMatches{}).fate("P::z") != "absent" || (*RootMatches)(nil).fate("P::z") != "absent" {
		t.Error("a key neither side has is not reported absent")
	}
}

// TestCompareRootRecordsTheInventory checks that each key of a root is recorded
// under its fate and that the inventory round-trips through the -lost-against file.
func TestCompareRootRecordsTheInventory(t *testing.T) {
	ours, document := probeSides(t)
	c := newComparison()
	elements, _ := c.compareRoot("probe", ours.Elements, pilotElements(t, document))
	inventory := c.matches["probe"]
	if inventory == nil {
		t.Fatal("no inventory recorded for the probe root")
	}
	if len(inventory.Matched) != elements.Matched || len(inventory.OursOnly) != elements.OursOnly || len(inventory.PilotOnly) != elements.PilotOnly {
		t.Fatalf("inventory sizes %d/%d/%d do not match the totals %d/%d/%d",
			len(inventory.Matched), len(inventory.OursOnly), len(inventory.PilotOnly), elements.Matched, elements.OursOnly, elements.PilotOnly)
	}
	if elements.Matched == 0 {
		t.Fatal("the probe matched nothing")
	}
	path := filepath.Join(t.TempDir(), "matches.json")
	encoded, err := json.Marshal(Matches{Roots: c.matches})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := readMatches(path)
	if err != nil {
		t.Fatal(err)
	}
	lost, gained := MatchMovements(read, &Matches{Roots: c.matches})
	if len(lost)+len(gained) != 0 {
		t.Fatalf("a run against its own inventory moved %d/%d keys", len(lost), len(gained))
	}
	if _, err := readMatches(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("a missing inventory file was read without error")
	}
	for _, content := range []string{"{}", "null", `{"roots":null}`, `{"roots":{"r":null}}`, `[]`} {
		path := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readMatches(path); err == nil {
			t.Errorf("%s was accepted as an inventory", content)
		}
	}
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestWriteMatchesReadsTheEarlierInventoryFirst(t *testing.T) {
	out := t.TempDir()
	inventory := filepath.Join(out, toolName+"-matches.json")
	first := Matches{Roots: map[string]*RootMatches{"r": {Matched: map[string]string{"P::a": "PartUsage", "P::b": "PartUsage"}}}}
	second := Matches{Roots: map[string]*RootMatches{"r": {Matched: map[string]string{"P::a": "PartUsage"}, PilotOnly: map[string]string{"P::b": "PartUsage"}}}}
	if err := writeMatches(options{out: out, log: io.Discard}, first); err != nil {
		t.Fatal(err)
	}
	if err := writeMatches(options{out: out, log: io.Discard, lostAgainst: inventory}, second); err != nil {
		t.Fatal(err)
	}
	var movements struct {
		Against string
		Lost    []Movement
		Gained  []Movement
	}
	encoded, err := os.ReadFile(filepath.Join(out, toolName+"-movements.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &movements); err != nil {
		t.Fatal(err)
	}
	if movements.Against != inventory || len(movements.Lost) != 1 || movements.Lost[0].Key != "P::b" || movements.Lost[0].Fate != "pilot-only" || len(movements.Gained) != 0 {
		t.Fatalf("a run compared against the inventory it overwrites lost %v, gained %v", movements.Lost, movements.Gained)
	}
	now, err := readMatches(inventory)
	if err != nil {
		t.Fatal(err)
	}
	if len(now.Roots["r"].Matched) != 1 {
		t.Fatalf("the inventory left behind is not this run's: %v", now.Roots["r"].Matched)
	}
	if err := writeMatches(options{out: out, log: io.Discard}, second); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"json", "txt"} {
		if _, err := os.Stat(filepath.Join(out, toolName+"-movements."+ext)); !os.IsNotExist(err) {
			t.Fatalf("a run without -lost-against left %s-movements.%s behind (%v)", toolName, ext, err)
		}
	}
}
