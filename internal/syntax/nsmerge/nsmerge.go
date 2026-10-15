// Package nsmerge folds a namespace declaration typed or generated anew into
// the same-named declaration a document already holds, so re-stating
// `package P { ... }` adds to its body instead of replacing it. The REPL's
// prompt and the recording of analysis runs share it.
package nsmerge

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Describe spells a member for a report, e.g. "part def A".
type Describe func(ast.Node) string

type Decl struct {
	Name    string
	Desc    string // "package P", as the summary renders it
	Header  string // everything before the body, whitespace-normalized
	Members []ast.Node
	Start   int
	Open    int
	Brace   int
}

// Edit replaces src[start:end] with text. Edits never overlap: each one covers a
// distinct member, or is an insertion at a body's closing brace. An own Edit
// marks where the submission lands, so its report scopes to what was typed.
type Edit struct {
	Start, End int
	Text       string
	Own        bool
}

func SoleNamespace(src string, root *ast.RootNamespace, describe Describe) (Decl, bool) {
	if root == nil || len(root.Members) != 1 {
		return Decl{}, false
	}
	return declOf(src, root.Members[0], describe)
}

// Chain names the nested namespace declarations inside decl, top to
// innermost — the package nesting a generated record writes.
func Chain(src string, decl Decl) []string {
	var segs []string
	for {
		var next Decl
		for _, m := range decl.Members {
			if sub, ok := declOf(src, m, nil); ok {
				next = sub
				break
			}
		}
		if next.Name == "" {
			return segs
		}
		segs = append(segs, next.Name)
		decl = next
	}
}

// Depth reports how many of the chain's segments decl's body already
// holds as nested namespaces, counting from the first until one is missing.
func Depth(src string, decl Decl, segs []string) int {
	for i, seg := range segs {
		found := false
		for _, m := range decl.Members {
			if MemberName(m) != seg {
				continue
			}
			if sub, ok := declOf(src, m, nil); ok {
				decl = sub
				found = true
				break
			}
		}
		if !found {
			return i
		}
	}
	return len(segs)
}

// Named finds the namespace declaration of the given name in an
// accepted snippet. It reports false unless exactly one member declares that
// name, so an ambiguous snippet keeps the replacement behavior.
func Named(src, name string, kind source.Kind, describe Describe) (Decl, bool) {
	return NamedIn(src, parser.New(sourceForKind("<merge>", []byte(src), kind)).ParseFile(), name, describe)
}

// NamedIn is Named over a parse the caller already has.
func NamedIn(src string, root *ast.RootNamespace, name string, describe Describe) (Decl, bool) {
	if root == nil {
		return Decl{}, false
	}
	var found ast.Node
	for _, m := range root.Members {
		if MemberName(m) != name {
			continue
		}
		if found != nil {
			return Decl{}, false
		}
		found = m
	}
	if found == nil {
		return Decl{}, false
	}
	return declOf(src, found, describe)
}

// declOf describes a member as a namespace declaration with a body,
// reporting false for anything else — a usage, a definition, a `package P;`
// without a body, or a body whose brace the parser never saw.
func declOf(src string, member ast.Node, describe Describe) (Decl, bool) {
	node := member
	if mem, ok := member.(*ast.Membership); ok {
		node = mem.Member
	}
	var d Decl
	switch n := node.(type) {
	case *ast.Package:
		if !n.HasBody {
			return d, false
		}
		d.Name, d.Members = n.Ident.Name, n.Members
	case *ast.Namespace:
		if !n.HasBody {
			return d, false
		}
		d.Name, d.Members = n.Ident.Name, n.Members
	default:
		return d, false
	}
	if d.Name == "" {
		return d, false
	}
	brace, ok := closingBrace(src, node)
	if !ok {
		return d, false
	}
	d.Start, d.Brace = member.Span().Offset, brace
	open := strings.IndexByte(src[d.Start:brace], '{')
	if open < 0 {
		return d, false
	}
	d.Open = d.Start + open
	d.Header = strings.Join(strings.Fields(src[d.Start:d.Start+open]), " ")
	if describe != nil {
		d.Desc = describe(member)
	}
	return d, true
}

// BodyMembers returns the members of a namespace member's body. Unlike
// declOf it does not need the body to be terminated, so it can also
// report what an unparseable re-declaration dropped.
func BodyMembers(member ast.Node) ([]ast.Node, bool) {
	node := member
	if mem, ok := member.(*ast.Membership); ok {
		node = mem.Member
	}
	switch n := node.(type) {
	case *ast.Package:
		return n.Members, n.HasBody
	case *ast.Namespace:
		return n.Members, n.HasBody
	}
	return nil, false
}

// closingBrace returns the offset of the brace that closes a declaration's body.
// Text that does not end in one belongs to an unterminated body, which is not
// safe to edit. A trailing comment is not part of the declaration, so it does
// not hide the brace.
func closingBrace(src string, node ast.Node) (int, bool) {
	end := node.Span().Offset + len(memberText(src, node))
	if end > len(src) {
		end = len(src)
	}
	for end > 0 && isSpaceByte(src[end-1]) {
		end--
	}
	if end == 0 || src[end-1] != '}' {
		return 0, false
	}
	return end - 1, true
}

// MergeEdits computes the edits on oldSrc that fold newDecl's members into
// oldDecl, plus a description of each member the new body replaces and its
// qualified name under path. A member redeclared as a namespace on both sides is
// merged in turn, so adding to a nested package keeps the nested members too.
func MergeEdits(oldSrc string, oldDecl Decl, newSrc string, newDecl Decl, path string, describe Describe) ([]Edit, []string, []string) {
	var (
		edits    []Edit
		replaced []string
		gone     []string
		folded   = make(map[int]bool, len(newDecl.Members))
		count    = nameCounts(oldDecl.Members)
	)
	for _, om := range oldDecl.Members {
		name := MemberName(om)
		if name == "" {
			continue
		}
		i, nm := FindNamed(newDecl.Members, name)
		if nm == nil {
			continue
		}
		// A name the body declares twice is ambiguous: merging would add the new
		// members to each of them, so it is replaced instead.
		if oldSub, ok := declOf(oldSrc, om, describe); ok && count[name] == 1 {
			// An empty body clears a nested namespace just as it clears a
			// top-level one, so it replaces the old member instead of merging.
			if newSub, ok := declOf(newSrc, nm, describe); ok && len(newSub.Members) > 0 && oldSub.Header == newSub.Header {
				subEdits, subReplaced, subGone := MergeEdits(oldSrc, oldSub, newSrc, newSub, path+"::"+name, describe)
				edits = append(edits, subEdits...)
				replaced = append(replaced, subReplaced...)
				gone = append(gone, subGone...)
				folded[i] = true
				continue
			}
		}
		// The new member is not a body to add to, so it supersedes the old one:
		// drop the old text and let the new member be added below.
		start, end := memberCut(oldSrc, om)
		edits = append(edits, Edit{Start: start, End: end})
		replaced = append(replaced, describe(om))
		gone = append(gone, path+"::"+name)
	}
	var added []string
	for i, nm := range newDecl.Members {
		if folded[i] {
			continue
		}
		text := trimmedText(newSrc, nm)
		if text == "" {
			continue
		}
		// A member declaring no name (an import, a comment) matches only on its
		// text, so an identical one in the body is it re-typed, not a second copy.
		if MemberName(nm) == "" && hasText(oldSrc, oldDecl.Members, memberText(newSrc, nm)) {
			continue
		}
		added = append(added, text)
	}
	// Trivia before the body's first member sits in no member's span, so it is
	// carried over explicitly or the comments typed there would be dropped.
	if lead := leadingTrivia(newSrc, newDecl); lead != "" && !strings.Contains(oldSrc, lead) {
		added = append([]string{lead}, added...)
	}
	if len(added) > 0 {
		e := insertion(oldSrc, oldDecl.Brace, added)
		e.Own = true
		edits = append(edits, e)
	}
	return edits, replaced, gone
}

// leadingTrivia returns the text between a body's opening brace and its first
// member: what the submission typed above everything else in the body.
func leadingTrivia(src string, decl Decl) string {
	if len(decl.Members) == 0 {
		return ""
	}
	end := decl.Members[0].Span().Offset
	if end <= decl.Open+1 || end > decl.Brace {
		return ""
	}
	return strings.TrimSpace(src[decl.Open+1 : end])
}

// memberCut is the range to delete to remove a member: its own text plus the
// line it owns, so no blank line is left behind, the closing brace keeps its
// indentation, and comments documenting the next member survive.
func memberCut(src string, member ast.Node) (start, end int) {
	start = member.Span().Offset
	end = start + len(memberText(src, member))
	lineStart := strings.LastIndexByte(src[:start], '\n') + 1
	if strings.TrimSpace(src[lineStart:start]) != "" {
		// Mid-line: take the spaces after it too, so the body does not end up
		// with a gap where the member was.
		for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
			end++
		}
		return start, end
	}
	rest := src[end:]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 || strings.TrimSpace(rest[:nl]) != "" {
		return start, end
	}
	return lineStart, end + nl + 1
}

// hasText reports whether one of the members is the given source text, comparing
// each member's own text so trailing comments do not hide a match.
func hasText(src string, members []ast.Node, text string) bool {
	for _, m := range members {
		if memberText(src, m) == text {
			return true
		}
	}
	return false
}

// nameCounts counts how many of the members declare each name.
func nameCounts(members []ast.Node) map[string]int {
	out := make(map[string]int, len(members))
	for _, m := range members {
		if name := MemberName(m); name != "" {
			out[name]++
		}
	}
	return out
}

// FindNamed returns the first member declaring name, and its index. Only the
// first is taken: a submission that declares one name twice keeps both members,
// so it still reports a duplicate declaration.
func FindNamed(members []ast.Node, name string) (int, ast.Node) {
	for i, m := range members {
		if MemberName(m) == name {
			return i, m
		}
	}
	return -1, nil
}

// insertion is the Edit adding members to a body, laid out like it: one indented
// line each in a multi-line body, space-separated in a one-line body. Lines go
// above a closing brace that owns its line, so that brace keeps its indentation.
func insertion(src string, brace int, members []string) Edit {
	lineStart := strings.LastIndexByte(src[:brace], '\n') + 1
	prefix := src[lineStart:brace]
	if strings.TrimSpace(prefix) != "" {
		// Text carrying a comment or its own lines cannot go inline: the
		// comment would swallow the rest of the line, brace included.
		if !multiline(members) {
			return Edit{Start: brace, End: brace, Text: strings.Join(members, " ") + " "}
		}
		indent := prefix[:len(prefix)-len(strings.TrimLeft(prefix, " \t"))]
		var b strings.Builder
		for _, m := range members {
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString("\t")
			b.WriteString(m)
		}
		b.WriteString("\n")
		b.WriteString(indent)
		return Edit{Start: brace, End: brace, Text: b.String()}
	}
	var b strings.Builder
	for _, m := range members {
		b.WriteString(prefix)
		b.WriteString("\t")
		b.WriteString(m)
		b.WriteString("\n")
	}
	return Edit{Start: lineStart, End: lineStart, Text: b.String()}
}

// multiline reports whether any member's text has to be written on lines of its
// own: one spanning several lines, or one a line comment runs to the end of.
func multiline(members []string) bool {
	for _, m := range members {
		if strings.ContainsRune(m, '\n') || strings.Contains(m, "//") {
			return true
		}
	}
	return false
}

// Apply rewrites src with the given edits applied, and returns where each
// Edit's text landed in the result. An insertion inside an already-rewritten
// range is written at the point reached rather than dropped, so no added text is
// ever silently lost.
func Apply(src string, edits []Edit) (string, []source.Span) {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].Start < edits[j].Start })
	var (
		b     strings.Builder
		added []source.Span
		pos   int
	)
	for _, e := range edits {
		if e.End > len(src) || (e.Start < pos && e.Start != e.End) {
			continue
		}
		if e.Start > pos {
			b.WriteString(src[pos:e.Start])
			pos = e.Start
		}
		if e.Own {
			added = append(added, source.Span{Offset: b.Len(), Len: len(e.Text)})
		}
		b.WriteString(e.Text)
		if e.End > pos {
			pos = e.End
		}
	}
	b.WriteString(src[pos:])
	return b.String(), added
}

// memberText returns a member's own source text: its span less the trailing
// comment trivia the span runs on to the next member, which documents that
// member rather than this one.
func memberText(src string, node ast.Node) string {
	text := trimmedText(src, node)
	lx := lexer.New(source.New("<merge>", []byte(text)))
	end, trailing := 0, false
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.IsTrivia() {
			trailing = end > 0
			continue
		}
		end, trailing = tok.Span.Offset+tok.Span.Len, false
	}
	// Trivia only documents what follows once the member itself is terminated;
	// anything else (a `doc /* ... */`) is the member's own text.
	if !trailing || end > len(text) || (text[end-1] != ';' && text[end-1] != '}') {
		return text
	}
	return text[:end]
}

// trimmedText returns a node's source text without the whitespace its span runs
// on to the next member.
func trimmedText(src string, node ast.Node) string {
	span := node.Span()
	start, end := span.Offset, span.Offset+span.Len
	if end > len(src) {
		end = len(src)
	}
	if start > end {
		return ""
	}
	return strings.TrimRight(src[start:end], " \t\r\n")
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// MemberName returns the replaceable name a namespace member declares, or "" for
// a member that declares none (an import, a comment, an error node).
func MemberName(m ast.Node) string {
	node := m
	if mem, ok := m.(*ast.Membership); ok {
		node = mem.Member
	}
	switch d := node.(type) {
	case *ast.Package:
		return d.Ident.Name
	case *ast.Namespace:
		return d.Ident.Name
	case *ast.Alias:
		return d.Ident.Name
	case *ast.RelationshipMember:
		return d.Ident.Name
	case *ast.Definition:
		return d.Ident.Name
	case *ast.Usage:
		// A usage that declares no name of its own answers to its reference's or
		// redefinition's, which is the name re-typing it supersedes
		// (ast.EffectiveName).
		name, _ := ast.EffectiveName(d)
		return name
	}
	return ""
}

func sourceForKind(name string, data []byte, kind source.Kind) *source.SourceFile {
	if kind == source.KindUnknown {
		return source.New(name, data)
	}
	return source.NewWithKind(name, data, kind)
}
