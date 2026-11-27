package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPuppeteerConfigIsAFileNeverAFlag checks the value of MermaidPuppeteerEnv reaches
// mmdc as an absolute path of a file that exists, whatever it opens with.
func TestPuppeteerConfigIsAFileNeverAFlag(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("-p.json", []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := puppeteerConfig("-p.json")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) || strings.HasPrefix(got, "-") || filepath.Base(got) != "-p.json" {
		t.Errorf("puppeteerConfig(-p.json) = %q, want the absolute path of the file", got)
	}
	var missing *Error
	for _, value := range []string{"--outputFormat=png", "absent.json", dir} {
		if _, err := puppeteerConfig(value); !errors.As(err, &missing) || missing.Kind != ErrorToolMissing || missing.EnvVar != MermaidPuppeteerEnv {
			t.Errorf("puppeteerConfig(%q): err = %v, want %s naming %s", value, err, ErrorToolMissing, MermaidPuppeteerEnv)
		}
	}
}
