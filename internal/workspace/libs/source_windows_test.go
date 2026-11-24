package libs

import (
	"os"
	"path/filepath"
	"testing"
)

// A directory named from the current drive's root, or on a drive without a root, is
// not absolute on Windows and is not taken from the working directory either.
func TestDirSourceReadsADriveRelativeDirectory(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Real.kerml"), []byte("package Real;"), 0o600); err != nil {
		t.Fatal(err)
	}
	volume := filepath.VolumeName(dir)
	t.Chdir(dir)
	for _, named := range []string{dir[len(volume):], volume + "."} {
		src := &dirSource{dir: named}
		if data, err := src.Read("Real.kerml"); err != nil || string(data) != "package Real;" {
			t.Errorf("Read from %q = %q, %v", named, data, err)
		}
	}
}
