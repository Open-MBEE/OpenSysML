package export

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// nameKey identifies the references written in one declaration to one element:
// the qualified names of the declaring member and of the element referenced.
type nameKey struct {
	member, target string
}

// nameChoices is the spelling chosen for each reference and chain segment: one
// that resolves, from where it is written, to the element the graph names.
type nameChoices struct {
	references map[nameKey]string
	// segments holds the segments that need more than their element's own name.
	segments map[segmentKey]string
}

// segmentKey identifies the chain segments of one name written in one
// declaration after one operand ("" for none) to name one element.
type segmentKey struct {
	member, operand, name, target string
}

// written is the key of the segments spelled as name in the same place,
// whatever they name.
func (k segmentKey) written(name string) segmentKey {
	return segmentKey{member: k.member, operand: k.operand, name: name}
}

// wanted is what one rendering notes for chooseNames: the references to spell
// and the names read in an operand or body to check.
type wanted struct {
	// references holds, per reference, its fully qualified spelling and the
	// spelling the rendering wrote.
	references map[nameKey]wantedReference
	// segments holds the chain segments written, each naming its element.
	segments map[segmentKey]bool
	// starts holds the member each `first` names, keyed by the initial node.
	starts map[string]string
}

type wantedReference struct {
	qualified, written string
	// count is how many times the rendering wrote the reference.
	count int
}

func newWanted() *wanted {
	return &wanted{
		references: map[nameKey]wantedReference{},
		segments:   map[segmentKey]bool{},
		starts:     map[string]string{},
	}
}

func (w *wanted) empty() bool {
	return len(w.references) == 0 && len(w.segments) == 0 && len(w.starts) == 0
}

// chooseNames reads a rendering as the language name says, in the place of the
// bundled library document it is a version of if any, and picks each reference
// and chain segment the shortest spelling that resolves to its target there,
// never shorter than what previous chose; none is a refusal. changed reports
// whether any choice differs from the spelling the rendering wrote.
func chooseNames(name, library string, text []byte, want *wanted, previous *nameChoices) (names *nameChoices, changed bool, err error) {
	names = &nameChoices{references: map[nameKey]string{}, segments: map[segmentKey]string{}}
	if want.empty() {
		return names, false, nil
	}
	// A rendering that cannot be read back leaves every spelling unchecked, so
	// none is chosen by guess.
	file, root, ok := readNotation(name, text)
	if !ok {
		return nil, false, &UnsupportedError{
			What: "the references the graph links",
			Note: "the notation written for them does not parse, so no spelling can be checked to reach its element",
		}
	}
	e, err := newEncoder(file, root, library)
	if err != nil {
		return nil, false, &UnsupportedError{
			What: "the references the graph links",
			Note: fmt.Sprintf("the notation written for them cannot be read back (%v), so no spelling can be checked to reach its element", err),
		}
	}
	declared := make(map[string]ast.Node, len(e.fqn))
	for node, fqn := range e.fqn {
		declared[fqn] = node
	}
	written := writtenKeys(want.references)
	occurrences := map[nameKey][]resolve.Reference{}
	var chains, misread []resolve.Reference
	for _, ref := range resolve.References(root, e.res.Index().DocumentRoot(file.Name())) {
		if ref.QN == nil || ref.Member == nil {
			continue
		}
		if ref.Chain != nil {
			chains = append(chains, ref)
			continue
		}
		key := nameKey{member: e.fqn[ref.Member], target: e.writtenTarget(ref)}
		if _, ok := want.references[key]; ok {
			occurrences[key] = append(occurrences[key], ref)
			continue
		}
		misread = append(misread, ref)
	}
	// A spelling read as another element is still an occurrence of the reference
	// written that way, unless every writing of it already read back correctly.
	for _, ref := range misread {
		key, ok := written[nameKey{member: e.fqn[ref.Member], target: qualifiedText(ref.QN)}]
		if !ok || len(occurrences[key]) >= want.references[key].count {
			continue
		}
		occurrences[key] = append(occurrences[key], ref)
	}
	if err := e.checkStarts(want.starts, declared); err != nil {
		return nil, false, err
	}
	for key := range want.references {
		if _, ok := occurrences[key]; !ok {
			// A reference written but never read back cannot be checked to reach
			// its element, so it is not spelled by guess.
			return nil, false, &UnsupportedError{
				What: fmt.Sprintf("the reference to %s from %s", key.target, key.member),
				Note: "the notation written for it does not read back as a reference, so the spelling cannot be checked",
			}
		}
	}
	chosen := map[*ast.QualifiedName]string{}
	for key, refs := range occurrences {
		target, ok := declared[key.target]
		if !ok {
			continue
		}
		ref := want.references[key]
		spellings := referenceSpellings(ref.qualified)
		if previous != nil {
			spellings = fromWritten(spellings, ref.written)
		}
		spelling, ok := spellingFor(e.res, refs, spellings, target)
		if !ok {
			return nil, false, &UnsupportedError{
				What: fmt.Sprintf("the reference to %s from %s", key.target, key.member),
				Note: "no spelling of the name resolves to that element from where it is written, so the notation cannot state it",
			}
		}
		names.references[key] = spelling
		changed = changed || spelling != ref.written
		for _, r := range refs {
			chosen[r.QN] = spelling
		}
	}
	// A chain reads from its root, so its segments are spelled once the root is:
	// what a segment reaches depends on the operand before it.
	writtenAs := writtenSegments(want.segments, previous)
	segments := map[segmentKey][]resolve.Reference{}
	for _, ref := range chains {
		read := ref
		read.Chain = respelled(ref.Chain, chosen)
		read.QN = read.Chain.Member
		as := segmentKey{member: e.fqn[ref.Member], name: qualifiedText(ref.QN)}
		for _, operand := range e.operandKeys(read) {
			as.operand = operand
			if _, ok := writtenAs[as]; ok {
				segments[as] = append(segments[as], read)
				break
			}
		}
	}
	for as, refs := range segments {
		for _, key := range writtenAs[as] {
			segmentChanged, err := e.chooseSegment(key, as.name, refs, previous, names.segments)
			if err != nil {
				return nil, false, err
			}
			changed = changed || segmentChanged
		}
	}
	return names, changed, nil
}

// writtenKeys indexes the wanted references by the member and the spelling
// they were written as; a spelling two targets share in one member is left out.
func writtenKeys(references map[nameKey]wantedReference) map[nameKey]nameKey {
	written := map[nameKey]nameKey{}
	shared := map[nameKey]bool{}
	for key, ref := range references {
		as := nameKey{member: key.member, target: ref.written}
		if _, dup := written[as]; dup {
			shared[as] = true
			continue
		}
		written[as] = key
	}
	for as := range shared {
		delete(written, as)
	}
	return written
}

// writtenSegments indexes the wanted segments by the spelling the rendering
// wrote for each: what previous chose, else the segment's own name.
func writtenSegments(segments map[segmentKey]bool, previous *nameChoices) map[segmentKey][]segmentKey {
	written := make(map[segmentKey][]segmentKey, len(segments))
	for key := range segments {
		name := key.name
		if previous != nil {
			if spelling, ok := previous.segments[key]; ok {
				name = spelling
			}
		}
		as := key.written(name)
		written[as] = append(written[as], key)
	}
	return written
}

// chooseSegment spells the segments of key the shortest way that reads as their
// element from every occurrence written alike there (refs); none is a refusal.
func (e *encoder) chooseSegment(key segmentKey, written string, refs []resolve.Reference, previous *nameChoices, spelled map[segmentKey]string) (bool, error) {
	spellings := segmentSpellings(key.name, key.target)
	if previous != nil {
		spellings = fromWritten(spellings, written)
	}
	for _, spelling := range spellings {
		if e.segmentReads(refs, spelling, key.target) {
			if spelling != key.name {
				spelled[key] = spelling
			}
			return spelling != written, nil
		}
	}
	return false, &UnsupportedError{
		What: fmt.Sprintf("the feature chain in %s reaching %s", key.member, key.target),
		Note: "no spelling of the segment reads as the element the graph names from the operand it is written after, so the notation cannot state it",
	}
}

// segmentReads reports whether spelling, written as the segment of every one
// of refs after its operand, reads as target.
func (e *encoder) segmentReads(refs []resolve.Reference, spelling, target string) bool {
	for _, ref := range refs {
		trial := ref
		trial.QN = spelledName(spelling)
		trial.Chain = &ast.FeatureChainExpr{Operand: ref.Chain.Operand, Member: trial.QN}
		if _, reached, ok := e.reads(trial); !ok || reached != target {
			return false
		}
	}
	return true
}

// referenceSpellings are the spellings tried for a reference written fully
// qualified as qname: its suffixes shortest first, then its global form.
func referenceSpellings(qname string) []string {
	segments := strings.Split(qname, "::")
	spellings := make([]string, 0, len(segments)+1)
	for i := len(segments) - 1; i >= 0; i-- {
		spellings = append(spellings, strings.Join(segments[i:], "::"))
	}
	return append(spellings, "$::"+qname)
}

// segmentSpellings are the spellings tried for a chain segment naming target:
// the name as written, then target's qualifications shortest first, then its
// global form.
func segmentSpellings(name, target string) []string {
	spellings := []string{name}
	parts := strings.Split(target, "::")
	for i := len(parts) - 2; i >= 0; i-- {
		spellings = append(spellings, strings.Join(parts[i:], "::"))
	}
	return append(spellings, "$::"+target)
}

// fromWritten is spellings from the one written on: kept first, then only the
// longer ones, so a choice checked in its own rendering never shortens again.
func fromWritten(spellings []string, written string) []string {
	for i, spelling := range spellings {
		if spelling == written {
			return spellings[i:]
		}
	}
	return spellings
}

// operandKeys are the operands a chain segment may be noted after: the element
// the operand reads as, then the operand as written, which is all the graph
// keeps where it links the operand to no element.
func (e *encoder) operandKeys(ref resolve.Reference) []string {
	operand := ref
	if inner, ok := ref.Chain.Operand.(*ast.FeatureChainExpr); ok {
		operand.Chain, operand.QN = inner, inner.Member
	} else {
		operand.Chain, operand.QN = nil, ast.AsQualifiedName(ref.Chain.Operand)
	}
	if operand.QN == nil {
		return []string{""}
	}
	_, fqn, _ := e.reads(operand)
	return []string{fqn, qualifiedText(operand.QN)}
}

// reads is the element a reference reads as where it is written.
func (e *encoder) reads(ref resolve.Reference) (ast.Node, string, bool) {
	sym, ok := e.res.ProbeReference(ref)
	return e.linkedElement(ref.QN, sym, ok)
}

// respelled is chain with its root written as chosen, on fresh nodes so the
// resolver's memo of the first spelling is kept; chain itself when unchanged.
func respelled(chain *ast.FeatureChainExpr, chosen map[*ast.QualifiedName]string) *ast.FeatureChainExpr {
	operand, changed := respelledOperand(chain.Operand, chosen)
	if !changed {
		return chain
	}
	return &ast.FeatureChainExpr{Operand: operand, Member: spelledName(qualifiedText(chain.Member))}
}

func respelledOperand(node ast.Node, chosen map[*ast.QualifiedName]string) (ast.Node, bool) {
	switch n := node.(type) {
	case *ast.FeatureChainExpr:
		inner := respelled(n, chosen)
		return inner, inner != n
	case *ast.IndexExpr:
		operand, changed := respelledOperand(n.Operand, chosen)
		if !changed {
			return n, false
		}
		return &ast.IndexExpr{Operand: operand, Index: n.Index, Bracket: n.Bracket}, true
	case *ast.FeatureReference:
		if spelling, ok := chosen[n.Name]; ok {
			return &ast.FeatureReference{Name: spelledName(spelling)}, true
		}
	case *ast.QualifiedName:
		if spelling, ok := chosen[n]; ok {
			return spelledName(spelling), true
		}
	}
	return node, false
}

// checkStarts refuses a `first` whose start does not read as the member the
// graph names in the body it is written in.
func (e *encoder) checkStarts(starts map[string]string, declared map[string]ast.Node) error {
	for fqn, target := range starts {
		initial, ok := declared[fqn].(*ast.InitialNode)
		if !ok {
			return &UnsupportedError{
				What: fmt.Sprintf("the initial node %s", fqn),
				Note: "the notation written for it does not read back as an initial node, so its start cannot be checked",
			}
		}
		if _, reached, ok := e.linked(e.res.InitialSymbol(initial)); ok && reached == target {
			continue
		}
		return &UnsupportedError{
			What: fmt.Sprintf("the initial node %s", fqn),
			Note: fmt.Sprintf("`first %s` does not name %s in the body it is written in, so the notation cannot state it", nameText(initial.Name()), target),
		}
	}
	return nil
}

// writtenTarget is the qualified name of the element a written reference names:
// the alias it is written through, else what it resolves to, else its text.
func (e *encoder) writtenTarget(ref resolve.Reference) string {
	if alias, ok := e.res.PartAlias(ref.QN, len(ref.QN.Parts)-1); ok {
		if fqn, ok := e.fqn[alias.Decl]; ok {
			return fqn
		}
	}
	if sym, ok := e.res.ProbeReference(ref); ok && sym != nil {
		if fqn, ok := e.fqn[sym.Decl]; ok {
			return fqn
		}
	}
	return qualifiedText(ref.QN)
}

// readNotation parses text as the language name's extension says, or as an
// extensionless buffer when it has none.
func readNotation(name string, text []byte) (*source.SourceFile, *ast.RootNamespace, bool) {
	file := source.New(name, text)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		return nil, nil, false
	}
	return file, root, true
}

// spellingFor is the first of spellings every occurrence resolves to target.
func spellingFor(res *resolve.Resolver, refs []resolve.Reference, spellings []string, target ast.Node) (string, bool) {
	for _, spelling := range spellings {
		if resolvesTo(res, refs, spelling, target) {
			return spelling, true
		}
	}
	return "", false
}

// spelledName is the qualified name written as text, on a fresh node: the
// resolver memoizes by node.
func spelledName(text string) *ast.QualifiedName {
	qn := &ast.QualifiedName{}
	if rest, ok := strings.CutPrefix(text, "$::"); ok {
		qn.Global, text = true, rest
	}
	for _, segment := range strings.Split(text, "::") {
		qn.Parts = append(qn.Parts, ast.NameSegment{Text: segment})
	}
	return qn
}

func resolvesTo(res *resolve.Resolver, refs []resolve.Reference, spelling string, target ast.Node) bool {
	for _, ref := range refs {
		qn := spelledName(spelling)
		sym, ok := res.ProbeReference(ref.Spelled(qn))
		if !ok || sym == nil {
			return false
		}
		// The graph links a name written through an alias to that alias.
		alias, aliased := res.PartAlias(qn, len(qn.Parts)-1)
		if sym.Decl != target && !(aliased && alias.Decl == target) {
			return false
		}
	}
	return true
}
