package passes

import "github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"

// Gathers holds workspace-wide audit state.
type Gathers = kit.Gathers

// NewGathers returns gathers with nothing gathered yet.
func NewGathers() *Gathers { return kit.NewGathers() }

func oosemUnionOf(ctx *Context, a *oosemAudit) *oosemUnion {
	_ = a
	return ctx.Gathers().UnionOf(ctx, "oosem", func() kit.Union {
		return newOOSEMUnion()
	}).(*oosemUnion)
}

func mosaUnionOf(ctx *Context, a *mosaAudit) *mosaUnion {
	_ = a
	return ctx.Gathers().UnionOf(ctx, "mosa", func() kit.Union {
		return newMOSAUnion()
	}).(*mosaUnion)
}

func identityUnionOf(ctx *Context) *identityUnion {
	return ctx.Gathers().UnionOf(ctx, "identity", func() kit.Union {
		return newIdentityUnion()
	}).(*identityUnion)
}
