// Package corpussubset answers, for the tests that sweep every model under
// examples/, whether OPENSYSML_CORPUS_SUBSET makes this run iterate a file.
package corpussubset

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EnvVar selects the subset of examples/ a sweep iterates.
const EnvVar = "OPENSYSML_CORPUS_SUBSET"

// ConstructCover names the committed subset that covers every syntactic
// construct the whole corpus exhibits, recorded by TestConstructCover.
const ConstructCover = "construct-cover"

// ListPath is the subset list, relative to the repository root: one sorted
// examples/-relative path per line, # lines being comments.
const ListPath = "tests/corpus/testdata/construct_cover.txt"

// Subset is the set of examples/ files a sweep iterates.
type Subset struct {
	name  string
	files map[string]bool
}

// Load reads OPENSYSML_CORPUS_SUBSET and, for the construct-cover subset, the
// committed list under repoRoot. An unset variable selects the whole corpus.
func Load(repoRoot string) (Subset, error) {
	name := os.Getenv(EnvVar)
	switch name {
	case "":
		return Subset{}, nil
	case ConstructCover:
		files, err := ReadList(filepath.Join(repoRoot, filepath.FromSlash(ListPath)))
		if err != nil {
			return Subset{}, fmt.Errorf("%s=%s: %w", EnvVar, name, err)
		}
		set := make(map[string]bool, len(files))
		for _, f := range files {
			set[f] = true
		}
		return Subset{name: name, files: set}, nil
	default:
		return Subset{}, fmt.Errorf("%s=%q: unknown subset; unset it for the whole corpus or set %q", EnvVar, name, ConstructCover)
	}
}

// Fataler is the part of testing.TB that Must needs.
type Fataler interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Must is Load for a test, failing it when the variable is unusable.
func Must(tb Fataler, repoRoot string) Subset {
	tb.Helper()
	s, err := Load(repoRoot)
	if err != nil {
		tb.Fatalf("%v", err)
	}
	return s
}

// Whole reports whether every file of the corpus is iterated.
func (s Subset) Whole() bool { return s.name == "" }

// Name is the value of OPENSYSML_CORPUS_SUBSET this subset was loaded from.
func (s Subset) Name() string { return s.name }

// Wants reports whether the examples/-relative, slash-separated path rel is
// iterated. Under the whole corpus every path is.
func (s Subset) Wants(rel string) bool {
	return s.Whole() || s.files[filepath.ToSlash(rel)]
}

// WantsPath is Wants for a path under examplesDir, however either is spelt.
// A path outside examplesDir is not part of the corpus and is always iterated.
func (s Subset) WantsPath(examplesDir, path string) bool {
	if s.Whole() {
		return true
	}
	rel, err := filepath.Rel(examplesDir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	return s.files[filepath.ToSlash(rel)]
}

// WantsAny reports whether any of paths is iterated: a sweep whose unit is a
// directory of models iterates the whole directory when one of its files is listed.
func (s Subset) WantsAny(examplesDir string, paths []string) bool {
	for _, p := range paths {
		if s.WantsPath(examplesDir, p) {
			return true
		}
	}
	return s.Whole()
}

// Filter keeps the examples/-relative paths of rel this subset iterates, in order.
func (s Subset) Filter(rel []string) []string {
	if s.Whole() {
		return rel
	}
	kept := make([]string, 0, len(s.files))
	for _, r := range rel {
		if s.Wants(r) {
			kept = append(kept, r)
		}
	}
	return kept
}

// FilterPaths keeps the paths this subset iterates, in order: those under
// examplesDir that are listed, and every path outside it.
func (s Subset) FilterPaths(examplesDir string, paths []string) []string {
	if s.Whole() {
		return paths
	}
	kept := make([]string, 0, len(paths))
	for _, p := range paths {
		if s.WantsPath(examplesDir, p) {
			kept = append(kept, p)
		}
	}
	return kept
}

// Recording is the error a recorder flag fails with under a subset: an
// expectation recorded from a partial sweep would drop every unvisited file.
func (s Subset) Recording(flag string) error {
	if s.Whole() {
		return nil
	}
	return fmt.Errorf("refusing %s under %s=%s: an expectation is recorded over the whole corpus, so unset the variable", flag, EnvVar, s.name)
}

// ReadList reads a subset list: one examples/-relative path per line, blank
// lines and # comments skipped. It is an error for the list to be unsorted or empty.
func ReadList(path string) ([]string, error) {
	f, err := os.Open(path) // #nosec G304 -- a committed testdata file
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var files []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		files = append(files, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s lists no files", path)
	}
	if !sort.StringsAreSorted(files) {
		return nil, fmt.Errorf("%s is not sorted; re-record it", path)
	}
	return files, nil
}
