package lsp

import (
	"path/filepath"
	"testing"

	"go.lsp.dev/uri"
)

// tempDocURI is the URI of a document named name in a directory of the test's
// own, so a test opening it under no folder has only that directory indexed,
// never a shared one such as the system temp directory and what sits there.
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
