package libs

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// loadIntoLogs runs loadInto over a one-file library and returns what it logged.
func loadIntoLogs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := "package Lib { part def Engine; }"
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })
	idx := symbols.NewIndex()
	loadInto(idx, NewDirSource(dir))
	if len(idx.LookupQualified("Lib::Engine")) != 1 {
		t.Fatalf("loadInto did not register Lib::Engine; logs:\n%s", logs.String())
	}
	return logs.String()
}

// A library loaded by a process without a cache root — the snapshot-miss path a
// custom library takes — is loaded without a cache and without a warning.
func TestLoadIntoIsQuietWithoutACacheRoot(t *testing.T) {
	for _, name := range []string{"XDG_CACHE_HOME", "HOME", "LocalAppData", "home"} {
		t.Setenv(name, "")
	}
	if logs := loadIntoLogs(t); strings.Contains(logs, "cache unavailable") {
		t.Errorf("no cache root was warned about:\n%s", logs)
	}
}

// A cache root that exists and refuses the directory is still reported.
func TestLoadIntoReportsARootThatRefuses(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", file)
	if logs := loadIntoLogs(t); !strings.Contains(logs, "stdlib symbol cache unavailable") {
		t.Errorf("a root that refuses was not warned about:\n%s", logs)
	}
}
