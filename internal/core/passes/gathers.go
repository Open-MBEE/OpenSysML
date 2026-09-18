package passes

import "github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"

// Gathers holds workspace-wide audit state.
type Gathers = kit.Gathers

// NewGathers returns gathers with nothing gathered yet.
func NewGathers() *Gathers { return kit.NewGathers() }

// oosemUnionOf returns the workspace-wide OOSEM union.
func oosemUnionOf(ctx *Context) *oosemUnion {
	return ctx.Gathers().UnionOf(ctx, "oosem", func() kit.Union {
		return newOOSEMUnion()
	}).(*oosemUnion)
}

// mosaUnionOf returns the workspace-wide MOSA union.
func mosaUnionOf(ctx *Context) *mosaUnion {
	return ctx.Gathers().UnionOf(ctx, "mosa", func() kit.Union {
		return newMOSAUnion()
	}).(*mosaUnion)
}
