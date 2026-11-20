package passes

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/identity"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
)

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

// hdbkUnionOf returns the workspace-wide HDBK-1009A union.
func hdbkUnionOf(ctx *Context) *hdbkUnion {
	return ctx.Gathers().UnionOf(ctx, "hdbk1009", func() kit.Regatherer {
		return newHDBKUnion()
	}).(*hdbkUnion)
}

// Contributors names the workspace documents the shared state called name is
// built from — a table of the model's or an audit union's, built on first use
// — and false when name is none of theirs (see kit.ContributorNamer).
func Contributors(ctx *Context, name string) ([]string, bool) {
	if docs, ok := ctx.Model().Contributors(name); ok {
		return docs, true
	}
	switch {
	case strings.HasPrefix(name, "\x00oosem/"):
		return oosemUnionOf(ctx).Contributors(name)
	case strings.HasPrefix(name, "\x00mosa/"):
		return mosaUnionOf(ctx).Contributors(name)
	case strings.HasPrefix(name, hdbkUnionPrefix):
		return hdbkUnionOf(ctx).Contributors(name)
	case strings.HasPrefix(name, signalUnionPrefix):
		return signalUnionOf(ctx).Contributors(name)
	}
	return identity.Contributors(ctx, name)
}
