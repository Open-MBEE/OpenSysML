package repl

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// nsDecl is a namespace declaration with a body, located in the text it was
// parsed from: where it starts, its members, and the brace an addition goes at.
type nsDecl struct {
	name    string
	desc    string // "package P", as the summary renders it
	header  string // everything before the body, whitespace-normalized
	members []ast.Node
	start   int
	open    int
	brace   int
}

// edit replaces src[start:end] with text. Edits never overlap: each one covers a
// distinct member, or is an insertion at a body's closing brace. An own edit
// marks where the submission lands, so its report scopes to what was typed.
type edit struct {
	start, end int
	text       string
	own        bool
}

// mergeSubmission folds a resubmitted namespace declaration into the same-named
// one in the buffer, so re-typing `package P { ... }` adds to it instead of
// replacing its body. It removes the snippet it merged with, since the returned
// text supersedes it, and reports the members the new body redeclares. An empty
// body is not merged: that is how a namespace is emptied.
//
// The returned spans locate the submitted text inside the merged result, so a
// report still covers what was typed rather than the whole absorbed snippet.
func (s *Session) mergeSubmission(src string, root *ast.RootNamespace, comments string) (string, []source.Span, dropReport, bool) {
	newDecl, ok := soleNamespace(src, root)
	if !ok || len(newDecl.members) == 0 {
		return "", nil, dropReport{}, false
	}
	for i, sn := range s.snippets {
		// Only what the prompt typed in an earlier submission merges: a loaded
		// file keeps its identity, so re-typing its package supersedes it, and
		// two snippets of one submission are both part of that submission. A
		// masked submission is not merged into either: its text is not analyzed,
		// so folding it in would put what the parser could not read back into the
		// buffer.
		if sn.origin != "" || sn.gen == s.version || sn.open {
			continue
		}
		oldDecl, ok := namedNamespace(sn.src, newDecl.name)
		// A different header is a different declaration, whatever it names: it
		// replaces the old one rather than adding to a body it did not write.
		if !ok || oldDecl.header != newDecl.header {
			continue
		}
		edits, replaced, gone := mergeEdits(sn.src, oldDecl, src, newDecl, newDecl.name)
		// Comments typed above the addition document the declaration, so they go
		// above it rather than at the tail of the buffer.
		if comments != "" {
			edits = append(edits, edit{start: oldDecl.start, end: oldDecl.start, text: comments})
		}
		// The declaration merged into is this submission's own, so it is echoed
		// even when the body it re-typed added nothing new.
		edits = append(edits, edit{start: oldDecl.start, end: oldDecl.start, own: true})
		merged, own := applyEdits(sn.src, edits)
		s.snippets = append(s.snippets[:i:i], s.snippets[i+1:]...)
		return merged, own, dropReport{merged: true, decl: newDecl.desc, lost: replaced, gone: gone}, true
	}
	return "", nil, dropReport{}, false
}

// reopenedNamespaces reports the namespaces a loaded file opens that another
// loaded file already opened. Two files that open `package P` declare two
// packages of that name rather than one shared package — KerML gives a package
// no way to be reopened — so the load says so instead of leaving the user with
// unresolved references between them.
func (s *Session) reopenedNamespaces(key string, root *ast.RootNamespace) []dropReport {
	if root == nil {
		return nil
	}
	var opened []string
	wanted := make(map[string]bool)
	for _, m := range root.Members {
		name := memberName(m)
		if name == "" || wanted[name] {
			continue
		}
		if _, hasBody := bodyMembers(m); !hasBody {
			continue
		}
		wanted[name] = true
		opened = append(opened, name)
	}
	if len(wanted) == 0 {
		return nil
	}
	// A snippet is parsed only when its recorded names say it could open one of
	// these, and then once for all of them, so a directory load stays linear.
	reopened := make(map[string]bool, len(wanted))
	for _, sn := range s.snippets {
		if sn.origin == "" || sn.key == key || sn.open || len(reopened) == len(wanted) {
			continue
		}
		var shared []string
		for _, name := range sn.names {
			if wanted[name] && !reopened[name] {
				shared = append(shared, name)
			}
		}
		if len(shared) == 0 {
			continue
		}
		snRoot := parser.New(source.New(parseDocName(sn.origin), []byte(sn.src))).ParseFile()
		for _, name := range shared {
			if _, ok := namedNamespaceIn(sn.src, snRoot, name); ok {
				reopened[name] = true
			}
		}
	}
	var out []dropReport
	for _, name := range opened {
		if reopened[name] {
			out = append(out, dropReport{reopened: name})
		}
	}
	return out
}

// soleNamespace returns the namespace declaration a submission consists of. A
// submission declaring more than one thing is replaced wholesale, since its text
// cannot be split between snippets without losing what sits between them.
func soleNamespace(src string, root *ast.RootNamespace) (nsDecl, bool) {
	if root == nil || len(root.Members) != 1 {
		return nsDecl{}, false
	}
	return namespaceDeclOf(src, root.Members[0])
}

// namedNamespace finds the namespace declaration of the given name in an
// accepted snippet. It reports false unless exactly one member declares that
// name, so an ambiguous snippet keeps the replacement behavior.
func namedNamespace(src, name string) (nsDecl, bool) {
	return namedNamespaceIn(src, parser.New(source.New(docName, []byte(src))).ParseFile(), name)
}

// namedNamespaceIn is namedNamespace over a parse the caller already has.
func namedNamespaceIn(src string, root *ast.RootNamespace, name string) (nsDecl, bool) {
	if root == nil {
		return nsDecl{}, false
	}
	var found ast.Node
	for _, m := range root.Members {
		if memberName(m) != name {
			continue
		}
		if found != nil {
			return nsDecl{}, false
		}
		found = m
	}
	if found == nil {
		return nsDecl{}, false
	}
	return namespaceDeclOf(src, found)
}

// namespaceDeclOf describes a member as a namespace declaration with a body,
// reporting false for anything else — a usage, a definition, a `package P;`
// without a body, or a body whose brace the parser never saw.
func namespaceDeclOf(src string, member ast.Node) (nsDecl, bool) {
	node := member
	if mem, ok := member.(*ast.Membership); ok {
		node = mem.Member
	}
	var d nsDecl
	switch n := node.(type) {
	case *ast.Package:
		if !n.HasBody {
			return d, false
		}
		d.name, d.members = n.Ident.Name, n.Members
	case *ast.Namespace:
		if !n.HasBody {
			return d, false
		}
		d.name, d.members = n.Ident.Name, n.Members
	default:
		return d, false
	}
	if d.name == "" {
		return d, false
	}
	brace, ok := closingBrace(src, node)
	if !ok {
		return d, false
	}
	d.start, d.brace = member.Span().Offset, brace
	open := strings.IndexByte(src[d.start:brace], '{')
	if open < 0 {
		return d, false
	}
	d.open = d.start + open
	d.header = strings.Join(strings.Fields(src[d.start:d.start+open]), " ")
	d.desc = renderMember(member)
	return d, true
}

// bodyMembers returns the members of a namespace member's body. Unlike
// namespaceDeclOf it does not need the body to be terminated, so it can also
// report what an unparseable re-declaration dropped.
func bodyMembers(member ast.Node) ([]ast.Node, bool) {
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

// mergeEdits computes the edits on oldSrc that fold newDecl's members into
// oldDecl, plus a description of each member the new body replaces and its
// qualified name under path. A member redeclared as a namespace on both sides is
// merged in turn, so adding to a nested package keeps the nested members too.
func mergeEdits(oldSrc string, oldDecl nsDecl, newSrc string, newDecl nsDecl, path string) ([]edit, []string, []string) {
	var (
		edits    []edit
		replaced []string
		gone     []string
		folded   = make(map[int]bool, len(newDecl.members))
		count    = nameCounts(oldDecl.members)
	)
	for _, om := range oldDecl.members {
		name := memberName(om)
		if name == "" {
			continue
		}
		i, nm := findNamed(newDecl.members, name)
		if nm == nil {
			continue
		}
		// A name the body declares twice is ambiguous: merging would add the new
		// members to each of them, so it is replaced instead.
		if oldSub, ok := namespaceDeclOf(oldSrc, om); ok && count[name] == 1 {
			// An empty body clears a nested namespace just as it clears a
			// top-level one, so it replaces the old member instead of merging.
			if newSub, ok := namespaceDeclOf(newSrc, nm); ok && len(newSub.members) > 0 && oldSub.header == newSub.header {
				subEdits, subReplaced, subGone := mergeEdits(oldSrc, oldSub, newSrc, newSub, path+"::"+name)
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
		edits = append(edits, edit{start: start, end: end})
		replaced = append(replaced, renderMember(om))
		gone = append(gone, path+"::"+name)
	}
	var added []string
	for i, nm := range newDecl.members {
		if folded[i] {
			continue
		}
		text := trimmedText(newSrc, nm)
		if text == "" {
			continue
		}
		// A member declaring no name (an import, a comment) matches only on its
		// text, so an identical one in the body is it re-typed, not a second copy.
		if memberName(nm) == "" && hasText(oldSrc, oldDecl.members, memberText(newSrc, nm)) {
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
		e := insertion(oldSrc, oldDecl.brace, added)
		e.own = true
		edits = append(edits, e)
	}
	return edits, replaced, gone
}

// leadingTrivia returns the text between a body's opening brace and its first
// member: what the submission typed above everything else in the body.
func leadingTrivia(src string, decl nsDecl) string {
	if len(decl.members) == 0 {
		return ""
	}
	end := decl.members[0].Span().Offset
	if end <= decl.open+1 || end > decl.brace {
		return ""
	}
	return strings.TrimSpace(src[decl.open+1 : end])
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
		if name := memberName(m); name != "" {
			out[name]++
		}
	}
	return out
}

// findNamed returns the first member declaring name, and its index. Only the
// first is taken: a submission that declares one name twice keeps both members,
// so it still reports a duplicate declaration.
func findNamed(members []ast.Node, name string) (int, ast.Node) {
	for i, m := range members {
		if memberName(m) == name {
			return i, m
		}
	}
	return -1, nil
}

// insertion is the edit adding members to a body, laid out like it: one indented
// line each in a multi-line body, space-separated in a one-line body. Lines go
// above a closing brace that owns its line, so that brace keeps its indentation.
func insertion(src string, brace int, members []string) edit {
	lineStart := strings.LastIndexByte(src[:brace], '\n') + 1
	prefix := src[lineStart:brace]
	if strings.TrimSpace(prefix) != "" {
		// Text carrying a comment or its own lines cannot go inline: the
		// comment would swallow the rest of the line, brace included.
		if !multiline(members) {
			return edit{start: brace, end: brace, text: strings.Join(members, " ") + " "}
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
		return edit{start: brace, end: brace, text: b.String()}
	}
	var b strings.Builder
	for _, m := range members {
		b.WriteString(prefix)
		b.WriteString("\t")
		b.WriteString(m)
		b.WriteString("\n")
	}
	return edit{start: lineStart, end: lineStart, text: b.String()}
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

// applyEdits rewrites src with the given edits applied, and returns where each
// edit's text landed in the result. An insertion inside an already-rewritten
// range is written at the point reached rather than dropped, so no added text is
// ever silently lost.
func applyEdits(src string, edits []edit) (string, []source.Span) {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var (
		b     strings.Builder
		added []source.Span
		pos   int
	)
	for _, e := range edits {
		if e.end > len(src) || (e.start < pos && e.start != e.end) {
			continue
		}
		if e.start > pos {
			b.WriteString(src[pos:e.start])
			pos = e.start
		}
		if e.own {
			added = append(added, source.Span{Offset: b.Len(), Len: len(e.text)})
		}
		b.WriteString(e.text)
		if e.end > pos {
			pos = e.end
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
	lx := lexer.New(source.New(docName, []byte(text)))
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
