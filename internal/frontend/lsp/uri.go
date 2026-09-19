package lsp

import (
	"net/url"
	"path/filepath"
	"strings"

	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// LibraryScheme is the URI scheme the server gives the bundled standard library:
// `sysml-stdlib:///<path>`, where the path is the file's path inside the library
// (`Kernel Libraries/Kernel Semantic Library/Base.kerml`), percent-encoded. A
// client opens one through the opensysml/stdlibContent request; the documents
// are read-only.
const LibraryScheme = "sysml-stdlib"

// uriToName converts an LSP document URI to the workspace document name: an OS
// path for a file URI, the library-relative path for a sysml-stdlib URI.
func uriToName(u uri.URI) string {
	if name, ok := libraryURIName(u); ok {
		return name
	}
	return u.Filename()
}

// nameToURI converts a workspace document name (an OS path) to an LSP URI.
func nameToURI(name string) uri.URI { return uri.File(name) }

// libraryURI is the sysml-stdlib URI of the named bundled library file.
func libraryURI(name string) uri.URI {
	u := url.URL{Scheme: LibraryScheme, Path: "/" + filepath.ToSlash(name)}
	return uri.URI(u.String())
}

// isLibraryURI reports whether u names a bundled library document.
func isLibraryURI(u uri.URI) bool {
	_, ok := libraryURIName(u)
	return ok
}

// libraryURIName is the library-relative path a sysml-stdlib URI names, with
// forward slashes, and whether u is one. A path that is empty or climbs out of
// the library is none.
func libraryURIName(u uri.URI) (string, bool) {
	parsed, err := url.Parse(string(u))
	if err != nil || !strings.EqualFold(parsed.Scheme, LibraryScheme) {
		return "", false
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	if name == "" || parsed.Host != "" {
		return "", false
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", false
		}
	}
	return name, true
}

// document returns the document a request names: the workspace's own, or the
// bundled library file when the name is one of those. Nil for neither.
func (s *Server) document(name string) *model.Document {
	if doc := s.ws.Document(name); doc != nil {
		return doc
	}
	return s.ws.LibraryDocument(name)
}

// documentURI is the URI a location in the named document is reported at: a
// sysml-stdlib URI for a bundled library file, a file URI otherwise.
func (s *Server) documentURI(name string) uri.URI {
	if s.ws.IsLibraryDocument(name) {
		return libraryURI(name)
	}
	return nameToURI(name)
}
