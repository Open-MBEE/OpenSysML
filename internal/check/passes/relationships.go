package passes

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// GatherRelationships reads from doc's usages the relationships the
// workspace-wide audits gather from a body, by reference, for its interface
// record to carry; nil when it states none. It fails when an end has no
// reference to restore it by, naming the end.
func GatherRelationships(ctx *Context, doc string) (*symbols.GatheredRelationships, error) {
	root := ctx.Index.DocumentRoot(doc)
	if root == nil {
		return nil, nil
	}
	g := &relationshipRecorder{idx: ctx.Index, out: &symbols.GatheredRelationships{}, oosem: newOOSEMAudit(ctx), mosa: newMOSAAudit(ctx)}
	kit.WalkSymbols(ctx, root, func(sym *symbols.Symbol) {
		if usage, ok := sym.Decl.(*ast.Usage); ok {
			g.gather(sym, usage)
		}
	})
	if g.err != nil {
		return nil, g.err
	}
	sent := map[string]bool{}
	gatherSentSignals(ctx, root, sent)
	g.out.SentSignals = slices.Sorted(maps.Keys(sent))
	if g.out.Empty() {
		return nil, nil
	}
	return g.out, nil
}

// relationshipRecorder collects into out, allocated apart from the recorder so
// the record does not keep the index it was written from reachable.
type relationshipRecorder struct {
	idx   *symbols.Index
	out   *symbols.GatheredRelationships
	oosem *oosemAudit
	mosa  *mosaAudit
	err   error
}

// gather records the relationships a usage states through the profiles loaded:
// a connection's derivation or conformance, an allocation, a satisfaction.
func (g *relationshipRecorder) gather(sym *symbols.Symbol, usage *ast.Usage) {
	switch {
	case usage.Kind == ast.UsageConnection:
		if g.oosem != nil {
			if originals, derived, ok := g.oosem.derivationEnds(sym); ok {
				g.out.Derivations = append(g.out.Derivations, g.ends(sym, originals, derived))
			}
		}
		if g.mosa != nil {
			if standards, conformant, ok := g.mosa.conformanceEnds(sym); ok {
				g.out.Conformances = append(g.out.Conformances, g.ends(sym, standards, conformant))
			}
		}
	case usage.Kind == ast.UsageAllocation && g.oosem != nil:
		if source, destination, ok := g.oosem.allocationEnds(sym, usage); ok {
			g.out.Allocations = append(g.out.Allocations, g.ends(sym, source, destination))
		}
	case usage.Kind == ast.UsageSatisfy && usage.Keyword != "verify" && !usage.IsNegated:
		var s symbols.GatheredSatisfaction
		if g.oosem != nil {
			s.Requirements = g.refs(sym, g.oosem.satisfied(sym, usage))
		}
		if g.mosa != nil {
			s.Satisfiers = g.refs(sym, g.mosa.satisfiers(sym, usage))
		}
		g.out.Satisfactions = append(g.out.Satisfactions, s)
	}
}

func (g *relationshipRecorder) ends(of *symbols.Symbol, sources, targets []*symbols.Symbol) symbols.GatheredEnds {
	return symbols.GatheredEnds{Sources: g.refs(of, sources), Targets: g.refs(of, targets)}
}

func (g *relationshipRecorder) refs(of *symbols.Symbol, syms []*symbols.Symbol) []symbols.ElementRef {
	var out []symbols.ElementRef
	for _, sym := range syms {
		ref, ok := g.idx.RefTo(sym)
		if !ok {
			if g.err == nil {
				g.err = fmt.Errorf("%s at %s:%d relates %s %q at %s:%d, which no reference restores",
					of.Kind, of.DocName, of.DeclSpan.Offset, sym.Kind, sym.Name, sym.DocName, sym.DeclSpan.Offset)
			}
			continue
		}
		out = append(out, ref)
	}
	return out
}
