package libs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbedSourceListsAndReads(t *testing.T) {
	src := DefaultSource()
	names := src.List()

	// Check that the bundled standard and OpenSysML library files are present.
	if len(names) == 0 {
		t.Fatal("expected embedded stdlib files, got empty list")
	}

	// Look for ScalarValues.kerml (in Kernel Data Type Library subdirectory)
	found := false
	for _, n := range names {
		if n == "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ScalarValues.kerml in embedded list, got %d files", len(names))
	}

	data, err := src.Read("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty embedded library content")
	}

	t.Logf("Found %d stdlib files", len(names))
}

func TestDirSourceOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Custom.kerml"), []byte("package Custom;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(LibraryPathEnvVar, dir)
	src := DefaultSource()
	names := src.List()
	if len(names) != 1 || names[0] != "Custom.kerml" {
		t.Fatalf("expected [Custom.kerml] from override dir, got %v", names)
	}
	data, err := src.Read("Custom.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(data) != "package Custom;\n" {
		t.Fatalf("unexpected override content: %q", data)
	}
}

func TestDirSourceRejectsPathsOutsideBase(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "libs")
	sibling := filepath.Join(root, "libs-evil")
	for _, dir := range []string{base, sibling} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sibling, "Secret.kerml"), []byte("package Secret;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Outside.kerml"), []byte("package Outside;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	src := NewDirSource(base)
	// A sibling directory whose name merely starts with the base directory's
	// name is not contained in it, and neither is a parent traversal.
	for _, name := range []string{"../libs-evil/Secret.kerml", "../Outside.kerml", "..", "/etc/passwd"} {
		if _, err := src.Read(name); err == nil {
			t.Errorf("Read(%q) succeeded, want path rejection", name)
		}
	}

	if err := os.WriteFile(filepath.Join(base, "Ok.kerml"), []byte("package Ok;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Read("Ok.kerml"); err != nil {
		t.Fatalf("Read of contained file: %v", err)
	}
}

func TestReadUnknownFileErrors(t *testing.T) {
	src := DefaultSource()
	if _, err := src.Read("Nope.kerml"); err == nil {
		t.Fatal("expected error reading unknown library file")
	}
}

// A library file that is a symbolic link out of the directory is not read: the
// containment the directory source promises holds for what the name resolves to.
func TestDirSourceRejectsSymlinksOutsideBase(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "Secret.kerml"), []byte("package Secret;"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "Real.kerml"), []byte("package Real;"), 0o600); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"Out.kerml":    filepath.Join(outside, "Secret.kerml"),
		"outdir":       outside,
		"Inside.kerml": "Real.kerml",
	} {
		if err := os.Symlink(target, filepath.Join(base, link)); err != nil {
			t.Skipf("symbolic links are not available: %v", err)
		}
	}
	src := &dirSource{dir: base}
	for _, name := range []string{"Out.kerml", filepath.Join("outdir", "Secret.kerml")} {
		if data, err := src.Read(name); err == nil {
			t.Errorf("Read(%q) = %q, want an error for a link out of the directory", name, data)
		}
	}
	if data, err := src.Read("Inside.kerml"); err != nil || string(data) != "package Real;" {
		t.Errorf("Read of a link within the directory = %q, %v", data, err)
	}
}
