package grpc

import (
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// NewSymbolContext builds the symbolfacts conversion context for cached-model symbols.
func NewSymbolContext(idx *symbols.Index) *symbolfacts.Context {
	return symbolfacts.NewContext(idx)
}
