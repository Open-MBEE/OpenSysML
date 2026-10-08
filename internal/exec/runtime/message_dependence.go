package runtime

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// signalConformers is what may carry an accept's type: the declarations
// conforming to it and every name or alias naming one; known is false where the
// model cannot be searched for them.
type signalConformers struct {
	decls map[ast.Node]bool
	names map[string]bool
	known bool
}

// channelsMeet is which message operations of two executors, run by objects a
// and b, may not commute. Two sends always do; a send and a consumer, or two
// consumers, are apart only where each type resolves and no message carries both.
func (ctx *Context) channelsMeet(a, b *Instance) lower.ChannelsMeet {
	return func(x lower.Channel, xSends bool, y lower.Channel, ySends bool) bool {
		switch {
		case xSends && ySends:
			return !(x.Own && y.Own && objectsApart(a, b))
		case xSends:
			return ctx.sendMeets(x, a, y, b)
		case ySends:
			return ctx.sendMeets(y, b, x, a)
		}
		return ctx.consumersMeet(x, a, y, b)
	}
}

// relation is what is known of executors a and b, which two moves belong to:
// whether they are two, run for two objects, and which message operations meet.
func (ctx *Context) relation(a, b clockWaiter) lower.Relation {
	sa, sb := executorSelf(a), executorSelf(b)
	rel := lower.Relation{Channels: ctx.channelsMeet(sa, sb), Apart: a != b}
	if rel.Apart && objectsApart(sa, sb) {
		rel.Objects = true
		rel.Held = func(p, q lower.Place) bool {
			x, y := ctx.slotHolder(a, p.Sym), ctx.slotHolder(b, q.Sym)
			return x != nil && y != nil && x != y
		}
	}
	return rel
}

// objectsApart reports whether a and b are two objects.
func objectsApart(a, b *Instance) bool {
	return objectID(a) != 0 && objectID(b) != 0 && objectID(a) != objectID(b)
}

// ownsFeature reports whether sym is a feature of the object's own, so a name written
// alone and resolving to it, in a behavior the object runs, names the object's slot.
func (ctx *Context) ownsFeature(inst *Instance, sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	for _, of := range ctx.FeaturesOfObject(inst) {
		if of.Feature.Symbol == sym {
			return true
		}
	}
	return false
}

// slotHolder is what holds the value a name resolving to sym names in a move of
// w's: the machine's own data or performance for its attribute, the object w runs
// for for that object's feature; nil when neither is known.
func (ctx *Context) slotHolder(w clockWaiter, sym *symbols.Symbol) any {
	if sym == nil {
		return nil
	}
	if e, ok := w.(*StateExecutor); ok {
		for _, attr := range e.graph.Attributes {
			if attr.Node == sym.Decl {
				if e.occurrence != nil {
					return e.occurrence.ID
				}
				return e
			}
		}
		for _, attrs := range e.graph.StateAttributes {
			for _, attr := range attrs {
				if attr.Node == sym.Decl {
					return e
				}
			}
		}
	}
	if self := executorSelf(w); ctx.ownsFeature(self, sym) {
		return self.ID
	}
	return nil
}

// executorSelf is the object an executor runs for, nil for none.
func executorSelf(w clockWaiter) *Instance {
	switch exec := w.(type) {
	case *ActionExecutor:
		return exec.self
	case *StateExecutor:
		return exec.self
	}
	return nil
}

// sendMeets reports whether a send by an executor run for from may give or take
// a message a consumer run for at would see: a message for the sending object
// reaches no other's, and a dispatch may drop one sent to its object, never one a port carries.
func (ctx *Context) sendMeets(send lower.Channel, from *Instance, consumer lower.Channel, at *Instance) bool {
	if send.Own && objectsApart(from, at) {
		return false
	}
	if consumer.Drops {
		return !send.Via
	}
	sent, want := ctx.sentSignal(send), ctx.channelType(consumer)
	if sent == nil || want == nil {
		return true
	}
	return ctx.signalConforms(sent, want)
}

// consumersMeet reports whether two consumers may compete for one message. One
// object's consumers always do: a machine yields to, or drops for, its siblings.
func (ctx *Context) consumersMeet(x lower.Channel, a *Instance, y lower.Channel, b *Instance) bool {
	if objectID(a) == 0 || objectID(b) == 0 || objectID(a) == objectID(b) {
		return true
	}
	if x.Drops || y.Drops {
		other, holder := y, b
		if y.Drops {
			other, holder = x, a
		}
		// A dispatch drops only portless messages to its object; another's own port takes none of them.
		return other.Port != "" && !ctx.ownPort(other, holder)
	}
	tx, ty := ctx.channelType(x), ctx.channelType(y)
	if tx == nil || ty == nil {
		return true
	}
	if ctx.signalConforms(tx, ty) || ctx.signalConforms(ty, tx) {
		return true
	}
	cx, cy := ctx.conformersOf(tx), ctx.conformersOf(ty)
	if !cx.known || !cy.known {
		return true
	}
	for decl := range cx.decls {
		if cy.decls[decl] {
			return true
		}
	}
	// A message no definition types matches by its name, the last segment written.
	nx, ny := withName(cx.names, lastWritten(x.Type)), withName(cy.names, lastWritten(y.Type))
	for name := range nx {
		if ny[name] {
			return true
		}
	}
	return false
}

// ownPort reports whether a consumer's port statically names a port feature of
// its own object, so the port's holder is that object.
func (ctx *Context) ownPort(c lower.Channel, self *Instance) bool {
	if self == nil || c.Port == "" || strings.Contains(c.Port, ".") {
		return false
	}
	sym, ok := ctx.portSymbol(c.Scope, c.Port)
	if !ok || sym == nil || sym.Kind != symbols.SymbolPortUsage {
		return false
	}
	if usage, isUsage := sym.Decl.(*ast.Usage); !isUsage || usage.IsReference {
		return false
	}
	for _, of := range ctx.FeaturesOfObject(self) {
		if of.Name == c.Port && isPortFeature(of.Feature) && of.Feature.Symbol == sym {
			return true
		}
	}
	return false
}

// withName is names with one more.
func withName(names map[string]bool, name string) map[string]bool {
	out := make(map[string]bool, len(names)+1)
	for n := range names {
		out[n] = true
	}
	out[name] = true
	return out
}

// lastWritten is the last segment of a type reference as written.
func lastWritten(qn *ast.QualifiedName) string {
	if qn == nil || len(qn.Parts) == 0 {
		return ""
	}
	return qn.Parts[len(qn.Parts)-1].Text
}

// channelType is the definition a consumer's type reference denotes, nil where
// it names none or matching would fall back to names.
func (ctx *Context) channelType(c lower.Channel) *symbols.Symbol {
	if c.Drops || c.Type == nil || len(c.Type.Parts) == 0 || c.Scope == nil || ctx.model.semantics == nil {
		return nil
	}
	return ctx.triggerType(c.Scope, c.Type)
}

// sentSignal is the signal a send's message statically carries, as buildMessage
// types it: a definition named, or the type a constructor names; nil otherwise.
func (ctx *Context) sentSignal(c lower.Channel) *symbols.Symbol {
	if c.Scope == nil || ctx.model.resolver == nil || ctx.model.semantics == nil {
		return nil
	}
	if qn := ast.AsQualifiedName(c.Message); qn != nil {
		sym, ok := ctx.model.resolver.ResolveQualified(c.Scope, qn)
		if ok && isDefinitionSymbol(sym) {
			return sym
		}
		return nil
	}
	if constructor, ok := c.Message.(*ast.ConstructorExpr); ok && constructor.Type != nil {
		sym, ok := ctx.model.resolver.ResolveQualified(c.Scope, constructor.Type)
		if !ok || sym == nil {
			return nil
		}
		switch sym.Decl.(type) {
		case *ast.Definition, *ast.Usage:
			return sym
		}
	}
	return nil
}

// conformersOf searches the workspace for what conforms to want; a type the
// library declares, which the library may specialize, is not searched.
func (ctx *Context) conformersOf(want *symbols.Symbol) *signalConformers {
	if ctx.model.conformers == nil {
		ctx.model.conformers = make(map[ast.Node]*signalConformers)
	}
	if found, ok := ctx.model.conformers[want.Decl]; ok {
		return found
	}
	found := &signalConformers{decls: make(map[ast.Node]bool), names: make(map[string]bool)}
	ctx.model.conformers[want.Decl] = found
	idx := ctx.model.resolver.Index()
	if want.Decl == nil || want.DocName == "" || idx.IsLibraryDocument(want.DocName) {
		return found
	}
	found.known = true
	seen := make(map[*symbols.Scope]bool)
	var walk func(scope *symbols.Scope)
	visit := func(sym *symbols.Symbol) bool {
		target := sym
		if sym.Kind == symbols.SymbolAlias {
			if resolved, ok := ctx.model.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
				target = resolved
			} else {
				return true
			}
		}
		if ctx.signalConforms(target, want) {
			if target.Decl == nil {
				found.known = false
			}
			found.decls[target.Decl] = true
			found.names[sym.Name] = true
			if sym.ShortName != "" {
				found.names[sym.ShortName] = true
			}
		}
		if sym.Scope != nil {
			walk(sym.Scope)
		}
		return true
	}
	walk = func(scope *symbols.Scope) {
		if scope == nil || seen[scope] {
			return
		}
		seen[scope] = true
		scope.ForEachMember(visit)
		scope.ForEachAnonymousMember(visit)
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	for _, doc := range idx.WorkspaceDocuments() {
		walk(idx.DocumentRoot(doc))
	}
	for _, scope := range ctx.model.scopes {
		walk(scope)
	}
	return found
}
