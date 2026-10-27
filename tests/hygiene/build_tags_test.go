package hygiene

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// usedTags finds every `-tags a,b` a build recipe passes to the Go tool.
var usedTags = regexp.MustCompile(`(?:^|[\s"'(])-tags[= ]+([A-Za-z0-9_,]+)`)

// declaredTags finds the names a `//go:build` constraint mentions.
var declaredTags = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// TestBuildTagsInRecipesAreDeclared checks that every build tag the Makefile,
// the CI configs and the scripts pass to `go build` is one some `//go:build`
// line in the tree declares: Go treats an unknown tag as unset, so a recipe
// naming a retired tag silently builds the full binary instead of failing.
func TestBuildTagsInRecipesAreDeclared(t *testing.T) {
	recipes := []string{"../../Makefile", "../../.circleci/config.yml"}
	for _, pattern := range []string{"../../.github/workflows/*.yml", "../../scripts/*.sh", "../../scripts/*.ps1"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		recipes = append(recipes, matches...)
	}

	used := map[string][]string{}
	for _, path := range recipes {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range usedTags.FindAllStringSubmatch(string(raw), -1) {
			for _, tag := range strings.Split(m[1], ",") {
				used[tag] = append(used[tag], filepath.Base(path))
			}
		}
	}
	if len(used) == 0 {
		t.Fatal("no -tags in any recipe; the pattern no longer matches the Makefile")
	}

	declared := map[string]bool{}
	err := filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "node_modules" || name == ".git" || name == "build" || name == "bin" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "//go:build ") {
				for _, name := range declaredTags.FindAllString(strings.TrimPrefix(line, "//go:build "), -1) {
					declared[name] = true
				}
			} else if strings.HasPrefix(line, "package ") {
				break
			}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatal(err)
	}

	tags := make([]string, 0, len(used))
	for tag := range used {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		if !declared[tag] {
			t.Errorf("%s passes -tags %s, which no //go:build constraint in the tree declares; a build with it is the untagged build", strings.Join(used[tag], ", "), tag)
		}
	}
}
