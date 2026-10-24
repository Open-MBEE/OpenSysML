package hygiene

import (
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var (
	makefileCoverageRecipe = regexp.MustCompile(`(?ms)^coverage:.*?\n\n`)
	recipeProfileWrites    = regexp.MustCompile(`-coverprofile=(\S+)|-o (\S+)|tail -n \+2 (\S+) >>`)
)

// TestCoverageIntermediatesCannotBeAShardProfile: make coverage-shard collects into
// coverage-<shard>.txt, so the recipe's intermediates live under GO_COUNTER_DIR and bear no shard's name.
func TestCoverageIntermediatesCannotBeAShardProfile(t *testing.T) {
	makefile := readRepoFile(t, "Makefile")
	recipe := makefileCoverageRecipe.FindString(makefile)
	if recipe == "" {
		t.Fatal("Makefile: no coverage recipe")
	}
	config := readRepoFile(t, ".circleci/config.yml")
	arrays := circleShardArrays.FindAllStringSubmatch(config, -1)
	if len(arrays) != 2 {
		t.Fatalf("found %d shard arrays in .circleci/config.yml, want 2", len(arrays))
	}
	var forbidden []string
	for _, shard := range append(strings.Fields(arrays[1][1]), strings.Fields(firstSubmatch(t, makefileCoverage, makefile, "Makefile COVERAGE_SHARDS"))...) {
		forbidden = append(forbidden, "coverage-"+shard+".txt")
	}
	var intermediates int
	for _, m := range recipeProfileWrites.FindAllStringSubmatch(recipe, -1) {
		file := m[1] + m[2] + m[3]
		if file == "$(COVERAGE_PROFILE)" {
			continue
		}
		intermediates++
		if !strings.HasPrefix(file, "$(GO_COUNTER_DIR)/") {
			t.Errorf("coverage recipe writes %s outside GO_COUNTER_DIR, where a shard's coverage-<shard>.txt can collide with it", file)
		}
		if base := path.Base(file); slices.Contains(forbidden, base) || base == "coverage.txt" {
			t.Errorf("coverage recipe intermediate %s is named like a shard's profile", file)
		}
	}
	if intermediates < 2 {
		t.Fatalf("coverage recipe: found %d intermediate profiles, want the counter and tools profiles", intermediates)
	}
}
