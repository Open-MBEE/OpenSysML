package apijson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMatchMovementsNameEveryMovedKey checks that a key matched in one run and
// not the other is listed once, under the right side, with its fate in the run
// that does not match it.
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

// TestCompareRootRecordsTheInventory checks that the comparison records each
// key of a root under the fate it gave it, and that the inventory round-trips
// through the file -lost-against reads.
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
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
