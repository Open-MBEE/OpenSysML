// Package positional registers the exporter's positional identities
// (`Owner::@3`) with the symbol facts for the frontends that link the exporter.
package positional

import (
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

func init() { symbolfacts.RegisterPositional(names) }

func names(index *symbols.Index, docs []symbolfacts.PositionalDocument, identifies func(*symbols.Symbol) bool) (map[*symbols.Symbol]string, map[string]*symbols.Symbol) {
	out := make([]export.PositionalDocument, 0, len(docs))
	for _, doc := range docs {
		out = append(out, export.PositionalDocument{File: doc.File, Root: doc.Root})
	}
	return export.PositionalIdentities(index, out, identifies)
}
