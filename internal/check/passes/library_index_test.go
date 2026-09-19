package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// newTestIndex returns an index holding the bundled standard library, which is
// the resource set a workspace analyzes a document in: an overlay over the
// process-wide frozen base, as a model gets in production.
func newTestIndex() *symbols.Index {
	return libs.NewModelIndex()
}

// newTestIndexFromDoc is newTestIndex with root indexed under name.
func newTestIndexFromDoc(name string, root *ast.RootNamespace) *symbols.Index {
	idx := newTestIndex()
	idx.AddDocument(name, root)
	return idx
}
