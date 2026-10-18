package repl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAFileGuardIsAskedAboutEveryFileLoadedOrSaved(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(model, []byte("package M { part def P; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("refused by the guard")
	var asked []string
	s := NewSession()
	s.SetFileGuard(func(path string) error {
		asked = append(asked, path)
		if strings.HasSuffix(path, "model.sysml") || strings.HasSuffix(path, "out.sysml") {
			return refused
		}
		return nil
	})
	if _, err := s.LoadPathsReport([]string{dir}); !errors.Is(err, refused) {
		t.Errorf("loading a directory holding a refused file = %v; want the guard's refusal", err)
	}
	if _, err := s.LoadFile(model); !errors.Is(err, refused) {
		t.Errorf("loading a refused file = %v; want the guard's refusal", err)
	}
	s.Submit("package S { part def Q; }")
	lines, written, _ := s.Save(filepath.Join(dir, "out.sysml"))
	if written || len(lines) == 0 || !strings.Contains(lines[0], "refused by the guard") {
		t.Errorf("saving to a refused path = %v, %v; want the refusal and no write", lines, written)
	}
	if _, err := os.Stat(filepath.Join(dir, "out.sysml")); !os.IsNotExist(err) {
		t.Errorf("a refused save wrote the file: %v", err)
	}
	if len(asked) < 3 {
		t.Errorf("the guard was asked %d times (%v); want every file", len(asked), asked)
	}
	s.SetFileGuard(nil)
	if _, err := s.LoadFile(model); err != nil {
		t.Errorf("with the guard lifted the load failed: %v", err)
	}
}
