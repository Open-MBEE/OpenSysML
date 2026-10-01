package grpc

import (
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func NewSymbolContext(idx *symbols.Index) *symbolfacts.Context {
	return symbolfacts.NewContext(idx)
}
