package passes

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// aboutGather names the gather of the `about`-annotated elements no workspace
// document declares — bundled library ones — which join the id space too.
const aboutGather = "\x00identity"

// identityKey is one effective id in one project scope: the unit the identity
// audit reads the union by.
type identityKey struct{ scope, id string }

func identityName(k identityKey) string { return "\x00identity/" + k.scope + "\x01" + k.id }

// identityHit is a declared id landing in the derived id space of the element
// with the key it is filed under.
type identityHit struct {
	info  *identity.Info
	decl  identity.Declaration
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
	byID map[identityKey][]*identity.Info
	hits map[identityKey][]identityHit
}

func newIdentityIndex() *identityIndex {
	return &identityIndex{byID: map[identityKey][]*identity.Info{}, hits: map[identityKey][]identityHit{}}
}

func keyOf(info *identity.Info) identityKey {
	return identityKey{scopeKey(info), info.EffectiveID}
}

// insert files info under its effective id and its declared ids under the
// elements whose derived id spaces they land in.
func (x *identityIndex) insert(info *identity.Info) {
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
func (x *identityIndex) remove(info *identity.Info) {
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
func keysOf(info *identity.Info) []identityKey {
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

// group is the elements sharing one effective id, read by name.
func (x *identityIndex) group(r *resolve.Resolver, k identityKey) []*identity.Info {
	r.ReadName(identityName(k))
	return x.byID[k]
}

// hitsOn is the declared ids landing in one element's derived id space, read by name.
func (x *identityIndex) hitsOn(r *resolve.Resolver, k identityKey) []identityHit {
	r.ReadName(identityName(k))
	return x.hits[k]
}

// identityContribution is what one gather — a document's own elements, or the
// `about`-annotated library elements — adds to the union.
type identityContribution struct {
	table *identity.Table
	infos []*identity.Info
	// facts spells, per key, what the contribution files there, so a regather
	// names only the keys whose entries it moved.
	facts map[identityKey]string
}

// contribute selects the infos of table that pass keep, spelling their facts.
func contribute(table *identity.Table, keep func(*identity.Info) bool) *identityContribution {
	c := &identityContribution{table: table, facts: map[identityKey]string{}}
	lines := map[identityKey][]string{}
	for _, sym := range table.Symbols() {
		info, ok := table.Info(sym)
		if !ok || !keep(info) {
			continue
		}
		c.infos = append(c.infos, info)
		spelled := spellInfo(info)
		for _, k := range keysOf(info) {
			lines[k] = append(lines[k], spelled)
		}
	}
	for k, l := range lines {
		sort.Strings(l)
		c.facts[k] = strings.Join(l, "\n")
	}
	return c
}

// spellInfo spells what the union's readers see of an element.
func spellInfo(info *identity.Info) string {
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

// identityUnion is the identity tables of every workspace document, indexed as
// one id space per project scope and read by name, as oosemUnion is. The
// `about`-annotated elements outside every document are one more contribution.
type identityUnion struct {
	*identityIndex
	perDoc map[string]*identityContribution
	about  *identityContribution
}

func newIdentityUnion() *identityUnion {
	return &identityUnion{identityIndex: newIdentityIndex(), perDoc: map[string]*identityContribution{}}
}

// tableOf is doc's identity table, as gathered.
func (u *identityUnion) tableOf(doc string) *identity.Table {
	if c := u.perDoc[doc]; c != nil {
		return c.table
	}
	return nil
}

// regather replaces doc's contribution — its own elements — with a fresh
// gather, none when doc is no workspace document, naming the keys it moved.
func (u *identityUnion) regather(ctx *Context, g *Gathers, doc string, changed map[string]bool) {
	var cur *identityContribution
	if g.docs[doc] {
		g.gather(ctx, doc, func(root *symbols.Scope) {
			table := identity.Build(ctx.Model(), ctx.Resolver(), root)
			cur = contribute(table, func(info *identity.Info) bool {
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
func (u *identityUnion) regatherAbout(ctx *Context, g *Gathers, changed map[string]bool) {
	var cur *identityContribution
	ctx.Resolver().Gather(aboutGather, func() {
		table := identity.Build(ctx.Model(), ctx.Resolver())
		cur = contribute(table, func(info *identity.Info) bool {
			return !g.docs[info.Symbol.DocName]
		})
	})
	u.replace(u.about, cur, changed)
	u.about = cur
}

// replace swaps one contribution for another in the index, naming the keys
// whose entries differ between them in changed when it is not nil.
func (u *identityUnion) replace(old, cur *identityContribution, changed map[string]bool) {
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
	if changed == nil {
		return
	}
	for k, spelled := range old.factsOf() {
		if cur.factsOf()[k] != spelled {
			changed[identityName(k)] = true
		}
	}
	for k, spelled := range cur.factsOf() {
		if old.factsOf()[k] != spelled {
			changed[identityName(k)] = true
		}
	}
}

func (c *identityContribution) factsOf() map[identityKey]string {
	if c == nil {
		return nil
	}
	return c.facts
}

// including is the union with one more table's elements filed in, for a
// document that is no workspace document — a library one — judged against
// the workspace: the union is copied, not changed.
func (u *identityUnion) including(table *identity.Table) *identityIndex {
	x := newIdentityIndex()
	for k, group := range u.byID {
		x.byID[k] = append([]*identity.Info(nil), group...)
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
func (u *identityUnion) holds(info *identity.Info) bool {
	for _, o := range u.byID[keyOf(info)] {
		if o.Symbol == info.Symbol {
			return true
		}
	}
	return false
}
