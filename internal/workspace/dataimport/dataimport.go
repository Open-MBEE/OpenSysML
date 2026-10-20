// Package dataimport lowers the rows of a CSV, TSV, JSON or JSON Lines file to
// the edits that set the feature values they assign, one row per element, as
// `sysml -import` and the service's ImportData both apply them.
package dataimport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sedit "github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/exec/ingest"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Options are how a data file is read: a mapping onto elements and features,
// and a format overriding the one its extension names.
type Options struct {
	// Mapping is the JSON mapping, nil for the file's own header.
	Mapping []byte
	// Format is csv, tsv, json or jsonl; "" reads the mapping's, else the extension's.
	Format string
}

// ReadFile reads the rows of the data file at path, named name in what goes
// wrong, with the mapping file at mapPath when one is named.
func ReadFile(path, name, mapPath string, format string) ([]ingest.Row, error) {
	opts := Options{Format: format}
	if mapPath != "" {
		data, err := os.ReadFile(filepath.Clean(mapPath))
		if err != nil {
			return nil, err
		}
		opts.Mapping = data
		rows, err := read(name, func() ([]byte, error) { return os.ReadFile(filepath.Clean(path)) }, path, opts)
		if err != nil {
			var mapErr *mappingError
			if errors.As(err, &mapErr) {
				return nil, fmt.Errorf("%s: %w", mapPath, mapErr.err)
			}
		}
		return rows, err
	}
	return read(name, func() ([]byte, error) { return os.ReadFile(filepath.Clean(path)) }, path, opts)
}

// Read reads the rows of data, a file named name, as opts say.
func Read(name string, data []byte, opts Options) ([]ingest.Row, error) {
	rows, err := read(name, func() ([]byte, error) { return data, nil }, name, opts)
	var mapErr *mappingError
	if errors.As(err, &mapErr) {
		return nil, fmt.Errorf("mapping: %w", mapErr.err)
	}
	return rows, err
}

// mappingError is a refused mapping, so the caller names the mapping's source.
type mappingError struct{ err error }

func (e *mappingError) Error() string { return e.err.Error() }
func (e *mappingError) Unwrap() error { return e.err }

func read(name string, load func() ([]byte, error), path string, opts Options) ([]ingest.Row, error) {
	var m *ingest.Map
	if opts.Mapping != nil {
		parsed, err := ingest.ParseMap(opts.Mapping)
		if err != nil {
			return nil, &mappingError{err}
		}
		m = parsed
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
	} else if f, ok := ingest.FormatOfPath(path); ok {
		format = f
	} else {
		return nil, fmt.Errorf("%s: the extension names no data format; name one as csv, tsv, json or jsonl", name)
	}
	data, err := load()
	if err != nil {
		return nil, err
	}
	return ingest.Read(name, data, format, m)
}

// Assignment is one edit an import makes: the value it sets on a feature, or the
// redefinition it declares to set one, and where in the data it came from.
type Assignment struct {
	// Op is the edit, on the document Doc at the declaration at offset At.
	Op  sedit.Operation
	Doc string
	At  int
	// Where is the row or cell of the data the assignment came from.
	Where string
	// Element is the qualified name of the element whose feature is set, Feature
	// the feature's name and Value the literal it is set to; Redefines is the
	// qualified name of the inherited feature redefined to set it, "" when the
	// element declares the feature itself. A redefinition made only to reach a
	// nested inherited feature sets no Value.
	Element   string
	Feature   string
	Value     string
	Redefines string
}

// Description spells the assignment as `sysml -import` reports it.
func (a Assignment) Description() string {
	target := a.Element + "::" + a.Feature
	switch {
	case a.Redefines == "":
		return target + " = " + a.Value
	case a.Value == "":
		return fmt.Sprintf("%s (redefines %s)", target, a.Redefines)
	}
	return fmt.Sprintf("%s = %s (redefines %s)", target, a.Value, a.Redefines)
}

// Summary counts what an import sets: the values, and the elements they are set on.
type Summary struct {
	Values   int
	Elements int
}

// Lower lowers rows to edits: a feature the element declares has its value set,
// one it inherits is redefined in the element's body with the value. own says
// whether a document is the model's own, so an element of another is refused.
func Lower(idx *symbols.Index, resolver *resolve.Resolver, sem *semantics.Model, own func(doc string) bool, rows []ingest.Row) ([]Assignment, Summary, error) {
	if idx == nil {
		return nil, Summary{}, errors.New("no model is loaded to import into")
	}
	l := &lowering{idx: idx, resolver: resolver, sem: sem,
		elements: map[string]bool{}, assigned: map[[2]string]string{}, redefined: map[string]bool{}}
	for _, row := range rows {
		if len(row.Cells) == 0 {
			continue
		}
		t, err := targetOf(idx, resolver, sem, row.Element)
		if err != nil {
			return nil, Summary{}, fmt.Errorf("%s: %w", row.Where, err)
		}
		if !own(t.anchor.DocName) {
			return nil, Summary{}, fmt.Errorf("%s: %s is a library element, which an import does not change", row.Where, row.Element)
		}
		l.elements[t.fqn] = true
		owner, err := l.redefineVia(row, t)
		if err != nil {
			return nil, Summary{}, err
		}
		for _, cell := range row.Cells {
			if err := l.assign(t, owner, cell); err != nil {
				return nil, Summary{}, err
			}
		}
	}
	return l.ops, Summary{Values: l.values, Elements: len(l.elements)}, nil
}

// lowering accumulates the edits of one import and what they have set so far.
type lowering struct {
	idx       *symbols.Index
	resolver  *resolve.Resolver
	sem       *semantics.Model
	ops       []Assignment
	elements  map[string]bool
	assigned  map[[2]string]string
	redefined map[string]bool
	values    int
}

// redefineVia redefines, once each, the inherited features a row's element is
// reached through, and names the body the row's own redefinitions go in.
func (l *lowering) redefineVia(row ingest.Row, t target) (string, error) {
	owner := source.QualifiedNameOf(symbols.NameChain(t.anchor))
	path := l.idx.GetFQN(t.anchor)
	for _, via := range t.via {
		path += "::" + via.Name
		if !l.redefined[path] {
			kind, ok := redefinitionKeyword(via.Kind)
			if !ok {
				return "", fmt.Errorf("%s: %s inherits %s, a %s an import cannot redefine", row.Where, strings.TrimSuffix(path, "::"+via.Name), via.Name, via.Kind)
			}
			op := sedit.AddMember(owner, kind, "")
			op.Redefines = []string{source.NameText(via.Name)}
			l.ops = append(l.ops, Assignment{
				Op: op, Where: row.Where, Doc: t.anchor.DocName, At: t.anchor.DeclSpan.Offset,
				Element: strings.TrimSuffix(path, "::"+via.Name), Feature: via.Name, Redefines: l.idx.GetFQN(via),
			})
			l.redefined[path] = true
		}
		owner += "::" + source.NameText(via.Name)
	}
	return owner, nil
}

// assign lowers one cell of the element t: a feature it declares has its value
// set, one it inherits is redefined with the value in the body named owner.
func (l *lowering) assign(t target, owner string, cell ingest.Cell) error {
	if first, ok := l.assigned[[2]string{t.fqn, cell.Feature}]; ok {
		return fmt.Errorf("%s: %s::%s is already set at %s", cell.Where, t.fqn, cell.Feature, first)
	}
	l.assigned[[2]string{t.fqn, cell.Feature}] = cell.Where
	feat, declared := feature(l.sem, l.resolver, t.sym, cell.Feature)
	if feat == nil {
		return fmt.Errorf("%s: %s has no feature named %s", cell.Where, t.fqn, cell.Feature)
	}
	declared = declared && len(t.via) == 0
	class, enum := valueClass(l.sem, l.idx, feat)
	value, err := ingest.Literal(cell, class, enum)
	if err != nil {
		return fmt.Errorf("%s: %w", cell.Where, err)
	}
	if class == ingest.ClassEnum && !enumValue(l.idx, enum, value) {
		return fmt.Errorf("%s: %s is not a value of %s", cell.Where, strings.TrimSpace(cell.Text), enum)
	}
	if err := checkDimension(l.sem, t.sym, feat, cell, value); err != nil {
		return fmt.Errorf("%s: %w", cell.Where, err)
	}
	l.values++
	if declared {
		target := l.idx.GetFQN(feat)
		l.ops = append(l.ops, Assignment{
			Op: sedit.SetValue(source.QualifiedNameOf(symbols.NameChain(feat)), value), Where: cell.Where,
			Doc: feat.DocName, At: feat.DeclSpan.Offset,
			Element: strings.TrimSuffix(target, "::"+feat.Name), Feature: feat.Name, Value: value,
		})
		return nil
	}
	kind, ok := redefinitionKeyword(feat.Kind)
	if !ok {
		return fmt.Errorf("%s: %s inherits %s, a %s an import cannot redefine", cell.Where, t.fqn, cell.Feature, feat.Kind)
	}
	op := sedit.AddMember(owner, kind, "")
	op.Redefines = []string{source.NameText(feat.Name)}
	op.Value = value
	l.ops = append(l.ops, Assignment{
		Op: op, Where: cell.Where, Doc: t.anchor.DocName, At: t.anchor.DeclSpan.Offset,
		Element: t.fqn, Feature: feat.Name, Value: value, Redefines: l.idx.GetFQN(feat),
	})
	return nil
}

// target is the element a row names: anchor is the nearest declared element,
// via the inherited features below it to redefine there, outermost first.
type target struct {
	sym    *symbols.Symbol
	fqn    string
	anchor *symbols.Symbol
	via    []*symbols.Symbol
}

func targetOf(idx *symbols.Index, resolver *resolve.Resolver, sem *semantics.Model, name string) (target, error) {
	segs, ok := source.QualifiedNameSegments(name)
	if !ok || len(segs) == 0 {
		return target{}, fmt.Errorf("%s is not a qualified name", name)
	}
	for n := len(segs); n >= 1; n-- {
		syms := idx.LookupQualified(strings.Join(segs[:n], "::"))
		if len(syms) == 0 {
			continue
		}
		if len(syms) > 1 {
			return target{}, fmt.Errorf("%s names more than one element", strings.Join(segs[:n], "::"))
		}
		sym := resolver.AliasedElement(syms[0])
		t := target{sym: sym, fqn: idx.GetFQN(sym), anchor: sym}
		for _, seg := range segs[n:] {
			feat, declared := feature(sem, resolver, t.sym, seg)
			switch {
			case feat == nil:
				return target{}, fmt.Errorf("no element named %s: %s has no member named %s", name, t.fqn, seg)
			case declared && len(t.via) == 0:
				t.sym, t.fqn, t.anchor = feat, idx.GetFQN(feat), feat
			default:
				t.sym, t.fqn = feat, t.fqn+"::"+feat.Name
				t.via = append(t.via, feat)
			}
		}
		return t, nil
	}
	return target{}, fmt.Errorf("no element named %s", name)
}

// checkDimension refuses a value whose dimension is not the feature's.
func checkDimension(sem *semantics.Model, elem, feat *symbols.Symbol, cell ingest.Cell, value string) error {
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

// feature is the feature of elem named name, declared by elem itself or
// inherited from a supertype or a type of elem; declared says which.
func feature(sem *semantics.Model, resolver *resolve.Resolver, elem *symbols.Symbol, name string) (feat *symbols.Symbol, declared bool) {
	supers := sem.AllSupertypes(elem)
	if elem.IsFeature() {
		for _, t := range sem.FeatureTypes(elem) {
			supers = append(supers, t)
			supers = append(supers, sem.AllSupertypes(t)...)
		}
	}
	if elem.Scope != nil {
		if m, local := memberFeature(resolver, elem, supers, elem.Scope, name, true); m != nil {
			return m, local
		}
	}
	for _, sup := range supers {
		if sup.Scope == nil {
			continue
		}
		if m, _ := memberFeature(resolver, elem, supers, sup.Scope, name, false); m != nil {
			return m, false
		}
	}
	return nil, false
}

// memberFeature is the feature named name among scope's own members, nil for a
// member that is none; local says whether scope is elem's own. An alias counts
// when it names a feature of elem or of one of its supertypes supers.
func memberFeature(resolver *resolve.Resolver, elem *symbols.Symbol, supers []*symbols.Symbol, scope *symbols.Scope, name string, local bool) (*symbols.Symbol, bool) {
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

// enumValue is whether value names an enumerated value of the enumeration enum.
func enumValue(idx *symbols.Index, enum, value string) bool {
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

// valueClass is the class of literal the feature's type takes, and the
// enumeration it is when it is one.
func valueClass(sem *semantics.Model, idx *symbols.Index, feat *symbols.Symbol) (ingest.Class, string) {
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

// redefinitionKeyword is the usage keyword a redefinition of a feature of the
// kind is declared with, false for a kind an import does not redefine.
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
