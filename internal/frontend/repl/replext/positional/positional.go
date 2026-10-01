// Package positional registers the names OSLC query gives elements without a
// qualified identity: the positional `@N` identities the RDF export writes, so
// a query answers with the identifiers a saved graph holds.
package positional

import (
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

func init() { replext.RegisterPositional(names) }

func names(index *symbols.Index, docs []replext.PositionalDocument, identifies func(*symbols.Symbol) bool) (map[*symbols.Symbol]string, map[string]*symbols.Symbol) {
	out := make([]export.PositionalDocument, 0, len(docs))
	for _, doc := range docs {
		out = append(out, export.PositionalDocument{File: doc.File, Root: doc.Root})
	}
	return export.PositionalIdentities(index, out, identifies)
}
