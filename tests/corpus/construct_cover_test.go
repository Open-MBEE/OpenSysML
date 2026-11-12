package corpus

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/testutil/constructcover"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/corpussubset"
)

var updateConstructCover = flag.Bool("update-construct-cover", false,
	"rewrite testdata/construct_cover.txt from a greedy cover of the current corpus")

const constructCoverList = "testdata/construct_cover.txt"

// TestConstructCover: the committed construct-cover subset still exhibits every
// syntactic feature any model under examples/ exhibits.
func TestConstructCover(t *testing.T) {
	// Parsing only, it sweeps the whole corpus whatever OPENSYSML_CORPUS_SUBSET
	// says; the roots skip or are required as in the other corpus gates.
	files, _ := corpusRoundTripFiles(t)
	features := make(map[string][]string, len(files))
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(corpusRoundTripExamples, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		features[rel] = constructcover.Parse(rel, data)
	}
	universe := map[string]bool{}
	for _, fs := range features {
		for _, f := range fs {
			universe[f] = true
		}
	}

	if *updateConstructCover {
		subset := corpussubset.Must(t, "../..")
		if err := subset.Recording("-update-construct-cover"); err != nil {
			t.Fatal(err)
		}
		committed := func(rel string) bool { return corpusRoundTripRootOf(rel) == corpusRoundTripCommitted }
		chosen := constructcover.Cover(features, committed)
		writeConstructCover(t, chosen, len(files), len(universe))
		t.Logf("recorded %d of %d files covering %d features", len(chosen), len(files), len(universe))
		return
	}

	chosen, err := corpussubset.ReadList(constructCoverList)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range chosen {
		if _, ok := features[rel]; !ok {
			t.Errorf("%s lists examples/%s, which is not in the corpus; re-record with -update-construct-cover", constructCoverList, rel)
		}
	}
	if uncovered := constructcover.Uncovered(features, chosen); len(uncovered) > 0 {
		t.Errorf("%s no longer covers the corpus: %d of %d features are exhibited only outside it (feature, a file that has it):\n\t%s\nre-record with: go test ./tests/corpus -run TestConstructCover -update-construct-cover",
			constructCoverList, len(uncovered), len(universe), strings.Join(uncovered, "\n\t"))
	}
	t.Logf("%d of %d files cover %d features", len(chosen), len(files), len(universe))
}

// writeConstructCover writes the subset list: a header naming the recorder and
// what the recording covered, then one sorted examples/-relative path per line.
func writeConstructCover(t *testing.T, chosen []string, files, features int) {
	t.Helper()
	sort.Strings(chosen)
	var out strings.Builder
	out.WriteString("# The construct-cover subset of examples/: the models the sweeps iterate under\n")
	out.WriteString("# OPENSYSML_CORPUS_SUBSET=construct-cover, one examples/-relative path per line.\n")
	out.WriteString("# Re-record with the downloaded corpora present:\n")
	out.WriteString("#   go test ./tests/corpus -run TestConstructCover -update-construct-cover\n")
	fmt.Fprintf(&out, "# %d of %d files covering %d features.\n", len(chosen), files, features)
	for _, rel := range chosen {
		out.WriteString(rel + "\n")
	}
	if err := os.WriteFile(constructCoverList, []byte(out.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
