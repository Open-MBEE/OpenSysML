package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// bindingReads is what one binding's value read of the model: declarations, what each name
// denoted, each type's hierarchy and the usage census; opaque when a read cannot be replayed.
type bindingReads struct {
	decls  map[*symbols.Symbol]string
	names  map[nameRead]string
	types  map[*symbols.Symbol]string
	census string
	opaque bool
}

func newBindingReads() *bindingReads {
	return &bindingReads{
		decls: make(map[*symbols.Symbol]string),
		names: make(map[nameRead]string),
		types: make(map[*symbols.Symbol]string),
	}
}

// nameQuery is the kind of lookup a name read replays.
type nameQuery uint8

const (
	// queryName resolves a simple name outward from a scope.
	queryName nameQuery = iota
	// queryNameExcluding is queryName with one declaration's own binding hidden.
	queryNameExcluding
	// queryQualified resolves a qualified name written in a scope.
	queryQualified
	// queryConstructed resolves a constructor argument's label against the constructed type.
	queryConstructed
	// queryCandidates lists every declaration a called name may denote.
	queryCandidates
)

// nameRead is one lookup a binding made: which scope it was made from, the name as written,
// and what else the lookup was made against.
type nameRead struct {
	scope     scopeKey
	query     nameQuery
	name      spelledName
	against   spelledName
	excluding *symbols.Symbol
}

// scopeKey names a scope across contexts: by the declaration owning it, or the document
// whose root it is.
type scopeKey struct {
	owner *symbols.Symbol
	doc   string
}

// spelledName is a qualified name as written, rendered so it can be written again.
type spelledName struct {
	parts  string
	global bool
}

const (
	segmentSeparator = "\x00"
	chainedMark      = "\x01"
)

func spell(qn *ast.QualifiedName) spelledName {
	if qn == nil {
		return spelledName{}
	}
	parts := make([]string, len(qn.Parts))
	for i, seg := range qn.Parts {
		parts[i] = seg.Text
		if seg.Chained {
			parts[i] = chainedMark + seg.Text
		}
	}
	return spelledName{parts: strings.Join(parts, segmentSeparator), global: qn.Global}
}

// node writes the spelled name out again as syntax no document owns.
func (s spelledName) node() *ast.QualifiedName {
	if s.parts == "" {
		return nil
	}
	qn := &ast.QualifiedName{Global: s.global}
	for _, text := range strings.Split(s.parts, segmentSeparator) {
		seg := ast.NameSegment{Text: strings.TrimPrefix(text, chainedMark)}
		seg.Chained = seg.Text != text
		qn.Parts = append(qn.Parts, seg)
	}
	return qn
}

// scopeKeyOf names scope for a later context; false for a scope neither a declaration nor a
// document root owns, which no later context finds again.
func (ctx *Context) scopeKeyOf(scope *symbols.Scope) (scopeKey, bool) {
	if scope == nil {
		return scopeKey{}, false
	}
	if owner := scope.Owner(); owner != nil {
		return scopeKey{owner: owner}, true
	}
	idx := ctx.model.resolver.Index()
	if idx != nil && scope.DocName() != "" && idx.DocumentRoot(scope.DocName()) == scope {
		return scopeKey{doc: scope.DocName()}, true
	}
	return scopeKey{}, false
}

// scopeOf finds the scope a key names in this context.
func (ctx *Context) scopeOf(key scopeKey) *symbols.Scope {
	if key.owner != nil {
		return key.owner.Scope
	}
	if idx := ctx.model.resolver.Index(); idx != nil {
		return idx.DocumentRoot(key.doc)
	}
	return nil
}

// readsUnderWay is the record of the innermost binding being made, nil outside one.
func (ctx *Context) readsUnderWay() (*bindingReads, *symbols.Symbol) {
	if len(ctx.bindingStack) == 0 {
		return nil, nil
	}
	top := ctx.bindingStack[len(ctx.bindingStack)-1]
	reads := ctx.bindingReads[top]
	if reads == nil {
		reads = newBindingReads()
		ctx.bindingReads[top] = reads
	}
	return reads, top
}

// noteDeclarationRead records that the binding being made read sym's declaration.
func (ctx *Context) noteDeclarationRead(sym *symbols.Symbol) {
	reads, top := ctx.readsUnderWay()
	if reads == nil || sym == nil || sym == top {
		return
	}
	if _, seen := reads.decls[sym]; !seen {
		reads.decls[sym] = ctx.declarationDigest(sym)
	}
}

// noteCensusRead records that the binding being made walked the model's usage census.
func (ctx *Context) noteCensusRead(census *usageCensus) {
	if reads, _ := ctx.readsUnderWay(); reads != nil && reads.census == "" {
		reads.census = census.digest
	}
}

// noteTypeRead records that the binding being made judged sym as a type: by what it
// specializes, what it holds, or what its value results in.
func (ctx *Context) noteTypeRead(sym *symbols.Symbol) {
	reads, _ := ctx.readsUnderWay()
	if reads == nil || sym == nil {
		return
	}
	if _, seen := reads.types[sym]; !seen {
		reads.types[sym] = ctx.typeDigest(sym)
	}
}

// noteOpaqueRead records a read no later context can replay, so the binding is made again.
func (ctx *Context) noteOpaqueRead() {
	if reads, _ := ctx.readsUnderWay(); reads != nil {
		reads.opaque = true
	}
}

// noteNameRead records what a lookup denoted, keyed so a later context can make it again.
func (ctx *Context) noteNameRead(scope *symbols.Scope, read nameRead, denoted string, ok bool) {
	reads, _ := ctx.readsUnderWay()
	if reads == nil {
		return
	}
	key, keyed := ctx.scopeKeyOf(scope)
	if !keyed || !ok {
		reads.opaque = true
		return
	}
	read.scope = key
	if _, seen := reads.names[read]; !seen {
		reads.names[read] = denoted
	}
}

// denotation renders what a lookup found: nothing, or a declaration by name and kind. An
// unnamed declaration cannot be found again, so it renders as false.
func (ctx *Context) denotation(sym *symbols.Symbol, found bool) (string, bool) {
	if !found || sym == nil {
		return "", true
	}
	fqn := ctx.fqnOf(sym)
	if fqn == "" {
		return "", false
	}
	return fqn + "/" + sym.Kind.String(), true
}

// denotations renders a list of candidates in order, false when one is unnamed.
func (ctx *Context) denotations(syms []*symbols.Symbol) (string, bool) {
	var b strings.Builder
	for _, sym := range syms {
		d, ok := ctx.denotation(sym, true)
		if !ok {
			return "", false
		}
		b.WriteString(d)
		b.WriteString(";")
	}
	return b.String(), true
}

// lookupName is the resolver's LookupName, recorded for the binding being made.
func (ctx *Context) lookupName(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	sym, ok := ctx.model.resolver.LookupName(scope, name)
	denoted, renders := ctx.denotation(sym, ok)
	ctx.noteNameRead(scope, nameRead{query: queryName, name: spelledName{parts: name}}, denoted, renders)
	return sym, ok
}

// lookupNameExcluding is the resolver's LookupNameExcluding with excluding's own binding hidden.
func (ctx *Context) lookupNameExcluding(scope *symbols.Scope, name string, excluding *symbols.Symbol) (*symbols.Symbol, bool) {
	sym, ok := ctx.model.resolver.LookupNameExcluding(scope, name, excluding.Decl)
	denoted, renders := ctx.denotation(sym, ok)
	read := nameRead{query: queryNameExcluding, name: spelledName{parts: name}, excluding: excluding}
	ctx.noteNameRead(scope, read, denoted, renders)
	return sym, ok
}

// resolveQualified is the resolver's ResolveQualified, recorded for the binding being made.
func (ctx *Context) resolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	sym, ok := ctx.model.resolver.ResolveQualified(scope, qn)
	ctx.noteQualifiedRead(scope, qn, sym, ok)
	return sym, ok
}

// readQualified is the resolver's ReadQualified, recorded for the binding being made.
func (ctx *Context) readQualified(scope *symbols.Scope, qn *ast.QualifiedName) resolve.Reading {
	rd := ctx.model.resolver.ReadQualified(scope, qn)
	sym, ok := rd.Symbol()
	ctx.noteQualifiedRead(scope, qn, sym, ok)
	return rd
}

// resolveTarget is the resolver's ResolveTarget; a target that is no qualified name is a
// read no later context spells again.
func (ctx *Context) resolveTarget(scope *symbols.Scope, target ast.Node) (*symbols.Symbol, bool) {
	sym, ok := ctx.model.resolver.ResolveTarget(scope, target)
	if qn := ast.AsQualifiedName(target); qn != nil {
		ctx.noteQualifiedRead(scope, qn, sym, ok)
	} else {
		ctx.noteOpaqueRead()
	}
	return sym, ok
}

// resolveReferenceTarget is the resolver's ResolveReferenceTarget, which hides the referring
// declaration's own binding: a read no later context replays.
func (ctx *Context) resolveReferenceTarget(scope *symbols.Scope, decl, target ast.Node) (*symbols.Symbol, bool) {
	ctx.noteOpaqueRead()
	return ctx.model.resolver.ResolveReferenceTarget(scope, decl, target)
}

// resolveConstructorLabel resolves the label of a constructor argument as a feature of the
// constructed type, recorded for the binding being made.
func (ctx *Context) resolveConstructorLabel(scope *symbols.Scope, typeRef, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	sym, ok := ctx.model.resolver.ResolveReference(resolve.Reference{Scope: scope, QN: qn, Constructed: typeRef})
	denoted, renders := ctx.denotation(sym, ok)
	read := nameRead{query: queryConstructed, name: spell(qn), against: spell(typeRef)}
	ctx.noteNameRead(scope, read, denoted, renders)
	return sym, ok
}

// resolveAliasTarget is the resolver's ResolveAliasTarget; the alias followed is a declaration read.
func (ctx *Context) resolveAliasTarget(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	ctx.noteDeclarationRead(sym)
	return ctx.model.resolver.ResolveAliasTarget(sym)
}

// selectInvocation is the checker's selection of the declaration e calls, recorded by the
// candidates the call chose among and what each of them declares.
func (ctx *Context) selectInvocation(scope *symbols.Scope, e *ast.InvocationExpr, performs semantics.Performs) *semantics.InvocationSelection {
	if reads, _ := ctx.readsUnderWay(); reads != nil {
		ctx.noteInvocationRead(scope, e.Type, ctx.model.resolver.InvocationCandidates(scope, e.Type))
	}
	return passes.SelectInvocation(ctx.model.resolver, ctx.model.semantics, scope, e, performs)
}

// noteInvocationRead records the candidates a call of qn chose among and what each declares.
func (ctx *Context) noteInvocationRead(scope *symbols.Scope, qn *ast.QualifiedName, candidates []*symbols.Symbol) {
	if reads, _ := ctx.readsUnderWay(); reads == nil {
		return
	}
	denoted, renders := ctx.denotations(candidates)
	ctx.noteNameRead(scope, nameRead{query: queryCandidates, name: spell(qn)}, denoted, renders)
	for _, candidate := range candidates {
		ctx.noteDeclarationRead(candidate)
	}
}

// noteQualifiedRead records what a qualified name written in scope denoted.
func (ctx *Context) noteQualifiedRead(scope *symbols.Scope, qn *ast.QualifiedName, sym *symbols.Symbol, ok bool) {
	denoted, renders := ctx.denotation(sym, ok)
	ctx.noteNameRead(scope, nameRead{query: queryQualified, name: spell(qn)}, denoted, renders)
}

// modelConforms is the model's Conforms, recording the type judged for the binding being made.
func (ctx *Context) modelConforms(a, b *symbols.Symbol) bool {
	ctx.noteTypeRead(a)
	return ctx.model.semantics.Conforms(a, b)
}

// replay makes the recorded lookup again in this context and renders what it denotes now.
// The syntax it writes is owned by no document, so what the resolver memoizes about it is dropped.
func (ctx *Context) replay(read nameRead) (string, bool) {
	scope := ctx.scopeOf(read.scope)
	if scope == nil {
		return "", false
	}
	r := ctx.model.resolver
	switch read.query {
	case queryName:
		return ctx.denotation(r.LookupName(scope, read.name.parts))
	case queryNameExcluding:
		return ctx.denotation(r.LookupNameExcluding(scope, read.name.parts, read.excluding.Decl))
	}
	qn := read.name.node()
	if qn == nil {
		return "", false
	}
	var denoted string
	var ok bool
	transient := map[ast.Node]bool{qn: true}
	switch read.query {
	case queryQualified:
		r.Scratch(transient, func() { denoted, ok = ctx.denotation(r.ReadQualified(scope, qn).Symbol()) })
	case queryConstructed:
		typeRef := read.against.node()
		transient[typeRef] = true
		r.Scratch(transient, func() {
			denoted, ok = ctx.denotation(r.ResolveReference(resolve.Reference{Scope: scope, QN: qn, Constructed: typeRef}))
		})
	case queryCandidates:
		r.Scratch(transient, func() { denoted, ok = ctx.denotations(r.InvocationCandidates(scope, qn)) })
	}
	return denoted, ok
}

// typeDigest renders what a declaration resolves to beyond its text: its types, supertypes,
// members, alias target and result types. Contexts agreeing on it judge it alike.
func (ctx *Context) typeDigest(sym *symbols.Symbol) string {
	var b strings.Builder
	ctx.writeTypeDigest(&b, sym, make(map[*symbols.Symbol]bool))
	return b.String()
}

func (ctx *Context) writeTypeDigest(b *strings.Builder, sym *symbols.Symbol, open map[*symbols.Symbol]bool) {
	sem := ctx.model.semantics
	open[sym] = true
	fmt.Fprintf(b, "%s/%s{", ctx.fqnOf(sym), sym.Kind)
	ctx.writeDenoted(b, ":", sem.FeatureTypes(sym))
	ctx.writeDenoted(b, ">", sem.AllSupertypes(sym))
	for _, member := range sem.MembersOf(sym) {
		fmt.Fprintf(b, ";%s=%s/%s", member.Name, ctx.fqnOf(member), member.Kind)
		ctx.writeDenoted(b, ":", sem.FeatureTypes(member))
	}
	if target, aliased := ctx.model.resolver.ResolveAliasTarget(sym); aliased && target != sym {
		fmt.Fprintf(b, "->%s/%s", ctx.fqnOf(target), target.Kind)
	}
	if value, valued := ctx.givenValue(sym); valued {
		ctx.writeDenoted(b, "=", sem.ExprResultTypes(sym.OwnerScope, value))
	}
	// A union's instances are its unioning types', so conformance reads their hierarchies too.
	for _, u := range sem.UnioningTypes(sym) {
		b.WriteString("|")
		if open[u] {
			fmt.Fprintf(b, "%s/%s", ctx.fqnOf(u), u.Kind)
			continue
		}
		ctx.writeTypeDigest(b, u, open)
	}
	b.WriteString("}")
}

func (ctx *Context) writeDenoted(b *strings.Builder, mark string, syms []*symbols.Symbol) {
	for _, s := range syms {
		fmt.Fprintf(b, "%s%s/%s", mark, ctx.fqnOf(s), s.Kind)
	}
}
