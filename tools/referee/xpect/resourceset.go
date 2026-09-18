package xpect

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// resourceSet is the workspace one fixture's XPECT_SETUP declares. The pilot's
// Xpect runner loads exactly that set — the fixture, its declared sources and
// the specific `/library*` files it names — so the oracle does the same rather
// than substituting our whole embedded standard library.
type resourceSet struct {
	ws *model.Workspace
	// libraryRoots names the root packages the declared library files bring in.
	libraryRoots []string
	// missing lists declared resources absent from the download.
	missing []string
}

// loadResourceSet builds the workspace for f: declared library files are indexed
// as library content first, then the declared sources and the fixture itself are
// opened, so a source's references see the library it asked for and no other.
func loadResourceSet(suiteDir string, f xtFile, libs *libraryCache) resourceSet {
	set := resourceSet{}
	idx := symbols.NewIndex()

	var sources []struct {
		rel     string
		content []byte
	}
	for _, r := range f.Resources {
		if r.ThisFile {
			continue
		}
		rel := resourcePath(f.Path, r.From)
		content, err := readResource(suiteDir, rel)
		if err != nil {
			set.missing = append(set.missing, r.From)
			continue
		}
		if isLibrary(r.From) {
			if root := libs.parse(suiteDir, rel, content); root != nil {
				idx.AddDocumentWithKind(rel, root, source.KindOf(rel))
				idx.MarkLibraryDocument(rel, symbols.LibraryDocument{Tier: pilotLibraryTier(r.From), Digest: symbols.TextDigest(content)})
				set.libraryRoots = append(set.libraryRoots, rootPackagesIn(content)...)
			}
			continue
		}
		sources = append(sources, struct {
			rel     string
			content []byte
		}{rel, content})
	}
	idx.ExpandWildcardImports()

	set.ws = model.NewWorkspaceWithIndex(idx)
	for _, s := range sources {
		set.ws.Open(s.rel, s.content, 1)
	}
	set.ws.Open(strings.TrimSuffix(f.Path, ".xt"), f.Content, 1)

	sort.Strings(set.missing)
	set.missing = dedupe(set.missing)
	sort.Strings(set.libraryRoots)
	set.libraryRoots = dedupe(set.libraryRoots)
	return set
}

// pilotLibraryTier classifies a declared `/library*` resource by the pilot's
// layout: `/library.systems` and `/library.domain` are the Systems and Domain
// libraries; `/library` and `/library.kernel` hold the Kernel tiers together.
func pilotLibraryTier(from string) symbols.LibraryTier {
	switch {
	case strings.HasPrefix(from, "/library.systems/"):
		return symbols.TierSystems
	case strings.HasPrefix(from, "/library.domain/"):
		return symbols.TierDomain
	default:
		return symbols.TierLibrary
	}
}

// resourcePath resolves a declared resource: a leading slash is suite-relative,
// anything else is beside the .xt file.
func resourcePath(xtPath, from string) string {
	if strings.HasPrefix(from, "/") {
		return strings.TrimPrefix(from, "/")
	}
	return path.Join(path.Dir(xtPath), from)
}

func readResource(suiteDir, rel string) ([]byte, error) {
	// #nosec G304 -- the suite directory is named on the command line.
	return os.ReadFile(filepath.Join(suiteDir, filepath.FromSlash(rel)))
}

// libraryCache memoizes the parse of a declared library file. Every fixture
// declares its own library set, and the suites' copies are read-only, so the
// same file is parsed once and its immutable AST indexed into each workspace.
type libraryCache struct {
	mu    sync.Mutex
	roots map[string]*ast.RootNamespace
}

func newLibraryCache() *libraryCache {
	return &libraryCache{roots: map[string]*ast.RootNamespace{}}
}

func (c *libraryCache) parse(suiteDir, rel string, content []byte) *ast.RootNamespace {
	key := suiteDir + "\x00" + rel
	c.mu.Lock()
	defer c.mu.Unlock()
	if root, ok := c.roots[key]; ok {
		return root
	}
	root := parser.New(source.New(rel, content)).ParseFile()
	c.roots[key] = root
	return root
}
