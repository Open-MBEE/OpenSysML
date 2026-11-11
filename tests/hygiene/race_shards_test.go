package hygiene

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The race shards are named in five places that must agree: scripts/race-shard.sh,
// the Makefile (test-shard, RACE_RUNTIME_CORPUS, RACE_EXPORT_CORPUS, RACE_EXPORT_SETTLED,
// RACE_CORPUS_ROUNDTRIP, RACE_CORPUS_APIJSON, COVERAGE_SHARDS),
// the two CircleCI jobs' shard arrays and parallelism, and the PR workflow's matrices.

const (
	raceRuntimeDir = "../../internal/exec/runtime"
	raceExportDir  = "../../tests/export"
	raceCorpusDir  = "../../tests/corpus"
)

var (
	shardScriptList    = regexp.MustCompile(`(?m)^shards='([^']+)'$`)
	makefileCorpus     = regexp.MustCompile(`(?m)^RACE_RUNTIME_CORPUS := (.+)$`)
	makefileExport     = regexp.MustCompile(`(?m)^RACE_EXPORT_CORPUS := (.+)$`)
	circleExport       = regexp.MustCompile(`(?m)^\s*exportcorpus='([^']+)'$`)
	makefileSettled    = regexp.MustCompile(`(?m)^RACE_EXPORT_SETTLED := (.+)$`)
	circleSettled      = regexp.MustCompile(`(?m)^\s*exportsettled='([^']+)'$`)
	makefileLibrary    = regexp.MustCompile(`(?m)^RACE_EXPORT_LIBRARY := (.+)$`)
	circleLibrary      = regexp.MustCompile(`(?m)^\s*exportlibrary='([^']+)'$`)
	makefileExamples   = regexp.MustCompile(`(?m)^RACE_EXPORT_EXAMPLES := (.+)$`)
	circleExamples     = regexp.MustCompile(`(?m)^\s*exportexamples='([^']+)'$`)
	makefileRoundTrip  = regexp.MustCompile(`(?m)^RACE_CORPUS_ROUNDTRIP := (.+)$`)
	circleRoundTrip    = regexp.MustCompile(`(?m)^\s*corpusroundtrip='([^']+)'$`)
	makefileAPIJSON    = regexp.MustCompile(`(?m)^RACE_CORPUS_APIJSON := (.+)$`)
	circleAPIJSON      = regexp.MustCompile(`(?m)^\s*corpusapijson='([^']+)'$`)
	makefileCoverage   = regexp.MustCompile(`(?m)^COVERAGE_SHARDS := (.+)$`)
	circleShardArrays  = regexp.MustCompile(`(?m)^\s*shards=\(([^)]+)\)$`)
	shardArrayEntry    = regexp.MustCompile(`"([^"]*)"|(\S+)`)
	circleParallelism  = regexp.MustCompile(`(?m)^  (go-race-test|go-coverage):\n(?:    .*\n)*?    parallelism: (\d+)$`)
	circleCorpus       = regexp.MustCompile(`(?m)^\s*corpus='([^']+)'$`)
	makefileExplore    = regexp.MustCompile(`(?m)^RACE_RUNTIME_EXPLORATION := (.+)$`)
	circleExplore      = regexp.MustCompile(`(?m)^\s*exploration='([^']+)'$`)
	workflowShards     = regexp.MustCompile(`go_shards=\[([^\]]+)\]`)
	workflowQuoted     = regexp.MustCompile(`"([^"]+)"`)
	workflowQuickGroup = regexp.MustCompile(`\s+`)
)

func firstSubmatch(t *testing.T, re *regexp.Regexp, text, what string) string {
	t.Helper()
	m := re.FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("%s: no match for %s", what, re)
	}
	return m[1]
}

// raceShards are the shard names scripts/race-shard.sh accepts, in its order.
func raceShards(t *testing.T) []string {
	t.Helper()
	return strings.Split(firstSubmatch(t, shardScriptList, readRepoFile(t, "scripts/race-shard.sh"), "scripts/race-shard.sh"), "|")
}

// TestCircleCIShardArraysMatchTheRaceShards: each CircleCI job dispatches one shard per
// node from an array that must list the shards (race) or COVERAGE_SHARDS (coverage) in
// the Makefile's sense, with parallelism equal to the array's length.
func TestCircleCIShardArraysMatchTheRaceShards(t *testing.T) {
	shards := raceShards(t)
	config := readRepoFile(t, ".circleci/config.yml")
	makefile := readRepoFile(t, "Makefile")

	arrays := circleShardArrays.FindAllStringSubmatch(config, -1)
	if len(arrays) != 2 {
		t.Fatalf(".circleci/config.yml: expected the race and coverage shard arrays, found %d", len(arrays))
	}
	raceNodes := shardArrayEntries(arrays[0][1])
	coverage := strings.Fields(arrays[1][1])

	// A race node runs one shard, or the shards of one quoted entry in turn; together the
	// nodes run every shard once, in whatever order.
	var race []string
	for _, node := range raceNodes {
		race = append(race, strings.Fields(node)...)
	}
	slices.Sort(race)
	if !slices.Equal(race, slices.Sorted(slices.Values(shards))) {
		t.Errorf("race job nodes %q run shards %v, scripts/race-shard.sh %v", raceNodes, race, shards)
	}
	// The coverage job has no corpus shards of the runtime: without -race the package runs
	// whole in its runtime shard.
	wantCoverage := slices.DeleteFunc(slices.Clone(shards), func(s string) bool {
		return s == "runtime-corpus" || s == "runtime-exploration"
	})
	if !slices.Equal(coverage, wantCoverage) {
		t.Errorf("coverage job shards %v, want the race shards but the runtime's corpus shards %v", coverage, wantCoverage)
	}
	makeCoverage := strings.Fields(firstSubmatch(t, makefileCoverage, makefile, "Makefile COVERAGE_SHARDS"))
	if !slices.Equal(coverage, makeCoverage) {
		t.Errorf("coverage job shards %v, Makefile COVERAGE_SHARDS %v", coverage, makeCoverage)
	}
	// A by-test-name shard and the package shard it is cut from each select their tests in
	// make coverage-shard; without that both would write the whole package's profile.
	for _, shard := range coverage {
		if !isCorpusShard(shard) {
			continue
		}
		for _, s := range []string{shard, strings.SplitN(shard, "-", 2)[0]} {
			if !strings.Contains(makefile, "\nCOVERAGE_SELECT_"+s+" = ") {
				t.Errorf("coverage shard %s: the Makefile has no COVERAGE_SELECT_%s", shard, s)
			}
		}
	}

	parallelism := map[string]string{}
	for _, m := range circleParallelism.FindAllStringSubmatch(config, -1) {
		parallelism[m[1]] = m[2]
	}
	if got, want := parallelism["go-race-test"], len(raceNodes); got != itoa(want) {
		t.Errorf("go-race-test parallelism %q, its shard array has %d entries", got, want)
	}
	if got, want := parallelism["go-coverage"], len(coverage); got != itoa(want) {
		t.Errorf("go-coverage parallelism %q, its shard array has %d entries", got, want)
	}
}

// shardArrayEntries splits the body of a bash array literal into its entries: words, and
// double-quoted strings (which may hold several words) as one entry each.
func shardArrayEntries(body string) []string {
	var entries []string
	for _, m := range shardArrayEntry.FindAllStringSubmatch(body, -1) {
		if m[1] != "" {
			entries = append(entries, m[1])
		} else {
			entries = append(entries, m[2])
		}
	}
	return entries
}

// TestPRWorkflowMatricesCoverTheRaceShards: the full tier runs every race shard but
// tools (tests/wasm is the WebAssembly gate job there, the tools module rides with
// rest), and the quick tier's groups together name each of those shards once (make
// test-quick runs a group's shards in turn, with their selections, without -race).

// isCorpusShard says whether a shard is a by-test-name part of another shard's package
// (runtime-corpus, export-corpus, export-settled, corpus-roundtrip, ...): the hyphenated names.
func isCorpusShard(shard string) bool { return strings.Contains(shard, "-") }
func TestPRWorkflowMatricesCoverTheRaceShards(t *testing.T) {
	shards := raceShards(t)
	workflow := readRepoFile(t, ".github/workflows/pr.yml")
	matrices := workflowShards.FindAllStringSubmatch(workflow, -1)
	if len(matrices) != 2 {
		t.Fatalf(".github/workflows/pr.yml: expected the full and quick go_shards, found %d", len(matrices))
	}
	quoted := func(list string) []string {
		var out []string
		for _, m := range workflowQuoted.FindAllStringSubmatch(list, -1) {
			out = append(out, m[1])
		}
		return out
	}
	full := quoted(matrices[0][1])
	wantFull := slices.DeleteFunc(slices.Clone(shards), func(s string) bool { return s == "tools" })
	if !slices.Equal(full, wantFull) {
		t.Errorf("full tier go_shards %v, want the race shards but tools %v", full, wantFull)
	}

	var quick []string
	for _, group := range quoted(matrices[1][1]) {
		quick = append(quick, workflowQuickGroup.Split(strings.TrimSpace(group), -1)...)
	}
	slices.Sort(quick)
	wantQuick := slices.Sorted(slices.Values(wantFull))
	if !slices.Equal(quick, wantQuick) {
		t.Errorf("quick tier groups name %v, want each full-tier shard once %v", quick, wantQuick)
	}
}

// TestRuntimeCorpusPatternPartitionsTheRuntimeTests: the runtime and runtime-corpus
// shards are internal/exec/runtime run with -skip and -run over one pattern, which the
// Makefile and the CircleCI job each spell out. The two must be the same, every name
// the pattern lists must be a test of the package (a renamed test would silently
// leave the corpus shard smaller), and each test must fall to exactly one shard.
func TestRuntimeCorpusPatternPartitionsTheRuntimeTests(t *testing.T) {
	makefile := readRepoFile(t, "Makefile")
	corpus, tests := corpusPatternPartitions(t, "RACE_RUNTIME_CORPUS", makefileCorpus, circleCorpus, "corpus=", raceRuntimeDir, 100)

	// The corpus shard runs the exploration suite as its own process (race memory is per
	// process); its pattern must name one real test that the corpus pattern also matches.
	exploration := strings.ReplaceAll(firstSubmatch(t, makefileExplore, makefile, "Makefile RACE_RUNTIME_EXPLORATION"), "$$", "$")
	if circle := firstSubmatch(t, circleExplore, readRepoFile(t, ".circleci/config.yml"), ".circleci/config.yml exploration="); circle != exploration {
		t.Errorf(".circleci/config.yml exploration pattern %q, Makefile RACE_RUNTIME_EXPLORATION %q", circle, exploration)
	}
	name := strings.TrimSuffix(strings.TrimPrefix(exploration, "^"), "$")
	if "^"+name+"$" != exploration || !slices.Contains(tests, name) {
		t.Fatalf("RACE_RUNTIME_EXPLORATION %q is not an anchored name of one test of %s", exploration, raceRuntimeDir)
	}
	if !corpus.MatchString(name) {
		t.Errorf("RACE_RUNTIME_EXPLORATION names %s, which RACE_RUNTIME_CORPUS does not match", name)
	}
}

// TestExportCorpusPatternsPartitionTheExportTests: the five export shards are tests/export split the
// same way by the four RACE_EXPORT_* patterns, of which no test may match more than one.
func TestExportCorpusPatternsPartitionTheExportTests(t *testing.T) {
	corpus, tests := corpusPatternPartitions(t, "RACE_EXPORT_CORPUS", makefileExport, circleExport, "exportcorpus=", raceExportDir, 50)
	settled, _ := corpusPatternPartitions(t, "RACE_EXPORT_SETTLED", makefileSettled, circleSettled, "exportsettled=", raceExportDir, 50)
	library, _ := corpusPatternPartitions(t, "RACE_EXPORT_LIBRARY", makefileLibrary, circleLibrary, "exportlibrary=", raceExportDir, 50)
	examples, _ := corpusPatternPartitions(t, "RACE_EXPORT_EXAMPLES", makefileExamples, circleExamples, "exportexamples=", raceExportDir, 50)
	for _, name := range tests {
		matches := 0
		for _, pattern := range []*regexp.Regexp{corpus, settled, library, examples} {
			if pattern.MatchString(name) {
				matches++
			}
		}
		if matches > 1 {
			t.Errorf("%s falls to %d by-name export shards", name, matches)
		}
	}
}

// TestCorpusPatternsPartitionTheCorpusTests: the corpus, corpus-roundtrip and corpus-apijson
// shards are tests/corpus split the same way by RACE_CORPUS_ROUNDTRIP and RACE_CORPUS_APIJSON,
// which no test may match both of.
func TestCorpusPatternsPartitionTheCorpusTests(t *testing.T) {
	roundTrip, tests := corpusPatternPartitions(t, "RACE_CORPUS_ROUNDTRIP", makefileRoundTrip, circleRoundTrip, "corpusroundtrip=", raceCorpusDir, 10)
	apiJSON, _ := corpusPatternPartitions(t, "RACE_CORPUS_APIJSON", makefileAPIJSON, circleAPIJSON, "corpusapijson=", raceCorpusDir, 10)
	for _, name := range tests {
		if roundTrip.MatchString(name) && apiJSON.MatchString(name) {
			t.Errorf("%s falls to both corpus-roundtrip and corpus-apijson", name)
		}
	}
}

// corpusPatternPartitions checks one by-test-name split: the Makefile variable and the
// CircleCI job spell the same anchored pattern, every name it lists is a test of dir, and
// it matches exactly the tests it names, so -run and -skip over it partition the package.
func corpusPatternPartitions(t *testing.T, variable string, makefileRe, circleRe *regexp.Regexp, circleWhat, dir string, minTests int) (*regexp.Regexp, []string) {
	t.Helper()
	pattern := strings.ReplaceAll(firstSubmatch(t, makefileRe, readRepoFile(t, "Makefile"), "Makefile "+variable), "$$", "$")
	if circle := firstSubmatch(t, circleRe, readRepoFile(t, ".circleci/config.yml"), ".circleci/config.yml "+circleWhat); circle != pattern {
		t.Errorf(".circleci/config.yml %s pattern %q, Makefile %s %q", circleWhat, circle, variable, pattern)
	}
	if !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") {
		t.Fatalf("%s %q is not anchored, so it could match more tests than it names", variable, pattern)
	}
	corpus, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("%s %q: %v", variable, pattern, err)
	}

	tests := topLevelTests(t, dir)
	if len(tests) < minTests {
		t.Fatalf("found only %d top-level tests under %s", len(tests), dir)
	}
	named := []string{strings.TrimSuffix(strings.TrimPrefix(pattern, "^"), "$")}
	if group := named[0]; strings.HasPrefix(group, "Test(") && strings.HasSuffix(group, ")") {
		named = nil
		for _, name := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(group, "Test("), ")"), "|") {
			named = append(named, "Test"+name)
		}
	}
	for _, name := range named {
		if !slices.Contains(tests, name) {
			t.Errorf("%s names %s, which is not a test of %s", variable, name, dir)
		}
	}
	var matched int
	for _, name := range tests {
		// go test -run and -skip apply the pattern to the top-level name; a test matched by
		// -run is the corpus shard's, every other the package shard's, so the shards partition.
		if corpus.MatchString(name) {
			matched++
			if !slices.Contains(named, name) {
				t.Errorf("%s matches %s, which it does not name", variable, name)
			}
		}
	}
	if matched != len(named) {
		t.Errorf("%s matches %d tests, names %d", variable, matched, len(named))
	}
	return corpus, tests
}

// topLevelTests lists the Test functions declared in the package directory's test files.
func topLevelTests(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") && len(fn.Name.Name) > 4 {
				names = append(names, fn.Name.Name)
			}
		}
	}
	slices.Sort(names)
	return names
}

func itoa(n int) string { return strconv.Itoa(n) }
