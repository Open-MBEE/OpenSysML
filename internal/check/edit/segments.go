package edit

import (
	"bytes"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// segmentCandidate pairs an operation with its write, read and name effects.
type segmentCandidate struct {
	op   Operation
	sp   splice
	add  *addMemberDetails
	path []string
	read source.Span
}

type segmentWrite struct {
	sp      splice
	add     *addMemberDetails
	oldText string
	chain   *memberChain
	benign  bool
}

// memberChain combines same-owner insertions in one segment.
type memberChain struct {
	owner       ast.Node
	span        source.Span
	text        string
	tailStart   int
	indent      string
	closeIndent string
	head        *segmentWrite
}

// editSegment accumulates independent writes against one parsed model.
type editSegment struct {
	model        Model
	writes       []*segmentWrite
	nonBenign    []int
	applied      []Applied
	chains       map[ast.Node]*memberChain
	names        *pathTrie
	ownerNames   map[ast.Node]map[string]bool
	deltas       fenwick
	tabCount     int
	originalTabs int
	sealed       bool
}

type pathTrie struct {
	children map[string]*pathTrie
	terminal bool
}

type fenwick struct {
	tree []int
}

func (f *fenwick) add(offset, delta int) {
	for i := offset + 1; i < len(f.tree); i += i & -i {
		f.tree[i] += delta
	}
}

func (f fenwick) sum(offset int) int {
	if offset+1 >= len(f.tree) {
		offset = len(f.tree) - 2
	}
	total := 0
	for i := offset + 1; i > 0; i -= i & -i {
		total += f.tree[i]
	}
	return total
}

// applySegments applies eligible operations in independent splice segments.
func applySegments(m Model, ops []Operation) (*Result, []int, error) {
	lastDeclaration := -1
	for i := range ops {
		if ops[i].Declaration.Len > 0 {
			lastDeclaration = i
		}
	}

	edited := unedited(m)
	current := m
	starts := make([]int, 0, len(ops))
	var segment *editSegment
	flush := func(final bool, next int) error {
		if segment == nil {
			return nil
		}
		out, err := segment.content()
		if err != nil {
			return err
		}
		edited[m.Source.Name()].content = out
		edited[m.Source.Name()].applied = append(edited[m.Source.Name()].applied, segment.applied...)
		if final {
			segment = nil
			return nil
		}
		current = reparseModel(m, edited)
		m.deferred.locate(current)
		segment = nil
		return current.relocateDeclarations(ops[next:], next)
	}
	runAlone := func(i int, op Operation) error {
		starts = append(starts, i)
		splices, err := current.splicesFor(i, op)
		if err != nil {
			return err
		}
		if err := current.rewrite(edited, splices); err != nil {
			return err
		}
		m.deferred.rebase(m.Source.Name(), splices)
		if err := current.rebaseDeclarations(ops[i+1:], i+1, splices); err != nil {
			return err
		}
		if i+1 < len(ops) {
			current = reparseModel(m, edited)
			m.deferred.locate(current)
			if err := current.relocateDeclarations(ops[i+1:], i+1); err != nil {
				return err
			}
		}
		return nil
	}

	for i, op := range ops {
		if i <= lastDeclaration || (op.Kind != OpSetValue && op.Kind != OpAddMember) {
			if err := flush(false, i); err != nil {
				return nil, starts, err
			}
			if err := runAlone(i, op); err != nil {
				return nil, starts, err
			}
			continue
		}

		candidate, err := current.segmentCandidate(i, op)
		if err != nil {
			if segment == nil {
				starts = append(starts, i)
				return nil, starts, err
			}
			if err := flush(false, i); err != nil {
				return nil, starts, err
			}
			candidate, err = current.segmentCandidate(i, op)
			if err != nil {
				starts = append(starts, i)
				return nil, starts, err
			}
		}
		if alone(op, candidate) {
			if err := flush(false, i); err != nil {
				return nil, starts, err
			}
			if err := runAlone(i, op); err != nil {
				return nil, starts, err
			}
			continue
		}
		if segment == nil {
			starts = append(starts, i)
			segment = newEditSegment(current)
			if err := segment.add(candidate, nil); err != nil {
				return nil, starts, err
			}
			continue
		}

		chain := segment.chains[candidate.addOwner()]
		if segment.sealed || !segment.independent(candidate, chain) {
			if err := flush(false, i); err != nil {
				return nil, starts, err
			}
			candidate, err = current.segmentCandidate(i, op)
			if err != nil {
				starts = append(starts, i)
				return nil, starts, err
			}
			if alone(op, candidate) {
				if err := runAlone(i, op); err != nil {
					return nil, starts, err
				}
				continue
			}
			starts = append(starts, i)
			segment = newEditSegment(current)
			if err := segment.add(candidate, nil); err != nil {
				return nil, starts, err
			}
			continue
		}
		if err := segment.add(candidate, chain); err != nil {
			return nil, starts, err
		}
	}

	if err := flush(true, len(ops)); err != nil {
		return nil, starts, err
	}
	if err := m.validate(edited); err != nil {
		return nil, starts, err
	}
	return edited.result(m.Source.Name()), starts, nil
}

// alone reports whether a candidate must retain sequential application.
func alone(op Operation, candidate segmentCandidate) bool {
	if op.Kind != OpAddMember {
		return false
	}
	details := candidate.add
	if details == nil {
		return false
	}
	switch details.owner.(type) {
	case *ast.RootNamespace, *ast.SubstateMember:
		return true
	}
	if parser.BodyIsCalculation(details.owner) {
		return true
	}
	switch op.MemberKind {
	case kindEntryAction, kindDoAction, kindExitAction:
		return true
	}
	return false
}

func newEditSegment(m Model) *editSegment {
	tabs := m.tokenData().tabs
	return &editSegment{
		model: m, chains: make(map[ast.Node]*memberChain), names: new(pathTrie),
		ownerNames: make(map[ast.Node]map[string]bool), nonBenign: []int{0},
		deltas:   fenwick{tree: make([]int, m.Source.Len()+2)},
		tabCount: tabs, originalTabs: tabs,
	}
}

// segmentCandidate resolves an operation against the segment-start model.
func (m Model) segmentCandidate(i int, op Operation) (segmentCandidate, error) {
	if op.Kind == OpAddMember {
		details, err := m.addMemberSpliceDetails(i, op)
		if err != nil {
			return segmentCandidate{}, err
		}
		read := details.owner.Span()
		end := read.End()
		read.Offset = lineStart(m.Source.Bytes(), read.Offset)
		read.Len = end - read.Offset
		return segmentCandidate{
			op: op, sp: details.splice, add: &details, path: details.ownerPath,
			read: read,
		}, nil
	}
	sym, err := m.target(i, op)
	if err != nil {
		return segmentCandidate{}, err
	}
	sp, err := m.valueSplice(i, op, sym)
	if err != nil {
		return segmentCandidate{}, err
	}
	usage := sym.Decl.(*ast.Usage)
	token := m.tokenSpan(usage.Span())
	return segmentCandidate{
		op: op, sp: sp, path: symbols.NameChain(sym),
		read: source.Span{Offset: usage.Span().Offset, Len: token.End() - usage.Span().Offset},
	}, nil
}

func (c segmentCandidate) addOwner() ast.Node {
	if c.add == nil {
		return nil
	}
	return c.add.owner
}

// independent reports whether a candidate can join the current segment.
func (s *editSegment) independent(c segmentCandidate, chain *memberChain) bool {
	if !s.namesIndependent(c) {
		return false
	}
	if c.add != nil && (s.tabCount > 0) != (s.originalTabs > 0) {
		return false
	}
	if !s.writeIndependent(c, chain) {
		return false
	}
	return true
}

func (s *editSegment) namesIndependent(c segmentCandidate) bool {
	if s.names.hasAncestor(c.path) {
		return false
	}
	if c.add != nil {
		takenName := symbolName(c.add.takenName)
		if takenName != "" && s.ownerNames[c.add.owner][takenName] {
			return false
		}
	}
	return true
}

func (t *pathTrie) insert(path []string) {
	node := t
	for _, part := range path {
		if node.children == nil {
			node.children = make(map[string]*pathTrie)
		}
		if node.children[part] == nil {
			node.children[part] = new(pathTrie)
		}
		node = node.children[part]
	}
	node.terminal = true
}

func (t *pathTrie) hasAncestor(path []string) bool {
	node := t
	for _, part := range path {
		if node.terminal {
			return true
		}
		node = node.children[part]
		if node == nil {
			return false
		}
	}
	return node.terminal
}

// writeIndependent checks whether a candidate's read and write ranges are safe.
func (s *editSegment) writeIndependent(c segmentCandidate, chain *memberChain) bool {
	pos := sort.Search(len(s.writes), func(i int) bool {
		return s.writes[i].sp.span.Offset >= c.sp.span.Offset
	})
	for _, index := range []int{pos - 1, pos} {
		if index < 0 || index >= len(s.writes) {
			continue
		}
		prior := s.writes[index]
		if prior.chain == chain && chain != nil {
			continue
		}
		if intervalsTouch(c.sp.span, prior.sp.span) {
			return false
		}
	}

	readStart := sort.Search(len(s.writes), func(i int) bool {
		return s.writes[i].sp.span.End() >= c.read.Offset
	})
	readEnd := sort.Search(len(s.writes), func(i int) bool {
		return s.writes[i].sp.span.Offset > c.read.End()
	})
	if readStart == readEnd {
		return true
	}
	if c.add == nil {
		return false
	}
	chainIndex := s.chainWriteIndex(chain)
	bad := s.nonBenign[readEnd] - s.nonBenign[readStart]
	if chainIndex >= readStart && chainIndex < readEnd && !s.writes[chainIndex].benign {
		bad--
	}
	if bad > 0 {
		return false
	}
	line := lineStart(s.model.Source.Bytes(), c.sp.span.Offset)
	threshold := sort.Search(len(s.writes), func(i int) bool {
		return s.writes[i].sp.span.End() >= line
	})
	if threshold < readStart {
		threshold = readStart
	}
	if threshold < readEnd {
		count := readEnd - threshold
		if chainIndex >= threshold && chainIndex < readEnd {
			count--
		}
		if count > 0 {
			return false
		}
	}
	return true
}

func (s *editSegment) chainWriteIndex(chain *memberChain) int {
	if chain == nil {
		return -1
	}
	index := sort.Search(len(s.writes), func(i int) bool {
		return s.writes[i].sp.span.Offset >= chain.span.Offset
	})
	if index < len(s.writes) && s.writes[index].chain == chain {
		return index
	}
	return -1
}

func intervalsTouch(a, b source.Span) bool {
	return a.Offset <= b.End() && b.Offset <= a.End()
}

func (s *editSegment) add(c segmentCandidate, chain *memberChain) error {
	sp := c.sp
	oldText := s.model.Source.Text(sp.span)
	if chain != nil {
		shift := 0
		if chain.span.Offset > 0 {
			shift = s.deltas.sum(chain.span.Offset - 1)
		}
		offset := chain.span.Offset + chain.tailStart + shift + len(chain.text) - len(chain.closeIndent)
		newText := chain.indent + c.add.memberText + "\n" + chain.closeIndent
		sp = splice{
			span: source.Span{Offset: offset, Len: len(chain.closeIndent)},
			text: newText, opIndex: c.sp.opIndex, target: c.sp.target,
		}
		oldText = chain.closeIndent
		s.tabCount += tabCount(newText) - tabCount(oldText)
		chain.text = chain.text[:len(chain.text)-len(chain.closeIndent)] + newText
		s.deltas.add(chain.span.End(), len(newText)-len(oldText))
		tail := chain.text
		if !bodyHasBody(chain.owner) {
			tail += "}"
		}
		chain.head.sp.text = chain.head.sp.text[:chain.tailStart] + tail
	} else {
		sp.span.Offset += s.deltas.sum(sp.span.Offset)
		s.tabCount += tabCount(sp.text) - tabCount(oldText)
		write := &segmentWrite{
			sp: c.sp, add: c.add, oldText: oldText,
			benign: c.op.Kind == OpSetValue && benignSetValue(s.model.Source.Bytes(), c.sp, oldText),
		}
		if c.add != nil {
			if head := newMemberChain(s.model, *c.add); head != nil {
				write.chain = head
				head.head = write
				s.chains[c.add.owner] = head
			}
		}
		insert := sort.Search(len(s.writes), func(i int) bool {
			return s.writes[i].sp.span.Offset >= c.sp.span.Offset
		})
		s.writes = append(s.writes, nil)
		copy(s.writes[insert+1:], s.writes[insert:])
		s.writes[insert] = write
		if insert == len(s.writes)-1 {
			s.nonBenign = append(s.nonBenign, s.nonBenign[len(s.nonBenign)-1]+boolInt(!write.benign))
		} else {
			s.nonBenign = append(s.nonBenign, 0)
			for i := insert; i < len(s.writes); i++ {
				s.nonBenign[i+1] = s.nonBenign[i] + boolInt(!s.writes[i].benign)
			}
		}
		s.deltas.add(c.sp.span.End(), len(c.sp.text)-len(oldText))
	}
	if c.add != nil {
		for _, name := range c.add.introducedNames {
			prefix := append(append([]string(nil), c.add.ownerPath...), name)
			s.names.insert(prefix)
			if s.ownerNames[c.add.owner] == nil {
				s.ownerNames[c.add.owner] = make(map[string]bool)
			}
			s.ownerNames[c.add.owner][name] = true
		}
	}
	s.applied = append(s.applied, Applied{
		OperationIndex: sp.opIndex, Target: sp.target, Span: sp.span,
		OldText: oldText, NewText: sp.text,
	})
	s.model.deferred.rebase(s.model.Source.Name(), []splice{sp})
	// No rebaseDeclarations: segment operations all follow the last declaration-bearing one.
	if !lexicallyClosed(sp.text, s.model.Source.Name(), s.model.Source.Kind()) {
		s.sealed = true
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func newMemberChain(m Model, details addMemberDetails) *memberChain {
	switch details.owner.(type) {
	case *ast.Package, *ast.Namespace, *ast.Definition, *ast.Usage:
	default:
		return nil
	}
	if parser.BodyIsCalculation(details.owner) {
		return nil
	}
	_, hasBody := bodyInfo(details.owner)
	tail := details.insertion.text[details.insertion.at:]
	if !strings.HasPrefix(tail, details.memberText+"\n") {
		return nil
	}
	closeIndent := ""
	chainText := tail
	if hasBody {
		closeIndent = tail[len(details.memberText)+1:]
	} else {
		closeIndent = lineIndent(m.Source.Bytes(), details.owner.Span().Offset)
		if !strings.HasSuffix(tail, closeIndent+"}") {
			return nil
		}
		chainText = tail[:len(tail)-1]
	}
	if !strings.HasSuffix(chainText, closeIndent) {
		return nil
	}
	return &memberChain{
		owner: details.owner, span: details.splice.span, text: chainText,
		tailStart: details.insertion.at, indent: details.indent,
		closeIndent: closeIndent,
	}
}

func bodyHasBody(owner ast.Node) bool {
	_, hasBody := bodyInfo(owner)
	return hasBody
}

func benignSetValue(content []byte, sp splice, oldText string) bool {
	if strings.ContainsAny(oldText, "\r\n") || strings.ContainsAny(sp.text, "\r\n") {
		return false
	}
	start := lineStart(content, sp.span.Offset)
	first := start
	for first < len(content) && (content[first] == ' ' || content[first] == '\t') {
		first++
	}
	return sp.span.Offset > first
}

func lineStart(content []byte, offset int) int {
	if offset > len(content) {
		offset = len(content)
	}
	for offset > 0 && content[offset-1] != '\n' {
		offset--
	}
	return offset
}

func tabCount(text string) int {
	return bytes.Count([]byte(text), []byte{'\t'})
}

// lexicallyClosed reports whether replacement text leaves the lexer outside tokens.
func lexicallyClosed(text, name string, kind source.Kind) bool {
	sf := source.NewWithKind(name, []byte(text), kind)
	lx := lexer.New(sf)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Unterminated || tok.Kind == lexer.Error {
			return false
		}
		if (tok.Kind == lexer.SLNote || tok.Kind == lexer.MLNote || tok.Kind == lexer.RegularComment) &&
			tok.Span.End() == len(text) && !strings.HasSuffix(text, "\n") {
			return false
		}
	}
	return true
}

func (s *editSegment) content() ([]byte, error) {
	sort.Slice(s.writes, func(i, j int) bool {
		return s.writes[i].sp.span.Offset < s.writes[j].sp.span.Offset
	})
	src := s.model.Source.Bytes()
	out := make([]byte, 0, len(src)+s.deltas.sum(len(src)))
	cursor := 0
	for _, write := range s.writes {
		span := write.sp.span
		if span.Offset < cursor || span.End() > len(src) {
			return nil, &Error{
				Failure: FailureResultInvalid, OperationIndex: write.sp.opIndex,
				Message: "edit segment has overlapping or out-of-range writes",
			}
		}
		out = append(out, src[cursor:span.Offset]...)
		out = append(out, write.sp.text...)
		cursor = span.End()
	}
	out = append(out, src[cursor:]...)
	return out, nil
}
