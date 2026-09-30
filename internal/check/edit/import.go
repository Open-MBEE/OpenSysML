package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func (m Model) addImportSplice(i int, op Operation) (splice, error) {
	owner, _, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if def, ok := owner.(*ast.Definition); ok && def.Kind == ast.DefEnumeration {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: "an enumeration body admits no import",
		}
	}
	visibility := op.ImportVisibility
	if visibility == "" {
		visibility = "private"
	}
	var wantVisibility ast.Visibility
	switch visibility {
	case "private":
		wantVisibility = ast.VisibilityPrivate
	case "public":
		wantVisibility = ast.VisibilityPublic
	case "protected":
		wantVisibility = ast.VisibilityProtected
	default:
		return splice{}, &Error{
			Failure: FailureInvalidValue, OperationIndex: i,
			Message: fmt.Sprintf("visibility %q is not private, public or protected", op.ImportVisibility),
		}
	}
	global, namespaceImport, segments, err := importTarget(i, op.ImportTarget, m.Source.Kind())
	if err != nil {
		return splice{}, err
	}
	for _, filter := range op.ImportFilters {
		if err := m.checkExpression(i, "import filter", ownerName(op.Owner), filter); err != nil {
			return splice{}, err
		}
	}
	text := writeImport(visibility, op.ImportTarget, op.ImportRecursive, op.ImportAll, op.ImportFilters)
	sf, parsed, err := parseImportText(i, m, text)
	if err != nil {
		return splice{}, err
	}
	if parsed.Visibility != wantVisibility || parsed.IsAll != op.ImportAll ||
		parsed.Kind != importKind(namespaceImport) || parsed.IsRecursive != op.ImportRecursive ||
		parsed.Imported.Global != global ||
		!nameSegmentsEqual(qualifiedNameSegments(parsed.Imported), segments) ||
		(parsed.FilterExpr != nil) != (len(op.ImportFilters) > 0) {
		return splice{}, invalidImportText(i)
	}
	for _, member := range importMembers(owner) {
		existing, ok := unwrapMembership(member).(*ast.Import)
		if !ok || existing.IsExpose {
			continue
		}
		if m.sameImport(existing, parsed, sf, wantVisibility) {
			return splice{}, &Error{
				Failure: FailureMemberNameTaken, OperationIndex: i,
				Message: fmt.Sprintf("%s already has %q", ownerName(op.Owner), text),
			}
		}
	}
	ins := m.importInsertion(owner, text)
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}

// importTarget reads the target as a qualified name optionally rooted with
// `$::` and optionally suffixed `::*` for a namespace import, refusing every
// other shape; it answers whether the name is global, whether the suffix was
// present and the name's segments.
func importTarget(i int, target string, kind source.Kind) (bool, bool, []string, error) {
	refuse := func(reason string) error {
		return &Error{
			Failure: FailureInvalidName, OperationIndex: i,
			Message: fmt.Sprintf("import target %q %s", target, reason),
		}
	}
	if target == "" {
		return false, false, nil, refuse("is empty")
	}
	if strings.HasSuffix(target, "::**") {
		return false, false, nil, refuse("ends in ::**; write recursion with is_recursive")
	}
	namespaceImport := false
	name := target
	if strings.HasSuffix(name, "::*") {
		namespaceImport = true
		name = name[:len(name)-len("::*")]
	}
	global := strings.HasPrefix(name, "$::")
	if global {
		name = name[len("$::"):]
	}
	if reserved, ok := qualifiedNameReserved(name, kind); !ok {
		if reserved != "" {
			return false, false, nil, refuse(fmt.Sprintf("uses the reserved word %q; quote it as '%s'", reserved, reserved))
		}
		return false, false, nil, refuse("is not a qualified name")
	}
	segments, ok := source.QualifiedNameSegments(name)
	if !ok || len(segments) == 0 {
		return false, false, nil, refuse("is not a qualified name")
	}
	return global, namespaceImport, segments, nil
}

// qualifiedNameReserved reports whether name lexes as exactly one qualified
// name in a file of kind: `::`-separated name tokens, where a keyword of the
// other language counts as a name and one of its own does not.
func qualifiedNameReserved(name string, kind source.Kind) (reserved string, ok bool) {
	if kind == source.KindUnknown {
		kind = source.KindSysML
	}
	lx := lexer.New(source.New("<target>", []byte(name)))
	for {
		tok := lx.Next()
		valid := !tok.Unterminated &&
			(tok.Kind == lexer.Identifier || tok.Kind == lexer.UnrestrictedName ||
				(tok.Kind == lexer.Keyword && !source.IsKeywordIn(tok.KeywordID, kind)))
		if !valid {
			if tok.Kind == lexer.Keyword && !tok.Unterminated {
				return tok.KeywordID, false
			}
			return "", false
		}
		switch lx.Next().Kind {
		case lexer.EOF:
			return "", true
		case lexer.ColonColon:
		default:
			return "", false
		}
	}
}

func importKind(namespaceImport bool) ast.ImportKind {
	if namespaceImport {
		return ast.ImportNamespace
	}
	return ast.ImportMembership
}

// writeImport renders the declaration as `<vis> import [all ]<target>[::**]` +
// `[<filter>]` per filter + `;`.
func writeImport(visibility, target string, recursive, all bool, filters []string) string {
	var text strings.Builder
	text.WriteString(visibility)
	text.WriteString(" import ")
	if all {
		text.WriteString("all ")
	}
	text.WriteString(target)
	if recursive {
		text.WriteString("::**")
	}
	for _, filter := range filters {
		text.WriteByte('[')
		text.WriteString(filter)
		text.WriteByte(']')
	}
	text.WriteByte(';')
	return text.String()
}

// parseImportText re-parses the written text alone inside a package body,
// answering its source and the import it made, and refusing unless it forms
// exactly one grammar-admissible import.
func parseImportText(i int, m Model, text string) (*source.SourceFile, *ast.Import, error) {
	sf := source.NewWithKind("<import>", []byte("package __Import {\n"+text+"\n}"), m.Source.Kind())
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 || len(root.Members) != 1 {
		return nil, nil, invalidImportText(i)
	}
	decl, ok := unwrapMembership(root.Members[0]).(*ast.Package)
	if !ok || len(decl.Members) != 1 {
		return nil, nil, invalidImportText(i)
	}
	imp, ok := unwrapMembership(decl.Members[0]).(*ast.Import)
	if !ok || imp.IsExpose || imp.HasBody {
		return nil, nil, invalidImportText(i)
	}
	return sf, imp, nil
}

func invalidImportText(i int) error {
	return &Error{
		Failure: FailureInvalidValue, OperationIndex: i,
		Message: "import target or filters do not form one grammar-admissible import",
	}
}

// importMembers lists the members of an import's owner: the root's, a package's
// or namespace's own list, or the body members of a definition or usage.
func importMembers(owner ast.Node) []ast.Node {
	switch n := owner.(type) {
	case *ast.RootNamespace:
		return n.Members
	case *ast.Package:
		return n.Members
	case *ast.Namespace:
		return n.Members
	default:
		return ast.DeclMembers(owner)
	}
}

// sameImport reports whether the existing import and the new one are identical:
// same visibility, all-flag, kind, recursion, imported name (quotes dropped)
// and filter tokens (whitespace dropped).
func (m Model) sameImport(existing, parsed *ast.Import, sf *source.SourceFile, want ast.Visibility) bool {
	if existing.Visibility != want || existing.IsAll != parsed.IsAll ||
		existing.Kind != parsed.Kind || existing.IsRecursive != parsed.IsRecursive ||
		existing.Imported.Global != parsed.Imported.Global {
		return false
	}
	if !nameSegmentsEqual(qualifiedNameSegments(existing.Imported), qualifiedNameSegments(parsed.Imported)) {
		return false
	}
	return filterTokensEqual(m.Source, existing.FilterExpr, parsed.FilterExpr, sf)
}

// qualifiedNameSegments reads a parsed name's segments with quotes dropped, so
// `'A'` and `A` compare equal.
func qualifiedNameSegments(name *ast.QualifiedName) []string {
	if name == nil {
		return nil
	}
	segments := make([]string, 0, len(name.Parts))
	for _, part := range name.Parts {
		if read, ok := source.QualifiedNameSegments(part.Text); ok && len(read) == 1 {
			segments = append(segments, read[0])
		} else {
			segments = append(segments, part.Text)
		}
	}
	return segments
}

func nameSegmentsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// filterTokensEqual compares two filter expressions by token kind and text,
// each lexed in its own source, so whitespace differences do not count.
func filterTokensEqual(aSF *source.SourceFile, a ast.Node, b ast.Node, bSF *source.SourceFile) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	at := filterTokens(aSF, a.Span())
	bt := filterTokens(bSF, b.Span())
	if len(at) != len(bt) {
		return false
	}
	for i := range at {
		if at[i].kind != bt[i].kind || at[i].text != bt[i].text {
			return false
		}
	}
	return true
}

type filterToken struct {
	kind lexer.Kind
	text string
}

// filterTokens reads the (kind, text) tokens spanning span in sf.
func filterTokens(sf *source.SourceFile, span source.Span) []filterToken {
	var tokens []filterToken
	lx := lexer.New(sf)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.Offset >= span.End() {
			break
		}
		if tok.Span.Offset >= span.Offset && tok.Span.End() <= span.End() {
			tokens = append(tokens, filterToken{kind: tok.Kind, text: sf.Text(tok.Span)})
		}
	}
	return tokens
}

// importInsertion places an import in owner: after the owner's last import when
// it has one, before its first member when it has none, and where memberInsertion
// puts members otherwise (an empty, bodyless or memberless owner).
func (m Model) importInsertion(owner ast.Node, text string) insertion {
	content := m.Source.Bytes()
	members := importMembers(owner)
	if len(members) == 0 {
		return m.memberInsertion(owner, text)
	}
	var lastImport *ast.Import
	for _, member := range members {
		if imp, ok := unwrapMembership(member).(*ast.Import); ok && !imp.IsExpose {
			lastImport = imp
		}
	}
	if lastImport == nil {
		return m.insertionBeforeMember(owner, members[0], text)
	}
	end := m.importStatementEnd(lastImport)
	lineEnd := end
	for lineEnd < len(content) && content[lineEnd] != '\n' {
		lineEnd++
	}
	rest := strings.TrimSpace(string(content[end:lineEnd]))
	if rest != "" && !strings.HasPrefix(rest, "//") {
		// Something else follows on the import's line: write inline after it.
		return insertion{span: source.Span{Offset: end}, text: " " + text, at: 1}
	}
	indent := lineIndent(content, lastImport.Span().Offset)
	lineStart := lastImport.Span().Offset
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	if strings.TrimSpace(string(content[lineStart:lastImport.Span().Offset])) != "" {
		// The import does not start its line: indent as a member of owner.
		indent = m.ownerMemberIndent(owner)
	}
	if lineEnd == len(content) {
		// The import's line has no newline: open one first.
		return insertion{
			span: source.Span{Offset: lineEnd},
			text: "\n" + indent + text + "\n",
			at:   1 + len(indent),
		}
	}
	return insertion{
		span: source.Span{Offset: lineEnd + 1},
		text: indent + text + "\n",
		at:   len(indent),
	}
}

// importStatementEnd is the offset just past the `;` terminating the import,
// or past its span when it opens a body. An import's node span reaches into the
// trivia ahead of the next member, so it cannot mark the line the import ends on.
func (m Model) importStatementEnd(imp *ast.Import) int {
	contentEnd := imp.Imported.Span().End()
	if imp.FilterExpr != nil {
		contentEnd = imp.FilterExpr.Span().End()
	}
	lx := lexer.New(m.Source)
	depth := 0
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.Offset < contentEnd {
			continue
		}
		if imp.HasBody {
			switch tok.Kind {
			case lexer.LBrace:
				depth++
			case lexer.RBrace:
				depth--
				if depth == 0 {
					return tok.Span.End()
				}
			}
		} else if tok.Kind == lexer.Semicolon {
			return tok.Span.End()
		}
		if tok.Span.Offset >= imp.Span().End() {
			break
		}
	}
	return imp.Span().End()
}

// insertionBeforeMember places text before the owner's first member: on the
// line above it, ahead of any whole-line trivia that precedes the member inside
// a body (a comment stays with what it documents), or inline when the member
// shares a line with the `{`.
func (m Model) insertionBeforeMember(owner ast.Node, first ast.Node, text string) insertion {
	content := m.Source.Bytes()
	offset := first.Span().Offset
	lineStart := offset
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	indent := string(content[lineStart:offset])
	if strings.TrimSpace(indent) != "" {
		// The member does not start its line: write inline ahead of it.
		return insertion{span: source.Span{Offset: offset}, text: text + " ", at: 0}
	}
	anchor := lineStart
	if owner != m.Root {
		anchor = precedingFullLineTriviaStart(content, lineStart)
	}
	return insertion{
		span: source.Span{Offset: anchor},
		text: indent + text + "\n",
		at:   len(indent),
	}
}
