package apijson

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	reports "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

// Matches is one run's inventory per root of which keys matched and which were
// left on one side, written beside the report for a later run's -lost-against.
type Matches struct {
	Roots map[string]*RootMatches `json:"roots"`
}

// RootMatches maps each key of one root to its metaclass (the pilot's for a
// matched pair) under the fate the comparison gave it.
type RootMatches struct {
	Matched   map[string]string `json:"matched"`
	OursOnly  map[string]string `json:"oursOnly"`
	PilotOnly map[string]string `json:"pilotOnly"`
}

func newRootMatches() *RootMatches {
	return &RootMatches{Matched: map[string]string{}, OursOnly: map[string]string{}, PilotOnly: map[string]string{}}
}

// fate names the side a key is on in this run, or "absent" when neither side has it.
func (r *RootMatches) fate(key string) string {
	switch {
	case r == nil:
		return "absent"
	case r.Matched[key] != "":
		return "matched"
	case r.OursOnly[key] != "" && r.PilotOnly[key] != "":
		return "both-unmatched"
	case r.OursOnly[key] != "":
		return "ours-only"
	case r.PilotOnly[key] != "":
		return "pilot-only"
	}
	return "absent"
}

func readMatches(path string) (*Matches, error) {
	// #nosec G304 -- path is the -lost-against flag naming an earlier run's inventory.
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var matches Matches
	if err := json.Unmarshal(content, &matches); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &matches, nil
}

// Movement is one key matched in exactly one of two runs (lost when only the
// earlier run matched it, else gained); Fate is where the other run has it.
type Movement struct {
	Root      string `json:"root"`
	Key       string `json:"key"`
	Metaclass string `json:"metaclass"`
	Fate      string `json:"fate"`
}

// MatchMovements lists the matches after lost against before, and gained since
// it, per root; roots only one run compared are left out, as nothing moved there.
func MatchMovements(before, after *Matches) (lost, gained []Movement) {
	for root, now := range after.Roots {
		then := before.Roots[root]
		if then == nil {
			continue
		}
		for key, typ := range then.Matched {
			if now.Matched[key] == "" {
				lost = append(lost, Movement{Root: root, Key: key, Metaclass: typ, Fate: now.fate(key)})
			}
		}
		for key, typ := range now.Matched {
			if then.Matched[key] == "" {
				gained = append(gained, Movement{Root: root, Key: key, Metaclass: typ, Fate: then.fate(key)})
			}
		}
	}
	sortMovements(lost)
	sortMovements(gained)
	return lost, gained
}

func sortMovements(list []Movement) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Root != list[j].Root {
			return list[i].Root < list[j].Root
		}
		if list[i].Metaclass != list[j].Metaclass {
			return list[i].Metaclass < list[j].Metaclass
		}
		return list[i].Key < list[j].Key
	})
}

// renderMovements writes the lost and gained matches per root, each list
// grouped by metaclass with the key's fate in the run that does not match it.
func renderMovements(lost, gained []Movement) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Matches lost: %d, gained: %d\n", len(lost), len(gained))
	for _, section := range []struct {
		title string
		list  []Movement
	}{{"Lost (matched before, not now; fate is where the key is now)", lost}, {"Gained (matched now, not before; fate is where the key was before)", gained}} {
		fmt.Fprintf(&b, "\n== %s: %d\n", section.title, len(section.list))
		sizes := make(map[string]int)
		for _, m := range section.list {
			sizes[m.Root+" / "+m.Metaclass]++
		}
		group := ""
		for _, m := range section.list {
			if g := m.Root + " / " + m.Metaclass; g != group {
				group = g
				fmt.Fprintf(&b, "\n-- %s (%d)\n", g, sizes[g])
			}
			fmt.Fprintf(&b, "%-15s %s\n", m.Fate, m.Key)
		}
	}
	return b.String()
}

// writeMatches records this run's inventory and, under -lost-against, the movements
// since the earlier run's, which it reads first as it may be the file overwritten.
func writeMatches(opts options, matches Matches) error {
	var before *Matches
	if opts.lostAgainst != "" {
		var err error
		if before, err = readMatches(opts.lostAgainst); err != nil {
			return err
		}
	}
	files, err := reports.Open(opts.out, toolName+"-matches")
	if err != nil {
		return err
	}
	if _, err := files.JSON(matches); err != nil {
		return err
	}
	fmt.Fprintf(opts.log, "  %s\n", files.Path("json"))
	movements, err := reports.Open(opts.out, toolName+"-movements")
	if err != nil {
		return err
	}
	if before == nil {
		return removeStale(movements.Path("json"), movements.Path("txt"))
	}
	lost, gained := MatchMovements(before, &matches)
	if _, err := movements.JSON(struct {
		Against string     `json:"against"`
		Lost    []Movement `json:"lost"`
		Gained  []Movement `json:"gained"`
	}{opts.lostAgainst, lost, gained}); err != nil {
		return err
	}
	if err := movements.Text(renderMovements(lost, gained)); err != nil {
		return err
	}
	fmt.Fprintf(opts.log, "  %s\n  %s\nmatches lost against %s: %d, gained: %d\n", movements.Path("json"), movements.Path("txt"), opts.lostAgainst, len(lost), len(gained))
	return nil
}

// removeStale deletes an earlier run's movement files, so a run without
// -lost-against leaves no movements beside its reports.
func removeStale(paths ...string) error {
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
