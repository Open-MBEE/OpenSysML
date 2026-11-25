package doccounts

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeSources(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		writeAt(t, root, SpecComplianceDir+"/"+name, text)
	}
	return root
}

func TestSpecComplianceSourcesInPageOrderSkippingReadme(t *testing.T) {
	root := writeSources(t, map[string]string{
		"00-preamble.md":          "# Title\n\nIntro.\n",
		"02-map/01-action.md":     "### Action\n\n| Rule | Status |\n|---|---|\n| a | ✅ Faithful |\n",
		"02-map/00-how-to.md":     "## Map\n\n### How to read\n",
		"01-status.md":            "## Status\n\n| Rule | Status |\n|---|---|\n| s | ⚠️ Approximate |\n",
		"README.md":               "Not a source.\n",
		"03-not-yet-supported.md": "## Not yet\n",
	})
	sources, err := SpecComplianceSources(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		SpecComplianceDir + "/00-preamble.md",
		SpecComplianceDir + "/01-status.md",
		SpecComplianceDir + "/02-map/00-how-to.md",
		SpecComplianceDir + "/02-map/01-action.md",
		SpecComplianceDir + "/03-not-yet-supported.md",
	}
	if !reflect.DeepEqual(sources, want) {
		t.Fatalf("sources %v, want %v", sources, want)
	}
	page, err := ReadSpecCompliance(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := "# Title\n\nIntro.\n\n## Status\n\n| Rule | Status |\n|---|---|\n| s | ⚠️ Approximate |\n\n## Map\n\n### How to read\n\n### Action\n\n| Rule | Status |\n|---|---|\n| a | ✅ Faithful |\n\n## Not yet\n"; page != want {
		t.Fatalf("page:\n%s\nwant:\n%s", page, want)
	}
	counts, err := CountSpecComplianceRules(root)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Total != 2 || counts.Faithful != 1 || counts.Approximate != 1 {
		t.Fatalf("counts %+v", counts)
	}
	whole, err := CountRules(page)
	if err != nil {
		t.Fatal(err)
	}
	if whole != counts {
		t.Fatalf("the sources count %+v, the page %+v", counts, whole)
	}
}

func TestSpecComplianceSourcesRejectStrays(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"a stray file":            {"00-a.md": "## A\n", "notes.md": "## N\n"},
		"an unnumbered directory": {"00-a.md": "## A\n", "extra/00-x.md": "## X\n"},
		"a nested directory":      {"00-a.md": "## A\n", "01-s/02-d/00-x.md": "## X\n"},
		"a non-markdown source":   {"00-a.md": "## A\n", "01-s/00-x.txt": "## X\n"},
		"no trailing newline":     {"00-a.md": "## A"},
		"two trailing newlines":   {"00-a.md": "## A\n\n"},
		"no heading":              {"00-a.md": "A\n"},
	} {
		t.Run(name, func(t *testing.T) {
			root := writeSources(t, files)
			if _, err := ReadSpecCompliance(root); err == nil {
				t.Fatal("want an error")
			}
		})
	}
	root := t.TempDir()
	if _, err := SpecComplianceSources(root); err == nil {
		t.Fatal("want an error for a missing directory")
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(SpecComplianceDir), "01-empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := SpecComplianceSources(root); err == nil || !strings.Contains(err.Error(), "01-empty") {
		t.Fatalf("want an error for an empty section directory, got %v", err)
	}
}

func TestSpecComplianceRuleErrorsNameTheSource(t *testing.T) {
	root := writeSources(t, map[string]string{
		"00-a.md": "## A\n\n| Rule | Status |\n|---|---|\n| a | ✅ Faithful |\n",
		"01-b.md": "## B\n\n| Rule | Status |\n|---|---|\n| b | ✅ ⚠️ |\n",
	})
	_, err := CountSpecComplianceRules(root)
	if err == nil || !strings.Contains(err.Error(), SpecComplianceDir+"/01-b.md:5: 2 status markers") {
		t.Fatalf("want the source named, got %v", err)
	}
}

// TestCheckedInSourcesCountAsOnePage pins that counting the tree's sources one
// by one agrees with counting the page they combine into.
func TestCheckedInSourcesCountAsOnePage(t *testing.T) {
	root := filepath.FromSlash("../../..")
	page, err := ReadSpecCompliance(root)
	if err != nil {
		t.Fatal(err)
	}
	whole, err := CountRules(page)
	if err != nil {
		t.Fatal(err)
	}
	split, err := CountSpecComplianceRules(root)
	if err != nil {
		t.Fatal(err)
	}
	if whole != split || whole.Total == 0 {
		t.Fatalf("the page counts %+v, the sources %+v", whole, split)
	}
}
