package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
)

const (
	selfModelDir            = "../../examples/self-model"
	markupToolMissingCode   = "document-plan-markup-tool-missing"
	markupToolMissingAdvice = "the AsciiDoc converter asciidoctor-json was not found on PATH (" + markup.AsciiDocEnv + " is unset); run ./scripts/download-doc-pdf-toolchain.sh and point " + markup.AsciiDocEnv + " at build/doc-pdf/asciidoctor/asciidoctor-json.js"
)

// hostEnv is the environment of a host whose AsciiDoc converter is converter
// — none when empty: the variable unset and nothing on PATH — with a record
// cache of its own, so no verdict of another host's run is held.
func hostEnv(t *testing.T, converter string) []string {
	t.Helper()
	env := []string{markup.AsciiDocEnv + "=" + converter, "XDG_CACHE_HOME=" + t.TempDir()}
	if converter == "" {
		env = append(env, "PATH="+t.TempDir())
	}
	return env
}

func selfModelFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{"*.sysml", "asciidoc/*.sysml"} {
		matches, err := filepath.Glob(filepath.Join(selfModelDir, pattern))
		if err != nil || len(matches) == 0 {
			t.Fatalf("glob %s: %v, %d files", pattern, err, len(matches))
		}
		files = append(files, matches...)
	}
	return files
}

// TestValidateWithoutMarkupConverterWarns checks that a model whose prose
// needs a converter the host lacks still validates: one warning names the
// converter, where it was looked for and the remedy, and the exit status is
// 0 unless -warnings-as-errors asks otherwise; a rendering, which needs the
// body, fails.
func TestValidateWithoutMarkupConverterWarns(t *testing.T) {
	binary := buildCLI(t)

	result := runSysml(t, binary, hostEnv(t, ""), "-validate", selfModelDir)
	wantReport(t, result, 0, "warning["+markupToolMissingCode+"]", "carries 3 asciidoc bodies that were not checked", markupToolMissingAdvice, "no errors")
	if got := strings.Count(result.output(), "warning["); got != 1 {
		t.Errorf("%d warnings, want exactly one:\n%s", got, result.output())
	}

	result = runSysml(t, binary, hostEnv(t, ""), "-validate", "-warnings-as-errors", selfModelDir)
	wantReport(t, result, 2, "warning["+markupToolMissingCode+"]", "-warnings-as-errors")

	result = runSysml(t, binary, hostEnv(t, ""), "-validate", "-format", "sarif", selfModelDir)
	if result.status != 0 || !strings.Contains(result.stdout, `"level": "warning"`) || !strings.Contains(result.stdout, markupToolMissingCode) {
		t.Errorf("-format sarif exited %d without the warning:\n%s", result.status, result.output())
	}

	args := append(selfModelFiles(t), "-render-document", "OpenSysMLAsciiDoc::MarkupNote")
	result = runSysml(t, binary, hostEnv(t, ""), args...)
	wantReport(t, result, 2, "carries a asciidoc body that could not be parsed: "+markupToolMissingAdvice)
}

// TestValidateWithMarkupConverterChecksBodies runs with the converter the
// environment names, as the PDF toolchain job does, and wants the bodies
// checked: no warning. Without one it skips, unless the job requires it.
func TestValidateWithMarkupConverterChecksBodies(t *testing.T) {
	converter := os.Getenv(markup.AsciiDocEnv)
	if converter == "" {
		if required := os.Getenv("OPENSYSML_REQUIRE_PDF_TOOLCHAIN"); required != "" {
			t.Fatalf("OPENSYSML_REQUIRE_PDF_TOOLCHAIN=%s but %s is unset", required, markup.AsciiDocEnv)
		}
		t.Skipf("%s is unset", markup.AsciiDocEnv)
	}
	binary := buildCLI(t)
	result := runSysml(t, binary, hostEnv(t, converter), "-validate", "-warnings-as-errors", selfModelDir)
	wantReport(t, result, 0, "no errors")
	if strings.Contains(result.output(), markupToolMissingCode) {
		t.Errorf("the converter %s was found, yet a body went unchecked:\n%s", converter, result.output())
	}
}
