package passes

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// signalUnion is the names of the signals every workspace document sends,
// counted per document and read by name, as oosemUnion is.
type signalUnion struct {
	perDoc map[string]map[string]bool
	sent   kit.CountSet[string]
}

func newSignalUnion() *signalUnion {
	return &signalUnion{perDoc: map[string]map[string]bool{}, sent: kit.CountSet[string]{}}
}

const signalUnionPrefix = "\x00signals/"

// signalAnySentName is read by a judgment listing every sent name, which any
// change to the set moves.
const signalAnySentName = signalUnionPrefix + "any"

func signalSentName(name string) string { return signalUnionPrefix + "sent/" + name }

// signalUnionOf returns the workspace-wide union of sent signal names.
func signalUnionOf(ctx *Context) *signalUnion {
	return ctx.Gathers().UnionOf(ctx, "signals", func() kit.Regatherer {
		return newSignalUnion()
	}).(*signalUnion)
}

// Regather replaces doc's sent names with a fresh gather — none when doc is no
// workspace document — naming what the union now answers differently.
func (u *signalUnion) Regather(ctx *Context, g *Gathers, doc string, changed map[string]bool) {
	old := u.perDoc[doc]
	var cur map[string]bool
	if g.Gathered(doc) {
		cur = map[string]bool{}
		g.Gather(ctx, doc, func(root *symbols.Scope) { gatherSentSignals(ctx, root, cur) })
	}
	moved := map[string]bool{}
	kit.Move(u.sent, old, cur, signalSentName, moved)
	if changed != nil {
		for name := range moved {
			changed[name] = true
		}
		if len(moved) > 0 {
			changed[signalAnySentName] = true
		}
	}
	if cur == nil {
		delete(u.perDoc, doc)
	} else {
		u.perDoc[doc] = cur
	}
}

// Contributors implements kit.Contributing: the documents that send any signal.
func (u *signalUnion) Contributors(name string) ([]string, bool) {
	if !strings.HasPrefix(name, signalUnionPrefix) {
		return nil, false
	}
	var out []string
	for doc, sent := range u.perDoc {
		if len(sent) > 0 {
			out = append(out, doc)
		}
	}
	return out, true
}
