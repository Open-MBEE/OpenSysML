package passes

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// hdbkRelation is a relationship of the Handbook's metamodel, read from one of its ends.
type hdbkRelation int

const (
	hdbkDerivedFrom hdbkRelation = iota
	hdbkOriginOf
	hdbkTracedTo
	hdbkRefines
	hdbkRefinedBy
	hdbkVerifiedBy
	hdbkSatisfies
	hdbkSatisfiedBy
)

// hdbkLink is one element related to an element of one kind; hdbkNone stands for any element.
type hdbkLink struct {
	relation hdbkRelation
	element  symbols.ElementKey
	kind     hdbkKind
}

// hdbkFacts is what the HDBK-1009A audit gathers from one document: the kinds
// it declares and the relationships it states.
type hdbkFacts struct {
	present map[hdbkKind]bool
	several map[hdbkKind]bool // kinds the document declares more than once
	links   map[hdbkLink]bool
}

func newHDBKFacts() *hdbkFacts {
	return &hdbkFacts{present: map[hdbkKind]bool{}, several: map[hdbkKind]bool{}, links: map[hdbkLink]bool{}}
}

// hdbkUnion is the HDBK-1009A facts of every workspace document, counted per
// document and read by name, as oosemUnion is.
type hdbkUnion struct {
	perDoc  map[string]*hdbkFacts
	present kit.CountSet[hdbkKind]
	several kit.CountSet[hdbkKind]
	links   kit.CountSet[hdbkLink]
}

func newHDBKUnion() *hdbkUnion {
	return &hdbkUnion{
		perDoc:  map[string]*hdbkFacts{},
		present: kit.CountSet[hdbkKind]{},
		several: kit.CountSet[hdbkKind]{},
		links:   kit.CountSet[hdbkLink]{},
	}
}

const hdbkUnionPrefix = "\x00hdbk1009/"

func hdbkPresentName(k hdbkKind) string { return hdbkUnionPrefix + "present/" + strconv.Itoa(int(k)) }
func hdbkPeersName(k hdbkKind) string   { return hdbkUnionPrefix + "peers/" + strconv.Itoa(int(k)) }
func hdbkLinkName(l hdbkLink) string {
	return hdbkUnionPrefix + "link/" + strconv.Itoa(int(l.relation)) + "/" + strconv.Itoa(int(l.kind)) + "/" + l.element.String()
}

// Regather replaces doc's facts with a fresh gather — none when doc is no
// workspace document — naming what the union now answers differently.
func (u *hdbkUnion) Regather(ctx *Context, g *Gathers, doc string, changed map[string]bool) {
	a := newHDBKAudit(ctx)
	if a == nil {
		return
	}
	old := u.perDoc[doc]
	var cur *hdbkFacts
	if g.Gathered(doc) {
		cur = newHDBKFacts()
		a.facts = cur
		g.Gather(ctx, doc, a.gather)
		a.facts = nil
	}
	peers := map[hdbkKind]bool{}
	for _, set := range []map[hdbkKind]bool{old.presentSet(), cur.presentSet()} {
		for k := range set {
			peers[k] = u.hasPeers(k)
		}
	}
	kit.Move(u.present, old.presentSet(), cur.presentSet(), hdbkPresentName, changed)
	kit.Move(u.several, old.severalSet(), cur.severalSet(), hdbkPeersName, nil)
	for k, was := range peers {
		if changed != nil && u.hasPeers(k) != was {
			changed[hdbkPeersName(k)] = true
		}
	}
	kit.Move(u.links, old.linkSet(), cur.linkSet(), hdbkLinkName, changed)
	if cur == nil {
		delete(u.perDoc, doc)
	} else {
		u.perDoc[doc] = cur
	}
}

// Contributors implements kit.ContributorNamer: the documents with any HDBK-1009A facts.
func (u *hdbkUnion) Contributors(name string) ([]string, bool) {
	if !strings.HasPrefix(name, hdbkUnionPrefix) {
		return nil, false
	}
	var out []string
	for doc, f := range u.perDoc {
		if len(f.present)+len(f.links) > 0 {
			out = append(out, doc)
		}
	}
	return out, true
}

func (f *hdbkFacts) presentSet() map[hdbkKind]bool {
	if f == nil {
		return nil
	}
	return f.present
}

func (f *hdbkFacts) severalSet() map[hdbkKind]bool {
	if f == nil {
		return nil
	}
	return f.several
}

// hasPeers reports a kind the workspace declares more than once, in one document or across two.
func (u *hdbkUnion) hasPeers(k hdbkKind) bool { return u.several.Has(k) || u.present[k] > 1 }

func (f *hdbkFacts) linkSet() map[hdbkLink]bool {
	if f == nil {
		return nil
	}
	return f.links
}

// The reads a judgment makes: the union by name, so a regather that flips the
// answer drops the reader, plus the facts of a root gathered for this run alone.

func (a *hdbkAudit) hasKind(k hdbkKind) bool {
	a.ctx.Resolver().ReadName(hdbkPresentName(k))
	return a.union.present.Has(k) || a.local.presentSet()[k]
}

// hasPeers reports whether an element of kind k has another of its kind beside it.
func (a *hdbkAudit) hasPeers(k hdbkKind) bool {
	a.ctx.Resolver().ReadName(hdbkPeersName(k))
	return a.union.hasPeers(k) || a.local.severalSet()[k] || a.local.presentSet()[k] && a.union.present.Has(k)
}

func (a *hdbkAudit) linked(relation hdbkRelation, element symbols.ElementKey, kinds ...hdbkKind) bool {
	for _, k := range kinds {
		l := hdbkLink{relation, element, k}
		a.ctx.Resolver().ReadName(hdbkLinkName(l))
		if a.union.links.Has(l) || a.local.linkSet()[l] {
			return true
		}
	}
	return false
}
