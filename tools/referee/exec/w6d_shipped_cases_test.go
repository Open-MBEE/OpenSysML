package exec

import (
	"path/filepath"
	"testing"
)

// The shipped case files are the referee's corpus, so a case naming a model
// that no longer exists, or reusing an id, has to fail here rather than at the
// next run against the pinned artifact.
func TestShippedCaseFilesResolve(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	files, err := readCaseFiles(filepath.Join(repo, filepath.FromSlash(defaultCases)))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no case files found")
	}
	ids := make(map[string]string)
	for _, file := range files {
		if _, err := resolveModels(repo, file.Path, file.Models); err != nil {
			t.Errorf("%s: %v", file.Path, err)
		}
		if len(file.Cases) == 0 {
			t.Errorf("%s: no cases", file.Path)
		}
		for _, c := range file.Cases {
			if where, dup := ids[c.ID]; dup {
				t.Errorf("%s: case id %q also used in %s", file.Path, c.ID, where)
			}
			ids[c.ID] = file.Path
		}
	}
}
