package edit

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// moveSplices are the byte ranges a move rewrites: the span a delete of the
// target removes, the insertion an add into the new owner makes, and every
// reference the new place would break, respelled to reach what it did.
func (m Model) moveSplices(i int, op Operation) ([]splice, error) {
	sym, err := m.target(i, op)
	if err != nil {
		return nil, err
	}
	owner, ownerScope, err := m.addOwner(op.NewOwner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return nil, e
	}
	if owner != m.Root && encloses(symbolSpan(sym), owner.Span()) {
		return nil, &Error{
			Failure:        FailureOwnerInsideTarget,
			OperationIndex: i,
			Message: fmt.Sprintf("%s cannot be moved into %s, which it declares",
				op.Target, ownerName(op.NewOwner)),
		}
	}
	kind := sym.Notation()
	if !parser.BodyAdmitsMember(owner, kind) {
		return nil, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message: fmt.Sprintf("%s is a %s, which is only declared in a %s body, which %s does not open",
				op.Target, kind, parser.MemberOwner(kind), ownerName(op.NewOwner)),
		}
	}
	sameOwner := ownerScope != nil && ownerScope == sym.OwnerScope
	if sym.Name != "" && ownerScope != nil && !sameOwner && len(ownerScope.LookupLocalAll(sym.Name)) > 0 {
		return nil, &Error{
			Failure:        FailureMemberNameTaken,
			OperationIndex: i,
			Message:        fmt.Sprintf("%s already declares %q", ownerName(op.NewOwner), sym.Name),
		}
	}
	r, _ := m.resolver()
	del, ok := m.symbolDeletion(r, m.Source.Name(), sym)
	if !ok {
		return nil, &Error{
			Failure:        FailureUnknownTarget,
			OperationIndex: i,
			Message:        fmt.Sprintf("%s is declared by no notation of its own to move", op.Target),
		}
	}
	if !sameOwner {
		if err := m.refuseReferencedElsewhere(i, op, r, del); err != nil {
			return nil, err
		}
	}
	mv := &mover{model: m, op: op, index: i, sym: sym, owner: owner, ownerScope: ownerScope}
	mv.remove = m.deleteSpan(del)
	mv.carried = m.carried(mv.remove, del.span.Offset, m.ownerMemberIndent(owner))
	mv.ins = m.memberInsertion(owner, mv.carried.text)
	if err := checkOverlap(mv.splices()); err != nil {
		return nil, err
	}
	if sameOwner {
		return mv.splices(), nil
	}
	if err := mv.follow(r); err != nil {
		return nil, err
	}
	return mv.splices(), nil
}

// refuseReferencedElsewhere is the refusal of operation i when another workspace
// document refers to target or a declaration inside it: a move respells the
// references of its own document only. Nil when none does.
func (m Model) refuseReferencedElsewhere(i int, op Operation, r *resolve.Resolver, target deletion) error {
	targets := []deletion{target}
	var elsewhere []Referrer
	for _, doc := range m.workspaceDocuments()[1:] {
		for _, referrer := range m.referrersIn(r, doc, targets, targets) {
			elsewhere = append(elsewhere, referrer.referrer())
		}
	}
	if len(elsewhere) == 0 {
		return nil
	}
	sortReferrers(elsewhere)
	return referencedElsewhere(i, m.Source.Name(), op.Target, elsewhere)
}

// mover is one move in progress: what it removes, what it inserts, the
// references it has respelled so far, and where each byte of the source lands.
type mover struct {
	model      Model
	op         Operation
	index      int
	sym        *symbols.Symbol
	owner      ast.Node
	ownerScope *symbols.Scope
	remove     source.Span
	carried    carried
	ins        insertion
	// outer are the respellings outside the removed span, in source order.
	outer []splice
	// inner are the respellings inside it, as ranges of the carried text.
	inner []patch
	// pending are the respellings of the round in progress, which landed
	// leaves out until the round's reading is done with.
	pending []splice
}

// patch replaces [start, end) of the carried text.
type patch struct {
	start, end int
	text       string
}

// splices are the move's byte ranges of the source: the removal, the
// insertion carrying the target as respelled so far, and the outer respellings.
func (mv *mover) splices() []splice {
	ins := mv.model.memberInsertion(mv.owner, mv.carriedText())
	out := make([]splice, 0, len(mv.outer)+2)
	out = append(out, mv.outer...)
	out = append(out, splice{span: mv.remove, opIndex: mv.index, target: mv.op.Target})
	return append(out, splice{span: ins.span, text: ins.text, opIndex: mv.index, target: mv.op.Target})
}

// carriedText is the carried notation with the inner respellings applied.
func (mv *mover) carriedText() string {
	text := mv.carried.text
	for k := len(mv.inner) - 1; k >= 0; k-- {
		p := mv.inner[k]
		text = text[:p.start] + p.text + text[p.end:]
	}
	return text
}

// rewrite records a respelling of span, wherever it lies, for the round.
func (mv *mover) rewrite(span source.Span, text string) error {
	if span.Offset >= mv.remove.Offset && span.End() <= mv.remove.End() {
		if mv.carried.at[span.Offset-mv.remove.Offset] < 0 || mv.carried.at[span.End()-1-mv.remove.Offset] < 0 {
			return &Error{
				Failure:        FailureMoveReferenced,
				OperationIndex: mv.index,
				Message:        fmt.Sprintf("%s carries a reference the move cannot respell", mv.op.Target),
			}
		}
	}
	mv.pending = append(mv.pending, splice{span: span, text: text, opIndex: mv.index, target: mv.op.Target})
	return nil
}

// commit takes the round's respellings into the move.
func (mv *mover) commit() {
	for _, sp := range mv.pending {
		if sp.span.Offset >= mv.remove.Offset && sp.span.End() <= mv.remove.End() {
			start := mv.carried.at[sp.span.Offset-mv.remove.Offset]
			end := mv.carried.at[sp.span.End()-1-mv.remove.Offset]
			mv.inner = append(mv.inner, patch{start: start, end: end + 1, text: sp.text})
		} else {
			mv.outer = append(mv.outer, sp)
		}
	}
	mv.pending = nil
	sort.Slice(mv.inner, func(a, b int) bool { return mv.inner[a].start < mv.inner[b].start })
	sort.Slice(mv.outer, func(a, b int) bool { return mv.outer[a].span.Offset < mv.outer[b].span.Offset })
}

// landed is where byte o of the source lands once the move and the respellings
// so far are applied, or -1 for a byte they drop or rewrite.
func (mv *mover) landed(o int) int {
	if o >= mv.remove.Offset && o < mv.remove.End() {
		rel := mv.carried.at[o-mv.remove.Offset]
		if rel < 0 {
			return -1
		}
		shift := 0
		for _, p := range mv.inner {
			if rel >= p.start && rel < p.end {
				return -1
			}
			if p.end <= rel {
				shift += len(p.text) - (p.end - p.start)
			}
		}
		rel += shift
		start := mv.ins.span.Offset
		if mv.remove.End() <= start {
			start -= mv.remove.Len
		}
		for _, sp := range mv.outer {
			if sp.span.End() <= mv.ins.span.Offset {
				start += len(sp.text) - sp.span.Len
			}
		}
		return start + mv.ins.at + rel
	}
	if o >= mv.ins.span.Offset && o < mv.ins.span.End() {
		return -1
	}
	for _, sp := range mv.outer {
		if o >= sp.span.Offset && o < sp.span.End() {
			return -1
		}
	}
	return mv.landedOutside(o)
}

// landedOutside shifts o, a byte outside every rewritten range, by the growth
// of the ranges before it.
func (mv *mover) landedOutside(o int) int {
	shifted := o
	if mv.remove.End() <= o {
		shifted -= mv.remove.Len
	}
	if mv.ins.span.End() <= o {
		shifted += len(mv.carriedText()) - len(mv.carried.text) + len(mv.ins.text) - mv.ins.span.Len
	}
	for _, sp := range mv.outer {
		if sp.span.End() <= o {
			shifted += len(sp.text) - sp.span.Len
		}
	}
	return shifted
}

// carried is the notation a move carries: the removed span less the blank
// lines around it, its later lines indented for the new owner.
type carried struct {
	text string
	// at is, for each byte of the removed span, its offset in text, or -1 for
	// a byte of indentation or of a blank line that text drops.
	at []int
}

// carried re-indents the lines of remove, whose declaration starts at
// declOffset, from that declaration's indentation to indent.
func (m Model) carried(remove source.Span, declOffset int, indent string) carried {
	content := m.Source.Bytes()
	base := len(lineIndent(content, declOffset))
	type line struct{ start, end int }
	var lines []line
	for start := remove.Offset; start < remove.End(); {
		end := start
		for end < remove.End() && content[end] != '\n' {
			end++
		}
		lines = append(lines, line{start, end})
		start = end + 1
	}
	blank := func(l line) bool { return onlyWhitespace(content[l.start:l.end]) }
	first, last := 0, len(lines)-1
	for first <= last && blank(lines[first]) {
		first++
	}
	for last >= first && blank(lines[last]) {
		last--
	}
	at := make([]int, remove.Len)
	for k := range at {
		at[k] = -1
	}
	var b strings.Builder
	for k := first; k <= last; k++ {
		l := lines[k]
		if k > first {
			b.WriteByte('\n')
		}
		if blank(l) {
			continue
		}
		strip := 0
		for strip < base && l.start+strip < l.end && (content[l.start+strip] == ' ' || content[l.start+strip] == '\t') {
			strip++
		}
		if k > first {
			b.WriteString(indent)
		}
		for o := l.start + strip; o < l.end; o++ {
			at[o-remove.Offset] = b.Len()
			b.WriteByte(content[o])
		}
	}
	return carried{text: b.String(), at: at}
}

// reference is one name of the source before the move with, per segment, the
// element it reached; nil where the segment reached none or declares the name.
type reference struct {
	ref     resolve.Reference
	reached []*symbols.Symbol
}

// referencesBefore resolves every name of the document as it stands, which is
// what each must still reach once the target is moved.
func (mv *mover) referencesBefore(r *resolve.Resolver) []reference {
	m := mv.model
	rootScope := m.Index.DocumentRoot(m.Source.Name())
	if rootScope == nil {
		return nil
	}
	seen := map[*ast.QualifiedName]bool{}
	var out []reference
	for _, ref := range resolve.References(m.Root, rootScope) {
		if ref.QN == nil || seen[ref.QN] {
			continue
		}
		seen[ref.QN] = true
		r.ResolveReference(ref)
		reached := make([]*symbols.Symbol, len(ref.QN.Parts))
		resolved := false
		for part, seg := range ref.QN.Parts {
			sym, ok := r.PartSymbol(ref.QN, part)
			if !ok || seg.Span == sym.NameSpan {
				continue
			}
			reached[part], resolved = sym, true
		}
		if resolved {
			out = append(out, reference{ref: ref, reached: reached})
		}
	}
	return out
}

// placed is a segment of a name in the moved source.
type placed struct {
	ref  resolve.Reference
	part int
}

// moved is the source with the move and the respellings so far applied, read
// again: where each name segment and each declaration of the document now is.
type moved struct {
	model    Model
	r        *resolve.Resolver
	segments map[int]placed
	declared map[int]*symbols.Symbol
}

func (mv *mover) reread() (*moved, error) {
	m := mv.model
	if m.reindex == nil {
		m.reindex = newReindexer(m)
	}
	edited := unedited(m)
	if err := m.rewrite(edited, mv.splices()); err != nil {
		return nil, err
	}
	model := reparseModel(m, edited)
	if introduced := introduced(parseDiagnostics(m.ParseDiags), parseDiagnostics(model.ParseDiags)); len(introduced) > 0 {
		return nil, &Error{
			Failure:        FailureResultInvalid,
			OperationIndex: mv.index,
			Diagnostics:    introduced,
			Diagnosed:      model.Source,
			Message:        "the moved notation does not parse: " + introduced[0].Message,
		}
	}
	r, _ := model.resolver()
	out := &moved{model: model, r: r, segments: map[int]placed{}, declared: map[int]*symbols.Symbol{}}
	name := model.Source.Name()
	rootScope := model.Index.DocumentRoot(name)
	if rootScope == nil {
		return out, nil
	}
	for _, ref := range resolve.References(model.Root, rootScope) {
		if ref.QN == nil {
			continue
		}
		r.ResolveReference(ref)
		for part, seg := range ref.QN.Parts {
			if _, done := out.segments[seg.Span.Offset]; !done {
				out.segments[seg.Span.Offset] = placed{ref: ref, part: part}
			}
		}
	}
	var visit func(scope *symbols.Scope)
	visit = func(scope *symbols.Scope) {
		scope.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym.Decl != nil && sym.DocName == name {
				at := symbolSpan(sym).Offset
				if prev, done := out.declared[at]; !done || (prev.Name == "" && sym.Name != "") {
					out.declared[at] = sym
				}
			}
			return true
		})
		for _, child := range scope.Children() {
			visit(child)
		}
	}
	visit(rootScope)
	return out, nil
}

// identityBefore names the element sym, of the source before the move, by where
// its declaration lands in the moved source; an element of another document or
// of a library, which the move does not touch, by its name.
func (mv *mover) identityBefore(sym *symbols.Symbol) string {
	if sym.DocName == mv.model.Source.Name() {
		return strconv.Itoa(mv.landed(symbolSpan(sym).Offset))
	}
	return foreignIdentity(mv.model.Index, sym)
}

// identityAfter names the element sym of the moved source as identityBefore
// names the same element of the source before it.
func (mv *mover) identityAfter(md *moved, sym *symbols.Symbol) string {
	if sym.DocName == mv.model.Source.Name() {
		return strconv.Itoa(symbolSpan(sym).Offset)
	}
	return foreignIdentity(md.model.Index, sym)
}

func foreignIdentity(idx *symbols.Index, sym *symbols.Symbol) string {
	fqn := idx.GetFQN(sym)
	if fqn == "" {
		fqn = strings.Join(symbols.NameChain(sym), "::")
	}
	return "\x00" + sym.DocName + "\x00" + fqn + "\x00" + sym.Kind.String()
}

// follow respells every reference the move breaks. Imports go first, since
// what the other names reach depends on them; then the rest, read against
// the source with those imports respelled.
func (mv *mover) follow(r *resolve.Resolver) error {
	before := mv.referencesBefore(r)
	for _, imports := range []bool{true, false} {
		md, err := mv.reread()
		if err != nil {
			return err
		}
		for _, ref := range before {
			if (ref.ref.Import != nil) != imports {
				continue
			}
			if err := mv.followOne(md, ref); err != nil {
				return err
			}
		}
		mv.commit()
	}
	return nil
}

// followOne respells ref where the moved source no longer reads it as before.
func (mv *mover) followOne(md *moved, ref reference) error {
	if imp, ok := mv.redundantImport(ref); ok {
		return mv.rewrite(mv.model.deleteSpan(deletion{node: imp, span: imp.Span()}), "")
	}
	broken, intact := -1, 0
	for part, sym := range ref.reached {
		if sym == nil {
			continue
		}
		at, ok := md.segments[mv.landed(ref.ref.QN.Parts[part].Span.Offset)]
		if !ok {
			return mv.refuseReference(md, ref, part, "is not read as a name once it is moved")
		}
		reached, ok := md.r.PartSymbol(at.ref.QN, at.part)
		if ok && mv.identityAfter(md, reached) == mv.identityBefore(sym) {
			if broken < 0 && intact == part {
				intact++
			}
			continue
		}
		broken = part
	}
	if broken < 0 {
		return nil
	}
	return mv.respell(md, ref, broken, intact)
}

// redundantImport is the membership import of the target written in the new
// owner, which the move makes an import of the owner's own member; nil else.
func (mv *mover) redundantImport(ref reference) (*ast.Import, bool) {
	imp := ref.ref.Import
	if imp == nil || imp.Kind != ast.ImportMembership || imp.HasBody || imp.FilterExpr != nil ||
		imp.IsExpose || imp.Imported != ref.ref.QN || ref.ref.Scope != mv.ownerScope {
		return nil, false
	}
	last := ref.reached[len(ref.reached)-1]
	if last == nil || !symbols.SameElement(last, mv.sym) {
		return nil, false
	}
	return imp, true
}

// respell rewrites ref's segments up to broken, the last one that no longer
// reaches its element. The leading intact segments are kept and the path from
// their element is written after them; failing that, the shortest qualified
// name of the element under which a trial reading reaches every element it did.
func (mv *mover) respell(md *moved, ref reference, broken, intact int) error {
	qn := ref.ref.QN
	if qn.Parts[broken].Chained || ref.ref.Chain != nil || (ref.ref.Head != nil && ref.ref.Head.Member != nil) {
		return mv.refuseReference(md, ref, broken, "is a step of a feature chain, which no qualified name respells")
	}
	if ref.ref.Constructed != nil && len(qn.Parts) == 1 {
		return mv.refuseReference(md, ref, broken, "labels a constructor argument, which no qualified name respells")
	}
	target := ref.reached[broken]
	if target.DocName == mv.model.Source.Name() {
		landed, ok := md.declared[mv.landed(symbolSpan(target).Offset)]
		if !ok {
			return mv.refuseReference(md, ref, broken, "names a declaration the moved source does not read")
		}
		target = landed
	}
	owners := namedOwners(target)
	chain := make([]string, len(owners))
	for i, owner := range owners {
		chain[i] = owner.Name
	}
	if len(chain) == 0 || chain[len(chain)-1] == "" {
		return mv.refuseReference(md, ref, broken, "reaches an unnamed element, which no qualified name respells")
	}
	at := md.segments[mv.landed(qn.Parts[broken].Span.Offset)]
	for keep := intact; keep > 0; keep-- {
		anchor := mv.identityBefore(ref.reached[keep-1])
		j := slices.IndexFunc(owners[:len(owners)-1], func(owner *symbols.Symbol) bool {
			return mv.identityAfter(md, owner) == anchor
		})
		if j < 0 {
			continue
		}
		spelling := make([]string, 0, keep+len(chain)-j-1)
		for _, part := range qn.Parts[:keep] {
			spelling = append(spelling, part.Text)
		}
		spelling = append(spelling, chain[j+1:]...)
		if mv.trial(md, ref, broken, at, spelling, qn.Global) {
			text := lexer.QualifiedNameOf(spelling)
			if qn.Global {
				text = "$::" + text
			}
			return mv.respelled(qn, broken, text)
		}
	}
	for n := 1; n <= len(chain); n++ {
		spelling := chain[len(chain)-n:]
		if mv.trial(md, ref, broken, at, spelling, false) {
			return mv.respelled(qn, broken, lexer.QualifiedNameOf(spelling))
		}
	}
	if mv.trial(md, ref, broken, at, chain, true) {
		return mv.respelled(qn, broken, "$::"+lexer.QualifiedNameOf(chain))
	}
	return mv.refuseReference(md, ref, broken,
		fmt.Sprintf("would not reach it as %s", lexer.QualifiedNameOf(chain)))
}

// namedOwners is sym after its named owners, outermost first: the elements
// whose names NameChain joins.
func namedOwners(sym *symbols.Symbol) []*symbols.Symbol {
	owners := []*symbols.Symbol{sym}
	for scope := sym.OwnerScope; scope != nil && scope.Owner() != nil; scope = scope.Owner().OwnerScope {
		if owner := scope.Owner(); owner.Name != "" {
			owners = append(owners, owner)
		}
	}
	slices.Reverse(owners)
	return owners
}

// respelled records the rewrite of qn's first segments, through broken, as text.
func (mv *mover) respelled(qn *ast.QualifiedName, broken int, text string) error {
	first := qn.Parts[0].Span
	return mv.rewrite(source.Span{Offset: first.Offset, Len: qn.Parts[broken].Span.End() - first.Offset}, text)
}

// trial reports whether ref, its segments through broken respelled as
// spelling, reaches in the moved source every element it reached before.
func (mv *mover) trial(md *moved, ref reference, broken int, at placed, spelling []string, global bool) bool {
	rest := at.ref.QN.Parts[at.part+1:]
	trial := &ast.QualifiedName{NodeBase: at.ref.QN.NodeBase, Global: global}
	trial.Parts = make([]ast.NameSegment, 0, len(spelling)+len(rest))
	for _, name := range spelling {
		trial.Parts = append(trial.Parts, ast.NameSegment{Text: name})
	}
	trial.Parts = append(trial.Parts, rest...)
	rd := md.r.ProbeReading(at.ref.Spelled(trial))
	if _, ambiguous := rd.Ambiguity(); ambiguous {
		return false
	}
	for part := broken; part < len(ref.reached); part++ {
		sym := ref.reached[part]
		if sym == nil {
			continue
		}
		reached, ok := rd.Part(len(spelling) - 1 + part - broken)
		if !ok || mv.identityAfter(md, reached) != mv.identityBefore(sym) {
			return false
		}
	}
	return true
}

// refuseReference is the refusal of a move that segment part of ref, which
// reached an element before it, cannot follow.
func (mv *mover) refuseReference(md *moved, ref reference, part int, reason string) error {
	name := mv.op.Target
	if sym := ref.reached[part]; sym != nil && sym.Name != "" {
		name = notationName(sym)
	}
	site := docLabel(mv.model.Source.Name())
	if at, ok := md.segments[mv.landed(ref.ref.QN.Parts[part].Span.Offset)]; ok {
		if referrer, ok := md.model.referrer(md.r, md.model.Source.Name(), at.ref, at.ref.QN.Parts[at.part].Span.Offset); ok {
			site = referrer.name
		}
	}
	return &Error{
		Failure:        FailureMoveReferenced,
		OperationIndex: mv.index,
		Referring:      []string{site},
		Message: fmt.Sprintf("%s cannot be moved into %s: the reference to %s in %s %s",
			mv.op.Target, ownerName(mv.op.NewOwner), name, site, reason),
	}
}
