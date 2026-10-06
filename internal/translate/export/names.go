package export

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
	// variants holds the feature each `variant x;` names, keyed by the variant.
	variants map[string]string
}

type wantedReference struct {
	qualified, written string
	// count is how many times the rendering wrote the reference.
	count int
	// qualifiedOnly holds a reference only a qualified name may spell: the
	// one-segment spelling reads back as a declared name (`variant x;`).
	qualifiedOnly bool
}

func newWanted() *wanted {
	return &wanted{
		references: map[nameKey]wantedReference{},
		segments:   map[segmentKey]bool{},
		starts:     map[string]string{},
		variants:   map[string]string{},
	}
}

func (w *wanted) empty() bool {
	return len(w.references) == 0 && len(w.segments) == 0 && len(w.starts) == 0 && len(w.variants) == 0
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
			Note: "the notation written for them does not parse, so no spelling can be checked to reach its element\n" + string(text),
		}
	}
	e, err := newEncoder(file, root, library, IDQualifiedName)
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
	memberAliases, targetAliases := parserAliases(want.references)
	written := writtenKeys(want.references)
	writtenAs := writtenSegments(want.segments, previous)
	occurrences := map[nameKey][]resolve.Reference{}
	// roots are the first segments of end chains (`connect t.fuel to …`, a
	// flow end), which read back as plain references and are spelled as
	// segments with no operand.
	roots := map[segmentKey][]resolve.Reference{}
	var chains, misread []resolve.Reference
	for _, ref := range resolve.References(root, e.res.Index().DocumentRoot(file.Name())) {
		if ref.QN == nil || ref.Member == nil {
			continue
		}
		if ref.Chain != nil {
			chains = append(chains, ref)
			continue
		}
		target := e.writtenTarget(ref)
		if alias, ok := targetAliases[target]; ok && alias != "" {
			target = alias
		}
		memberGroups := e.memberOfCandidates(ref)
		var key nameKey
		matched := false
		for _, members := range memberGroups {
			var matches []nameKey
			seen := map[nameKey]bool{}
			for _, member := range members {
				if alias, ok := memberAliases[member]; ok && alias != "" {
					member = alias
				}
				candidate := nameKey{member: member, target: target}
				if _, ok := want.references[candidate]; ok && !seen[candidate] {
					matches = append(matches, candidate)
					seen[candidate] = true
				}
			}
			if len(matches) > 1 {
				return nil, false, &UnsupportedError{
					What: fmt.Sprintf("the reference to %s from %s", target, e.memberOf(ref)),
					Note: "the member's qualified name matches multiple graph references, so its spelling cannot be checked unambiguously",
				}
			}
			if len(matches) == 1 {
				key, matched = matches[0], true
				break
			}
		}
		if matched {
			occurrences[key] = append(occurrences[key], ref)
			continue
		}
		rootMatched := false
		for _, members := range memberGroups {
			for _, member := range members {
				if alias, ok := memberAliases[member]; ok && alias != "" {
					member = alias
				}
				as := segmentKey{member: member, name: qualifiedText(ref.QN)}
				if len(writtenAs[as]) > 0 {
					roots[as] = append(roots[as], ref)
					rootMatched = true
					break
				}
			}
			if rootMatched {
				break
			}
		}
		if rootMatched {
			continue
		}
		misread = append(misread, ref)
	}
	// A spelling read as another element is still an occurrence of the reference
	// written that way, unless every writing of it already read back correctly.
	for _, ref := range misread {
		found := false
		for _, members := range e.memberOfCandidates(ref) {
			for _, member := range members {
				if alias, ok := memberAliases[member]; ok && alias != "" {
					member = alias
				}
				key, ok := written[nameKey{member: member, target: qualifiedText(ref.QN)}]
				if !ok || len(occurrences[key]) >= want.references[key].count {
					continue
				}
				occurrences[key] = append(occurrences[key], ref)
				found = true
				break
			}
			if found {
				break
			}
		}
	}
	if err := e.checkStarts(want.starts, declared); err != nil {
		return nil, false, err
	}
	if err := e.checkVariants(want.variants, declared); err != nil {
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
		if _, ok := declared[key.target]; !ok && !e.ids.libraryName(key.target) {
			continue
		}
		ref := want.references[key]
		spellings := referenceSpellings(ref.qualified)
		if ref.qualifiedOnly {
			spellings = qualifiedSpellings(spellings)
		}
		if previous != nil {
			spellings = fromWritten(spellings, ref.written)
		}
		spelling, ok := e.spellingFor(refs, spellings, key.target)
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
	for as, refs := range roots {
		for _, key := range writtenAs[as] {
			spelling, rootChanged, err := e.chooseRoot(key, as.name, refs, previous, names.segments)
			if err != nil {
				return nil, false, err
			}
			changed = changed || rootChanged
			for _, r := range refs {
				chosen[r.QN] = spelling
			}
		}
	}
	// A chain reads from its root, so its segments are spelled once the root is:
	// what a segment reaches depends on the operand before it.
	segments := map[segmentKey][]resolve.Reference{}
	for _, ref := range chains {
		read := ref
		read.Chain = respelled(ref.Chain, chosen)
		read.QN = read.Chain.Member
		as := segmentKey{member: e.memberOf(ref), name: qualifiedText(ref.QN)}
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

func parserAliases(references map[nameKey]wantedReference) (map[string]string, map[string]string) {
	members := map[string]string{}
	targets := map[string]string{}
	for key := range references {
		addParserAlias(members, parserQualifiedName(key.member), key.member)
		addParserAlias(targets, parserQualifiedName(key.target), key.target)
	}
	return members, targets
}

func addParserAlias(aliases map[string]string, parsed, identity string) {
	if previous, ok := aliases[parsed]; ok && previous != identity {
		aliases[parsed] = ""
		return
	}
	aliases[parsed] = identity
}

// memberOf is the qualified name of the member a reference is written in: the
// declaration itself, or the one whose expression body declares that declaration.
func (e *encoder) memberOf(ref resolve.Reference) string {
	if ref.Within != nil {
		return e.qualifiedMemberName(ref.Within)
	}
	if member := e.triggerMembers[ref.Member]; member != "" {
		return member
	}
	// The type of `flow of T` is the typing of the payload feature the flow
	// owns, which the graph states it from.
	if u, ok := ref.Member.(*ast.Usage); ok && u.FlowEnds != nil && u.FlowEnds.PayloadDecl == nil &&
		ref.QN != nil && ast.Node(ref.QN) == u.FlowEnds.Payload {
		if payload := e.payloadOf(u); payload != nil {
			return e.qualifiedMemberName(payload)
		}
	}
	return e.qualifiedMemberName(ref.Member)
}

func (e *encoder) memberOfCandidates(ref resolve.Reference) [][]string {
	if ref.Within != nil {
		return [][]string{e.memberNameCandidates(ref.Within)}
	}
	if member := e.triggerMembers[ref.Member]; member != "" {
		return [][]string{{member}}
	}
	if u, ok := ref.Member.(*ast.Usage); ok && u.FlowEnds != nil && u.FlowEnds.PayloadDecl == nil &&
		ref.QN != nil && ast.Node(ref.QN) == u.FlowEnds.Payload {
		if payload := e.payloadOf(u); payload != nil {
			return [][]string{e.memberNameCandidates(payload), e.memberNameCandidates(ref.Member)}
		}
	}
	return [][]string{e.memberNameCandidates(ref.Member)}
}

func (e *encoder) memberNameCandidates(node ast.Node) []string {
	qualified, positional := e.qualifiedMemberName(node), e.fqn[node]
	if qualified != positional {
		return []string{qualified, positional}
	}
	return []string{qualified}
}

func (e *encoder) qualifiedMemberName(node ast.Node) string {
	fqn := e.fqn[node]
	sym := e.ids.declSym[node]
	if sym == nil || fqn == "" {
		return fqn
	}
	var chain []*symbols.Symbol
	for current := sym; current != nil; {
		chain = append(chain, current)
		if current.OwnerScope == nil {
			break
		}
		owner := current.OwnerScope.Owner()
		if owner == current {
			break
		}
		current = owner
	}
	segments := identitySegments(fqn)
	if len(segments) != len(chain) {
		return fqn
	}
	for i, current := range chain {
		if current.EffectiveName() {
			segments[len(chain)-1-i] = identitySegment(current.Name)
		}
	}
	return strings.Join(segments, "::")
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

// chooseRoot spells the first segment of an end chain the shortest way that
// reads as its element from every occurrence written alike (refs), as a
// reference is spelled. A root no spelling reaches is refused, unless it is an
// unnamed member, whose chain is written by position.
func (e *encoder) chooseRoot(key segmentKey, written string, refs []resolve.Reference, previous *nameChoices, spelled map[segmentKey]string) (string, bool, error) {
	spellings := referenceSpellings(key.target)
	if previous != nil {
		spellings = fromWritten(spellings, written)
	}
	for _, spelling := range spellings {
		reads := true
		for _, ref := range refs {
			trial := ref
			trial.QN = spelledName(spelling)
			if _, reached, ok := e.reads(trial); !ok || reached != key.target {
				reads = false
				break
			}
		}
		if reads {
			if spelling != key.name {
				spelled[key] = spelling
			}
			return spelling, spelling != written, nil
		}
	}
	// An unnamed member (`@0`) has no name to read as it; its chain is written by
	// position, unchecked, as it always has been. A named root no spelling
	// reaches cannot be stated: writing its name would name something else.
	last := key.target
	if i := strings.LastIndex(last, "::"); i >= 0 {
		last = last[i+len("::"):]
	}
	if strings.HasPrefix(last, "@") {
		return written, false, nil
	}
	return "", false, &UnsupportedError{
		What: fmt.Sprintf("the end chain in %s starting at %s", key.member, key.target),
		Note: "no spelling of its first segment reads as that element from where the chain is written, so the notation cannot state it",
	}
}

// segmentReads reports whether spelling, written as the segment of every one
// of refs after its operand, reads as target.
func (e *encoder) segmentReads(refs []resolve.Reference, spelling, target string) bool {
	for _, ref := range refs {
		trial := ref
		if !trial.Redefines {
			// The referrer's own bindings hide the name it borrows, not the
			// feature a chain's operand names (getOperandSymbol hides none).
			trial.Referrer, trial.Subsetting = nil, nil
		}
		trial.QN = spelledName(spelling)
		trial.Chain = &ast.FeatureChainExpr{Operand: ref.Chain.Operand, Member: trial.QN}
		if _, reached, ok := e.reads(trial); !ok || reached != target {
			return false
		}
	}
	return true
}

// referenceSpellings are the spellings tried for a reference written fully
// qualified as qname: its qualifications shortest first, then its global form.
func referenceSpellings(qname string) []string {
	return append(qualifications(identitySegments(qname)), "$::"+qname)
}

// qualifications are the ways of naming the last of parts through some of the
// namespaces before it, fewest first and a suffix before a skipping form, so an
// element reached through a namespace's import is named the way it is imported.
func qualifications(parts []string) []string {
	last := len(parts) - 1
	if last > maxSkippedQualifiers {
		spellings := make([]string, 0, len(parts))
		for i := last; i >= 0; i-- {
			spellings = append(spellings, strings.Join(parts[i:], "::"))
		}
		return spellings
	}
	var spellings []string
	for n := 0; n <= last; n++ {
		var picks func(from, left int, chosen []string)
		picks = func(from, left int, chosen []string) {
			if left == 0 {
				spellings = append(spellings, strings.Join(append(chosen, parts[last]), "::"))
				return
			}
			for i := last - left; i >= from; i-- {
				picks(i+1, left-1, append(chosen[:len(chosen):len(chosen)], parts[i]))
			}
		}
		picks(0, n, nil)
	}
	return spellings
}

// maxSkippedQualifiers bounds the namespaces a spelling may skip between, past
// which only suffixes are tried.
const maxSkippedQualifiers = 8

// segmentSpellings are the spellings tried for a chain segment naming target:
// the name as written, then target's qualifications shortest first, then its
// global form.
func segmentSpellings(name, target string) []string {
	spellings := []string{name}
	parts := identitySegments(target)
	for _, spelling := range qualifications(parts) {
		if spelling != name {
			spellings = append(spellings, spelling)
		}
	}
	return append(spellings, "$::"+target)
}

// fromWritten is spellings from the one written on: kept first, then only the
// longer ones, so a choice checked in its own rendering never shortens again.
// qualifiedSpellings are the spellings of more than one segment.
func qualifiedSpellings(spellings []string) []string {
	var out []string
	for _, spelling := range spellings {
		if len(identitySegments(strings.TrimPrefix(spelling, "$::"))) > 1 || strings.HasPrefix(spelling, "$::") {
			out = append(out, spelling)
		}
	}
	return out
}

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
		switch initial := declared[fqn].(type) {
		case *ast.InitialNode:
			if _, reached, ok := e.linked(e.res.InitialSymbol(initial)); ok && reached == target {
				continue
			}
			return &UnsupportedError{
				What: fmt.Sprintf("the initial node %s", fqn),
				Note: fmt.Sprintf("`first %s` does not name %s in the body it is written in, so the notation cannot state it", nameText(initial.Name()), target),
			}
		case *ast.TransitionMember:
			// A guarded `first a if g then b` of a state body reads back as a
			// transition whose source the `first` names.
			if initial.Source != nil {
				if _, reached, ok := e.linked(e.res.EndSymbol(initial.Source)); ok && reached == target {
					continue
				}
			}
			return &UnsupportedError{
				What: fmt.Sprintf("the initial node %s", fqn),
				Note: fmt.Sprintf("`first %s` does not name %s in the body it is written in, so the notation cannot state it", qualifiedText(initial.Source), target),
			}
		case *ast.Usage:
			if initial.Kind == ast.UsageSuccession && len(initial.ConnectorEnds) == 2 {
				if source, ok := initial.ConnectorEnds[0].Target.(*ast.QualifiedName); ok {
					if _, reached, ok := e.linked(e.res.EndSymbol(source)); ok {
						if reached == target {
							continue
						}
						return &UnsupportedError{
							What: fmt.Sprintf("the initial node %s", fqn),
							Note: fmt.Sprintf("`first %s` does not name %s in the body it is written in, so the notation cannot state it", qualifiedText(source), target),
						}
					}
				}
			}
		default:
			return &UnsupportedError{
				What: fmt.Sprintf("the initial node %s", fqn),
				Note: "the notation written for it does not read back as an initial node, so its start cannot be checked",
			}
		}
		return &UnsupportedError{
			What: fmt.Sprintf("the initial node %s", fqn),
			Note: "the notation written for it does not read back as an initial node, so its start cannot be checked",
		}
	}
	return nil
}

// checkVariants refuses a `variant x;` written for a variant whose x does not
// reach, from the variation, the feature the variant references.
func (e *encoder) checkVariants(variants map[string]string, declared map[string]ast.Node) error {
	for fqn, target := range variants {
		usage, ok := declared[fqn].(*ast.Usage)
		if !ok || !usage.IsVariantReference() || referencesFeature(usage) {
			return &UnsupportedError{
				What: fmt.Sprintf("the variant %s", fqn),
				Note: "the notation written for it does not read back as a variant reference, so the feature it names cannot be checked",
			}
		}
		if sym := e.ids.declSym[usage]; sym != nil {
			if _, reached, ok := e.linked(e.ids.model.ReferencedFeature(sym), true); ok && reached == target {
				continue
			}
		}
		return &UnsupportedError{
			What: fmt.Sprintf("the variant %s", fqn),
			Note: fmt.Sprintf("`variant %s` does not name %s from the variation it is written in, so the notation cannot state it", nameText(usage.Ident.Name), target),
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
	if _, fqn, ok := e.linked(e.res.ProbeReference(ref)); ok {
		return fqn
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

// spellingFor is the first of spellings every occurrence resolves to the
// element target names.
func (e *encoder) spellingFor(refs []resolve.Reference, spellings []string, target string) (string, bool) {
	for _, spelling := range spellings {
		if e.resolvesTo(refs, spelling, target) {
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
	for _, segment := range identitySegments(text) {
		qn.Parts = append(qn.Parts, ast.NameSegment{Text: identityName(segment)})
	}
	return qn
}

// resolvesTo reports whether every occurrence spelled that way reads as target;
// the spelling the rendering wrote is judged by how the rendering read it.
func (e *encoder) resolvesTo(refs []resolve.Reference, spelling, target string) bool {
	for _, ref := range refs {
		qn := spelledName(spelling)
		var sym *symbols.Symbol
		var ok bool
		if qualifiedText(ref.QN) == spelling {
			qn = ref.QN
			sym, ok = e.links[qn]
		} else {
			sym, ok = e.res.ProbeReference(ref.Spelled(qn))
		}
		if !ok || sym == nil {
			return false
		}
		if _, fqn, ok := e.linked(sym, true); ok && fqn == target {
			continue
		}
		// An element's own identity may already claim a normative target's
		// qualified name; linked refuses to repeat it, so ask the norm itself.
		if fqn := e.normativeFQN(sym); fqn != "" && fqn == target {
			continue
		}
		// The graph links a name written through an alias to that alias.
		if _, fqn, ok := e.linked(e.res.PartAlias(qn, len(qn.Parts)-1)); !ok || fqn != target {
			return false
		}
	}
	return true
}
