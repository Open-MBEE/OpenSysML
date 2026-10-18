package libs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestLibraryPathSourceReportsABrokenArchive(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.kpar")
	if err := os.WriteFile(broken, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"archive": broken, "directory of archives": dir} {
		err := NewLoader(libraryPathSource(path), nil).LoadAll(symbols.NewIndex())
		if err == nil || !strings.Contains(err.Error(), "broken.kpar") {
			t.Errorf("%s: LoadAll error = %v, want one naming broken.kpar and its reason", name, err)
		}
	}
}
