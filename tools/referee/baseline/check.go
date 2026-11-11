package baseline

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	oracle "github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
)

// Commands names the referee's own ways of mending the directory, for Check's advice.
type Commands struct {
	// Refresh re-records the whole baseline with the reference toolchain.
	Refresh string
	// Reindex rebuilds index.json from the committed fragments without it.
	Reindex string
}

// maxDifferences bounds a report so that a wholesale movement stays readable.
const maxDifferences = 40

// Check holds a fresh run to the committed directory: the provenance first, then every
// missing, orphaned or moved fragment (rows and summary), and last the index figures.
func Check[S, R, A any](dir string, fresh Baseline[S, R, A], commands Commands) error {
	indexPath := filepath.Join(dir, IndexFile)
	index, err := readIndex[A](dir)
	if err != nil {
		return err
	}
	if err := oracle.Validate(index.Provenance); err != nil {
		return fmt.Errorf("%s %v; re-record with %s", indexPath, err, commands.Refresh)
	}
	if mismatches := oracle.Compare(index.Provenance, fresh.Provenance); len(mismatches) > 0 {
		return errors.New(oracle.Explain(indexPath, commands.Refresh, mismatches))
	}
	if err := sortFragments(fresh.Fragments); err != nil {
		return err
	}

	var differences []string
	recorded := make(map[string]bool)
	for root, files := range index.Files {
		for _, file := range files {
			recorded[root+"/"+file] = true
		}
	}
	compared := make(map[string]bool, len(fresh.Fragments))
	for _, f := range fresh.Fragments {
		key := f.Root + "/" + f.File
		compared[key] = true
		path := FragmentPath(dir, f.Root, f.File)
		if !recorded[key] {
			differences = append(differences, fmt.Sprintf("%s: no fragment recorded for a file the run compares", path))
			continue
		}
		committed, err := readFragment[json.RawMessage, json.RawMessage](path)
		if err != nil {
			return err
		}
		differences = append(differences, fragmentDifferences(path, committed, f)...)
	}
	for _, root := range sortedRoots(index.Files) {
		for _, file := range index.Files[root] {
			if !compared[root+"/"+file] {
				differences = append(differences, fmt.Sprintf("%s: records a file the run no longer compares", FragmentPath(dir, root, file)))
			}
		}
	}
	onDisk, err := fragmentFiles(dir)
	if err != nil {
		return err
	}
	listed := make(map[string]bool)
	for root, files := range index.Files {
		for _, file := range files {
			listed[FragmentPath(dir, root, file)] = true
		}
	}
	for _, path := range onDisk {
		if !listed[path] {
			differences = append(differences, fmt.Sprintf("%s: a fragment %s does not list", path, indexPath))
		}
	}
	fragmentsMoved := len(differences) > 0

	for _, d := range summaryDifferences(index.Summary, fresh.Summary) {
		differences = append(differences, fmt.Sprintf("%s: summary%s", indexPath, d))
	}
	if len(differences) == 0 {
		return nil
	}
	advice := "implementation movement: adjudicate each line in the referee's record, then re-record with " + commands.Refresh
	if !fragmentsMoved {
		advice = "every fragment reproduces and only the index figures are stale: rebuild them with " + commands.Reindex
	}
	return fmt.Errorf("%s does not reproduce (%s):\n  %s", dir, advice, strings.Join(bound(differences), "\n  "))
}

// CheckRoots holds a run over the named roots only: their fragments are the run's, the
// other roots' the committed ones, and the index is held to the fold of that whole.
func CheckRoots[S, R, A any](dir string, fresh []Fragment[S, R], provenance oracle.Record, roots []string, fold Fold[S, R, A], commands Commands) error {
	named := make(map[string]bool, len(roots))
	for _, root := range roots {
		named[root] = true
	}
	for _, f := range fresh {
		if !named[f.Root] {
			return fmt.Errorf("fragment %s/%s is of a root the run did not name", f.Root, f.File)
		}
	}
	committed, err := Read[S, R, A](dir)
	if err != nil {
		return err
	}
	merged := make([]Fragment[S, R], 0, len(committed.Fragments)+len(fresh))
	for _, f := range committed.Fragments {
		if !named[f.Root] {
			merged = append(merged, f)
		}
	}
	merged = append(merged, fresh...)
	return Check(dir, Baseline[S, R, A]{Provenance: provenance, Summary: fold(merged), Fragments: merged}, commands)
}

// Verify holds a committed directory to itself without the reference toolchain:
// every fragment listed, present and filed under its own file, the index their fold.
func Verify[S, R, A any](dir string, fold Fold[S, R, A]) error {
	committed, err := Read[S, R, A](dir)
	if err != nil {
		return err
	}
	differences := summaryDifferences(committed.Summary, fold(committed.Fragments))
	if len(differences) == 0 {
		return nil
	}
	for i, d := range differences {
		differences[i] = "summary" + d
	}
	return fmt.Errorf("%s does not sum its fragments (recorded -> folded):\n  %s", filepath.Join(dir, IndexFile), strings.Join(bound(differences), "\n  "))
}

// fragmentDifferences names every row of one file that moved, appeared or
// vanished, and every per-file summary figure that moved.
func fragmentDifferences[S, R any](path string, committed Fragment[json.RawMessage, json.RawMessage], fresh Fragment[S, R]) []string {
	var out []string
	for _, d := range summaryDifferences(committed.Summary, fresh.Summary) {
		out = append(out, fmt.Sprintf("%s: summary%s", path, d))
	}
	keys := make(map[string]bool, len(committed.Rows)+len(fresh.Rows))
	for key := range committed.Rows {
		keys[key] = true
	}
	for key := range fresh.Rows {
		keys[key] = true
	}
	sorted := make([]string, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)
	for _, key := range sorted {
		was, recorded := committed.Rows[key]
		now, compared := fresh.Rows[key]
		switch {
		case !recorded:
			out = append(out, fmt.Sprintf("%s: row %s added: %s", path, key, mustMarshal(now)))
		case !compared:
			out = append(out, fmt.Sprintf("%s: row %s removed: was %s", path, key, compact(was)))
		default:
			wasGeneric, nowGeneric := generic(was), generic(mustMarshal(now))
			if !reflect.DeepEqual(wasGeneric, nowGeneric) {
				out = append(out, fmt.Sprintf("%s: row %s moved: %s -> %s", path, key, compact(was), mustMarshal(now)))
			}
		}
	}
	return out
}

// summaryDifferences compares two summaries through their JSON form, so that a
// committed document and a referee's own type meet on equal terms.
func summaryDifferences(was, now any) []string {
	differences := oracle.Differences(generic(mustMarshal(was)), generic(mustMarshal(now)))
	for i, d := range differences {
		differences[i] = "." + strings.TrimLeft(d, " ")
	}
	return differences
}

// generic decodes JSON into the plain maps, slices and numbers reflect compares.
func generic(encoded []byte) any {
	if len(encoded) == 0 {
		return nil
	}
	var out any
	if err := json.Unmarshal(encoded, &out); err != nil {
		panic(err)
	}
	return out
}

func compact(raw json.RawMessage) string {
	return string(mustMarshal(generic(raw)))
}

func bound(differences []string) []string {
	if len(differences) <= maxDifferences {
		return differences
	}
	return append(differences[:maxDifferences:maxDifferences], fmt.Sprintf("... and %d more", len(differences)-maxDifferences))
}
