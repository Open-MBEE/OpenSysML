package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// maxDifferences bounds the printed diff: a wholly different baseline would
// otherwise bury the first, usually sufficient, line.
const maxDifferences = 40

// Write records a fresh report as the committed baseline. The oracle stamps the
// recording date itself, so the bytes written here are the run's own.
func Write(committedPath string, fresh []byte) error {
	var record struct {
		Provenance Record `json:"provenance"`
	}
	if err := json.Unmarshal(fresh, &record); err != nil {
		return fmt.Errorf("parse the fresh report: %w", err)
	}
	if err := Validate(record.Provenance); err != nil {
		return fmt.Errorf("the fresh report's provenance %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(committedPath), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(committedPath, fresh, 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "recorded %s (provenance dated %s)\n", committedPath, record.Provenance.Recorded)
	return nil
}

// CheckCommitted is the Java-free guard: it reads a committed baseline's
// provenance and reports how it differs from the repository as it stands, naming
// the command that re-records it.
func CheckCommitted(committedPath, refresh string, current Record) error {
	content, err := os.ReadFile(committedPath) // #nosec G304 -- callers name a committed baseline in this repository
	if err != nil {
		return fmt.Errorf("read %s: %w", committedPath, err)
	}
	var recorded struct {
		Provenance Record `json:"provenance"`
	}
	if err := json.Unmarshal(content, &recorded); err != nil {
		return fmt.Errorf("parse %s: %w", committedPath, err)
	}
	if err := Validate(recorded.Provenance); err != nil {
		return fmt.Errorf("%s %w; re-record it with: %s", committedPath, err, refresh)
	}
	if mismatches := Compare(recorded.Provenance, current); len(mismatches) > 0 {
		return fmt.Errorf("%s", Explain(committedPath, refresh, mismatches))
	}
	return nil
}

// Reproduces reports whether a fresh report still matches the committed
// baseline. The recording date is excluded: it states when the baseline was
// committed, not what was measured.
func Reproduces(committedPath string, fresh []byte) error {
	committed, err := os.ReadFile(committedPath) // #nosec G304 -- callers name a committed baseline in this repository
	if err != nil {
		return fmt.Errorf("read %s: %w", committedPath, err)
	}
	was, err := decodeForComparison(committed)
	if err != nil {
		return fmt.Errorf("%s: %w", committedPath, err)
	}
	now, err := decodeForComparison(fresh)
	if err != nil {
		return fmt.Errorf("the fresh report: %w", err)
	}
	differences := Differences(was, now)
	if len(differences) == 0 {
		return nil
	}
	return fmt.Errorf("%s does not reproduce:\n%s%s", committedPath,
		strings.Join(differences, "\n"), classify(differences))
}

// classify names the action the differences call for: a moved reference is a
// provisioning defect, a moved count is a movement to adjudicate.
func classify(differences []string) string {
	provenance := false
	for _, line := range differences {
		if strings.HasPrefix(strings.TrimSpace(line), "provenance.") {
			provenance = true
		}
	}
	if provenance {
		return "\n\nthe run's provenance differs from the baseline's: the pinned reference or a compared" +
			" input changed underneath this baseline. Investigate the provisioning before re-recording."
	}
	return "\n\nthe provenance matches, so this is an implementation movement: adjudicate it, then" +
		" re-record the baseline with the oracle's -update flag."
}

func decodeForComparison(content []byte) (any, error) {
	var report any
	if err := json.Unmarshal(content, &report); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if top, ok := report.(map[string]any); ok {
		if provenance, ok := top["provenance"].(map[string]any); ok {
			delete(provenance, "recorded")
		}
	}
	return report, nil
}

// Differences reports where two decoded reports differ, as field paths with
// their values, deepest-first per member so the first line names a real leaf.
func Differences(was, now any) []string {
	var out []string
	walk("", was, now, &out)
	if len(out) > maxDifferences {
		out = append(out[:maxDifferences], fmt.Sprintf("  ... and %d more difference(s)", len(out)-maxDifferences))
	}
	return out
}

func walk(path string, was, now any, out *[]string) {
	if len(*out) > maxDifferences {
		return
	}
	switch left := was.(type) {
	case map[string]any:
		right, ok := now.(map[string]any)
		if !ok {
			break
		}
		for _, key := range sortedKeys(left, right) {
			lv, lok := left[key]
			rv, rok := right[key]
			switch {
			case !lok:
				*out = append(*out, fmt.Sprintf("  %s: absent in the baseline, now %s", join(path, key), render(rv)))
			case !rok:
				*out = append(*out, fmt.Sprintf("  %s: %s in the baseline, now absent", join(path, key), render(lv)))
			default:
				walk(join(path, key), lv, rv, out)
			}
		}
		return
	case []any:
		right, ok := now.([]any)
		if !ok {
			break
		}
		if len(left) != len(right) {
			*out = append(*out, fmt.Sprintf("  %s: %d entr(y|ies) in the baseline, now %d", path, len(left), len(right)))
			return
		}
		for i := range left {
			walk(fmt.Sprintf("%s[%d]", path, i), left[i], right[i], out)
		}
		return
	}
	if !reflect.DeepEqual(was, now) {
		*out = append(*out, fmt.Sprintf("  %s: %s -> %s", path, render(was), render(now)))
	}
}

func sortedKeys(left, right map[string]any) []string {
	seen := make(map[string]bool, len(left)+len(right))
	keys := make([]string, 0, len(left)+len(right))
	for _, m := range []map[string]any{left, right} {
		for key := range m {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func render(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return fmt.Sprintf("%q", typed)
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprint(int64(typed))
		}
		return fmt.Sprint(typed)
	case map[string]any:
		return fmt.Sprintf("an object of %d member(s)", len(typed))
	case []any:
		return fmt.Sprintf("a list of %d entr(y|ies)", len(typed))
	default:
		return fmt.Sprint(typed)
	}
}
