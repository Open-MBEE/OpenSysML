package repl

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/nsmerge"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// nsmerge.Decl is a namespace declaration with a body, located in the text it was
// parsed from: where it starts, its members, and the brace an addition goes at.

// mergeSubmission folds a resubmitted namespace declaration into the same-named
// one in the buffer, so re-typing `package P { ... }` adds to it instead of
// replacing its body. It removes the snippet it merged with, since the returned
// text supersedes it, and reports the members the new body redeclares. An empty
// body is not merged: that is how a namespace is emptied.
//
// The returned spans locate the submitted text inside the merged result, so a
// report still covers what was typed rather than the whole absorbed snippet.
// The last result reports the merge rewrote a snippet where it stands: the
// submission's text is wholly inside it, so the caller appends nothing.
func (s *Session) mergeSubmission(src string, root *ast.RootNamespace, comments string) (string, []source.Span, dropReport, bool, bool) {
	newDecl, ok := nsmerge.SoleNamespace(src, root, describeMember)
	if !ok || len(newDecl.Members) == 0 {
		return "", nil, dropReport{}, false, false
	}
	// A record nests its target package inside the same-named top namespace
	// several files may open: the file already holding the deepest prefix of
	// that nesting is where it belongs.
	chain := nsmerge.Chain(src, newDecl)
	target, depth := -1, -1
	var oldDecl nsmerge.Decl
	for i, sn := range s.snippets {
		// Only what the prompt typed in an earlier submission merges: a loaded
		// file keeps its identity, so re-typing its package supersedes it, and
		// two snippets of one submission are both part of that submission. A
		// masked submission is not merged into either: its text is not analyzed,
		// so folding it in would put what the parser could not read back into the
		// buffer. A recorded run is the one exception: it merges into a loaded
		// file's package, which keeps its file's identity on the result.
		if sn.gen == s.version || sn.open || (sn.origin != "" && !s.recordMerge) {
			continue
		}
		cand, ok := nsmerge.Named(sn.src, newDecl.Name, sn.kind, describeMember)
		// A different header is a different declaration, whatever it names: it
		// replaces the old one rather than adding to a body it did not write.
		if !ok || cand.Header != newDecl.Header {
			continue
		}
		if !s.recordMerge {
			target, oldDecl = i, cand
			break
		}
		if d := nsmerge.Depth(sn.src, cand, chain); d > depth {
			target, depth, oldDecl = i, d, cand
		}
	}
	for i, sn := range s.snippets {
		if i != target {
			continue
		}
		edits, replaced, gone := nsmerge.MergeEdits(sn.src, oldDecl, src, newDecl, newDecl.Name, describeMember)
		// Comments typed above the addition document the declaration, so they go
		// above it rather than at the tail of the buffer.
		if comments != "" {
			edits = append(edits, nsmerge.Edit{Start: oldDecl.Start, End: oldDecl.Start, Text: comments})
		}
		// The declaration merged into is this submission's own, so it is echoed
		// even when the body it re-typed added nothing new.
		edits = append(edits, nsmerge.Edit{Start: oldDecl.Start, End: oldDecl.Start, Own: true})
		merged, own := nsmerge.Apply(sn.src, edits)
		if sn.origin != "" {
			// A file's text is updated where it is, its origin and key kept, so
			// a later reload of the file still supersedes it; the submission's
			// own text is wholly inside it and is not appended again.
			names := declaredNames(parser.New(sourceForKind(parseDocName(sn.origin), []byte(merged), sn.kind)).ParseFile())
			s.snippets[i] = snippet{src: merged, names: names, origin: sn.origin, key: sn.key, kind: sn.kind, gen: s.version, own: own}
			return "", nil, dropReport{merged: true, decl: newDecl.Desc, lost: replaced, gone: gone}, true, true
		}
		s.snippets = append(s.snippets[:i:i], s.snippets[i+1:]...)
		return merged, own, dropReport{merged: true, decl: newDecl.Desc, lost: replaced, gone: gone}, true, false
	}
	return "", nil, dropReport{}, false, false
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
		name := nsmerge.MemberName(m)
		if name == "" || wanted[name] {
			continue
		}
		if _, hasBody := nsmerge.BodyMembers(m); !hasBody {
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
		snRoot := parser.New(sourceForKind(parseDocName(sn.origin), []byte(sn.src), sn.kind)).ParseFile()
		for _, name := range shared {
			if _, ok := nsmerge.NamedIn(sn.src, snRoot, name, describeMember); ok {
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

// nsmerge.SoleNamespace returns the namespace declaration a submission consists of. A
// submission declaring more than one thing is replaced wholesale, since its text
// cannot be split between snippets without losing what sits between them.
