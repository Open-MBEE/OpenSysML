package metaclassmap

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/tools/census/grammar"
)

const relationshipFQN = "KerML::Root::Relationship"

type reflectiveModel struct {
	index *symbols.Index
	sem   *semantics.Model
}

// Analyze builds one workspace from the evidence documents and checks each
// shape against its coverage row and the OpenSysML reflective model.
func Analyze(coverage *grammar.Report, shapes []grammar.Shape, files map[string][]byte) (*Report, error) {
	if coverage == nil {
		return nil, fmt.Errorf("grammar-coverage report is nil")
	}
	ws := model.NewWorkspace()
	documents := evidenceDocuments(coverage)
	for _, doc := range documents {
		content, ok := files[doc]
		if !ok {
			return nil, fmt.Errorf("evidence file %s is not available", doc)
		}
		ws.Open(filepath.ToSlash(doc), content, 1)
	}
	reflection := &reflectiveModel{}
	queryDoc := ""
	if len(documents) > 0 {
		queryDoc = documents[0]
	}
	err := ws.Read(func(reading *model.Reading) error {
		reflection.index = reading.Index()
		reading.Query(queryDoc, func(_ *resolve.Resolver, sem *semantics.Model) {
			reflection.sem = sem
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return analyze(coverage, shapes, files, reflection)
}

func analyze(coverage *grammar.Report, shapes []grammar.Shape, files map[string][]byte,
	reflection *reflectiveModel,
) (*Report, error) {
	out := &Report{PilotTag: coverage.PilotTag, Totals: newTotals()}
	shapeByKey := make(map[string]grammar.Shape, len(shapes))
	for _, shape := range shapes {
		shapeByKey[shapeKey(shape.Grammar, shape.Name)] = shape
	}
	allShapes := shapes
	for _, grammarRows := range coverage.Grammars {
		group := GrammarReport{Name: grammarRows.Name, Totals: newTotals()}
		for _, row := range grammarRows.Productions {
			shape, ok := shapeByKey[shapeKey(row.Grammar, row.Name)]
			if !ok {
				return nil, fmt.Errorf("shape not found for %s:%s", row.Grammar, row.Name)
			}
			production := Production{
				Shape:    shape,
				Coverage: row,
				Differential: Differential{
					Creates: append([]string(nil), shape.Creates...),
				},
			}
			production.Types = typeChecks(shape, reflection.sem)
			production.Assignments = assignmentChecks(shape, reflection)
			production.Differential = differential(row, shape, allShapes, files, reflection)
			group.Productions = append(group.Productions, production)
			group.Totals.add(production)
			out.Totals.add(production)
		}
		out.Grammars = append(out.Grammars, group)
	}
	return out, nil
}

func evidenceDocuments(coverage *grammar.Report) []string {
	seen := map[string]bool{}
	var out []string
	for _, g := range coverage.Grammars {
		for _, row := range g.Productions {
			for _, citation := range row.Evidence {
				if citation.File != "" && !seen[citation.File] {
					seen[citation.File] = true
					out = append(out, filepath.ToSlash(citation.File))
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func shapeKey(grammarName, production string) string {
	return grammarName + "\x00" + production
}

func typeChecks(shape grammar.Shape, sem *semantics.Model) []TypeCheck {
	var out []TypeCheck
	add := func(context, name string) {
		if name == "" {
			return
		}
		check := TypeCheck{Context: context, Name: name}
		if strings.HasPrefix(name, "Ecore::") {
			check.Status, check.FQN = "ecore", name
		} else {
			simple := name
			for _, prefix := range []string{"SysML::", "KerML::"} {
				simple = strings.TrimPrefix(simple, prefix)
			}
			if meta := sem.Metaclass(simple); meta != nil {
				check.FQN = symbols.FQNOf(meta)
				switch {
				case strings.HasPrefix(check.FQN, "SysML::"):
					check.Status = "sysml"
				case strings.HasPrefix(check.FQN, "KerML::"):
					check.Status = "kerml"
				default:
					check.Status = "missing"
				}
			} else {
				check.Status = "missing"
			}
		}
		out = append(out, check)
	}
	add("returns", shape.Returns)
	for _, action := range shape.Actions {
		add("action", action.Metaclass)
	}
	for _, assignment := range shape.Assignments {
		for _, crossRef := range assignment.CrossRef {
			add("cross-reference", crossRef)
		}
		for _, owner := range assignment.Owners {
			add("assignment-owner", owner)
		}
	}
	return out
}

func assignmentChecks(shape grammar.Shape, reflection *reflectiveModel) []AssignmentCheck {
	var out []AssignmentCheck
	for _, assignment := range shape.Assignments {
		if shape.Kind == grammar.KindEnum {
			found := enumHasLiteral(reflection, shape.Returns, assignment.Feature)
			status := "missing"
			if found {
				status = "found"
			}
			out = append(out, AssignmentCheck{
				Feature: assignment.Feature, Op: assignment.Op, Value: assignment.Value,
				Owner: shape.Returns, Status: status, Enum: true,
			})
			continue
		}
		for _, ownerName := range assignment.Owners {
			owner := lookupMetaclass(reflection.sem, ownerName)
			if owner == nil {
				out = append(out, AssignmentCheck{
					Feature: assignment.Feature, Op: assignment.Op, Value: assignment.Value,
					Owner: ownerName, Status: "owner-missing",
				})
				continue
			}
			declaredBy := featureDeclaredBy(reflection.sem, owner, assignment.Feature)
			status := "missing"
			if declaredBy != "" {
				status = "found"
			}
			out = append(out, AssignmentCheck{
				Feature: assignment.Feature, Op: assignment.Op, Value: assignment.Value,
				Owner: ownerName, DeclaredBy: declaredBy, Status: status,
			})
		}
	}
	return out
}

func lookupMetaclass(sem *semantics.Model, qualified string) *symbols.Symbol {
	if sem == nil || qualified == "" || strings.HasPrefix(qualified, "Ecore::") {
		return nil
	}
	simple := strings.TrimPrefix(strings.TrimPrefix(qualified, "SysML::"), "KerML::")
	return sem.Metaclass(simple)
}

func featureDeclaredBy(sem *semantics.Model, owner *symbols.Symbol, feature string) string {
	candidates := append([]*symbols.Symbol{owner}, sem.AllSupertypes(owner)...)
	for _, candidate := range candidates {
		member, ok := sem.LookupMember(candidate, feature)
		if !ok || member.OwnerScope == nil || member.OwnerScope.Owner() == nil {
			continue
		}
		declaring := member.OwnerScope.Owner()
		fqn := symbols.FQNOf(declaring)
		if fqn != symbols.FQNOf(candidate) {
			continue
		}
		if strings.HasPrefix(fqn, "SysML::") || strings.HasPrefix(fqn, "KerML::") {
			return fqn
		}
	}
	return ""
}

func enumHasLiteral(reflection *reflectiveModel, enumName, literal string) bool {
	if reflection == nil || reflection.index == nil || reflection.sem == nil {
		return false
	}
	matches := reflection.index.LookupQualified(enumName)
	for _, enum := range matches {
		if _, ok := reflection.sem.LookupMember(enum, literal); ok {
			return true
		}
		if enum.Scope != nil {
			if _, ok := enum.Scope.LookupLocal(literal); ok {
				return true
			}
		}
	}
	return false
}

func differential(row grammar.Row, shape grammar.Shape, allShapes []grammar.Shape, files map[string][]byte,
	reflection *reflectiveModel,
) Differential {
	result := Differential{Creates: append([]string(nil), shape.Creates...)}
	undecided := func(reason string) Differential {
		result.Bucket, result.Reason = "undecided", reason
		return result
	}
	if row.Bucket != grammar.BucketEvidence {
		return undecided("no-input")
	}
	if len(shape.Creates) == 0 {
		return undecided("no-element")
	}
	anchor := ""
	for _, literal := range shape.Anchors {
		for _, required := range row.Required {
			if literal == required {
				anchor = literal
				break
			}
		}
		if anchor != "" {
			break
		}
	}
	if anchor == "" {
		return undecided("no-anchor")
	}
	var citation *grammar.Citation
	for i := range row.Evidence {
		if row.Evidence[i].Literal == anchor {
			citation = &row.Evidence[i]
			break
		}
	}
	if citation == nil {
		return undecided("unlocated")
	}
	content, ok := files[citation.File]
	if !ok {
		return undecided("unlocated")
	}
	offset, ok := grammar.LocateLiteral(string(content), citation.Line, anchor)
	if !ok {
		return undecided("unlocated")
	}
	lineStart := strings.LastIndex(string(content[:offset]), "\n") + 1
	result.Location = &Location{
		File: citation.File, Line: citation.Line, Column: offset - lineStart + 1,
	}
	sym := elementAt(reflection.index, citation.File, content, offset)
	if sym == nil {
		return undecided("no-element-at-anchor")
	}
	result.SymbolFQN = symbols.FQNOf(sym)
	result.Symbol = result.SymbolFQN
	if result.Symbol == "" {
		result.Symbol = "<anonymous>"
		result.SymbolFQN = "<anonymous>"
	}
	meta := reflection.sem.MetaclassOf(sym)
	if meta == nil {
		return undecided("no-reflective-metaclass")
	}
	result.Metaclass = symbols.FQNOf(meta)
	for _, created := range shape.Creates {
		if created == meta.Name || lastName(created) == meta.Name {
			result.Bucket = "agree"
			return result
		}
	}
	if createsConformToRelationship(reflection, shape.Creates) &&
		!conformsToRelationship(reflection.sem, meta) {
		return undecided("relationship-not-reified")
	}
	var shared []string
	for _, other := range allShapes {
		if shapeKey(other.Grammar, other.Name) == shapeKey(shape.Grammar, shape.Name) ||
			!containsString(other.Anchors, anchor) {
			continue
		}
		for _, created := range other.Creates {
			if created == meta.Name || lastName(created) == meta.Name {
				shared = appendUnique(shared, other.Name)
				break
			}
		}
	}
	if len(shared) > 0 {
		sort.Strings(shared)
		result.Bucket, result.Reason, result.SharedWith = "undecided", "anchor-shared", shared
		return result
	}
	result.Bucket = "disagree"
	return result
}

func createsConformToRelationship(reflection *reflectiveModel, creates []string) bool {
	if reflection == nil || reflection.sem == nil || len(creates) == 0 {
		return false
	}
	for _, created := range creates {
		var candidates []*symbols.Symbol
		if reflection.index != nil {
			candidates = reflection.index.LookupQualified(created)
		}
		if len(candidates) == 0 {
			candidates = []*symbols.Symbol{reflection.sem.Metaclass(lastName(created))}
		}
		if len(candidates) == 0 || candidates[0] == nil {
			return false
		}
		for _, candidate := range candidates {
			if !conformsToRelationship(reflection.sem, candidate) {
				return false
			}
		}
	}
	return true
}

func conformsToRelationship(sem *semantics.Model, metaclass *symbols.Symbol) bool {
	if sem == nil || metaclass == nil {
		return false
	}
	if symbols.FQNOf(metaclass) == relationshipFQN {
		return true
	}
	for _, supertype := range sem.AllSupertypes(metaclass) {
		if symbols.FQNOf(supertype) == relationshipFQN {
			return true
		}
	}
	return false
}

func elementAt(index *symbols.Index, doc string, content []byte, offset int) *symbols.Symbol {
	if index == nil {
		return nil
	}
	root := index.DocumentRoot(doc)
	if root == nil {
		return nil
	}
	stripped := stripSource(string(content))
	var symbolsInDocument []*symbols.Symbol
	seenSymbols := map[*symbols.Symbol]bool{}
	seenScopes := map[*symbols.Scope]bool{}
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil || seenScopes[scope] {
			return
		}
		seenScopes[scope] = true
		for _, sym := range scope.AllMembers() {
			if sym.DocName != doc || seenSymbols[sym] {
				continue
			}
			seenSymbols[sym] = true
			symbolsInDocument = append(symbolsInDocument, sym)
			walk(sym.Scope)
		}
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	walk(root)
	var best *symbols.Symbol
	for _, sym := range symbolsInDocument {
		start, end, ok := declarationHeader(sym, stripped)
		if !ok || offset < start || offset >= end {
			continue
		}
		if best == nil || sym.DeclSpan.Offset > best.DeclSpan.Offset ||
			(sym.DeclSpan.Offset == best.DeclSpan.Offset && symbols.FQNOf(sym) < symbols.FQNOf(best)) {
			best = sym
		}
	}
	return best
}

func declarationHeader(sym *symbols.Symbol, stripped string) (int, int, bool) {
	start := sym.DeclSpan.Offset
	if start < 0 || start > len(stripped) {
		return 0, 0, false
	}
	end := sym.DeclSpan.End()
	if end < start {
		return 0, 0, false
	}
	if end > len(stripped) {
		end = len(stripped)
	}
	if sym.NameSpan.Len > 0 && sym.NameSpan.Offset >= start && sym.NameSpan.Offset <= end {
		return start, sym.NameSpan.Offset, true
	}
	for i := start; i < end; i++ {
		if !source.IsIdentCont(stripped[i]) && !isHeaderWhitespace(stripped[i]) {
			end = i
			break
		}
	}
	return start, end, true
}

func isHeaderWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\r' || char == '\n'
}

func stripSource(content string) string {
	return grammar.StripModelSource(content)
}

func lastName(qualified string) string {
	if at := strings.LastIndex(qualified, "::"); at >= 0 {
		return qualified[at+2:]
	}
	return qualified
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
