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
// the Makefile (test-shard, RACE_RUNTIME_CORPUS, COVERAGE_SHARDS), the two CircleCI
// jobs' shard arrays and parallelism, and the PR workflow's matrices.

const raceRuntimeDir = "../../internal/exec/runtime"

var (
	shardScriptList    = regexp.MustCompile(`(?m)^shards='([^']+)'$`)
	makefileCorpus     = regexp.MustCompile(`(?m)^RACE_RUNTIME_CORPUS := (.+)$`)
	makefileCoverage   = regexp.MustCompile(`(?m)^COVERAGE_SHARDS := (.+)$`)
	circleShardArrays  = regexp.MustCompile(`(?m)^\s*shards=\(([^)]+)\)$`)
	circleParallelism  = regexp.MustCompile(`(?m)^  (go-race-test|go-coverage):\n(?:    .*\n)*?    parallelism: (\d+)$`)
	circleCorpus       = regexp.MustCompile(`(?m)^\s*corpus='([^']+)'$`)
	makefileExplore    = regexp.MustCompile(`(?m)^RACE_RUNTIME_EXPLORATION := (.+)$`)
	circleExplore      = regexp.MustCompile(`(?m)^\s*exploration='([^']+)'$`)
	workflowShards     = regexp.MustCompile(`go_shards=\[([^\]]+)\]`)
	workflowProfiles   = regexp.MustCompile(`-ne (\d+) \]; then echo "error: expected (\d+) shard profiles`)
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
	race := strings.Fields(arrays[0][1])
	coverage := strings.Fields(arrays[1][1])

	if !slices.Equal(race, shards) {
		t.Errorf("race job shards %v, scripts/race-shard.sh %v", race, shards)
	}
	wantCoverage := slices.DeleteFunc(slices.Clone(shards), func(s string) bool { return s == "runtime-corpus" })
	if !slices.Equal(coverage, wantCoverage) {
		t.Errorf("coverage job shards %v, want the race shards but runtime-corpus %v", coverage, wantCoverage)
	}
	makeCoverage := strings.Fields(firstSubmatch(t, makefileCoverage, makefile, "Makefile COVERAGE_SHARDS"))
	if !slices.Equal(coverage, makeCoverage) {
		t.Errorf("coverage job shards %v, Makefile COVERAGE_SHARDS %v", coverage, makeCoverage)
	}

	parallelism := map[string]string{}
	for _, m := range circleParallelism.FindAllStringSubmatch(config, -1) {
		parallelism[m[1]] = m[2]
	}
	if got, want := parallelism["go-race-test"], len(race); got != itoa(want) {
		t.Errorf("go-race-test parallelism %q, its shard array has %d entries", got, want)
	}
	if got, want := parallelism["go-coverage"], len(coverage); got != itoa(want) {
		t.Errorf("go-coverage parallelism %q, its shard array has %d entries", got, want)
	}
}

// TestPRWorkflowMatricesCoverTheRaceShards: the full tier runs every race shard but
// tools (tests/wasm is the WebAssembly gate job there, the tools module rides with
// rest), expects one coverage profile per shard, and the quick tier's groups together
// name every package shard once (runtime covers both runtime shards' package).
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
	m := workflowProfiles.FindStringSubmatch(workflow)
	if m == nil || m[1] != itoa(len(full)) || m[2] != itoa(len(full)) {
		t.Errorf("race-coverage expects %v shard profiles, the full tier has %d shards", m, len(full))
	}

	var quick []string
	for _, group := range quoted(matrices[1][1]) {
		quick = append(quick, workflowQuickGroup.Split(strings.TrimSpace(group), -1)...)
	}
	slices.Sort(quick)
	wantQuick := slices.DeleteFunc(slices.Clone(wantFull), func(s string) bool { return s == "runtime-corpus" })
	slices.Sort(wantQuick)
	if !slices.Equal(quick, wantQuick) {
		t.Errorf("quick tier groups name %v, want each package shard once %v", quick, wantQuick)
	}
}

// TestRuntimeCorpusPatternPartitionsTheRuntimeTests: the runtime and runtime-corpus
// shards are internal/exec/runtime run with -skip and -run over one pattern, which the
// Makefile and the CircleCI job each spell out. The two must be the same, every name
// the pattern lists must be a test of the package (a renamed test would silently
// leave the corpus shard smaller), and each test must fall to exactly one shard.
func TestRuntimeCorpusPatternPartitionsTheRuntimeTests(t *testing.T) {
	makefile := readRepoFile(t, "Makefile")
	pattern := strings.ReplaceAll(firstSubmatch(t, makefileCorpus, makefile, "Makefile RACE_RUNTIME_CORPUS"), "$$", "$")
	if circle := firstSubmatch(t, circleCorpus, readRepoFile(t, ".circleci/config.yml"), ".circleci/config.yml corpus="); circle != pattern {
		t.Errorf(".circleci/config.yml corpus pattern %q, Makefile RACE_RUNTIME_CORPUS %q", circle, pattern)
	}
	if !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") {
		t.Fatalf("RACE_RUNTIME_CORPUS %q is not anchored, so it could match more tests than it names", pattern)
	}
	corpus, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("RACE_RUNTIME_CORPUS %q: %v", pattern, err)
	}

	tests := topLevelTests(t, raceRuntimeDir)
	if len(tests) < 100 {
		t.Fatalf("found only %d top-level tests under %s", len(tests), raceRuntimeDir)
	}
	var named []string
	for _, name := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(pattern, "^Test("), ")$"), "|") {
		named = append(named, "Test"+name)
	}
	if len(named) < 2 {
		t.Fatalf("RACE_RUNTIME_CORPUS %q names fewer than two tests", pattern)
	}
	for _, name := range named {
		if !slices.Contains(tests, name) {
			t.Errorf("RACE_RUNTIME_CORPUS names %s, which is not a test of %s", name, raceRuntimeDir)
		}
	}
	var matched int
	for _, name := range tests {
		// go test -run and -skip apply the pattern to the top-level name; a test matched by
		// -run is the corpus shard's, every other the runtime shard's, so the shards partition.
		if corpus.MatchString(name) {
			matched++
			if !slices.Contains(named, name) {
				t.Errorf("RACE_RUNTIME_CORPUS matches %s, which it does not name", name)
			}
		}
	}
	if matched != len(named) {
		t.Errorf("RACE_RUNTIME_CORPUS matches %d tests, names %d", matched, len(named))
	}

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
