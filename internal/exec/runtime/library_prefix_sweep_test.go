package runtime

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
)

var updateLibraryPrefixSweep = flag.Bool("update-library-prefix-sweep", false, "Rewrite docs/project/library-and-prefix-sweep-outcomes.txt from this run")

// libraryPrefixSweepCommand reproduces the committed sweep outcomes.
const libraryPrefixSweepCommand = "go test ./internal/exec/runtime -run TestLibraryAndPrefixSweep -update-library-prefix-sweep"

// TestLibraryAndPrefixSweep pins the outcome of every library-feature read, Kernel Function
// Library call and prefix-by-usage-kind probe, so a gap or regression on any axis is a reviewed diff.
func TestLibraryAndPrefixSweep(t *testing.T) {
	var lines []string
	lines = append(lines, sweepSystemsLibraryReads(t)...)
	lines = append(lines, sweepKernelFunctionCalls(t)...)
	lines = append(lines, sweepPrefixUsageKinds(t)...)
	got := "# " + libraryPrefixSweepCommand + "\n# axis\tprobe\t…\toutcome\n" + strings.Join(lines, "\n") + "\n"
	path := filepath.Join("..", "..", "..", filepath.FromSlash(fixtures.LibraryPrefixSweepPath))
	if *updateLibraryPrefixSweep {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil { // #nosec G306 -- a committed documentation file
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path) // #nosec G304 -- the committed sweep this test maintains
	if err != nil {
		t.Fatalf("%v; run `%s`", err, libraryPrefixSweepCommand)
	}
	if string(want) != got {
		t.Errorf("%s disagrees with this run; run `%s` and commit the result\n%s",
			fixtures.LibraryPrefixSweepPath, libraryPrefixSweepCommand, censusDiff(string(want), got))
	}
}
