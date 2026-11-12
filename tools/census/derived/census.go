// Package derived measures, over the repository's example models, how often
// each served metamodel rule answers a value, answers nothing, or is not
// derived, against the OMG pilot's export of the same models as the reference.
package derived

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Counts is how one side answered one served rule over the elements whose
// metaclass carries it.
type Counts struct {
	// Carriers are the elements the rule applies to.
	Carriers int `json:"carriers"`
	// Value counts the carriers answered with at least one value.
	Value int `json:"value"`
	// Empty counts the carriers answered with no value: null or an empty collection.
	Empty int `json:"empty"`
	// NotDerived counts the carriers the evaluator declined (ours) or the
	// pilot's payload left the property out of (pilot).
	NotDerived int `json:"notDerived"`
}

func (c *Counts) add(o Counts) {
	c.Carriers += o.Carriers
	c.Value += o.Value
	c.Empty += o.Empty
	c.NotDerived += o.NotDerived
}

// RuleCounts is one rule's row: ours and the pilot's answers.
type RuleCounts struct {
	Ours  Counts `json:"ours"`
	Pilot Counts `json:"pilot"`
}

// Silent reports whether the rule answers nothing on every element while the
// pilot answers something on at least one: the class the gate fails on.
func (r RuleCounts) Silent() bool {
	return r.Ours.Carriers > 0 && r.Ours.Value == 0 && r.Pilot.Value > 0
}

// RootCensus is our side of one corpus root.
type RootCensus struct {
	Files    int
	Rejected []string
	Elements int
	// Implied counts the relationships the semantic model synthesizes, which
	// the walk visits like the stated ones.
	Implied int
	Rules   map[string]Counts
}

// ruleName spells a rule as the baseline keys it.
func ruleName(rule metamodel.Rule) string { return rule.DefiningClass + "::" + rule.Property }

// RuleNames lists the served rules in baseline order.
func RuleNames() []string {
	names := make([]string, 0, len(metamodel.Rules()))
	for _, rule := range metamodel.Rules() {
		names = append(names, ruleName(rule))
	}
	sort.Strings(names)
	return names
}

// analysis is one root's model analyzed once: every file shares the index, as
// the pilot resolves a root's files together.
type analysis struct {
	res       *resolve.Resolver
	model     *semantics.Model
	structure *metamodel.SymbolStructure
	evaluator *metamodel.Evaluator
	docs      []string
}

// analyze parses and resolves the files (relative to dir) as one model and
// evaluates through the symbol-backed Structure, the one GetElementProperties reads.
func analyze(dir string, files []string) (*analysis, []string, error) {
	idx := libs.NewModelIndex()
	var rejected, docs []string
	type parsed struct {
		name string
		root *ast.RootNamespace
	}
	var roots []parsed
	for _, rel := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		data, err := os.ReadFile(path) // #nosec G304 -- a corpus file under the repository
		if err != nil {
			return nil, nil, err
		}
		sf := source.New(rel, data)
		p := parser.New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			rejected = append(rejected, rel)
			continue
		}
		idx.AddDocumentWithKind(rel, root, sf.Kind())
		roots = append(roots, parsed{rel, root})
		docs = append(docs, rel)
	}
	idx.ExpandWildcardImports()
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	for _, doc := range roots {
		res.ResolveDocument(doc.name, doc.root)
	}
	structure := metamodel.NewSymbolStructure(res, model)
	evaluator := metamodel.New(res, model, metamodel.Options{Structure: structure})
	return &analysis{res: res, model: model, structure: structure, evaluator: evaluator, docs: docs}, rejected, nil
}

// censusRoot measures our side of one root.
func censusRoot(dir string, files []string) (*RootCensus, error) {
	a, rejected, err := analyze(dir, files)
	if err != nil {
		return nil, err
	}
	out := &RootCensus{Files: len(files), Rejected: rejected, Rules: map[string]Counts{}}
	rules := metamodel.Rules()
	seen := map[*symbols.Symbol]bool{}
	var visit func(sym *symbols.Symbol)
	visit = func(sym *symbols.Symbol) {
		if sym == nil || seen[sym] {
			return
		}
		seen[sym] = true
		out.Elements++
		if sym.Implicit != nil {
			out.Implied++
		}
		a.countRules(out, rules, sym)
		for _, rel := range a.model.OwnedRelationships(sym) {
			visit(rel)
			if related, ok := a.model.ReflectiveElements(rel, "ownedRelatedElement"); ok {
				for _, owned := range related {
					visit(owned)
				}
			}
		}
	}
	for _, doc := range a.docs {
		scope := a.res.Index().DocumentRoot(doc)
		if scope == nil {
			return nil, fmt.Errorf("%s: no document root after analysis", doc)
		}
		for _, member := range scope.Members() {
			visit(member)
		}
	}
	return out, nil
}

// countRules adds sym's element to the counts of every rule its metaclass carries.
func (a *analysis) countRules(out *RootCensus, rules []metamodel.Rule, sym *symbols.Symbol) {
	element := a.structure.ElementFor(sym)
	class, known := a.structure.Metaclass(element)
	if !known {
		return
	}
	for _, rule := range rules {
		if !a.structure.Specializes(class, rule.DefiningClass) {
			continue
		}
		counts := out.Rules[ruleName(rule)]
		counts.Carriers++
		value, ok := a.evaluator.PropertyReadOnlyUncached(element, rule.DefiningClass, rule.Property)
		switch {
		case !ok:
			counts.NotDerived++
		case isEmpty(value):
			counts.Empty++
		default:
			counts.Value++
		}
		out.Rules[ruleName(rule)] = counts
	}
}

// isEmpty is an answer with nothing in it: null, or a collection of no values.
func isEmpty(value metamodel.Value) bool {
	switch value.Kind {
	case metamodel.NullValue, metamodel.InvalidValue:
		return true
	case metamodel.SequenceValue:
		return len(value.Values) == 0
	}
	return false
}
