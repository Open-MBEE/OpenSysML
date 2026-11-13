package constructcover

import (
	"slices"
	"strings"
	"testing"
)

func TestFeaturesNameNodesEdgesEnumsAndFlags(t *testing.T) {
	features := Parse("m.sysml", []byte(`package P { abstract part def A; part a : A; }`))
	for _, want := range []string{
		"kind SysML", "clean", "node ast.RootNamespace", "node ast.Package",
		"edge ast.RootNamespace.", "enum ast.Definition.Kind=", "enum ast.Usage.Kind=", "flag ast.Definition.",
	} {
		if !slices.ContainsFunc(features, func(f string) bool { return strings.HasPrefix(f, want) }) {
			t.Errorf("no feature with prefix %q in\n%s", want, strings.Join(features, "\n"))
		}
	}
	if !slices.IsSorted(features) {
		t.Errorf("features are not sorted")
	}
}

func TestDiagnosticsAreAFeature(t *testing.T) {
	features := Parse("m.sysml", []byte(`package P { part`))
	if !slices.Contains(features, "diagnostics") || slices.Contains(features, "clean") {
		t.Errorf("a file that fails to parse exhibits diagnostics, got %v", features)
	}
}

func TestCoverIsGreedyDeterministicAndPrefersCommitted(t *testing.T) {
	features := map[string][]string{
		"z.sysml":                  {"a", "b", "c"},
		"downloaded/big.sysml":     {"a", "b", "c"},
		"b.sysml":                  {"d"},
		"a.sysml":                  {"d"},
		"downloaded/only.sysml":    {"e"},
		"downloaded/redundant.txt": {"a"},
	}
	committed := func(p string) bool { return !strings.HasPrefix(p, "downloaded/") }
	got := Cover(features, committed)
	want := []string{"a.sysml", "downloaded/only.sysml", "z.sysml"}
	if !slices.Equal(got, want) {
		t.Fatalf("Cover = %v, want %v", got, want)
	}
	if u := Uncovered(features, got); len(u) != 0 {
		t.Errorf("the cover leaves %v uncovered", u)
	}
	if u := Uncovered(features, []string{"z.sysml"}); !slices.Equal(u, []string{"d\ta.sysml", "e\tdownloaded/only.sysml"}) {
		t.Errorf("Uncovered = %v", u)
	}
}
