package specexamples

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCorpus(t *testing.T, dir, marker string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, marker), []byte(marker), 0o600); err != nil {
		t.Fatal(err)
	}
}

func hasMarker(dir, marker string) bool {
	_, err := os.Stat(filepath.Join(dir, marker))
	return err == nil
}

func TestInstallSwapsAndRecovers(t *testing.T) {
	t.Run("replaces the previous corpus and drops the backup", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "corpus")
		writeCorpus(t, out, "old")
		writeCorpus(t, out+".new", "new")
		if err := install(out+".new", out); err != nil {
			t.Fatal(err)
		}
		if !hasMarker(out, "new") || hasMarker(out, "old") {
			t.Errorf("corpus not replaced")
		}
		if _, err := os.Stat(out + ".old"); !os.IsNotExist(err) {
			t.Errorf("backup kept after a successful install")
		}
	})
	t.Run("an orphaned backup is recovered, never discarded", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "corpus")
		writeCorpus(t, out+".old", "old")
		if err := install(filepath.Join(t.TempDir(), "missing"), out); err == nil {
			t.Fatal("install with no staged corpus succeeded")
		}
		if !hasMarker(out, "old") {
			t.Errorf("the orphaned backup was not restored as the corpus")
		}
		writeCorpus(t, out+".new", "new")
		if err := install(out+".new", out); err != nil {
			t.Fatal(err)
		}
		if !hasMarker(out, "new") {
			t.Errorf("recovered corpus not replaced by the staged one")
		}
	})
	t.Run("a failed install restores the previous corpus", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "corpus")
		writeCorpus(t, out, "old")
		if err := install(filepath.Join(t.TempDir(), "missing"), out); err == nil {
			t.Fatal("install with no staged corpus succeeded")
		}
		if !hasMarker(out, "old") {
			t.Errorf("previous corpus not restored")
		}
	})
}
