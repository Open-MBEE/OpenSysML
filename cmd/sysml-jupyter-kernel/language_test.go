package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/buildinfo"
)

// The extension registers each language in one addLanguage call; these read
// the sysml one's fields out of it.
var registration = regexp.MustCompile(`(?s)addLanguage\(\{\s*name: 'sysml',(.*?)\}\)`)

func TestTheLanguageInfoIsWhatTheJupyterLabExtensionRegisters(t *testing.T) {
	info := kernelInfo(buildinfo.Info{Version: "v0.0.0-test"})
	lang := info.Language
	if lang.Name != "sysml" || lang.CodeMirrorMode != "sysml" || lang.MIMEType != "text/x-sysml" || lang.FileExtension != ".sysml" {
		t.Fatalf("language_info = %+v", lang)
	}
	if lang.Version != "2.0" || lang.PygmentsLexer != "text" {
		t.Errorf("language_info = %+v", lang)
	}

	path := filepath.Join("..", "..", "editors", "jupyterlab", "src", "index.ts")
	src, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	m := registration.FindSubmatch(src)
	if m == nil {
		t.Fatalf("%s registers no language named %q", path, lang.CodeMirrorMode)
	}
	fields := string(m[1])
	for _, want := range []string{
		"mime: '" + lang.MIMEType + "'",
		"extensions: ['" + strings.TrimPrefix(lang.FileExtension, ".") + "']",
	} {
		if !strings.Contains(fields, want) {
			t.Errorf("%s registers sysml without %s:\n%s", path, want, fields)
		}
	}
}
