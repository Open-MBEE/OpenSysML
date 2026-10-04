package repl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	sedit "github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/exec/ingest"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const importUsage = "usage: %import <file> [as values] [map <file>] [format csv|tsv|json|jsonl] [dry-run]"

// ImportOptions are how ImportData reads its file: a mapping file, a format
// overriding the extension's, and whether the model is left as it was.
type ImportOptions struct {
	Map    string
	Format string
	DryRun bool
}

// ImportData sets the feature values a CSV, TSV, JSON or JSON Lines file
// assigns, one row per element, as one edit of the model's source: a value
// the model refuses leaves every other row unapplied too.
func (s *Session) ImportData(path string, opts ImportOptions) Verdict {
	defer s.enter()()
	return s.importData(path, opts)
}

func (s *Session) doImport(tail string) ([]string, bool, error) {
	args := parseArgs(strings.TrimSpace(tail))
	if len(args) == 0 {
		return []string{importUsage}, false, nil
	}
	path := nameText(args[0])
	var opts ImportOptions
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "dry-run":
			opts.DryRun = true
		case "map", "format":
			if i+1 >= len(args) {
				return []string{errPrefix + args[i] + " names a value", importUsage}, false, nil
			}
			if args[i] == "map" {
				opts.Map = nameText(args[i+1])
			} else {
				opts.Format = args[i+1]
			}
			i++
		case "as":
			if i+1 >= len(args) || args[i+1] != "values" {
				return []string{errPrefix + "only `as values` is supported", importUsage}, false, nil
			}
			i++
		default:
			return []string{errPrefix + "unexpected " + args[i], importUsage}, false, nil
		}
	}
	return s.importData(path, opts).Lines, false, nil
}

// importOp is one edit an import makes, with the input value it came from.
type importOp struct {
	op    sedit.Operation
	where string
	desc  string
	doc   string
	at    int
}

func (s *Session) importData(path string, opts ImportOptions) Verdict {
	fail := func(err error) Verdict {
		return Verdict{Subject: path, Status: VerdictFails, Lines: []string{errPrefix + err.Error(), "  the model was left unchanged"}}
	}
	rows, err := readImport(expandHome(path), path, opts)
	if err != nil {
		return fail(err)
	}
	ops, values, elements, err := s.importOps(rows)
	if err != nil {
		return fail(err)
	}
	if values == 0 {
		return Verdict{Subject: path, Status: VerdictHolds, Lines: []string{"✓ " + path + ": no values to import"}}
	}
	edited, err := s.importEdits(ops)
	if err != nil {
		return fail(err)
	}
	head := fmt.Sprintf("✓ imported %s into %s from %s", countOf(values, "value", "values"), countOf(elements, "element", "elements"), path)
	if opts.DryRun {
		head = fmt.Sprintf("dry run: would import %s into %s from %s; the model is unchanged", countOf(values, "value", "values"), countOf(elements, "element", "elements"), path)
	} else if err := s.commitImport(edited); err != nil {
		return fail(err)
	}
	lines := []string{head}
	for _, o := range ops {
		lines = append(lines, "  "+o.desc)
	}
	return Verdict{Subject: path, Status: VerdictHolds, Lines: lines}
}

func readImport(file, name string, opts ImportOptions) ([]ingest.Row, error) {
	var m *ingest.Map
	if opts.Map != "" {
		data, err := os.ReadFile(expandHome(opts.Map))
		if err != nil {
			return nil, err
		}
		m, err = ingest.ParseMap(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", opts.Map, err)
		}
	}
	formatName := opts.Format
	if formatName == "" && m != nil {
		formatName = m.Format
	}
	var format ingest.Format
	if formatName != "" {
		f, err := ingest.ParseFormat(formatName)
		if err != nil {
			return nil, err
		}
		format = f
	} else if f, ok := ingest.FormatOfPath(file); ok {
		format = f
	} else {
		return nil, fmt.Errorf("%s: the extension names no data format; name one as csv, tsv, json or jsonl", name)
	}
	data, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		return nil, err
	}
	return ingest.Read(name, data, format, m)
}

// importOps lowers rows to edits: a feature the element declares has its value
// set, one it inherits is redefined in the element's body with the value. It
// also counts the values set and the elements they are set on.
func (s *Session) importOps(rows []ingest.Row) ([]importOp, int, int, error) {
	idx := s.symbolIndex()
	if idx == nil {
		return nil, 0, 0, errors.New("no model is loaded to import into")
	}
	resolver := resolve.New(idx)
	sem := semantics.NewModel(resolver)
	resolver.SetModel(sem)
	own := map[string]bool{docName: true}
	for _, sn := range s.snippets {
		if sn.origin != "" && !sn.open {
			own[sn.origin] = true
		}
	}
	var ops []importOp
	elements := map[string]bool{}
	assigned := map[[2]string]string{}
	redefined := map[string]bool{}
	values := 0
	for _, row := range rows {
		if len(row.Cells) == 0 {
			continue
		}
		t, err := importTargetOf(idx, resolver, sem, row.Element)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%s: %w", row.Where, err)
		}
		if !own[t.anchor.DocName] {
			return nil, 0, 0, fmt.Errorf("%s: %s is a library element, which an import does not change", row.Where, row.Element)
		}
		elements[t.fqn] = true
		owner := source.QualifiedNameOf(symbols.NameChain(t.anchor))
		path := idx.GetFQN(t.anchor)
		for _, via := range t.via {
			path += "::" + via.Name
			if !redefined[path] {
				kind, ok := redefinitionKeyword(via.Kind)
				if !ok {
					return nil, 0, 0, fmt.Errorf("%s: %s inherits %s, a %s an import cannot redefine", row.Where, strings.TrimSuffix(path, "::"+via.Name), via.Name, via.Kind)
				}
				op := sedit.AddMember(owner, kind, "")
				op.Redefines = []string{source.NameText(via.Name)}
				ops = append(ops, importOp{
					op: op, where: row.Where,
					desc: fmt.Sprintf("%s (redefines %s)", path, idx.GetFQN(via)),
					doc:  t.anchor.DocName, at: t.anchor.DeclSpan.Offset,
				})
				redefined[path] = true
			}
			owner += "::" + source.NameText(via.Name)
		}
		for _, cell := range row.Cells {
			if first, ok := assigned[[2]string{t.fqn, cell.Feature}]; ok {
				return nil, 0, 0, fmt.Errorf("%s: %s::%s is already set at %s", cell.Where, t.fqn, cell.Feature, first)
			}
			assigned[[2]string{t.fqn, cell.Feature}] = cell.Where
			feat, declared := importFeature(sem, resolver, t.sym, cell.Feature)
			if feat == nil {
				return nil, 0, 0, fmt.Errorf("%s: %s has no feature named %s", cell.Where, t.fqn, cell.Feature)
			}
			declared = declared && len(t.via) == 0
			class, enum := importClass(sem, idx, feat)
			value, err := ingest.Literal(cell, class, enum)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("%s: %w", cell.Where, err)
			}
			if class == ingest.ClassEnum && !importEnumValue(idx, enum, value) {
				return nil, 0, 0, fmt.Errorf("%s: %s is not a value of %s", cell.Where, strings.TrimSpace(cell.Text), enum)
			}
			if err := importDimension(sem, t.sym, feat, cell, value); err != nil {
				return nil, 0, 0, fmt.Errorf("%s: %w", cell.Where, err)
			}
			values++
			if declared {
				target := idx.GetFQN(feat)
				ops = append(ops, importOp{
					op: sedit.SetValue(source.QualifiedNameOf(symbols.NameChain(feat)), value), where: cell.Where,
					desc: target + " = " + value, doc: feat.DocName, at: feat.DeclSpan.Offset,
				})
				continue
			}
			kind, ok := redefinitionKeyword(feat.Kind)
			if !ok {
				return nil, 0, 0, fmt.Errorf("%s: %s inherits %s, a %s an import cannot redefine", cell.Where, t.fqn, cell.Feature, feat.Kind)
			}
			op := sedit.AddMember(owner, kind, "")
			op.Redefines = []string{source.NameText(feat.Name)}
			op.Value = value
			ops = append(ops, importOp{
				op: op, where: cell.Where,
				desc: fmt.Sprintf("%s::%s = %s (redefines %s)", t.fqn, feat.Name, value, idx.GetFQN(feat)),
				doc:  t.anchor.DocName, at: t.anchor.DeclSpan.Offset,
			})
		}
	}
	return ops, values, len(elements), nil
}

// importTarget is the element a row names: anchor is the nearest declared element,
// via the inherited features below it to redefine there, outermost first.
type importTarget struct {
	sym    *symbols.Symbol
	fqn    string
	anchor *symbols.Symbol
	via    []*symbols.Symbol
}

// importTargetOf resolves name, following an alias to the element it names.
func importTargetOf(idx *symbols.Index, resolver *resolve.Resolver, sem *semantics.Model, name string) (importTarget, error) {
	segs, ok := source.QualifiedNameSegments(name)
	if !ok || len(segs) == 0 {
		return importTarget{}, fmt.Errorf("%s is not a qualified name", name)
	}
	for n := len(segs); n >= 1; n-- {
		syms := idx.LookupQualified(strings.Join(segs[:n], "::"))
		if len(syms) == 0 {
			continue
		}
		if len(syms) > 1 {
			return importTarget{}, fmt.Errorf("%s names more than one element", strings.Join(segs[:n], "::"))
		}
		sym := resolver.AliasedElement(syms[0])
		t := importTarget{sym: sym, fqn: idx.GetFQN(sym), anchor: sym}
		for _, seg := range segs[n:] {
			feat, declared := importFeature(sem, resolver, t.sym, seg)
			switch {
			case feat == nil:
				return importTarget{}, fmt.Errorf("no element named %s: %s has no member named %s", name, t.fqn, seg)
			case declared && len(t.via) == 0:
				t.sym, t.fqn, t.anchor = feat, idx.GetFQN(feat), feat
			default:
				t.sym, t.fqn = feat, t.fqn+"::"+feat.Name
				t.via = append(t.via, feat)
			}
		}
		return t, nil
	}
	return importTarget{}, fmt.Errorf("no element named %s", name)
}

// importDimension refuses a value whose unit measures another dimension than
// the quantity feat is declared to hold.
func importDimension(sem *semantics.Model, elem, feat *symbols.Symbol, cell ingest.Cell, value string) error {
	want, ok := sem.DimensionOfFeature(feat)
	if !ok {
		return nil
	}
	node, ok := parser.ParseOneExpression("<import>", value)
	if !ok {
		return nil
	}
	got, ok := sem.DimensionOfExpr(elem.Scope, node)
	if !ok || want.Term.Commensurable(got.Term) {
		return nil
	}
	if cell.Unit == "" {
		return fmt.Errorf("%s has no unit, but %s is measured in dimension %s; name its unit, as `%s [unit]`", cell.Text, feat.Name, want, cell.Feature)
	}
	return fmt.Errorf("%s is measured in dimension %s, but %s is measured in dimension %s", value, got, feat.Name, want)
}

// importFeature is the feature named name that elem declares, else the one it
// inherits from its types and supertypes; declared reports which. An alias
// there stands for the feature it names, when that is one of elem's own.
func importFeature(sem *semantics.Model, resolver *resolve.Resolver, elem *symbols.Symbol, name string) (feat *symbols.Symbol, declared bool) {
	supers := sem.AllSupertypes(elem)
	if elem.IsFeature() {
		for _, t := range sem.FeatureTypes(elem) {
			supers = append(supers, t)
			supers = append(supers, sem.AllSupertypes(t)...)
		}
	}
	member := func(scope *symbols.Scope, local bool) (*symbols.Symbol, bool) {
		m, ok := scope.LookupLocal(name)
		if !ok {
			return nil, false
		}
		if m.Kind == symbols.SymbolAlias {
			m = resolver.AliasedElement(m)
			if !m.IsFeature() {
				return nil, false
			}
			if m.OwnerScope == elem.Scope {
				return m, true
			}
			for _, sup := range supers {
				if sup.Scope != nil && m.OwnerScope == sup.Scope {
					return m, false
				}
			}
			return nil, false
		}
		if !m.IsFeature() || (local && m.OwnerScope != elem.Scope) {
			return nil, false
		}
		return m, local
	}
	if elem.Scope != nil {
		if m, local := member(elem.Scope, true); m != nil {
			return m, local
		}
	}
	for _, sup := range supers {
		if sup.Scope == nil {
			continue
		}
		if m, _ := member(sup.Scope, false); m != nil {
			return m, false
		}
	}
	return nil, false
}

// importEnumValue reports whether value, an enumeration literal as notation
// spells it, is one of enum's.
func importEnumValue(idx *symbols.Index, enum, value string) bool {
	names, ok := source.QualifiedNameSegments(value)
	if !ok {
		return false
	}
	for _, sym := range idx.LookupQualified(strings.Join(names, "::")) {
		if sym.OwnerScope != nil && idx.GetFQN(sym.OwnerScope.Owner()) == enum {
			return true
		}
	}
	return false
}

// importClass is the kind of value feat's types admit, and the enumeration
// naming its literals when it is one.
func importClass(sem *semantics.Model, idx *symbols.Index, feat *symbols.Symbol) (ingest.Class, string) {
	names := map[string]bool{}
	for _, t := range sem.FeatureTypes(feat) {
		if t.Kind == symbols.SymbolEnumerationDef {
			return ingest.ClassEnum, idx.GetFQN(t)
		}
		names[idx.GetFQN(t)] = true
		for _, sup := range sem.AllSupertypes(t) {
			names[idx.GetFQN(sup)] = true
		}
	}
	switch {
	case names["ScalarValues::String"]:
		return ingest.ClassString, ""
	case names["ScalarValues::Boolean"]:
		return ingest.ClassBoolean, ""
	case names["ScalarValues::Integer"]:
		return ingest.ClassInteger, ""
	case names["ScalarValues::Real"], names["Quantities::ScalarQuantityValue"]:
		return ingest.ClassReal, ""
	}
	return ingest.ClassUnknown, ""
}

// redefinitionKeyword is the keyword a redefinition of a feature of kind is declared with.
func redefinitionKeyword(kind symbols.SymbolKind) (string, bool) {
	switch kind {
	case symbols.SymbolAttributeUsage:
		return "attribute", true
	case symbols.SymbolPartUsage:
		return "part", true
	case symbols.SymbolItemUsage:
		return "item", true
	case symbols.SymbolPortUsage:
		return "port", true
	case symbols.SymbolOccurrenceUsage:
		return "occurrence", true
	case symbols.SymbolReferenceUsage:
		return "ref", true
	}
	return "", false
}

// importEdits applies the operations to the snippets declaring their targets,
// each snippet's as one edit, and returns each edited snippet's new text.
func (s *Session) importEdits(ops []importOp) (map[int]string, error) {
	type group struct {
		doc     string
		snippet int
		start   int
		ops     []int
	}
	var groups []*group
	byKey := map[[2]int]*group{}
	docIDs := map[string]int{}
	for i, o := range ops {
		snippet, start := s.importSnippet(o.doc, o.at)
		if snippet < 0 {
			return nil, fmt.Errorf("%s: %s is not in a session document", o.where, o.op.Target+o.op.Owner)
		}
		id, ok := docIDs[o.doc]
		if !ok {
			id = len(docIDs)
			docIDs[o.doc] = id
		}
		key := [2]int{id, snippet}
		g := byKey[key]
		if g == nil {
			g = &group{doc: o.doc, snippet: snippet, start: start}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.ops = append(g.ops, i)
	}
	out := map[int]string{}
	for _, g := range groups {
		list := make([]sedit.Operation, len(g.ops))
		for j, i := range g.ops {
			list[j] = ops[i].op
		}
		res, _, ok, err := s.ws.ApplyEdit(g.doc, list, nil)
		if err == nil && !ok {
			err = fmt.Errorf("%s is not open", g.doc)
		}
		if err != nil {
			return nil, s.importFailure(g.doc, ops, g.ops, err)
		}
		doc := res.Documents[0]
		src := s.snippets[g.snippet].src
		text, ok := spliceSnippet(src, g.start, doc.Original, doc.Content)
		if !ok {
			return nil, fmt.Errorf("the import changed %s outside the declaration it edits", g.doc)
		}
		out[g.snippet] = text
	}
	return out, nil
}

// importFailure locates a refused edit at the input value it came from; an
// edit refused as a whole is retried an operation at a time to find it.
func (s *Session) importFailure(doc string, ops []importOp, group []int, err error) error {
	var e *sedit.Error
	if !errors.As(err, &e) {
		return err
	}
	if e.OperationIndex >= 0 && e.OperationIndex < len(group) {
		return fmt.Errorf("%s: %s", ops[group[e.OperationIndex]].where, e.Message)
	}
	for _, i := range group {
		if _, _, _, one := s.ws.ApplyEdit(doc, []sedit.Operation{ops[i].op}, nil); one != nil {
			var oneErr *sedit.Error
			if errors.As(one, &oneErr) {
				return fmt.Errorf("%s: %s", ops[i].where, oneErr.Message)
			}
			return fmt.Errorf("%s: %w", ops[i].where, one)
		}
	}
	return err
}

// importSnippet is the snippet holding offset in doc and where its text starts in
// that document, or -1.
func (s *Session) importSnippet(doc string, offset int) (int, int) {
	if doc != docName {
		for i := len(s.snippets) - 1; i >= 0; i-- {
			if sn := s.snippets[i]; sn.origin == doc && !sn.open {
				return i, 0
			}
		}
		return -1, 0
	}
	acc := 0
	for i, sn := range s.snippets {
		end := acc + len(sn.src)
		if offset >= acc && offset < end && sn.origin == "" && !sn.open {
			return i, acc
		}
		acc = end + 1
	}
	return -1, 0
}

// spliceSnippet carries the change between a document's original and edited
// text into the snippet starting at start in it; false when the change reaches
// outside the snippet.
func spliceSnippet(src string, start int, original, edited []byte) (string, bool) {
	p := 0
	for p < len(original) && p < len(edited) && original[p] == edited[p] {
		p++
	}
	q := 0
	for q < len(original)-p && q < len(edited)-p && original[len(original)-1-q] == edited[len(edited)-1-q] {
		q++
	}
	from, to := p-start, len(original)-q-start
	if from < 0 || to > len(src) || from > to {
		return "", false
	}
	return src[:from] + string(edited[p:len(edited)-q]) + src[to:], true
}

// commitImport replaces the edited snippets and rebuilds the model, restoring
// the buffer when the result reports an error it did not before.
func (s *Session) commitImport(edited map[int]string) error {
	before := slices.Clone(s.snippets)
	beforeErrors := s.errorCounts()
	s.version++
	for i, text := range edited {
		s.snippets[i].src = text
		if s.snippets[i].origin != "" {
			s.snippets[i].gen = s.version
		}
	}
	s.rebuildOver(nil)
	s.idxVersion = 0
	s.names = nil
	if problems := newProblems(beforeErrors, s.diagnostics()); len(problems) > 0 {
		s.rollbackSubmit(before)
		return fmt.Errorf("the imported values make the model invalid: %s", strings.Join(problems, "; "))
	}
	return nil
}
