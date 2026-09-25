package passes

import "github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"

// Gathers holds workspace-wide audit state.
type Gathers = kit.Gathers

// NewGathers returns gathers with nothing gathered yet.
func NewGathers() *Gathers { return kit.NewGathers() }

// oosemUnionOf returns the workspace-wide OOSEM union.
func oosemUnionOf(ctx *Context) *oosemUnion {
	return ctx.Gathers().UnionOf(ctx, "oosem", func() kit.Regatherer {
		return newOOSEMUnion()
	}).(*oosemUnion)
}

// mosaUnionOf returns the workspace-wide MOSA union.
func mosaUnionOf(ctx *Context) *mosaUnion {
	return ctx.Gathers().UnionOf(ctx, "mosa", func() kit.Regatherer {
		return newMOSAUnion()
	}).(*mosaUnion)
}
