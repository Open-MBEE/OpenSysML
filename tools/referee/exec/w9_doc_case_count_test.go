package exec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The referee's bucket counts cannot be re-derived offline — they need the
// pinned artifact — but the corpus size can, and it is the number that went
// stale twice: the shipped corpus grew from 32 to 94 cases while the prose and
// the skill anchor kept quoting 32. This guards the denominator in both places
// so growing the corpus without re-running the referee fails here.
func TestDocumentedCaseCount(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	files, err := readCaseFiles(filepath.Join(repo, filepath.FromSlash(defaultCases)))
	if err != nil {
		t.Fatal(err)
	}
	cases := 0
	for _, file := range files {
		cases += len(file.Cases)
	}
	if cases == 0 {
		t.Fatal("no cases found")
	}

	// The prose says "the N committed cases"; the skill anchor says "(N cases, …".
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(\d+) committed cases`),
		regexp.MustCompile(`\((\d+) cases,`),
	}
	for _, doc := range []string{
		filepath.Join(repo, "docs", "project", "pilot-execution-referee.md"),
		filepath.Join(repo, ".agents", "skills", "testing-pilot-execution-referee", "SKILL.md"),
	} {
		body, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for i, line := range strings.Split(string(body), "\n") {
			for _, pattern := range patterns {
				m := pattern.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				want, err := strconv.Atoi(m[1])
				if err != nil {
					t.Fatal(err)
				}
				found = true
				if want != cases {
					t.Errorf("%s:%d: cases: documents %d, corpus holds %d", doc, i+1, want, cases)
				}
			}
		}
		if !found {
			t.Errorf("%s: no %q claim found; the corpus size must be stated where the buckets are",
				doc, fmt.Sprintf("%d cases", cases))
		}
	}
}
