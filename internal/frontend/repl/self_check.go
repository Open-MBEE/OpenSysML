package repl

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

type selfCheckConstraint struct {
	symbol    *symbols.Symbol
	paramType *symbols.Symbol
}

type selfCheckElement struct {
	symbol    *symbols.Symbol
	metaclass *symbols.Symbol
}

type selfCheckVisit struct {
	offset int
	member *symbols.Symbol
	child  *symbols.Scope
}

type selfCheckCounts struct {
	elements         int
	checks           int
	violations       int
	unevaluated      int
	evaluationErrors int
	applications     map[string]int
}

// SelfCheck applies the SysMLValidation constraints to the workspace model.
func (s *Session) SelfCheck() []Verdict {
	defer s.enter()()
	verdicts, _ := s.selfCheckWithCounts("SysMLValidation", false)
	return verdicts
}

func (s *Session) selfCheckWithCounts(pkg string, includeWorkspacePackages bool) ([]Verdict, selfCheckCounts) {
	counts := selfCheckCounts{applications: make(map[string]int)}
	idx := s.browseIndex()
	if idx == nil {
		return []Verdict{{Subject: "Self-model check", Status: VerdictHolds,
			Lines: []string{"Self-model check: 0 elements, 0 checks, 0 violations, 0 unevaluated"}}}, counts
	}

	resolver := resolve.New(idx)
	sem := passes.NewTypedModel(resolver)
	sem.SetSourceText(s.sessionSourceText())
	elements := selfCheckElements(idx, sem)
	counts.elements = len(elements)
	sources := selfCheckSources(s.sessionDocs())

	constraints, err := selfCheckConstraints(idx, sem, pkg, includeWorkspacePackages)
	var verdicts []Verdict
	if err != nil {
		counts.evaluationErrors++
		verdicts = append(verdicts, selfCheckErrorVerdict(pkg, "", err))
	}

	var ctx *runtime.Context
	canEvaluate := err == nil
	if canEvaluate && len(constraints) > 0 {
		ctx, err = s.getOrCreateRuntime()
		if err != nil {
			canEvaluate = false
			counts.evaluationErrors++
			verdicts = append(verdicts, selfCheckErrorVerdict(pkg, "", err))
		}
	}

	for _, element := range elements {
		if !canEvaluate {
			break
		}
		for _, constraint := range constraints {
			if constraint.paramType == nil ||
				(element.metaclass != constraint.paramType && !sem.Conforms(element.metaclass, constraint.paramType)) {
				continue
			}
			counts.checks++
			constraintName := selfCheckName(constraint.symbol)
			counts.applications[constraintName]++
			value := runtime.NewMetaobject(element.symbol, element.metaclass)
			holds, invokeErr := ctx.InvokePredicate(constraint.symbol, []runtime.Value{value}, constraint.symbol.OwnerScope)
			location := selfCheckLocation(element.symbol, sources)
			elementName := selfCheckName(element.symbol)
			subject := constraintName + " for " + elementName
			switch {
			case invokeErr == nil && holds:
			case invokeErr == nil:
				counts.violations++
				verdicts = append(verdicts, Verdict{
					Subject: subject,
					Status:  VerdictFails,
					Lines:   []string{fmt.Sprintf("%s: error: %s fails for %s", location, constraintName, elementName)},
				})
			case selfCheckUnevaluable(invokeErr):
				counts.unevaluated++
				v := unevaluableVerdict(subject, "Constraint "+constraintName, invokeErr, nil, "")
				v.Lines[0] = fmt.Sprintf("? %s: %s could not be evaluated", location, subject)
				verdicts = append(verdicts, v)
			default:
				counts.evaluationErrors++
				verdicts = append(verdicts, selfCheckErrorVerdict(subject, location, invokeErr))
			}
		}
	}

	summaryStatus := VerdictHolds
	if counts.violations > 0 || counts.evaluationErrors > 0 {
		summaryStatus = VerdictFails
	}
	summary := fmt.Sprintf("Self-model check: %d elements, %d checks, %d violations, %d unevaluated",
		counts.elements, counts.checks, counts.violations, counts.unevaluated)
	return append(verdicts, Verdict{Subject: "Self-model check", Status: summaryStatus, Lines: []string{summary}}), counts
}

func selfCheckConstraints(idx *symbols.Index, sem *semantics.Model, pkg string, includeWorkspacePackages bool) ([]selfCheckConstraint, error) {
	var packageSymbols []*symbols.Symbol
	for _, sym := range idx.LookupQualified(pkg) {
		if sym.Kind == symbols.SymbolPackage && sym.Scope != nil &&
			(includeWorkspacePackages || idx.IsLibraryDocument(sym.DocName)) {
			packageSymbols = append(packageSymbols, sym)
		}
	}
	if len(packageSymbols) == 0 {
		return nil, fmt.Errorf("validation package %s was not found", pkg)
	}
	var constraints []selfCheckConstraint
	for _, packageSymbol := range packageSymbols {
		for _, sym := range packageSymbol.Scope.AllMembers() {
			if sym.Kind != symbols.SymbolConstraintDef {
				continue
			}
			if sym.Scope == nil {
				return nil, fmt.Errorf("constraint %s has no declaration scope", selfCheckName(sym))
			}
			var paramType *symbols.Symbol
			for _, param := range sym.Scope.AllMembers() {
				usage, ok := param.Decl.(*ast.Usage)
				if !ok || usage.Direction != ast.DirIn {
					continue
				}
				if types := sem.FeatureTypeSet(param); len(types) > 0 {
					paramType = types[0]
				}
				break
			}
			if paramType == nil {
				return nil, fmt.Errorf("constraint %s has no typed input parameter", selfCheckName(sym))
			}
			constraints = append(constraints, selfCheckConstraint{symbol: sym, paramType: paramType})
		}
	}
	return constraints, nil
}

func selfCheckElements(idx *symbols.Index, sem *semantics.Model) []selfCheckElement {
	var out []selfCheckElement
	seenSymbols := make(map[*symbols.Symbol]bool)
	seenScopes := make(map[*symbols.Scope]bool)
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil || seenScopes[scope] {
			return
		}
		seenScopes[scope] = true
		members := scope.AllMembers()
		owned := make(map[*symbols.Scope]bool, len(members))
		visits := make([]selfCheckVisit, 0, len(members)+len(scope.Children()))
		for _, sym := range members {
			if sym.Scope != nil {
				owned[sym.Scope] = true
			}
			visits = append(visits, selfCheckVisit{offset: sym.DeclSpan.Offset, member: sym})
		}
		for _, child := range scope.Children() {
			if owned[child] {
				continue
			}
			offset := 0
			if child.Node() != nil {
				offset = child.Node().Span().Offset
			}
			visits = append(visits, selfCheckVisit{offset: offset, child: child})
		}
		sort.SliceStable(visits, func(i, j int) bool { return visits[i].offset < visits[j].offset })
		for _, visit := range visits {
			if visit.member != nil {
				sym := visit.member
				if !seenSymbols[sym] {
					seenSymbols[sym] = true
					if meta := sem.MetaclassOf(sym); meta != nil {
						out = append(out, selfCheckElement{symbol: sym, metaclass: meta})
					}
				}
				walk(sym.Scope)
				continue
			}
			walk(visit.child)
		}
	}
	for _, doc := range idx.WorkspaceDocuments() {
		walk(idx.DocumentRoot(doc))
	}
	return out
}

func selfCheckSources(docs []*model.Document) map[string]*source.SourceFile {
	out := make(map[string]*source.SourceFile, len(docs))
	for _, doc := range docs {
		out[doc.Name] = source.New(doc.Name, doc.Content)
	}
	return out
}

func selfCheckName(sym *symbols.Symbol) string {
	if fqn := symbols.FQNOf(sym); fqn != "" {
		return fqn
	}
	return "<anonymous>"
}

func selfCheckLocation(sym *symbols.Symbol, sources map[string]*source.SourceFile) string {
	if file := sources[sym.DocName]; file != nil {
		pos := file.Lines().PosAt(sym.DeclSpan.Offset)
		return fmt.Sprintf("%s:%d:%d", sym.DocName, pos.Line, pos.Col)
	}
	return sym.DocName
}

func selfCheckUnevaluable(err error) bool {
	return unevaluable(err) && (errors.Is(err, runtime.ErrReflectiveFeatureUnsupported) || errors.Is(err, runtime.ErrNoMetaclass))
}

func selfCheckErrorVerdict(subject, location string, err error) Verdict {
	prefix := "error: "
	if location != "" {
		prefix = location + ": error: "
	}
	return Verdict{Subject: subject, Status: VerdictUnresolved, Lines: []string{
		prefix + "could not be evaluated: " + err.Error(),
	}}
}
