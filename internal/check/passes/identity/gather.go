package identity

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	ids "github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// identityKey is one effective id in one project scope: the unit the identity
// audit reads the union by.
type identityKey struct{ scope, id string }

// identityJudgment names what the identity audit of doc read of the union: the
// groups its elements are filed in. A regather that moves a group names the
// judgments of every document filed there, and of every non-workspace document.
func identityJudgment(doc string) string { return "\x00identity/" + doc }

const identityJudgments = "\x00identity/*"

// identityHit is a declared id landing in the derived id space of the element
// with the key it is filed under.
type identityHit struct {
	info  *ids.Info
	decl  ids.Declaration
	space string
}

// derivedTarget is the base id whose derived id space (a membership id, an
// expression-node id) a declared id lands in.
type derivedTarget struct{ base, space string }

// derivedTargets lists the derived id spaces a declared id lands in: the
// owner of `…_om`, and of each `…_p…` tail that is a chain of encoded positions.
func derivedTargets(id string) []derivedTarget {
	var out []derivedTarget
	if base, ok := strings.CutSuffix(id, "_om"); ok {
		out = append(out, derivedTarget{base, "the owning-membership id"})
	}
	for i := strings.Index(id, "_p"); i >= 0; {
		if expressionPositions(id[i+2:]) {
			out = append(out, derivedTarget{id[:i], "an expression-node id"})
		}
		next := strings.Index(id[i+1:], "_p")
		if next < 0 {
			break
		}
		i += 1 + next
	}
	return out
}

// identityIndex is a generated id space indexed by effective id: the elements
// sharing each id, and the declared ids landing in each element's derived ids.
type identityIndex struct {
	byID map[identityKey][]*ids.Info
	hits map[identityKey][]identityHit
}

func newIdentityIndex() *identityIndex {
	return &identityIndex{byID: map[identityKey][]*ids.Info{}, hits: map[identityKey][]identityHit{}}
}

func keyOf(info *ids.Info) identityKey {
	return identityKey{scopeKey(info), info.EffectiveID}
}

// insert files info under its effective id and its declared ids under the
// elements whose derived id spaces they land in.
func (x *identityIndex) insert(info *ids.Info) {
	x.byID[keyOf(info)] = append(x.byID[keyOf(info)], info)
	scope := scopeKey(info)
	for _, d := range info.Declarations {
		if !d.Declared || d.ID == "" {
			continue
		}
		for _, t := range derivedTargets(d.ID) {
			k := identityKey{scope, t.base}
			x.hits[k] = append(x.hits[k], identityHit{info, d, t.space})
		}
	}
}

// remove undoes insert.
func (x *identityIndex) remove(info *ids.Info) {
	k := keyOf(info)
	group := x.byID[k]
	for i, o := range group {
		if o == info {
			x.byID[k] = append(group[:i:i], group[i+1:]...)
			break
		}
	}
	if len(x.byID[k]) == 0 {
		delete(x.byID, k)
	}
	scope := scopeKey(info)
	for _, d := range info.Declarations {
		if !d.Declared || d.ID == "" {
			continue
		}
		for _, t := range derivedTargets(d.ID) {
			hk := identityKey{scope, t.base}
			hits := x.hits[hk]
			for i, h := range hits {
				if h.info == info && h.decl == d {
					x.hits[hk] = append(hits[:i:i], hits[i+1:]...)
					break
				}
			}
			if len(x.hits[hk]) == 0 {
				delete(x.hits, hk)
			}
		}
	}
}

// keysOf lists the keys an element's entries are filed under.
func keysOf(info *ids.Info) []identityKey {
	out := []identityKey{keyOf(info)}
	scope := scopeKey(info)
	for _, d := range info.Declarations {
		if !d.Declared || d.ID == "" {
			continue
		}
		for _, t := range derivedTargets(d.ID) {
			out = append(out, identityKey{scope, t.base})
		}
	}
	return out
}

// group is the elements sharing one effective id.
func (x *identityIndex) group(k identityKey) []*ids.Info { return x.byID[k] }

// hitsOn is the declared ids landing in one element's derived id space.
func (x *identityIndex) hitsOn(k identityKey) []identityHit { return x.hits[k] }

// readers names the documents whose elements are filed under k.
func (x *identityIndex) readers(k identityKey, into map[string]bool) {
	for _, info := range x.byID[k] {
		into[info.Symbol.DocName] = true
	}
	for _, h := range x.hits[k] {
		into[h.info.Symbol.DocName] = true
	}
}

// Contribution is what one gather — a document's own elements, or the
// `about`-annotated library elements — adds to the union.
type Contribution struct {
	table *ids.Table
	infos []*ids.Info
}

// contribute selects the infos of table that pass keep.
func contribute(table *ids.Table, keep func(*ids.Info) bool) *Contribution {
	c := &Contribution{table: table}
	for _, sym := range table.Symbols() {
		if info, ok := table.Info(sym); ok && keep(info) {
			c.infos = append(c.infos, info)
		}
	}
	return c
}

// facts spells, per key, what the contribution files there, so a regather
// names only the keys whose entries it moved.
func (c *Contribution) facts() map[identityKey]string {
	if c == nil {
		return nil
	}
	lines := map[identityKey][]string{}
	for _, info := range c.infos {
		spelled := spellInfo(info)
		for _, k := range keysOf(info) {
			lines[k] = append(lines[k], spelled)
		}
	}
	facts := make(map[identityKey]string, len(lines))
	for k, l := range lines {
		sort.Strings(l)
		facts[k] = strings.Join(l, "\n")
	}
	return facts
}

// spellInfo spells what the union's readers see of an element.
func spellInfo(info *ids.Info) string {
	var b strings.Builder
	b.WriteString(info.FQN)
	b.WriteString("\x01")
	b.WriteString(info.EffectiveID)
	if info.Annotated {
		b.WriteString("\x01@")
	}
	for _, d := range info.Declarations {
		if d.Declared {
			b.WriteString("\x01")
			b.WriteString(d.ID)
		}
	}
	return b.String()
}

// Union is the identity tables of every workspace document, indexed as
// one id space per project scope and read by name, as oosemUnion is. The
// `about`-annotated elements outside every document are one more contribution.
type Union struct {
	*identityIndex
	perDoc map[string]*Contribution
	about  *Contribution
}

func newUnion() *Union {
	return &Union{identityIndex: newIdentityIndex(), perDoc: map[string]*Contribution{}}
}

// tableOf is doc's identity table, as gathered.
func (u *Union) tableOf(doc string) *ids.Table {
	if c := u.perDoc[doc]; c != nil {
		return c.table
	}
	return nil
}

// regather replaces doc's contribution — its own elements — with a fresh
// gather, none when doc is no workspace document, naming the keys it moved.
func (u *Union) Regather(ctx *kit.Context, g *kit.Gathers, doc string, changed map[string]bool) {
	var cur *Contribution
	if g.Gathered(doc) {
		g.Gather(ctx, doc, func(root *symbols.Scope) {
			table := ids.Build(ctx.Model(), ctx.Resolver(), root)
			cur = contribute(table, func(info *ids.Info) bool {
				return info.Symbol.DocName == doc
			})
		})
	}
	u.replace(u.perDoc[doc], cur, changed)
	if cur == nil {
		delete(u.perDoc, doc)
	} else {
		u.perDoc[doc] = cur
	}
}

// regatherAbout replaces the contribution of the `about`-annotated elements no
// workspace document declares.
func (u *Union) RegatherAbout(ctx *kit.Context, g *kit.Gathers, changed map[string]bool) {
	var cur *Contribution
	ctx.Resolver().Gather(kit.AboutGather, func() {
		table := ids.Build(ctx.Model(), ctx.Resolver())
		cur = contribute(table, func(info *ids.Info) bool {
			return !g.Gathered(info.Symbol.DocName)
		})
	})
	u.replace(u.about, cur, changed)
	u.about = cur
}

// replace swaps one contribution for another in the index, naming in changed,
// when it is not nil, the judgments that read a group the swap moved.
func (u *Union) replace(old, cur *Contribution, changed map[string]bool) {
	var moved []identityKey
	if changed != nil {
		before, after := old.facts(), cur.facts()
		for k, spelled := range before {
			if after[k] != spelled {
				moved = append(moved, k)
			}
		}
		for k := range after {
			if _, had := before[k]; !had {
				moved = append(moved, k)
			}
		}
	}
	readers := map[string]bool{}
	for _, k := range moved {
		u.readers(k, readers)
	}
	if old != nil {
		for _, info := range old.infos {
			u.remove(info)
		}
	}
	if cur != nil {
		for _, info := range cur.infos {
			u.insert(info)
		}
	}
	for _, k := range moved {
		u.readers(k, readers)
	}
	if len(moved) > 0 {
		for doc := range readers {
			changed[identityJudgment(doc)] = true
		}
		changed[identityJudgments] = true
	}
}

// judged reads the union for the identity audit of doc, whose table it returns
// when doc is a workspace document.
func (u *Union) judged(r *resolve.Resolver, doc string) *ids.Table {
	if t := u.tableOf(doc); t != nil {
		r.ReadName(identityJudgment(doc))
		return t
	}
	r.ReadName(identityJudgments)
	return nil
}

// including is the union with one more table's elements filed in, for a
// document that is no workspace document — a library one — judged against
// the workspace: the union is copied, not changed.
func (u *Union) including(table *ids.Table) *identityIndex {
	x := newIdentityIndex()
	for k, group := range u.byID {
		x.byID[k] = append([]*ids.Info(nil), group...)
	}
	for k, hits := range u.hits {
		x.hits[k] = append([]identityHit(nil), hits...)
	}
	for _, sym := range table.Symbols() {
		if info, ok := table.Info(sym); ok && !u.holds(info) {
			x.insert(info)
		}
	}
	return x
}

// holds reports whether the union files an element for info's symbol.
func (u *Union) holds(info *ids.Info) bool {
	for _, o := range u.byID[keyOf(info)] {
		if o.Symbol == info.Symbol {
			return true
		}
	}
	return false
}

// Contributions returns the per-document gathers the union currently holds.
func (u *Union) Contributions() map[string]*Contribution {
	return u.perDoc
}

// unionOf returns the workspace-wide identity union.
func unionOf(ctx *kit.Context) *Union {
	return ctx.Gathers().UnionOf(ctx, "identity", func() kit.Union {
		return newUnion()
	}).(*Union)
}
