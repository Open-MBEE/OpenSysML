package apijson

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	reports "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

// Matches is one run's element inventory by key: which keys matched and which
// were left on one side, per root. The run writes it beside the report so a
// later run can name, with -lost-against, exactly which matches it lost or gained.
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

// Movement is one key that matched in exactly one of two runs: Lost when the
// earlier run matched it and this one does not, Gained the other way round.
// Fate is where the key is in the run that does not match it.
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
		group := ""
		for _, m := range section.list {
			if g := m.Root + " / " + m.Metaclass; g != group {
				group = g
				fmt.Fprintf(&b, "\n-- %s (%d)\n", g, countGroup(section.list, m.Root, m.Metaclass))
			}
			fmt.Fprintf(&b, "%-15s %s\n", m.Fate, m.Key)
		}
	}
	return b.String()
}

func countGroup(list []Movement, root, metaclass string) int {
	n := 0
	for _, m := range list {
		if m.Root == root && m.Metaclass == metaclass {
			n++
		}
	}
	return n
}

// writeMatches records this run's inventory and, under -lost-against, the
// movements from the earlier run's as <toolName>-movements.txt and .json.
func writeMatches(opts options, matches Matches) error {
	files, err := reports.Open(opts.out, toolName+"-matches")
	if err != nil {
		return err
	}
	if _, err := files.JSON(matches); err != nil {
		return err
	}
	fmt.Fprintf(opts.log, "  %s\n", files.Path("json"))
	if opts.lostAgainst == "" {
		return nil
	}
	before, err := readMatches(opts.lostAgainst)
	if err != nil {
		return err
	}
	lost, gained := MatchMovements(before, &matches)
	movements, err := reports.Open(opts.out, toolName+"-movements")
	if err != nil {
		return err
	}
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
