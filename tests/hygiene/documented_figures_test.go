package hygiene

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const extensionLibraryDir = "OpenSysML Libraries"

// documentedLibraryClaims lists the sentences stating how many library files are bundled. The groups
// total, omg and ext are held to the tree; names asks the paragraph to name every extension too.
var documentedLibraryClaims = []struct {
	path    string
	pattern string
	names   bool
}{
	{path: "README.md", pattern: `(?P<total>\d+) of (?P<total>\d+) standard library files parse cleanly: (?P<omg>\d+) vendored OMG files and (?P<ext>\d+) OpenSysML extensions`},
	{path: "README.md", pattern: `(?P<total>\d+)/(?P<total>\d+) stdlib clean`},
	{path: "README.md", pattern: `(?P<total>\d+)/(?P<total>\d+) bundled library files parse cleanly — the (?P<omg>\d+) official SysML v2 standard library files and the (?P<ext>\d+) non-normative OpenSysML extensions`, names: true},
	{path: "README.md", pattern: `(?P<omg>\d+) files from \[SysML v2 Pilot Implementation [^\n]*?plus the (?P<ext>[\w-]+) non-normative`, names: true},
	{path: "docs/internals/architecture.md", pattern: `the (?P<total>\d+) library files under`},
	{path: "docs/internals/architecture.md", pattern: `(?P<total>\d+)/(?P<total>\d+) stdlib clean`},
	{path: "docs/internals/architecture.md", pattern: `(?P<total>\d+)/(?P<total>\d+) bundled library files parse cleanly — the (?P<omg>\d+) official SysML v2 standard library files and the (?P<ext>\d+) non-normative OpenSysML extensions`, names: true},
	{path: "docs/internals/architecture.md", pattern: `loads all (?P<total>\d+) bundled library files`},
	{path: "docs/internals/architecture.md", pattern: `(?P<total>\d+)/(?P<total>\d+) files parse without errors`},
	{path: "docs/internals/testing.md", pattern: `(?P<total>\d+)/(?P<total>\d+) bundled library files — the (?P<omg>\d+) official SysML v2 standard library files and (?P<ext>[\w-]+) non-normative OpenSysML extensions`},
	{path: "docs/project/extension-libraries.md", pattern: `same (?P<ext>[\w-]+) units`},
	{path: "docs/project/roadmap.md", pattern: `(?P<total>\d+)/(?P<total>\d+) clean — (?P<omg>\d+) vendored OMG files and (?P<ext>\d+) non-normative OpenSysML extensions`},
	{path: "docs/project/roadmap.md", pattern: `The (?P<ext>[\w-]+) non-normative libraries under`},
	{path: "docs/project/spec-compliance/02-detailed-semantic-compliance-map/20-opensysml-math-extension-library.md", pattern: `bundled-library gate reports (?P<total>\d+)/(?P<total>\d+) clean`, names: true},
	{path: "docs/project/spec-compliance/05-testing-infrastructure.md", pattern: `(?P<total>\d+)/(?P<total>\d+) stdlib files clean \((?P<omg>\d+) vendored OMG, (?P<ext>\d+) OpenSysML extensions\)`},
	{path: "docs/reference/grammar/README.md", pattern: `all (?P<total>\d+) bundled library files \((?P<omg>\d+) OMG standard library files and (?P<ext>\d+) OpenSysML extensions\)`},
}

// documentedGoFloorClaims lists the sentences stating the oldest Go that builds the module.
var documentedGoFloorClaims = []struct{ path, pattern string }{
	{"README.md", `Go (\d+\.\d+) or later`},
	{"CONTRIBUTING.md", `Go (\d+\.\d+) or later`},
	{"AGENTS.md", `\*\*Go (\d+\.\d+)\+\*\*`},
	{"docs/guide/01-install.md", `Go (\d+\.\d+) or later`},
	{"docs/guide/10-troubleshooting.md", `\((\d+\.\d+) or later is required\)`},
	{"docs/internals/architecture.md", `\*\*Language:\*\* Go (\d+\.\d+)\+`},
}

// TestDocumentationStatesTheBundledLibraryCount holds every documented library count to the files under
// internal/workspace/libs/stdlib, so vendoring a library fails here until each sentence follows.
func TestDocumentationStatesTheBundledLibraryCount(t *testing.T) {
	omg, extensions := bundledLibraryFiles(t)
	want := map[string]int{"total": omg + len(extensions), "omg": omg, "ext": len(extensions)}
	for _, claim := range documentedLibraryClaims {
		content := readRepoFile(t, claim.path)
		pattern := regexp.MustCompile(claim.pattern)
		matches := pattern.FindAllStringSubmatchIndex(content, -1)
		if len(matches) == 0 {
			t.Errorf("%s: no sentence matches %q; update the claim beside the sentence it follows", claim.path, claim.pattern)
			continue
		}
		for _, match := range matches {
			reported := map[string]bool{}
			for group, kind := range pattern.SubexpNames() {
				if kind == "" || reported[kind] {
					continue
				}
				stated := content[match[2*group]:match[2*group+1]]
				if got, ok := cardinal(stated); !ok || got != want[kind] {
					reported[kind] = true
					t.Errorf("%s: %q states %s where the tree bundles %d", claim.path, content[match[0]:match[1]], stated, want[kind])
				}
			}
			if !claim.names {
				continue
			}
			paragraph := paragraphAround(content, match[0])
			for _, name := range extensions {
				if !regexp.MustCompile("[`/]" + regexp.QuoteMeta(name) + "[`.]").MatchString(paragraph) {
					t.Errorf("%s: the paragraph stating %q does not name the bundled extension %s", claim.path, content[match[0]:match[1]], name)
				}
			}
		}
	}
}

// TestDocumentationStatesTheGoFloor holds every documented minimum Go version to go.mod's go line.
func TestDocumentationStatesTheGoFloor(t *testing.T) {
	floor := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`).FindStringSubmatch(readRepoFile(t, "go.mod"))
	if floor == nil {
		t.Fatal("go.mod has no go line")
	}
	for _, claim := range documentedGoFloorClaims {
		content := readRepoFile(t, claim.path)
		matches := regexp.MustCompile(claim.pattern).FindAllStringSubmatch(content, -1)
		if len(matches) == 0 {
			t.Errorf("%s: no sentence matches %q; update the claim beside the sentence it follows", claim.path, claim.pattern)
		}
		for _, match := range matches {
			if match[1] != floor[1] {
				t.Errorf("%s: %q where go.mod requires go %s", claim.path, match[0], floor[1])
			}
		}
	}
}

// bundledLibraryFiles counts the OMG library files and names the OpenSysML extensions beside them.
func bundledLibraryFiles(t *testing.T) (omg int, extensions []string) {
	t.Helper()
	root := filepath.Join("..", "..", "internal", "workspace", "libs", "stdlib")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		extension := filepath.Ext(path)
		if extension != ".sysml" && extension != ".kerml" {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == extensionLibraryDir {
			extensions = append(extensions, strings.TrimSuffix(entry.Name(), extension))
		} else {
			omg++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if omg == 0 || len(extensions) == 0 {
		t.Fatalf("%s holds %d OMG files and %d extensions; both must be present", root, omg, len(extensions))
	}
	return omg, extensions
}

// paragraphAround returns the blank-line-delimited paragraph holding offset.
func paragraphAround(content string, offset int) string {
	start := strings.LastIndex(content[:offset], "\n\n") + 1
	end := strings.Index(content[offset:], "\n\n")
	if end < 0 {
		return content[start:]
	}
	return content[start : offset+end]
}

var (
	cardinalUnits = []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
		"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	cardinalTens = []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
)

// cardinal reads a count written in digits or, below one hundred, in words.
func cardinal(text string) (int, bool) {
	if n, err := strconv.Atoi(text); err == nil {
		return n, true
	}
	tens, units, hyphenated := strings.Cut(strings.ToLower(text), "-")
	for n, word := range cardinalUnits {
		if word == tens && !hyphenated {
			return n, true
		}
	}
	for ten, word := range cardinalTens {
		if word == "" || word != tens {
			continue
		}
		if !hyphenated {
			return ten * 10, true
		}
		for n, unit := range cardinalUnits[1:10] {
			if unit == units {
				return ten*10 + n + 1, true
			}
		}
	}
	return 0, false
}
