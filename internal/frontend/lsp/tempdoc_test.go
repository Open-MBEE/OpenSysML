package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/uri"
)

// tempDocURI names a document in a directory of the test's own, so opening it
// under no folder indexes that directory and never a shared one such as /tmp.
func tempDocURI(t testing.TB, name string) uri.URI {
	t.Helper()
	return uri.File(filepath.Join(t.TempDir(), name))
}

// testDocURI is the URI a helper opens name under: an absolute name as given,
// a bare one in a directory of the test's own.
func testDocURI(t testing.TB, name string) uri.URI {
	t.Helper()
	if filepath.IsAbs(name) {
		u := uri.File(name)
		requireTestOwnedDir(t, u)
		return u
	}
	return tempDocURI(t, name)
}

// requireTestOwnedDir fails a test about to open u in a directory it does not
// own: the working directory, where a bare name lands, or the system temp directory.
func requireTestOwnedDir(t testing.TB, u uri.URI) {
	t.Helper()
	dir := filepath.Dir(u.FsPath())
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if dir == wd || dir == os.TempDir() {
		t.Fatalf("document %s opens in shared directory %s; use tempDocURI", u, dir)
	}
}
