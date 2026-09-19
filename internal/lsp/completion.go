package lsp

import (
	"context"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Completion implements textDocument/completion: members of what a member
// access names, else the names in scope plus library names and keywords.
// Prefix filtering is left to the client.
func (s *Server) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	c := &completionItems{seen: map[string]bool{}}

	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc != nil && doc.Scope != nil {
		offset := positionToOffset(doc.Content, params.Position)
		scope := enclosingScope(doc.Scope, offset)
		if path, ok := memberPathBefore(doc.Content, offset); ok {
			members := s.ws.MembersOnPath(scope, path)
			// A qualified name in a calc usage's type position still names a
			// query, so the package's members are filtered the same way.
			if calcTypingPositionAt(doc.Content, memberPathStart(doc.Content, offset)) && s.insideDocumentDefinition(scope) {
				members = queryTypeMembers(s, members)
			}
			for _, sym := range members {
				c.addSymbol(s, sym)
			}
			return c.list(), nil
		}
		// A declaration in a metadata annotation body redefines a feature of
		// the metadata definition, so only its features are offered there. A
		// value position resolves in the enclosing scope chain as usual.
		if body := s.ws.EnclosingMetadataBody(bodyScopeAt(doc.Content, scope, offset)); body != nil && !valuePositionAt(doc.Content, offset) {
			for _, sym := range s.ws.MetadataBodyMembers(body) {
				c.addSymbol(s, sym)
			}
			return c.list(), nil
		}
		// The type of a calc usage inside a document definition is a query, so
		// only query definitions (and the packages qualifying one) are offered.
		if calcTypingPositionAt(doc.Content, offset) && s.insideDocumentDefinition(scope) {
			for _, cand := range s.ws.QueryTypeCandidates(scope) {
				c.addNamedSymbol(s, cand.Name, cand.Sym)
			}
			// Library root packages, which a visible-name walk never offers as
			// top-level names, may still qualify a query.
			for _, sym := range s.ws.TopLevelSymbols(name) {
				if sym != nil && (sym.Kind == symbols.SymbolPackage || sym.Kind == symbols.SymbolNamespace) {
					c.addSymbol(s, sym)
				}
			}
			return c.list(), nil
		}
		// Inside a query-typed calc usage, a binding names one of the query's
		// parameters; its value resolves in the enclosing scope chain as usual.
		if !valuePositionAt(doc.Content, offset) {
			for sc := scope; sc != nil; sc = sc.Parent() {
				owner := sc.Owner()
				if owner == nil {
					continue
				}
				if params, ok := s.ws.QueryUsageParameters(owner); ok {
					for _, sym := range params {
						c.addSymbol(s, sym)
					}
					return c.list(), nil
				}
			}
		}
		for ; scope != nil; scope = scope.Parent() {
			for _, sym := range scope.Members() {
				c.addSymbol(s, sym)
			}
		}
	}

	for _, sym := range s.ws.TopLevelSymbols(name) {
		c.addSymbol(s, sym)
	}
	for _, kw := range source.Keywords() {
		c.add(protocol.CompletionItem{
			Label:  kw,
			Kind:   protocol.CompletionItemKindKeyword,
			Detail: "keyword",
		})
	}
	// Words the parser reads as syntax positionally but the lexer does not
	// reserve, so they are offered without being usable only as a keyword.
	for _, kw := range lexer.ContextualWords(source.KindOf(name)) {
		c.add(protocol.CompletionItem{
			Label:  kw,
			Kind:   protocol.CompletionItemKindKeyword,
			Detail: "contextual keyword",
		})
	}

	return c.list(), nil
}

// completionItems accumulates items, keeping the first offered under a label so
// a nearer declaration wins over an inherited or library name.
type completionItems struct {
	seen  map[string]bool
	items []protocol.CompletionItem
}

func (c *completionItems) add(item protocol.CompletionItem) {
	if item.Label == "" || c.seen[item.Label] {
		return
	}
	c.seen[item.Label] = true
	c.items = append(c.items, item)
}

// addSymbol offers a declared element with its real kind, detail and docs, under
// both names a short-named declaration is referable by (`part def <v> Vehicle`).
func (c *completionItems) addSymbol(s *Server, sym *symbols.Symbol) {
	if sym == nil {
		return
	}
	item := protocol.CompletionItem{
		Label:  leafName(sym.Name),
		Kind:   completionKind(sym.Kind),
		Detail: completionDetail(sym),
	}
	if doc, ok := s.symbolDocumentation(sym); ok {
		item.Documentation = doc
	}
	c.add(item)
	if short := sym.ShortName; short != "" && short != item.Label {
		item.Label = short
		c.add(item)
	}
}

// addNamedSymbol offers sym under a visible spelling of its own — an alias name
// rather than the target's declared name.
func (c *completionItems) addNamedSymbol(s *Server, label string, sym *symbols.Symbol) {
	if sym == nil || label == "" {
		return
	}
	item := protocol.CompletionItem{
		Label:  label,
		Kind:   completionKind(sym.Kind),
		Detail: completionDetail(sym),
	}
	if doc, ok := s.symbolDocumentation(sym); ok {
		item.Documentation = doc
	}
	c.add(item)
}

func (c *completionItems) list() *protocol.CompletionList {
	return &protocol.CompletionList{IsIncomplete: false, Items: c.items}
}

// symbolDocumentation returns the comment trivia preceding a symbol's
// declaration, when the document declaring it is loaded. A client that renders
// Markdown is sent the comments as prose, as hover does, rather than the source
// with its delimiters.
func (s *Server) symbolDocumentation(sym *symbols.Symbol) (protocol.MarkupContent, bool) {
	if len(sym.LeadingTrivia) == 0 || sym.DocName == "" {
		return protocol.MarkupContent{}, false
	}
	doc := s.ws.Document(sym.DocName)
	if doc == nil {
		return protocol.MarkupContent{}, false
	}
	comments := leadingDocComments(doc.Content, sym.LeadingTrivia)
	if s.wantsMarkdownCompletion() {
		prose := docCommentProse(comments)
		if prose == "" {
			return protocol.MarkupContent{}, false
		}
		return protocol.MarkupContent{Kind: protocol.Markdown, Value: prose}, true
	}
	text := strings.Join(comments, "\n")
	if text == "" {
		return protocol.MarkupContent{}, false
	}
	return protocol.MarkupContent{Kind: protocol.PlainText, Value: text}, true
}

// completionDetail describes an element the way hover does: its kind, plus the
// type it declares when it declares one.
func completionDetail(sym *symbols.Symbol) string {
	detail := sym.Notation()
	if t := declaredTypeText(sym); t != "" {
		detail += " : " + t
	}
	return detail
}

// declaredTypeText returns the qualified name a usage declares as its type, or
// "" for anything else — a definition, or an untyped usage.
func declaredTypeText(sym *symbols.Symbol) string {
	var rels []*ast.Relationship
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		rels = decl.Relationships
	case *ast.CrossFeatureMember:
		rels = decl.Relationships
	default:
		return ""
	}
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok {
			return qn.Text()
		}
	}
	return ""
}

// leafName returns the last segment of a name: a cached library symbol is named
// by its qualified name, which is not what completing a member inserts.
func leafName(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[i+2:]
	}
	return name
}

// completionKind maps a symbol kind to the LSP completion kind, so client icons
// distinguish definitions, usages, behaviors and packages.
func completionKind(k symbols.SymbolKind) protocol.CompletionItemKind {
	switch k {
	case symbols.SymbolPackage, symbols.SymbolNamespace, symbols.SymbolDependency,
		symbols.SymbolRelationship:
		return protocol.CompletionItemKindModule
	case symbols.SymbolAlias:
		return protocol.CompletionItemKindReference
	case symbols.SymbolComment, symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation:
		return protocol.CompletionItemKindText

	// Definitions name types.
	case symbols.SymbolEnumerationDef:
		return protocol.CompletionItemKindEnum
	case symbols.SymbolActionDef, symbols.SymbolCalcDef,
		symbols.SymbolConstraintDef, symbols.SymbolRequirementDef,
		symbols.SymbolCaseDef, symbols.SymbolAnalysisCaseDef,
		symbols.SymbolVerificationCaseDef, symbols.SymbolUseCaseDef:
		return protocol.CompletionItemKindFunction
	case symbols.SymbolPartDef, symbols.SymbolAttributeDef, symbols.SymbolItemDef,
		symbols.SymbolOccurrenceDef, symbols.SymbolIndividualDef,
		symbols.SymbolMetadataDef, symbols.SymbolMetaclass,
		symbols.SymbolViewDef, symbols.SymbolViewpointDef, symbols.SymbolRenderingDef,
		symbols.SymbolConcernDef, symbols.SymbolConnectionDef, symbols.SymbolFlowDef,
		symbols.SymbolPortDef, symbols.SymbolInterfaceDef, symbols.SymbolAllocationDef,
		symbols.SymbolStateDef:
		return protocol.CompletionItemKindClass

	// Usages name features of an enclosing element.
	case symbols.SymbolEnumerationUsage:
		return protocol.CompletionItemKindEnumMember
	case symbols.SymbolActionUsage, symbols.SymbolCalcUsage,
		symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage,
		symbols.SymbolCaseUsage, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolVerificationCaseUsage, symbols.SymbolUseCaseUsage:
		return protocol.CompletionItemKindMethod
	case symbols.SymbolAttributeUsage:
		return protocol.CompletionItemKindProperty
	default:
		return protocol.CompletionItemKindField
	}
}

// memberPathBefore returns the segments of the member access ending at offset:
// `v.`, `v.wh` and `A::B::` yield [v], [v] and [A B]. The partial name being
// typed is excluded, since the client filters on it.
func memberPathBefore(content []byte, offset int) ([]string, bool) {
	if offset > len(content) {
		offset = len(content)
	}
	i := identStart(content, offset)
	var path []string
	for {
		sep := separatorBefore(content, i)
		if sep == 0 {
			break
		}
		i -= sep
		start := identStart(content, i)
		if start == i || isDigit(content[start]) {
			// A separator with no name before it, or a numeric literal such
			// as `1.` — neither is a member access.
			return nil, false
		}
		path = append([]string{string(content[start:i])}, path...)
		i = start
	}
	return path, len(path) > 0
}

// separatorBefore returns the length of the member separator ending at i — 2
// for `::`, 1 for `.` — or 0 when no separator ends there.
func separatorBefore(content []byte, i int) int {
	if i >= 2 && content[i-1] == ':' && content[i-2] == ':' {
		return 2
	}
	if i >= 1 && content[i-1] == '.' {
		return 1
	}
	return 0
}

// identStart returns the offset at which the identifier ending at i begins,
// or i itself when no identifier character precedes i.
func identStart(content []byte, i int) int {
	start := i
	for start > 0 && isIdentByte(content[start-1]) {
		start--
	}
	return start
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || isDigit(b)
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// bodyScopeAt returns scope, or its parent when offset lies in the header of
// scope's owning declaration rather than inside its body.
func bodyScopeAt(content []byte, scope *symbols.Scope, offset int) *symbols.Scope {
	owner := scope.Owner()
	if owner == nil {
		return scope
	}
	if body, hasBody := bodyOf(content, owner.DeclSpan); !hasBody || offset <= body.Offset {
		return scope.Parent()
	}
	return scope
}

// enclosingScope returns the deepest scope whose owning declaration span
// contains offset, starting from root. Falls back to root. A metadata
// annotation body scope has no owning symbol, so it is found by its own
// node span before the annotated declaration's members are consulted.
func enclosingScope(root *symbols.Scope, offset int) *symbols.Scope {
	best := root
	for _, child := range root.Children() {
		if !isAnnotationBodyScope(child) {
			continue
		}
		sp := child.Node().Span()
		if offset >= sp.Offset && offset < sp.End() {
			return enclosingScope(child, offset)
		}
	}
	for _, sym := range root.Members() {
		if sym.Scope == nil {
			continue
		}
		sp := sym.DeclSpan
		if offset >= sp.Offset && offset < sp.End() {
			return enclosingScope(sym.Scope, offset)
		}
	}
	return best
}

// calcTypingPositionAt reports whether offset sits in the type position of a
// calc usage declaration: after the `:` of `calc <name> :`.
func calcTypingPositionAt(content []byte, offset int) bool {
	if offset > len(content) {
		offset = len(content)
	}
	i := identStart(content, offset)
	i = skipSpacesBefore(content, i)
	if i < 1 || content[i-1] != ':' || (i < len(content) && content[i] == '>') {
		return false
	}
	if i >= 2 && (content[i-2] == ':' || content[i-2] == '>') {
		return false // `::` is a qualifier, `>:` ends nothing
	}
	i = skipSpacesBefore(content, i-1)
	start := identStart(content, i)
	if start == i {
		return false
	}
	i = skipSpacesBefore(content, start)
	start = identStart(content, i)
	return string(content[start:i]) == "calc"
}

// memberPathStart returns the offset at which the member access ending at
// offset begins: the start of `A::B::` for an offset after it.
func memberPathStart(content []byte, offset int) int {
	if offset > len(content) {
		offset = len(content)
	}
	i := identStart(content, offset)
	for {
		sep := separatorBefore(content, i)
		if sep == 0 {
			return i
		}
		i = identStart(content, i-sep)
	}
}

// queryTypeMembers filters members to what a query type may be spelled with:
// query definitions and the namespaces qualifying one.
func queryTypeMembers(s *Server, members []*symbols.Symbol) []*symbols.Symbol {
	queries := map[*symbols.Symbol]bool{}
	for _, sym := range s.ws.QueryDefinitions(members) {
		queries[sym] = true
	}
	var out []*symbols.Symbol
	for _, sym := range members {
		if sym == nil {
			continue
		}
		if queries[sym] || sym.Kind == symbols.SymbolPackage || sym.Kind == symbols.SymbolNamespace {
			out = append(out, sym)
		}
	}
	return out
}

// skipSpacesBefore returns the offset before any run of blanks ending at i.
func skipSpacesBefore(content []byte, i int) int {
	for i > 0 && (content[i-1] == ' ' || content[i-1] == '\t' || content[i-1] == '\n' || content[i-1] == '\r') {
		i--
	}
	return i
}

// insideDocumentDefinition reports whether the scope chain sits inside a native
// document definition.
func (s *Server) insideDocumentDefinition(scope *symbols.Scope) bool {
	for sc := scope; sc != nil; sc = sc.Parent() {
		if owner := sc.Owner(); owner != nil && s.ws.IsDocumentDefinition(owner) {
			return true
		}
	}
	return false
}

// valuePositionAt reports whether offset sits after the `=` of the statement
// it is in — a value, which resolves in the enclosing scope chain rather than
// against the metadata definition's features.
func valuePositionAt(content []byte, offset int) bool {
	if offset > len(content) {
		offset = len(content)
	}
	for i := offset - 1; i >= 0; i-- {
		switch content[i] {
		case '=':
			return true
		case ';', '{', '}':
			return false
		}
	}
	return false
}
