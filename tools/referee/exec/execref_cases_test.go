package exec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCaseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.cases")
	content := "# comment\nmodel: models/a.sysml\nmodel: models/b.sysml\none ::  :: 2 + 3\ntwo :: P::Q :: value::member\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readCaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Models) != 2 || got.Models[1].Path != "models/b.sysml" || got.Models[1].Line != 3 || len(got.Cases) != 2 {
		t.Fatalf("readCaseFile() = %+v", got)
	}
	if got.Cases[0].Target != "" || got.Cases[1].Target != "P::Q" {
		t.Fatalf("targets = %+v", got.Cases)
	}
}

// A by-design line marks every case after it with its clause.
func TestReadCaseFileByDesign(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.cases")
	content := "plain ::  :: 1\nby-design: KerML 1.0 §9.3.2.2.8\nexact ::  :: 0.1 + 0.2 == 0.3\nalso ::  :: 1 / 3\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readCaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cases[0].ByDesign != "" || got.Cases[1].ByDesign != "KerML 1.0 §9.3.2.2.8" || got.Cases[2].ByDesign != got.Cases[1].ByDesign {
		t.Fatalf("by-design clauses = %+v", got.Cases)
	}
	if err := os.WriteFile(path, []byte("by-design:\none ::  :: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCaseFile(path); err == nil || !strings.Contains(err.Error(), path+":1: by-design states no clause") {
		t.Fatalf("readCaseFile() error = %v", err)
	}
}

func TestReadCaseFileMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.cases")
	if err := os.WriteFile(path, []byte("bad line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readCaseFile(path)
	if err == nil || !strings.Contains(err.Error(), path+":1:") {
		t.Fatalf("readCaseFile() error = %v", err)
	}
}

func TestReadCaseFileEmptyFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.cases")
	if err := os.WriteFile(path, []byte("id :: target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readCaseFile(path)
	if err == nil || !strings.Contains(err.Error(), "expected id :: target :: expression") {
		t.Fatalf("readCaseFile() error = %v", err)
	}
}

func TestUnknownModelIncludesCaseFileLine(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "bad.cases")
	if err := os.WriteFile(path, []byte("# comment\nmodel: no/such/model.sysml\none ::  :: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := readCaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolveModels(repo, file.Path, file.Models)
	if err == nil || !strings.Contains(err.Error(), path+":2: model no/such/model.sysml:") {
		t.Fatalf("resolveModels() error = %v", err)
	}
}
