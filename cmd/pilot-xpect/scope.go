package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// metatype is the reference type an XPECT scope query is filtered by: the
// pilot's scope holds only elements the anchor's cross-reference can name.
type metatype int

const (
	// mtAny admits every element, as an import or an alias names a Membership.
	mtAny metatype = iota
	// mtType admits Types — KerML Feature is a Type, a Package is not.
	mtType
	// mtFeature admits Features, which is what a redefinition names.
	mtFeature
	// mtClassifier admits Classifiers, which is what a subclassification names.
	mtClassifier
)

// scopeCounts is the numeric side of a scope adjudication, kept beside the
// prose so a reader can total the classes without parsing sentences.
type scopeCounts struct {
	Declared int `json:"declared"`
	Ours     int `json:"ours"`
	// Missing is declared names we do not offer; OtherPath is the subset of
	// them whose element we do offer under a different path.
	Missing   int `json:"missing,omitempty"`
	OtherPath int `json:"otherPath,omitempty"`
	Extra     int `json:"extra,omitempty"`
}

// scopeRow adjudicates one XPECT scope assertion: the pilot declares the whole
// set of names visible at the anchor, so both a missing and an extra name are
// findings.
func scopeRow(ws *model.Workspace, main string, a assertion, src squeezed, libraryRoots []string) row {
	r := row{Kind: a.Kind, Block: a.Block, Line: a.Line, At: a.At,
		Declared: fmt.Sprintf("%d name(s) visible at %q", len(a.Names), a.At)}

	doc := ws.Document(main)
	if doc == nil {
		r.Verdict = verdictUnlocated
		r.Actual = "the file did not parse into a document"
		return r
	}
	// The anchor is normally a name reference, whose cross-reference type also
	// filters the scope; where it is a declaration's own name instead, the
	// scope is the namespace that position sits in and nothing filters it.
	offset, ref, ok, found := scopeAnchor(doc, a, src)
	if !found {
		r.Verdict = verdictUnlocated
		r.Actual = fmt.Sprintf("the declared text %q does not occur after the assertion", a.At)
		return r
	}
	want, scope := mtAny, ws.ScopeAt(main, offset)
	if ok {
		want = anchorMetatype(src.Original, offset, ref)
		scope = anchorScope(src.Original, offset, ref.Scope)
	}
	if scope == nil {
		r.Verdict = verdictUnlocated
		r.Actual = fmt.Sprintf("no scope encloses offset %d", offset)
		return r
	}
	// Whether a fixture sees the library's implicit members is its own
	// declaration: only a resource set loading /library has them in scope.
	opts := model.VisibleNamesOptions{
		Redefinition: narrowsToInherited(ref),
		LibraryRoots: libraryRoots,
	}
	d := scopeDiffOf(ws, scope, ws.VisibleNames(scope, opts), want, a.Names)

	r.Names = &scopeCounts{
		Declared: len(a.Names), Ours: len(d.ours),
		Missing: len(d.missing), OtherPath: len(d.otherPath), Extra: len(d.extra),
	}
	if d.distance() == 0 {
		r.Verdict = verdictAgree
		r.Actual = fmt.Sprintf("the same %d name(s)", len(d.ours))
		return r
	}

	r.Verdict = verdictDisagree
	switch {
	case d.implicitOnly:
		r.Tolerance = toleranceScopeLibrary
	case len(d.realMissing) == 0:
		r.Tolerance = toleranceScopeExtra
	case len(d.realExtra) == 0 && len(d.realMissing) == len(d.otherPath):
		r.Tolerance = toleranceScopeSpelling
	case len(d.realExtra) > 0:
		r.Tolerance = toleranceScopeBoth
	default:
		r.Tolerance = toleranceScopeMissing
	}
	r.Actual = fmt.Sprintf("%d name(s): missing %d (%s), extra %d (%s)",
		len(d.ours), len(d.missing), sample(d.missing), len(d.extra), sample(d.extra))
	if len(d.otherPath) > 0 {
		r.Actual += fmt.Sprintf("; %d missing name(s) reachable by another path (%s)",
			len(d.otherPath), sample(d.otherPath))
	}
	return r
}

// narrowsToInherited reports whether the pilot scopes the anchor to inherited
// members only: a redefinition does, a subsetting may name any accessible feature.
func narrowsToInherited(ref resolve.Reference) bool {
	return ref.Redefines && ref.Subsetting == nil
}

// scopeAnchor locates the position a scope assertion is taken at. The `at` text
// names the reference the question is about, so an occurrence that starts a
// longer name — `c_Public` in `specializes c_Public_Id` — is the anchor when it
// carries one; otherwise the first whole identifier is, and nothing filters it.
func scopeAnchor(doc *model.Document, a assertion, src squeezed) (int, resolve.Reference, bool, bool) {
	refs := resolve.References(doc.AST, doc.Scope)
	var first match
	for i, m := range src.locateAll(a.Region, a.At) {
		if i == 0 {
			first = m
		}
		if ref, _, ok := referenceAt(refs, m.begin, m.end); ok {
			return m.begin, ref, true, true
		}
		if m.whole {
			return m.begin, resolve.Reference{}, false, true
		}
	}
	if first.end == 0 {
		return 0, resolve.Reference{}, false, false
	}
	return first.begin, resolve.Reference{}, false, true
}

// scopeDiff is one enumeration set measured against the declared one. The
// implicit members Base contributes to every declaration — `self` and `that` —
// are counted apart, because the pilot's own traversal truncates paths through
// them unevenly (see docs/project/pilot-xpect.md).
type scopeDiff struct {
	ours                      map[string]bool
	missing, otherPath, extra []string
	// implicitOnly reports that every difference is a path ending in an
	// implicit library member.
	implicitOnly bool
	// realMissing and realExtra are the differences left once those paths are
	// set aside, which is the worklist the row contributes.
	realMissing, realExtra []string
}

// distance is the size of the symmetric difference with the declared set.
func (d scopeDiff) distance() int { return len(d.missing) + len(d.extra) }

// scopeDiffOf compares one enumeration, filtered by the anchor's metatype,
// against the names an assertion declares. A declared name we do not offer is
// looked up as a path to tell a lost element from a differently spelled one.
func scopeDiffOf(
	ws *model.Workspace,
	scope *symbols.Scope,
	visible []model.VisibleName,
	want metatype,
	names []string,
) scopeDiff {
	d := scopeDiff{ours: map[string]bool{}}
	byFQN := map[string]bool{}
	for _, n := range visible {
		if !admits(want, n.Kind) {
			continue
		}
		d.ours[n.Name] = true
		byFQN[n.FQN] = true
	}
	declared := map[string]bool{}
	for _, name := range names {
		declared[name] = true
		if d.ours[name] {
			continue
		}
		d.missing = append(d.missing, name)
		if reachableAs(ws, scope, byFQN, name) {
			d.otherPath = append(d.otherPath, name)
		}
	}
	for name := range d.ours {
		if !declared[name] {
			d.extra = append(d.extra, name)
		}
	}
	d.realMissing = notImplicit(d.missing)
	d.realExtra = notImplicit(d.extra)
	d.implicitOnly = len(d.realMissing) == 0 && len(d.realExtra) == 0
	sort.Strings(d.missing)
	sort.Strings(d.otherPath)
	sort.Strings(d.extra)
	sort.Strings(d.realMissing)
	sort.Strings(d.realExtra)
	return d
}

// notImplicit drops the paths that end in an implicit library member.
func notImplicit(names []string) []string {
	var out []string
	for _, name := range names {
		switch name[strings.LastIndex(name, ".")+1:] {
		case "self", "that":
		default:
			out = append(out, name)
		}
	}
	return out
}

// sample renders at most the first five names of a class, so a row stays
// readable when a declared list runs to hundreds of names.
func sample(names []string) string {
	const limit = 5
	if len(names) == 0 {
		return "none"
	}
	if len(names) <= limit {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:limit], ", ") + ", …"
}

// reachableAs reports whether a declared path we do not offer names an element
// we do offer under another path: the path is resolved from the anchor scope
// and its target's qualified name looked for among the ones we hold.
func reachableAs(
	ws *model.Workspace,
	scope *symbols.Scope,
	byFQN map[string]bool,
	name string,
) bool {
	sym, ok := ws.ElementOnPath(scope, strings.Split(name, "."))
	if !ok {
		return false
	}
	return byFQN[ws.FQNOf(sym)]
}

// anchorMetatype is the reference type the pilot filters the anchor's scope by,
// read back from the declaration the anchor sits in: a subclassification names
// a Classifier, a feature typing a Type, a subsetting or redefinition a
// Feature, and an import or alias a Membership, whose element is any Element.
func anchorMetatype(source []byte, offset int, ref resolve.Reference) metatype {
	head := declarationHead(source, offset)
	if ref.Redefines {
		return mtFeature
	}
	for _, word := range head {
		switch word {
		case "import", "alias":
			return mtAny
		}
	}
	switch relationshipOf(head) {
	case "redefines", ":>>", "references", "::>", "subsets":
		return mtFeature
	case "specializes", ":>":
		if classifierHead(head) {
			return mtClassifier
		}
		return mtFeature
	default:
		return mtType
	}
}

// anchorScope is the namespace the pilot enumerates the anchor's scope from: a
// name written in a declaration's own head is resolved in the namespace the
// declaration sits in, not inside the element being declared.
func anchorScope(source []byte, offset int, scope *symbols.Scope) *symbols.Scope {
	start := headStart(source, offset)
	for scope != nil && scope.Parent() != nil {
		owner := scope.Owner()
		if owner == nil || owner.DeclSpan.Offset < start || owner.DeclSpan.Offset >= offset {
			break
		}
		scope = scope.Parent()
	}
	return scope
}

// headStart is the offset the statement containing offset begins at.
func headStart(source []byte, offset int) int {
	if offset < 0 || offset > len(source) {
		return 0
	}
	for i := offset - 1; i >= 0; i-- {
		if c := source[i]; c == ';' || c == '{' || c == '}' || c == ',' {
			return i + 1
		}
	}
	return 0
}

// declarationHead is the words of the statement the anchor sits in, up to the
// anchor itself.
func declarationHead(source []byte, offset int) []string {
	if offset < 0 || offset > len(source) {
		return nil
	}
	return strings.Fields(string(source[headStart(source, offset):offset]))
}

// relationshipOf is the last relationship keyword a declaration head writes,
// which is the one the anchor is the target of.
func relationshipOf(head []string) string {
	for i := len(head) - 1; i >= 0; i-- {
		switch word := head[i]; word {
		case "specializes", ":>", "subsets", "redefines", ":>>", "references", "::>", ":", "typed":
			return word
		}
	}
	return ""
}

// classifierHead reports whether a declaration head declares a classifier
// rather than a feature, which decides whether its `specializes` names a
// Classifier or a Feature.
func classifierHead(head []string) bool {
	for _, word := range head {
		switch word {
		case "classifier", "class", "struct", "datatype", "assoc", "association",
			"behavior", "function", "predicate", "interaction", "metaclass",
			"type", "def", "package":
			return true
		}
	}
	return false
}

// admits reports whether a name of this kind is in a scope filtered by want.
func admits(want metatype, kind symbols.SymbolKind) bool {
	switch want {
	case mtFeature:
		return isFeatureKind(kind)
	case mtClassifier:
		return isTypeKind(kind) && !isFeatureKind(kind)
	case mtType:
		return isTypeKind(kind)
	default:
		return kind != symbols.SymbolUnknown
	}
}

// isTypeKind reports whether a kind declares a KerML Type. Features are Types
// (KerML 8.3.3), so only namespaces and annotations are excluded.
func isTypeKind(kind symbols.SymbolKind) bool {
	switch kind {
	case symbols.SymbolUnknown, symbols.SymbolPackage, symbols.SymbolNamespace,
		symbols.SymbolAlias, symbols.SymbolDependency, symbols.SymbolComment,
		symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation:
		return false
	default:
		return true
	}
}

// isFeatureKind reports whether a kind declares a KerML Feature: a usage, a
// connector end, or a step.
func isFeatureKind(kind symbols.SymbolKind) bool {
	if !isTypeKind(kind) {
		return false
	}
	return strings.HasSuffix(kind.String(), "Usage") || kind == symbols.SymbolConnectorEnd || kind == symbols.SymbolCrossFeature
}
