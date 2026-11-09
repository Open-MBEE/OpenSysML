// Package positional registers the REPL's positional namer, which names the
// unnamed members of a query the way the export and the symbol facts do.
package positional

import (
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func init() { replext.RegisterPositional(names) }

func names(index *symbols.Index, docs []replext.PositionalDocument, identifies func(*symbols.Symbol) bool) (map[*symbols.Symbol]string, map[string]*symbols.Symbol) {
	out := make([]naming.Document, 0, len(docs))
	for _, doc := range docs {
		out = append(out, naming.Document{File: doc.File, Root: doc.Root, Scope: doc.Scope})
	}
	return naming.PositionalIdentities(index, out, identifies)
}
