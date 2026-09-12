package main

import (
	"os"
	"path/filepath"
	"testing"
)

// census pins the assertion population of both suites per kind: line notes,
// block notes, and declared expectations. The reader is trusted only if it matches.
var census = map[string]struct {
	Files      int
	Assertions map[string][3]int // kind -> {line notes, block notes, expectations}
}{
	"kerml": {
		Files: 303,
		Assertions: map[string][3]int{
			kindErrors:        {297, 17, 328},
			kindNoErrors:      {200, 0, 200},
			kindLinkedName:    {191, 0, 191},
			kindWarnings:      {18, 0, 18},
			kindScope:         {1, 229, 230},
			"exportedObjects": {0, 1, 1},
		},
	},
	"sysml": {
		Files: 126,
		Assertions: map[string][3]int{
			kindErrors:     {164, 10, 184},
			kindNoErrors:   {76, 0, 76},
			kindLinkedName: {3, 0, 3},
			kindWarnings:   {40, 17, 95},
		},
	},
}

// lineNotes is the published `// XPECT <kind>` census. linkedName is +2: two
// notes are tab-indented after `//`, which that grep misses and Xpect does not.
var lineNotes = map[string]int{
	kindErrors:     461,
	kindNoErrors:   276,
	kindLinkedName: 192 + 2,
	kindWarnings:   58,
	kindScope:      1,
}

// TestW5CSuiteCensus checks the read population against the pin. It skips while
// the corpus is absent, and fails when OPENSYSML_REQUIRE_PILOT_XPECT is set.
func TestW5CSuiteCensus(t *testing.T) {
	repo, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range defaultSuites {
		dir := filepath.Join(repo, filepath.FromSlash(s.Dir))
		if _, err := os.Stat(dir); err != nil {
			if os.Getenv("OPENSYSML_REQUIRE_PILOT_XPECT") != "" {
				t.Fatalf("%s is absent; run scripts/download-pilot-xpect.sh", s.Dir)
			}
			t.Skipf("%s is absent; run scripts/download-pilot-xpect.sh", s.Dir)
		}
		t.Run(s.Name, func(t *testing.T) { checkCensus(t, dir, s.Name) })
	}

	for kind, want := range lineNotes {
		got := 0
		for _, suite := range census {
			got += suite.Assertions[kind][0]
		}
		if got != want {
			t.Errorf("%s line notes = %d, want %d", kind, got, want)
		}
	}
}

func checkCensus(t *testing.T, dir, name string) {
	t.Helper()
	want := census[name]
	files, err := collectXT(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != want.Files {
		t.Errorf("%d .xt file(s), want %d", len(files), want.Files)
	}

	got := map[string][3]int{}
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		f := parseXT(rel, name, content)
		if len(f.Problems) > 0 {
			t.Errorf("%s unparsed: %v", rel, f.Problems)
		}
		for _, a := range f.Assertions {
			counts := got[a.Kind]
			if a.Block {
				counts[1]++
			} else {
				counts[0]++
			}
			counts[2] += max(len(a.Expect), 1)
			got[a.Kind] = counts
		}
	}
	for kind, counts := range want.Assertions {
		if got[kind] != counts {
			t.Errorf("%s = %v, want %v (line notes, block notes, expectations)", kind, got[kind], counts)
		}
	}
	for kind := range got {
		if _, ok := want.Assertions[kind]; !ok {
			t.Errorf("unexpected assertion kind %q: %v", kind, got[kind])
		}
	}
}
