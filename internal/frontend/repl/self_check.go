package repl

import (
	"errors"
	"fmt"
	"sort"
	"strings"

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

// ErrSelfCheckPackageNotFound marks an error naming a -self-check-package that
// no loaded model or library declares.
var ErrSelfCheckPackageNotFound = errors.New("self-check package not found")

// SelfCheck applies the SysMLValidation constraints to the workspace model.
func (s *Session) SelfCheck() []Verdict {
	defer s.enter()()
	verdicts, _ := s.selfCheckWithCounts("SysMLValidation", false)
	return verdicts
}

// ResolveSelfCheckPackages reports the -self-check-package names no loaded model
// or library declares a package for, or nil when every name resolves.
func (s *Session) ResolveSelfCheckPackages(pkgs []string) error {
	defer s.enter()()
	_, err := s.resolveSelfCheckPackages(pkgs)
	return err
}

// SelfCheckPackages applies the SysMLValidation constraints together with the
// constraint defs of each named rule package. A name nothing declares is an
// error wrapping ErrSelfCheckPackageNotFound and nothing is evaluated.
func (s *Session) SelfCheckPackages(pkgs []string) ([]Verdict, error) {
	defer s.enter()()
	verdicts, _, err := s.selfCheckRun("SysMLValidation", false, pkgs)
	return verdicts, err
}

func (s *Session) selfCheckWithCounts(pkg string, includeWorkspacePackages bool) ([]Verdict, selfCheckCounts) {
	verdicts, counts, _ := s.selfCheckRun(pkg, includeWorkspacePackages, nil)
	return verdicts, counts
}

// selfCheckRulePackage is one distinct -self-check-package name resolved to
// every package symbol declaring it — a package may be re-opened across files.
type selfCheckRulePackage struct {
	name    string
	symbols []*symbols.Symbol
}

// resolveSelfCheckPackages resolves each distinct name to every package symbol
// it declares, in argument order. Quoted segments are read the way the notation
// reads them, and a name the notation cannot read counts as missing.
func (s *Session) resolveSelfCheckPackages(pkgs []string) ([]selfCheckRulePackage, error) {
	idx := s.browseIndex()
	var groups []selfCheckRulePackage
	seen := make(map[string]bool)
	var missing []string
	for _, name := range pkgs {
		segs, ok := nameSegments(name)
		if !ok {
			missing = append(missing, name)
			continue
		}
		key := strings.Join(segs, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		group := selfCheckRulePackage{name: name}
		if idx != nil {
			for _, sym := range idx.LookupQualified(strings.Join(segs, "::")) {
				if sym.Kind == symbols.SymbolPackage && sym.Scope != nil {
					group.symbols = append(group.symbols, sym)
				}
			}
		}
		if len(group.symbols) == 0 {
			missing = append(missing, name)
			continue
		}
		groups = append(groups, group)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("-self-check-package %s names no package the loaded models or libraries declare: %w",
			strings.Join(missing, ", "), ErrSelfCheckPackageNotFound)
	}
	return groups, nil
}

func (s *Session) selfCheckRun(pkg string, includeWorkspacePackages bool, rulePackages []string) ([]Verdict, selfCheckCounts, error) {
	counts := selfCheckCounts{applications: make(map[string]int)}
	idx := s.browseIndex()
	rulePackageGroups, err := s.resolveSelfCheckPackages(rulePackages)
	if err != nil {
		return nil, counts, err
	}
	if idx == nil {
		return []Verdict{{Subject: "Self-model check", Status: VerdictHolds,
			Lines: []string{"Self-model check: 0 elements, 0 checks, 0 violations, 0 unevaluated"}}}, counts, nil
	}

	resolver := resolve.New(idx)
	sem := passes.NewTypedModel(resolver)
	sem.SetSourceText(s.sessionSourceText())
	elements := selfCheckElements(idx, sem, rulePackageGroups)
	counts.elements = len(elements)
	sources := selfCheckSources(s.sessionDocs())

	constraints, err := selfCheckConstraints(idx, sem, pkg, includeWorkspacePackages)
	var verdicts []Verdict
	if err != nil {
		counts.evaluationErrors++
		verdicts = append(verdicts, selfCheckErrorVerdict(pkg, "", err))
	}

	// The rule packages' constraints apply after the bundled ones; a constraint
	// whose input is not metaclass-typed, and a package yielding none, warns.
	ruleConstraints, warnings := selfCheckRuleConstraints(sem, rulePackageGroups, constraints, sources)
	verdicts = append(verdicts, warnings...)
	constraints = append(constraints, ruleConstraints...)

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
	if counts.violations > 0 {
		summaryStatus = VerdictFails
	} else if counts.evaluationErrors > 0 {
		summaryStatus = VerdictUnresolved
	}
	summary := fmt.Sprintf("Self-model check: %d elements, %d checks, %d violations, %d unevaluated",
		counts.elements, counts.checks, counts.violations, counts.unevaluated)
	return append(verdicts, Verdict{Subject: "Self-model check", Status: summaryStatus, Lines: []string{summary}}), counts, nil
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
			paramType, ok := selfCheckParamType(sem, sym)
			if !ok {
				return nil, fmt.Errorf("constraint %s has no typed input parameter", selfCheckName(sym))
			}
			constraints = append(constraints, selfCheckConstraint{symbol: sym, paramType: paramType})
		}
	}
	return constraints, nil
}

// selfCheckRuleConstraints collects the constraint defs the rule packages
// declare, recursing into nested packages. skipped are the warning verdicts for
// constraints whose first in parameter cannot type an element and for named
// packages yielding no applicable constraint; both come before the verdicts.
func selfCheckRuleConstraints(sem *semantics.Model, groups []selfCheckRulePackage, bundled []selfCheckConstraint,
	sources map[string]*source.SourceFile) ([]selfCheckConstraint, []Verdict) {
	seen := make(map[symbols.ElementKey]bool, len(bundled))
	applicable := make(map[symbols.ElementKey]bool, len(bundled))
	for _, c := range bundled {
		seen[symbols.KeyOf(c.symbol)] = true
		applicable[symbols.KeyOf(c.symbol)] = true
	}
	var constraints []selfCheckConstraint
	var warnings []Verdict
	var walk func(*symbols.Scope, *bool)
	walk = func(scope *symbols.Scope, found *bool) {
		for _, sym := range scope.AllMembers() {
			switch {
			case sym.Kind == symbols.SymbolPackage && sym.Scope != nil:
				walk(sym.Scope, found)
			case sym.Kind == symbols.SymbolConstraintDef:
				key := symbols.KeyOf(sym)
				if seen[key] {
					if applicable[key] {
						*found = true
					}
					continue
				}
				seen[key] = true
				fqn := selfCheckName(sym)
				paramType, typed := selfCheckParamType(sem, sym)
				var meta *symbols.Symbol
				if typed {
					meta = sem.Metaclass(paramType.Name)
				}
				switch {
				case !typed:
					warnings = append(warnings, selfCheckSkipVerdict(fqn, selfCheckLocation(sym, sources),
						fqn+" is skipped: it has no typed in parameter"))
				case meta == nil || !symbols.SameElement(meta, paramType):
					warnings = append(warnings, selfCheckSkipVerdict(fqn, selfCheckLocation(sym, sources),
						fqn+" is skipped: its first in parameter is not typed by a SysML or KerML metaclass"))
				default:
					applicable[key] = true
					*found = true
					constraints = append(constraints, selfCheckConstraint{symbol: sym, paramType: paramType})
				}
			}
		}
	}
	for _, group := range groups {
		found := false
		for _, pkg := range group.symbols {
			walk(pkg.Scope, &found)
		}
		if !found {
			warnings = append(warnings, Verdict{
				Subject: selfCheckName(group.symbols[0]),
				Status:  VerdictUnresolved,
				Lines:   []string{fmt.Sprintf("warning: self-check package %s has no applicable constraint definitions", selfCheckName(group.symbols[0]))},
			})
		}
	}
	return constraints, warnings
}

// selfCheckParamType is the type of sym's first in parameter, as the bundled
// collector reads it; typed is false where no in parameter declares a type.
func selfCheckParamType(sem *semantics.Model, sym *symbols.Symbol) (*symbols.Symbol, bool) {
	if sym.Scope != nil {
		for _, param := range sym.Scope.AllMembers() {
			usage, ok := param.Decl.(*ast.Usage)
			if !ok || usage.Direction != ast.DirIn {
				continue
			}
			if types := sem.FeatureTypeSet(param); len(types) > 0 {
				return types[0], true
			}
			return nil, false
		}
	}
	return nil, false
}

func selfCheckSkipVerdict(subject, location, message string) Verdict {
	return Verdict{Subject: subject, Status: VerdictUnresolved, Lines: []string{
		fmt.Sprintf("%s: warning: %s", location, message),
	}}
}

func selfCheckElements(idx *symbols.Index, sem *semantics.Model, exclude []selfCheckRulePackage) []selfCheckElement {
	var out []selfCheckElement
	seenSymbols := make(map[*symbols.Symbol]bool)
	seenScopes := make(map[*symbols.Scope]bool)
	// A rule package's elements are checked by nothing, at any depth.
	for _, group := range exclude {
		for _, pkg := range group.symbols {
			seenSymbols[pkg] = true
			seenScopes[pkg.Scope] = true
		}
	}
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
