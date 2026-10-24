// Package workspace holds the checks made of a workspace as a whole rather than
// of one document: how the documents it holds fit together, which no document's
// own analysis sees. Each finding is a diagnostic attached to the document it is
// best acted on in, reported once per workspace.
package workspace

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// CodeLibraryDeclaredTwice is the code of the warning that two documents in
// the workspace declare the same library package.
const CodeLibraryDeclaredTwice = "library-declared-twice"

// Source is the Source every finding of this package carries.
const Source = "workspace"

// Finding is one workspace-level diagnostic.
type Finding struct {
	// Document is the document the diagnostic is reported in: the one holding
	// the declaration references do not resolve to, where removing or renaming
	// it resolves the finding. Its span lies in that document's text.
	Document string
	// Library reports whether Document is a library document (bundled, or
	// loaded from the library path) rather than one of the workspace's own,
	// whose text the workspace does not hold.
	Library bool
	// Used is the document whose declarations references resolve to.
	Used string
	// Packages are the duplicated library packages, sorted.
	Packages   []string
	Diagnostic diag.Diagnostic
}

// duplicate is one library package declared by more than one document.
type duplicate struct {
	name     string
	used     *symbols.Symbol
	shadowed []*symbols.Symbol // the declarations references do not reach, by document
}

// DuplicateLibraries reports the library packages the index holds more than one
// declaration of. Two documents declaring the same `library package` — two copies
// of the standard library under the workspace folder or the library path, or a
// user library vendored twice — leave every reference to it resolving to one copy
// (the first the index registered) with the other silently unused. The findings
// are grouped by the pair of trees the copies lie in, so two copies of the whole
// standard library are one warning, not one per file.
func DuplicateLibraries(idx *symbols.Index) []Finding {
	byName := map[string][]*symbols.Symbol{}
	var order []string
	for _, doc := range idx.Documents() {
		root := idx.DocumentRoot(doc)
		if root == nil {
			continue
		}
		library := idx.IsLibraryDocument(doc)
		root.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym.Name == "" || sym.Kind != symbols.SymbolPackage || !(library || symbols.IsLibraryPackage(sym)) {
				return true
			}
			if _, seen := byName[sym.Name]; !seen {
				order = append(order, sym.Name)
			}
			byName[sym.Name] = append(byName[sym.Name], sym)
			return true
		})
	}
	var dups []duplicate
	for _, name := range order {
		syms := byName[name]
		if len(syms) < 2 {
			continue
		}
		used := usedDeclaration(idx, name, syms)
		d := duplicate{name: name, used: used}
		seen := map[string]bool{used.DocName: true}
		for _, sym := range syms {
			if !seen[sym.DocName] {
				seen[sym.DocName] = true
				d.shadowed = append(d.shadowed, sym)
			}
		}
		if len(d.shadowed) > 0 {
			dups = append(dups, d)
		}
	}
	return group(idx, dups)
}

// usedDeclaration is the declaration of name a qualified reference resolves to:
// the first the index registered under the name, the one every lookup by
// qualified name reads first.
func usedDeclaration(idx *symbols.Index, name string, syms []*symbols.Symbol) *symbols.Symbol {
	for _, sym := range idx.LookupQualified(name) {
		for _, s := range syms {
			if s == sym {
				return sym
			}
		}
	}
	return syms[0]
}

// treePair names the two trees a duplication lies in: what is left of each
// document's path once the path the copies share is stripped.
type treePair struct{ used, shadowed string }

// grouped is one finding in the making: the duplicated packages of one tree pair.
type grouped struct {
	pair     treePair
	names    []string
	used     string          // the used declaration's document, paired with shadowed
	shadowed *symbols.Symbol // the shadowed declaration the finding is attached to
}

// group folds the duplicates into one finding per pair of trees.
func group(idx *symbols.Index, dups []duplicate) []Finding {
	groups := map[treePair]*grouped{}
	var order []treePair
	for _, d := range dups {
		for _, sh := range d.shadowed {
			pair := treesOf(d.used.DocName, sh.DocName)
			g := groups[pair]
			if g == nil {
				g = &grouped{pair: pair, used: d.used.DocName, shadowed: sh}
				groups[pair] = g
				order = append(order, pair)
			}
			g.names = append(g.names, d.name)
			if sh.DocName < g.shadowed.DocName {
				g.shadowed, g.used = sh, d.used.DocName
			}
		}
	}
	sort.Slice(order, func(i, j int) bool {
		return groups[order[i]].shadowed.DocName < groups[order[j]].shadowed.DocName
	})
	out := make([]Finding, 0, len(order))
	for _, pair := range order {
		g := groups[pair]
		sort.Strings(g.names)
		span := g.shadowed.NameSpan
		if span.Len == 0 {
			span = g.shadowed.DeclSpan
		}
		out = append(out, Finding{
			Document: g.shadowed.DocName,
			Library:  idx.IsLibraryDocument(g.shadowed.DocName),
			Used:     g.used,
			Packages: g.names,
			Diagnostic: diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Span:     span,
				Message:  message(g),
				Code:     CodeLibraryDeclaredTwice,
				Source:   Source,
			},
		})
	}
	return out
}

// treesOf strips the path the two documents share from their ends, keeping the
// outermost directory they share: two copies of one library tree differ only in
// where the tree lies, and that tree is named by its own directory.
func treesOf(used, shadowed string) treePair {
	a := strings.Split(path.Clean(used), "/")
	b := strings.Split(path.Clean(shadowed), "/")
	n := 0
	for n < len(a) && n < len(b) && a[len(a)-1-n] == b[len(b)-1-n] {
		n++
	}
	if n > 1 {
		n-- // the file's own name is shared; keep the directory above the shared path
	}
	return treePair{used: strings.Join(a[:len(a)-n], "/"), shadowed: strings.Join(b[:len(b)-n], "/")}
}

// message words the finding: what is declared twice, where both copies lie and
// which one references resolve to.
func message(g *grouped) string {
	if len(g.names) == 1 {
		return fmt.Sprintf("library package %s is declared both here and in %s; references resolve to the declaration in %s, and this one is unused",
			g.names[0], g.used, g.used)
	}
	usedAt, shadowedAt := treeName(g.pair.used), treeName(g.pair.shadowed)
	return fmt.Sprintf("%s declared both under %s and under %s; references resolve to the declarations under %s, and the ones under %s are unused",
		packagesPhrase(g.names), shadowedAt, usedAt, usedAt, shadowedAt)
}

// treeName names a tree; the empty one is the library tree the index was loaded from.
func treeName(tree string) string {
	if tree == "" {
		return "the loaded standard library"
	}
	return tree
}

// packagesPhrase names the duplicated packages, the first few by name.
func packagesPhrase(names []string) string {
	const shown = 3
	if len(names) <= shown {
		return fmt.Sprintf("library packages %s are", strings.Join(names, ", "))
	}
	return fmt.Sprintf("%d library packages (%s, …) are", len(names), strings.Join(names[:shown], ", "))
}
