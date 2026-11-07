package query

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// QualifiedIdentity returns the qualified name that identifies sym in index.
func QualifiedIdentity(index *symbols.Index, sym *symbols.Symbol) string {
	return index.QualifiedIdentity(sym)
}
