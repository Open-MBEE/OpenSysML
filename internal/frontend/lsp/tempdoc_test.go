package lsp

import (
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
		return uri.File(name)
	}
	return tempDocURI(t, name)
}
