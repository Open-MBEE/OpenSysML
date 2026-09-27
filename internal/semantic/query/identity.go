package query

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// QualifiedIdentity returns the qualified name that identifies sym in index.
func QualifiedIdentity(index *symbols.Index, sym *symbols.Symbol) string {
	if index == nil || sym == nil {
		return ""
	}
	fqn := index.GetFQN(sym)
	if fqn == "" {
		return ""
	}
	for _, segment := range strings.Split(fqn, "::") {
		if segment == "" {
			return ""
		}
	}
	if !slices.Contains(index.LookupQualified(fqn), sym) {
		return ""
	}
	return fqn
}
