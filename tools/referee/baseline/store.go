package baseline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	oracle "github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
)

// IndexFile is the name of the directory's index.
const IndexFile = "index.json"

// Fragment is one source file's share of a baseline: the referee's summary of
// the file and its rows, keyed by the row's name.
type Fragment[S, R any] struct {
	Root    string       `json:"root"`
	File    string       `json:"file"`
	Summary S            `json:"summary"`
	Rows    map[string]R `json:"rows"`
}

// Index is index.json: the provenance of the run that recorded the directory,
// the files recorded per root and the referee's aggregate over the fragments.
type Index[A any] struct {
	Provenance oracle.Record       `json:"provenance"`
	Files      map[string][]string `json:"files"`
	Summary    A                   `json:"summary"`
}

// Baseline is a whole baseline in memory, as a run produces it or Read loads it.
type Baseline[S, R, A any] struct {
	Provenance oracle.Record
	Summary    A
	Fragments  []Fragment[S, R]
}

// Fold aggregates the fragments into the referee's summary.
type Fold[S, R, A any] func([]Fragment[S, R]) A

// FragmentPath names the fragment of one source file: <dir>/<root>/<file>.json.
func FragmentPath(dir, root, file string) string {
	return filepath.Join(dir, root, filepath.FromSlash(file)+".json")
}

// locate validates a fragment's coordinates, which become a path under dir.
func locate(root, file string) error {
	for _, part := range []struct{ name, value string }{{"root", root}, {"file", file}} {
		if part.value == "" || strings.HasPrefix(part.value, "/") || slices.Contains(strings.Split(part.value, "/"), "..") {
			return fmt.Errorf("fragment %s %q is not a relative path", part.name, part.value)
		}
	}
	if strings.Contains(root, "/") {
		return fmt.Errorf("fragment root %q names a directory", root)
	}
	return nil
}

// sortFragments orders fragments by root then file and refuses two for one file.
func sortFragments[S, R any](fragments []Fragment[S, R]) error {
	for _, f := range fragments {
		if err := locate(f.Root, f.File); err != nil {
			return err
		}
	}
	sort.Slice(fragments, func(i, j int) bool {
		if fragments[i].Root != fragments[j].Root {
			return fragments[i].Root < fragments[j].Root
		}
		return fragments[i].File < fragments[j].File
	})
	for i := 1; i < len(fragments); i++ {
		if fragments[i].Root == fragments[i-1].Root && fragments[i].File == fragments[i-1].File {
			return fmt.Errorf("two fragments for %s/%s", fragments[i].Root, fragments[i].File)
		}
	}
	return nil
}

// filesOf lists the fragments' files per root, sorted.
func filesOf[S, R any](fragments []Fragment[S, R]) map[string][]string {
	files := make(map[string][]string)
	for _, f := range fragments {
		files[f.Root] = append(files[f.Root], f.File)
	}
	return files
}

// Write records a whole baseline under dir: every fragment, index.json, and
// nothing else — a fragment of a file the baseline no longer holds is removed.
func Write[S, R, A any](dir string, b Baseline[S, R, A], log io.Writer) error {
	if err := oracle.Validate(b.Provenance); err != nil {
		return fmt.Errorf("the fresh baseline's provenance %w", err)
	}
	if err := sortFragments(b.Fragments); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	kept := make(map[string]bool, len(b.Fragments)+1)
	kept[filepath.Join(dir, IndexFile)] = true
	for _, f := range b.Fragments {
		path := FragmentPath(dir, f.Root, f.File)
		kept[path] = true
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(path, encodeFragment(f), 0o600); err != nil {
			return err
		}
	}
	index := Index[A]{Provenance: b.Provenance, Files: filesOf(b.Fragments), Summary: b.Summary}
	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, IndexFile), append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	removed, err := removeOthers(dir, kept)
	if err != nil {
		return err
	}
	fmt.Fprintf(log, "recorded %s: %d fragment(s) in %d root(s), %d stale fragment(s) removed\n", dir, len(b.Fragments), len(index.Files), removed)
	return nil
}

// Rewrite re-records the replaced roots' fragments beside the other roots' committed ones
// and refolds the index (with nothing replaced, the index alone); vouches bounds the provenance.
func Rewrite[S, R, A any](dir string, replaced []string, fresh []Fragment[S, R], provenance oracle.Record, fold Fold[S, R, A], commands Commands, log io.Writer) error {
	for _, f := range fresh {
		if !slices.Contains(replaced, f.Root) {
			return fmt.Errorf("fragment %s/%s is not of a replaced root", f.Root, f.File)
		}
	}
	committed, err := read[S, R, A](dir, replaced)
	if err != nil {
		return err
	}
	if err := vouches(dir, committed.Provenance, provenance, replaced, commands); err != nil {
		return err
	}
	fragments := append(slices.Clone(fresh), committed.Fragments...)
	if err := sortFragments(fragments); err != nil {
		return err
	}
	return Write(dir, Baseline[S, R, A]{Provenance: provenance, Summary: fold(fragments), Fragments: fragments}, log)
}

// vouches refuses a provenance movement the replaced roots cannot account for: a moved
// pin, tool, errata digest or other input changes what every fragment would measure.
func vouches(dir string, recorded, current oracle.Record, replaced []string, commands Commands) error {
	var stale []oracle.Mismatch
	for _, m := range oracle.Compare(recorded, current) {
		if input, ok := inputOf(m.Field); ok && slices.Contains(replaced, input) {
			continue
		}
		stale = append(stale, m)
	}
	if len(stale) == 0 {
		return nil
	}
	scope := "the index alone"
	if len(replaced) > 0 {
		scope = "root(s) " + strings.Join(replaced, ", ")
	}
	explained := oracle.Explain(filepath.Join(dir, IndexFile), commands.Refresh, stale)
	return fmt.Errorf("re-recording %s cannot vouch for the moved provenance, which only a run over every root can:\n%s", scope, strings.TrimRight(explained, "\n"))
}

// inputOf names the input a provenance mismatch is about (inputs[<name>]...), if any.
func inputOf(field string) (string, bool) {
	rest, ok := strings.CutPrefix(field, "inputs[")
	if !ok {
		return "", false
	}
	name, _, ok := strings.Cut(rest, "]")
	return name, ok
}

// removeOthers deletes every .json file under dir that is not kept, then the
// directories that emptied, and counts the files removed.
func removeOthers(dir string, kept map[string]bool) (int, error) {
	var stale, dirs []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			if path != dir {
				dirs = append(dirs, path)
			}
		case strings.HasSuffix(path, ".json") && !kept[path]:
			stale = append(stale, path)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return 0, err
		}
	}
	// Deepest first, so a directory empties before its parent is tried.
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, path := range dirs {
		entries, err := os.ReadDir(path)
		if err != nil {
			return 0, err
		}
		if len(entries) == 0 {
			if err := os.Remove(path); err != nil {
				return 0, err
			}
		}
	}
	return len(stale), nil
}

// encodeFragment writes a fragment with the summary indented and one row per
// line in key order, the shape a line-based merge resolves row by row.
func encodeFragment[S, R any](f Fragment[S, R]) []byte {
	var b bytes.Buffer
	b.WriteString("{\n  \"root\": ")
	b.Write(mustMarshal(f.Root))
	b.WriteString(",\n  \"file\": ")
	b.Write(mustMarshal(f.File))
	b.WriteString(",\n  \"summary\": ")
	summary, err := json.MarshalIndent(f.Summary, "  ", "  ")
	if err != nil {
		panic(err)
	}
	b.Write(summary)
	b.WriteString(",\n  \"rows\": {")
	keys := make([]string, 0, len(f.Rows))
	for key := range f.Rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    ")
		b.Write(mustMarshal(key))
		b.WriteString(": ")
		b.Write(mustMarshal(f.Rows[key]))
	}
	if len(keys) > 0 {
		b.WriteString("\n  ")
	}
	b.WriteString("}\n}\n")
	return b.Bytes()
}

// mustMarshal is json.Marshal of a value a referee's own types produced.
func mustMarshal(v any) []byte {
	encoded, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return encoded
}

// Read loads index.json and every fragment it lists, each checked to be the file
// it is filed under, and refuses a directory holding a fragment the index does not list.
func Read[S, R, A any](dir string) (Baseline[S, R, A], error) {
	return read[S, R, A](dir, nil)
}

// read is Read less the replaced roots, which are about to be re-recorded whole: their
// listed fragments are skipped and their directories may hold fragments the index does
// not list (two branches that each added a file of one root merge that way).
func read[S, R, A any](dir string, replaced []string) (Baseline[S, R, A], error) {
	var out Baseline[S, R, A]
	index, err := readIndex[A](dir)
	if err != nil {
		return out, err
	}
	out.Provenance, out.Summary = index.Provenance, index.Summary
	listed := make(map[string]bool)
	for _, root := range sortedRoots(index.Files) {
		if slices.Contains(replaced, root) {
			continue
		}
		for _, file := range index.Files[root] {
			path := FragmentPath(dir, root, file)
			listed[path] = true
			f, err := readFragment[S, R](path)
			if err != nil {
				return out, err
			}
			if f.Root != root || f.File != file {
				return out, fmt.Errorf("%s records %s/%s, not the file it is filed under", path, f.Root, f.File)
			}
			out.Fragments = append(out.Fragments, f)
		}
	}
	unlisted, err := fragmentFiles(dir)
	if err != nil {
		return out, err
	}
	for _, path := range unlisted {
		if !listed[path] && !slices.Contains(replaced, rootOf(dir, path)) {
			return out, fmt.Errorf("%s is not listed in %s: a fragment of a file the baseline no longer compares; remove it or re-record", path, filepath.Join(dir, IndexFile))
		}
	}
	return out, nil
}

// rootOf is the root directory a path under dir is filed in.
func rootOf(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return ""
	}
	root, _, _ := strings.Cut(rel, string(filepath.Separator))
	return root
}

func readIndex[A any](dir string) (Index[A], error) {
	var index Index[A]
	path := filepath.Join(dir, IndexFile)
	content, err := os.ReadFile(path) // #nosec G304 -- callers name a committed baseline in this repository
	if err != nil {
		return index, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(content, &index); err != nil {
		return index, fmt.Errorf("parse %s: %w", path, err)
	}
	return index, nil
}

func readFragment[S, R any](path string) (Fragment[S, R], error) {
	var f Fragment[S, R]
	content, err := os.ReadFile(path) // #nosec G304 -- a fragment index.json lists under the baseline directory
	if errors.Is(err, fs.ErrNotExist) {
		return f, fmt.Errorf("%s is listed in index.json but missing: a fragment of a file the baseline compares; restore it or re-record", path)
	}
	if err != nil {
		return f, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(content, &f); err != nil {
		return f, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, nil
}

// fragmentFiles lists every .json file under dir other than the index, sorted.
func fragmentFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".json") && path != filepath.Join(dir, IndexFile) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

func sortedRoots(files map[string][]string) []string {
	roots := make([]string, 0, len(files))
	for root := range files {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}
