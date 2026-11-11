// Package identity computes the effective repository identity of the symbols
// of an analyzed model: the id of an element's IdentityMetadata::ElementId
// annotation when it carries one, the normative UUID of a named standard
// library element, the id derived from its qualified name otherwise, plus the
// project scope its nearest enclosing ProjectRef declares. It is a side table
// keyed by symbol; the AST is never touched.
package identity

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"strconv"
)

// FQNs of the identity metadata definitions in the bundled library.
const (
	ElementIdFQN  = "IdentityMetadata::ElementId"
	ProjectRefFQN = "IdentityMetadata::ProjectRef"
)

// Scope is one ProjectRef annotation's project binding. Two scopes name the
// same project when org and projectId agree; branch selects a version of one
// project, never another identity, so it plays no part in equality.
type Scope struct {
	ProjectID string
	Branch    string
	Org       string
	// Symbol is the namespace carrying the ProjectRef annotation.
	Symbol *symbols.Symbol
	// Declarations lists every ProjectRef annotation of the namespace, inline
	// ones first, `about`-form ones after. The first supplies the binding.
	Declarations []ScopeDeclaration
}

// ScopeDeclaration is one ProjectRef annotation of a namespace, whether
// written inline or from elsewhere with an `about` clause.
type ScopeDeclaration struct {
	ProjectID string
	Branch    string
	Org       string
	// Node is the annotating node itself, so a consumer that absorbs the
	// annotation into provenance can identify it in the tree.
	Node ast.Node
	// Span locates the annotating node, for diagnostics.
	Span source.Span
	// Scope is where the annotating node is declared.
	Scope *symbols.Scope
}

// Key names the project a declaration binds to; branch selects a version of
// one project, never another identity, so it plays no part.
func (d ScopeDeclaration) Key() string {
	return d.Org + "\x00" + d.ProjectID
}

// Key names the project a scope binds to, for scope-equality grouping.
func (s *Scope) Key() string {
	if s == nil {
		return ""
	}
	return s.Org + "\x00" + s.ProjectID
}

// Declaration is one ElementId annotation of an element, whether written
// inline (`@ElementId {...}`) or from elsewhere (`metadata : ElementId about x;`).
type Declaration struct {
	// ID is the annotation's evaluated id; meaningful when Declared.
	ID string
	// Declared reports the id evaluated to a constant string.
	Declared bool
	// About marks an `about`-form annotation stated away from the element.
	About bool
	// Node is the annotating node itself, so a consumer that absorbs the
	// annotation into identity can identify it in the tree.
	Node ast.Node
	// Span locates the annotating node, for diagnostics.
	Span source.Span
	// Scope is where the annotating node is declared; for an `about`-form
	// annotation that may be another document than the element's.
	Scope *symbols.Scope
}

// Source is where an element's effective id comes from.
type Source uint8

const (
	// SourceDerived is the encoding of the qualified name, the default.
	SourceDerived Source = iota
	// SourceNormative is the UUID the norm fixes for a named standard library element.
	SourceNormative
	// SourceDeclared is the id an ElementId annotation declares.
	SourceDeclared
	// SourceExtension is the UUID this implementation fixes for a named element
	// of its own extension libraries, derived as a normative id under their URL.
	SourceExtension
)

func (s Source) String() string {
	switch s {
	case SourceNormative:
		return "normative"
	case SourceDeclared:
		return "declared"
	case SourceExtension:
		return "extension"
	}
	return "derived"
}

// Fixed reports an id a bundled library fixes: normative or extension.
func (s Source) Fixed() bool { return s == SourceNormative || s == SourceExtension }

// Info is one symbol's identity: its effective id, where it came from, and
// the project scope it resolves against.
type Info struct {
	Symbol *symbols.Symbol
	// FQN is the symbol's qualified name, which the derived id encodes.
	FQN string
	// EffectiveID is the annotated id when declared, the fixed id of a bundled
	// library element otherwise, the derived id failing both.
	EffectiveID string
	// Source is which of the four EffectiveID is.
	Source Source
	// Language is the library a fixed id is minted under; zero otherwise.
	Language Language
	// MembershipID is the id this implementation mints for the owning membership
	// of an unnamed library member; "" where the membership id follows the name.
	MembershipID string
	// Annotated reports an ElementId annotation on the element.
	Annotated bool
	// Declared reports that the annotation's id evaluated to a constant
	// string; when false the derived id stands.
	Declared bool
	// DeclaredID is the evaluated ElementId id, possibly empty.
	DeclaredID string
	// AnnotationSpan locates the ElementId annotation, for diagnostics.
	AnnotationSpan source.Span
	// Declarations lists every ElementId annotation of the element, inline
	// ones first, `about`-form ones after.
	Declarations []Declaration
	// Scope is the nearest enclosing ProjectRef binding; nil when unbound.
	Scope *Scope
}

// Normative reports an effective id the norm fixes for the element.
func (i *Info) Normative() bool { return i.Source == SourceNormative }

// Fixed reports an effective id a bundled library fixes for the element: the
// norm's for a standard library element, this implementation's for an extension's.
func (i *Info) Fixed() bool { return i.Source.Fixed() }

// OwningMembershipID is the fixed id of the membership owning a bundled library
// element; "" for any other, whose membership id derives from its own.
func (i *Info) OwningMembershipID() string {
	if !i.Fixed() {
		return ""
	}
	if i.MembershipID != "" {
		return i.MembershipID
	}
	return OwningMembershipID(i.Language, i.FQN)
}

// Table is the identity side table of one analyzed scope tree.
type Table struct {
	infos map[*symbols.Symbol]*Info
	order []*symbols.Symbol
}

// Info returns the identity of sym, if the table holds it.
func (t *Table) Info(sym *symbols.Symbol) (*Info, bool) {
	info, ok := t.infos[sym]
	return info, ok
}

// Symbols returns the table's symbols in deterministic walk order.
func (t *Table) Symbols() []*symbols.Symbol { return t.order }

// Build computes the identity side table of every named symbol under the
// given roots, in root order.
func Build(model *semantics.Model, res *resolve.Resolver, roots ...*symbols.Scope) *Table {
	b := newBuilder(model, res)
	t := &Table{infos: make(map[*symbols.Symbol]*Info)}
	syms := collectSymbols(roots)
	// `about` annotations may target elements outside the roots — bundled
	// library ones included — and those targets still carry the identity
	// their annotations declare. A ProjectRef binds the target's whole
	// subtree to a project, so its descendants join the id space too.
	for _, sym := range model.AboutAnnotatedSymbols() {
		syms = append(syms, sym)
		if hasProjectRefSite(model, sym) {
			syms = append(syms, collectSymbols([]*symbols.Scope{sym.Scope})...)
		}
	}
	for _, sym := range syms {
		if _, ok := t.infos[sym]; ok {
			continue
		}
		info := b.infoOf(sym)
		if info == nil {
			continue
		}
		t.infos[sym] = info
		t.order = append(t.order, sym)
	}
	return t
}

// Of computes the identity of one symbol without building a whole table; ok
// is false for a symbol that has no qualified name to derive an id from.
func Of(model *semantics.Model, res *resolve.Resolver, sym *symbols.Symbol) (*Info, bool) {
	return NewLookup(model, res).Of(sym)
}

// Lookup computes identities one symbol at a time, sharing between them what Of
// would rebuild per call (the library catalog); for a graph's worth of symbols.
type Lookup struct {
	b *builder
}

// NewLookup is a Lookup over the model and resolver.
func NewLookup(model *semantics.Model, res *resolve.Resolver) *Lookup {
	return &Lookup{b: newBuilder(model, res)}
}

// Of is the identity of one symbol; ok is false for a symbol that has no
// qualified name to derive an id from.
func (l *Lookup) Of(sym *symbols.Symbol) (*Info, bool) {
	info := l.b.infoOf(sym)
	return info, info != nil
}

// Root is the outermost declaration enclosing sym, sym itself at a root; a
// ProjectRef written there binds sym's whole document subtree.
func Root(sym *symbols.Symbol) *symbols.Symbol {
	for {
		outer := enclosingSymbol(sym)
		if outer == nil {
			return sym
		}
		sym = outer
	}
}

type builder struct {
	model  *semantics.Model
	res    *resolve.Resolver
	idx    *symbols.Index
	scopes map[*symbols.Symbol]*Scope
	known  map[*symbols.Symbol]bool // scope memo: sym has an entry in scopes
	// norm decides which library symbols the norm fixes an id for.
	norm *qualifier
	// catalog places an unnamed library member under its catalogued owner; built on first need.
	catalog *Catalog
}

func newBuilder(model *semantics.Model, res *resolve.Resolver) *builder {
	return &builder{
		model:  model,
		res:    res,
		idx:    res.Index(),
		scopes: make(map[*symbols.Symbol]*Scope),
		known:  make(map[*symbols.Symbol]bool),
		norm:   newQualifier(res.Index(), model),
	}
}

// infoOf computes one symbol's identity, or nil for a symbol that has no
// qualified name to derive an id from (an alias adds no element of its own).
func (b *builder) infoOf(sym *symbols.Symbol) *Info {
	if sym == nil || (sym.Decl == nil && !sym.Recorded()) || sym.Kind == symbols.SymbolAlias {
		return nil
	}
	fqn := b.qualifiedName(sym)
	if fqn == "" {
		return nil
	}
	info := &Info{Symbol: sym, FQN: fqn, EffectiveID: rdf.EncodeElementID(fqn)}
	if lang, ok := b.norm.language(sym); ok {
		info.Source, info.Language = fixedSource(lang), lang
		info.EffectiveID = ElementID(lang, fqn)
	} else if member, lang, ok := b.anonymousMember(sym); ok {
		info.Source, info.Language = fixedSource(lang), lang
		info.EffectiveID, info.MembershipID = member.ID, member.OwningMembershipID
	}
	for _, site := range b.model.AnnotationSitesOf(sym) {
		if site.TypeFQN != ElementIdFQN {
			continue
		}
		d := Declaration{About: site.About, Node: site.Node, Span: site.Span, Scope: site.Scope}
		if id, ok := siteString(site, "id"); ok {
			d.Declared = true
			d.ID = id
		}
		info.Declarations = append(info.Declarations, d)
	}
	if len(info.Declarations) > 0 {
		first := info.Declarations[0]
		info.Annotated = true
		info.AnnotationSpan = first.Span
		info.Declared = first.Declared
		info.DeclaredID = first.ID
		for _, d := range info.Declarations {
			if !d.Declared || d.ID == "" {
				continue
			}
			// An annotation restating the library's fixed id declares nothing it does not.
			if d.ID != info.EffectiveID || !info.Fixed() {
				info.EffectiveID = d.ID
				info.Source, info.Language = SourceDeclared, 0
			}
			break
		}
	}
	info.Scope = b.scopeOf(sym)
	return info
}

// fixedSource is the source of an id a bundled library of language lang fixes.
func fixedSource(lang Language) Source {
	if lang == OpenSysML {
		return SourceExtension
	}
	return SourceNormative
}

// anonymousMember is the id this implementation mints for sym, an unnamed member
// of a catalogued library element (AnonymousMemberOf); ok is false for any other symbol.
func (b *builder) anonymousMember(sym *symbols.Symbol) (AnonymousMember, Language, bool) {
	if b.model.QualifiedNameOf(sym) != "" {
		return AnonymousMember{}, 0, false
	}
	lang, ok := LibraryLanguage(b.idx.LibraryTier(sym))
	if !ok {
		return AnonymousMember{}, 0, false
	}
	if b.catalog == nil {
		b.catalog = LibraryCatalogWithOpenSysML(b.idx)
	}
	member, ok := AnonymousMemberOf(b.model, b.catalog, sym)
	return member, lang, ok
}

// scopeOf resolves the nearest enclosing ProjectRef binding of sym, the
// symbol's own annotation included, memoized over the enclosing chain.
func (b *builder) scopeOf(sym *symbols.Symbol) *Scope {
	if sym == nil {
		return nil
	}
	if b.known[sym] {
		return b.scopes[sym]
	}
	b.known[sym] = true
	var decls []ScopeDeclaration
	for _, site := range b.model.AnnotationSitesOf(sym) {
		if site.TypeFQN != ProjectRefFQN {
			continue
		}
		d := ScopeDeclaration{Node: site.Node, Span: site.Span, Scope: site.Scope}
		d.ProjectID, _ = siteString(site, "projectId")
		d.Branch, _ = siteString(site, "branch")
		d.Org, _ = siteString(site, "org")
		decls = append(decls, d)
	}
	if len(decls) > 0 {
		first := decls[0]
		s := &Scope{
			ProjectID:    first.ProjectID,
			Branch:       first.Branch,
			Org:          first.Org,
			Symbol:       sym,
			Declarations: decls,
		}
		b.scopes[sym] = s
		return s
	}
	s := b.scopeOf(enclosingSymbol(sym))
	b.scopes[sym] = s
	return s
}

// hasProjectRefSite reports a ProjectRef annotation among sym's sites.
func hasProjectRefSite(model *semantics.Model, sym *symbols.Symbol) bool {
	for _, site := range model.AnnotationSitesOf(sym) {
		if site.TypeFQN == ProjectRefFQN {
			return true
		}
	}
	return false
}

// enclosingSymbol is the declaration sym is nested in, or nil at a root.
func enclosingSymbol(sym *symbols.Symbol) *symbols.Symbol {
	for sc := sym.OwnerScope; sc != nil; sc = sc.Parent() {
		if owner := sc.Owner(); owner != nil && owner != sym {
			return owner
		}
	}
	return nil
}

// siteString is the constant string value one annotation binds to feature.
func siteString(site semantics.AnnotationSite, feature string) (string, bool) {
	for _, v := range site.Values {
		if v.Feature == feature && v.Value.Kind == symbols.FilterValueString {
			return v.Value.Str, true
		}
	}
	return "", false
}

// collectSymbols visits every symbol of the scope subtrees exactly once, in
// declaration order.
func collectSymbols(roots []*symbols.Scope) []*symbols.Symbol {
	seenSyms := make(map[*symbols.Symbol]bool)
	seenScopes := make(map[*symbols.Scope]bool)
	var out []*symbols.Symbol
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil || seenScopes[scope] {
			return
		}
		seenScopes[scope] = true
		scope.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym == nil || seenSyms[sym] {
				return true
			}
			seenSyms[sym] = true
			out = append(out, sym)
			walk(sym.Scope)
			return true
		})
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	return out
}

// qualifiedName is the symbol's qualified name as the model derives it, through an effective name too
// (`T::obj::massRequirement`); a metadata body feature's (`mission = …`) is under the usage owning it.
func (b *builder) qualifiedName(sym *symbols.Symbol) string {
	if b.model.RestatesMetadataBodyFeature(sym) {
		if fqn := b.metadataBodyFQN(sym); fqn != "" {
			return fqn
		}
	}
	if fqn := b.model.QualifiedNameOf(sym); fqn != "" {
		return fqn
	}
	return b.idx.GetFQN(sym)
}

// metadataBodyFQN qualifies a metadata body feature under the usage owning it — an unnamed usage
// by its position among its owner's members, as the conversion names it — or under its body feature.
func (b *builder) metadataBodyFQN(sym *symbols.Symbol) string {
	scope := sym.OwnerScope
	if scope == nil || scope.Parent() == nil {
		return ""
	}
	usage := scope.Parent().MemberDeclaring(scope.Node())
	if usage == nil {
		return ""
	}
	usageFQN := b.qualifiedName(usage)
	if usage.Name == "" {
		index, ok := b.memberIndex(usage)
		if !ok {
			return ""
		}
		usageFQN = "@" + strconv.Itoa(index)
		if owner := usage.OwnerScope.Owner(); owner != nil {
			usageFQN = b.qualifiedName(owner) + "::" + usageFQN
		}
	}
	return usageFQN + "::" + sym.Name
}

// memberIndex is the position of the metadata usage among the members its owner declares,
// identity annotations left out as the conversion leaves them; a `#M` prefix follows the body.
func (b *builder) memberIndex(usage *symbols.Symbol) (int, bool) {
	if usage.Decl == nil || usage.OwnerScope == nil {
		return 0, false
	}
	prefix, _ := usage.Decl.(*ast.PrefixMetadata)
	var members []ast.Node
	var prefixes []*ast.PrefixMetadata
	switch n := usage.OwnerScope.Node().(type) {
	case *ast.RootNamespace:
		members = n.Members
	case *ast.Package:
		members, prefixes = n.Members, n.Prefixes
	case *ast.Namespace:
		members, prefixes = n.Members, n.Prefixes
	case *ast.Definition:
		members, prefixes = n.Members, n.Prefixes
	case *ast.Usage:
		members, prefixes = n.Members, n.Prefixes
	default:
		return 0, false
	}
	index := 0
	for _, member := range members {
		node := member
		if m, ok := member.(*ast.Membership); ok {
			node = m.Member
		}
		if node == nil || b.identityAnnotation(usage.OwnerScope, node) {
			continue
		}
		if node == usage.Decl {
			return index, true
		}
		index++
	}
	for _, p := range prefixes {
		if prefix != nil && p == prefix {
			return index, true
		}
		index++
	}
	return 0, false
}

// identityAnnotation reports a member the conversion leaves out of its positions: an ElementId or
// ProjectRef annotation, by the type it resolves to, of the element it is about or the owner it is in.
func (b *builder) identityAnnotation(scope *symbols.Scope, node ast.Node) bool {
	sym := scope.MemberDeclaring(node)
	if sym == nil {
		return false
	}
	targets := b.model.AnnotatedElementsOf(sym)
	if len(targets) == 0 {
		owner := scope.Owner()
		if owner == nil {
			return false
		}
		targets = []*symbols.Symbol{owner}
	}
	for _, target := range targets {
		for _, site := range b.model.AnnotationSitesOf(target) {
			if site.Node == node && (site.TypeFQN == ElementIdFQN || site.TypeFQN == ProjectRefFQN) {
				return true
			}
		}
	}
	return false
}
