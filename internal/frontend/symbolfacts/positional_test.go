package symbolfacts

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// The tests derive positional ids the way the service does, through the exporter.
func init() {
	RegisterPositional(func(index *symbols.Index, docs []PositionalDocument, identifies func(*symbols.Symbol) bool) (map[*symbols.Symbol]string, map[string]*symbols.Symbol) {
		out := make([]export.PositionalDocument, 0, len(docs))
		for _, doc := range docs {
			out = append(out, export.PositionalDocument{File: doc.File, Root: doc.Root})
		}
		return export.PositionalIdentities(index, out, identifies)
	})
}
