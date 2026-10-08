package view

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

var matrixKeywordOrder = []string{"satisfy", "verify", "allocate", "connect", "derive", "refine", "dependency"}

type matrixSelector struct {
	fqn         string
	keywords    []string
	specialized bool
}

var matrixSelectors = []matrixSelector{
	{fqn: "SysML::SatisfyRequirementUsage", keywords: []string{"satisfy"}},
	{fqn: "SysML::VerificationCaseUsage", keywords: []string{"verify"}},
	{fqn: "SysML::VerificationCaseDefinition", keywords: []string{"verify"}},
	{fqn: "SysML::AllocationUsage", keywords: []string{"allocate"}},
	{fqn: "SysML::ConnectionUsage", keywords: []string{"connect", "allocate", "derive"}},
	{fqn: "SysML::InterfaceUsage", keywords: []string{"connect"}},
	{fqn: "SysML::Dependency", keywords: []string{"dependency", "refine"}},
	{fqn: "ModelingMetadata::Refinement", keywords: []string{"refine"}, specialized: true},
	{fqn: "RequirementDerivation::DerivationMetadata", keywords: []string{"derive"}, specialized: true},
}

type matrixPair struct {
	source symbols.ElementKey
	target symbols.ElementKey
}

func allMatrixKinds() []string {
	return append([]string(nil), matrixKeywordOrder...)
}

func (r *Renderer) matrixShownKinds(view *symbols.Symbol) []string {
	shown := map[string]bool{}
	for _, condition := range r.model.ViewExposureConditions(view) {
		if condition.Expr == nil {
			continue
		}
		var negations []source.Span
		ast.Inspect(condition.Expr, func(node ast.Node) bool {
			if op, ok := node.(*ast.OperatorExpr); ok && op.Operator == ast.OpNot {
				negations = append(negations, op.Span())
			}
			return true
		})
		ast.Inspect(condition.Expr, func(node ast.Node) bool {
			op, ok := node.(*ast.OperatorExpr)
			if !ok || op.Operator != ast.OpAt || op.TypeRef == nil || inSpans(op.Span(), negations) {
				return true
			}
			var selector *symbols.Symbol
			var selectorOK bool
			r.resolver.InCondition(func() {
				selector, selectorOK = r.resolver.ResolveQualified(condition.Scope, op.TypeRef)
			})
			if !selectorOK || selector == nil {
				return true
			}
			for _, candidate := range matrixSelectors {
				if r.matchesMatrixSelector(selector, candidate) {
					for _, keyword := range candidate.keywords {
						shown[keyword] = true
					}
				}
			}
			return true
		})
	}
	var out []string
	for _, keyword := range matrixKeywordOrder {
		if shown[keyword] {
			out = append(out, keyword)
		}
	}
	return out
}

func inSpans(inner source.Span, outers []source.Span) bool {
	for _, outer := range outers {
		if inner.Offset >= outer.Offset && inner.Offset+inner.Len <= outer.Offset+outer.Len {
			return true
		}
	}
	return false
}

func (r *Renderer) matchesMatrixSelector(selector *symbols.Symbol, candidate matrixSelector) bool {
	for _, target := range r.resolver.Index().LookupQualified(candidate.fqn) {
		if symbols.SameElement(selector, target) || candidate.specialized && r.model.Conforms(selector, target) {
			return true
		}
	}
	return false
}

func (r *Renderer) renderMatrix(exposed []*symbols.Symbol, shown []string, pseudo bool, out *Rendering) {
	allowed := make(map[string]bool, len(shown))
	for _, keyword := range shown {
		allowed[keyword] = true
	}
	var elements []*symbols.Symbol
	owners := map[symbols.ElementKey]map[symbols.ElementKey]bool{}
	seen := map[symbols.ElementKey]bool{}
	for _, origin := range exposed {
		if origin == nil {
			continue
		}
		originKey := symbols.KeyOf(origin)
		visited := map[symbols.ElementKey]bool{}
		var addOwned func(*symbols.Symbol, int)
		addOwned = func(elem *symbols.Symbol, depth int) {
			if elem == nil || semantics.IsView(elem) || depth > r.treeDepth() {
				return
			}
			key := symbols.KeyOf(elem)
			if visited[key] {
				return
			}
			visited[key] = true
			if owners[key] == nil {
				owners[key] = map[symbols.ElementKey]bool{}
			}
			owners[key][originKey] = true
			if !seen[key] {
				seen[key] = true
				elements = append(elements, elem)
			}
			if depth >= r.treeDepth() {
				return
			}
			for _, member := range r.matrixContainedMembers(elem) {
				addOwned(member, depth+1)
			}
		}
		addOwned(origin, 0)
	}

	var edges []matrixEdge
	contributed := map[symbols.ElementKey]bool{}
	for _, elem := range elements {
		keyword, targets := r.matrixEdges(elem)
		if !allowed[keyword] {
			continue
		}
		for _, edge := range targets {
			if edge.Source == nil || edge.Target == nil {
				continue
			}
			edges = append(edges, matrixEdge{keyword: keyword, source: edge.Source, target: edge.Target})
			for owner := range owners[symbols.KeyOf(elem)] {
				contributed[owner] = true
			}
		}
	}
	if len(edges) == 0 {
		if !pseudo && len(exposed) != 0 {
			out.Notices = append(out.Notices, matrixOmittedNotice(exposed, shown, r))
		} else if pseudo && len(exposed) != 0 {
			out.emptyReason = "no relationship edges are exposed; the rendering is empty"
		}
		return
	}

	var rowSources, columnTargets []*symbols.Symbol
	rowIndexes := map[symbols.ElementKey]int{}
	columnIndexes := map[symbols.ElementKey]int{}
	cells := map[matrixPair]map[string]bool{}
	for _, edge := range edges {
		sourceKey, targetKey := symbols.KeyOf(edge.source), symbols.KeyOf(edge.target)
		if _, ok := rowIndexes[sourceKey]; !ok {
			rowIndexes[sourceKey] = len(rowSources)
			rowSources = append(rowSources, edge.source)
		}
		if _, ok := columnIndexes[targetKey]; !ok {
			columnIndexes[targetKey] = len(columnTargets)
			columnTargets = append(columnTargets, edge.target)
		}
		pair := matrixPair{source: sourceKey, target: targetKey}
		if cells[pair] == nil {
			cells[pair] = map[string]bool{}
		}
		cells[pair][edge.keyword] = true
	}

	out.Columns = make([]string, 1, len(columnTargets)+1)
	out.Columns[0] = "Source / Target"
	for _, target := range columnTargets {
		out.Columns = append(out.Columns, r.matrixLabel(target))
	}
	for _, source := range rowSources {
		row := make([]string, len(out.Columns))
		row[0] = r.matrixLabel(source)
		for _, target := range columnTargets {
			var keywords []string
			for _, keyword := range matrixKeywordOrder {
				if cells[matrixPair{source: symbols.KeyOf(source), target: symbols.KeyOf(target)}][keyword] {
					keywords = append(keywords, keyword)
				}
			}
			row[columnIndexes[symbols.KeyOf(target)]+1] = strings.Join(keywords, ", ")
		}
		out.appendRow(row, symbolOrigin(source))
	}
	if !pseudo {
		var omitted []*symbols.Symbol
		for _, elem := range exposed {
			if !contributed[symbols.KeyOf(elem)] {
				omitted = append(omitted, elem)
			}
		}
		if len(omitted) != 0 {
			out.Notices = append(out.Notices, matrixOmittedNotice(omitted, shown, r))
		}
	}
}

type matrixEdge struct {
	keyword string
	source  *symbols.Symbol
	target  *symbols.Symbol
}

func (r *Renderer) matrixContainedMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	return sym.Scope.AllMembers()
}

func (r *Renderer) matrixEdges(sym *symbols.Symbol) (string, []semantics.RelationshipEdge) {
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		switch usage.Kind {
		case ast.UsageSatisfy:
			if usage.Keyword == "verify" {
				return "verify", r.model.RelationshipEdgesOf(sym, semantics.RelationshipVerification)
			}
			return "satisfy", r.model.RelationshipEdgesOf(sym, semantics.RelationshipSatisfaction)
		case ast.UsageAllocation:
			return "allocate", r.model.RelationshipEdgesOf(sym, semantics.RelationshipAllocation)
		case ast.UsageConnection, ast.UsageConnector, ast.UsageInterface:
			if edges := r.model.RelationshipEdgesOf(sym, semantics.RelationshipDerivation); len(edges) != 0 {
				return "derive", edges
			}
			if edges := r.model.RelationshipEdgesOf(sym, semantics.RelationshipAllocation); len(edges) != 0 {
				return "allocate", edges
			}
			return "connect", r.model.RelationshipEdgesOf(sym, semantics.RelationshipConnection)
		}
	}
	if _, ok := sym.Decl.(*ast.Dependency); ok {
		if edges := r.model.RelationshipEdgesOf(sym, semantics.RelationshipRefinement); len(edges) != 0 {
			return "refine", edges
		}
		return "dependency", r.model.RelationshipEdgesOf(sym, semantics.RelationshipDependency)
	}
	if definition, ok := sym.Decl.(*ast.Definition); ok && definition.Kind == ast.DefConnection {
		if edges := r.model.RelationshipEdgesOf(sym, semantics.RelationshipDerivation); len(edges) != 0 {
			return "derive", edges
		}
		return "", nil
	}
	return "", nil
}

func (r *Renderer) matrixLabel(sym *symbols.Symbol) string { return r.notationName(sym) }

func matrixOmittedNotice(elements []*symbols.Symbol, shown []string, r *Renderer) string {
	kindList := strings.Join(shown, ", ")
	names := make([]string, len(elements))
	for i, elem := range elements {
		names[i] = r.notationName(elem)
	}
	count := len(elements)
	elementWord, verb, agreement := "elements", "state", "are"
	if count == 1 {
		elementWord, verb, agreement = "element", "states", "is"
	}
	return fmt.Sprintf("%d exposed %s %s no %s relationship and %s left out of the matrix: %s",
		count, elementWord, verb, kindList, agreement, strings.Join(names, ", "))
}
