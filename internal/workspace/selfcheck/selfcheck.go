// Package selfcheck applies the reflective SysMLValidation constraints, and
// those of any rule packages, to every element of a model: the REPL's
// -self-check and the service's SelfCheck share it.
package selfcheck

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// ErrPackageNotFound is wrapped by ResolvePackages when a rule package name
// resolves to no package.
var ErrPackageNotFound = errors.New("self-check package not found")

// Status is what one finding reports about a constraint applied to an element.
type Status int

const (
	// Fails is a constraint the element answered false.
	Fails Status = iota
	// Unevaluated is a constraint the runtime cannot yet apply reflectively.
	Unevaluated
	// Error is a constraint whose evaluation failed, or a package that could not be collected.
	Error
	// Warning is a rule constraint skipped, or a rule package yielding none.
	Warning
)

// String spells a status as the wire and the REPL name it.
func (s Status) String() string {
	switch s {
	case Fails:
		return "fails"
	case Unevaluated:
		return "unevaluated"
	case Error:
		return "error"
	}
	return "warning"
}

// Finding is one thing the check reports: a violation, an evaluation it could
// not make, or a constraint it skipped.
type Finding struct {
	Status     Status
	Subject    string // "<constraint> for <element>", or the constraint or package alone
	Location   string // file:line:col of the element, empty when it has none
	Constraint string // the constraint's qualified name, for a constraint applied
	Element    string // the element's name, for a constraint applied
	Message    string // a warning's reason
	Err        error  // why an evaluation failed or could not be made
}

// CodeRule is the registered diagnostic code of a failing model rule.
const CodeRule = "self-check-rule"

// Line spells the finding the way the REPL prints it.
func (f Finding) Line() string {
	switch f.Status {
	case Fails:
		return fmt.Sprintf("%s: error[%s]: %s fails for %s", f.Location, CodeRule, f.Constraint, f.Element)
	case Unevaluated:
		return fmt.Sprintf("? %s: %s could not be evaluated", f.Location, f.Subject)
	case Error:
		prefix := "error: "
		if f.Location != "" {
			prefix = f.Location + ": error: "
		}
		return prefix + "could not be evaluated: " + f.Err.Error()
	}
	if f.Location != "" {
		return f.Location + ": warning: " + f.Message
	}
	return "warning: " + f.Message
}

// Counts tallies what the check did.
type Counts struct {
	Elements         int
	Checks           int
	Violations       int
	Unevaluated      int
	EvaluationErrors int
	// Applications is how many elements each constraint was applied to, by name.
	Applications map[string]int
}

// Summary is the one-line total the REPL prints last.
func (c Counts) Summary() string {
	return fmt.Sprintf("Self-model check: %d elements, %d checks, %d violations, %d unevaluated",
		c.Elements, c.Checks, c.Violations, c.Unevaluated)
}

// Input is a model to check and how to check it.
type Input struct {
	// Index and Model are the model's symbols and typed semantics; a nil Index
	// has nothing to check.
	Index *symbols.Index
	Model *semantics.Model
	// Sources locates elements by document name, for the findings' locations.
	Sources map[string]*source.SourceFile
	// Runtime makes the context the constraints evaluate in, once, when any applies.
	Runtime func() (*runtime.Context, error)
	// Package is the bundled constraint package, SysMLValidation.
	Package string
	// IncludeWorkspacePackages also collects Package's constraints from the
	// workspace's own documents, not the libraries alone.
	IncludeWorkspacePackages bool
	// RulePackages are the resolved -self-check-package packages, whose
	// constraint defs apply after the bundled ones.
	RulePackages []RulePackage
}

// Report is what Run found and counted.
type Report struct {
	Findings []Finding
	Counts   Counts
}

// Sources maps a model's documents to source files, for Input.Sources.
func Sources(docs []*model.Document) map[string]*source.SourceFile {
	return selfCheckSources(docs)
}

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

func errorFinding(subject, location string, err error) Finding {
	return Finding{Status: Error, Subject: subject, Location: location, Err: err}
}

// Run applies the bundled and rule constraints to every element of the model.
func Run(in Input) Report {
	counts := Counts{Applications: make(map[string]int)}
	idx, sem := in.Index, in.Model
	if idx == nil {
		return Report{Counts: counts}
	}
	elements := selfCheckElements(idx, sem, in.RulePackages)
	counts.Elements = len(elements)

	constraints, err := selfCheckConstraints(idx, sem, in.Package, in.IncludeWorkspacePackages)
	var findings []Finding
	if err != nil {
		counts.EvaluationErrors++
		findings = append(findings, errorFinding(in.Package, "", err))
	}

	// The rule packages' constraints apply after the bundled ones; a constraint
	// whose input is not metaclass-typed, and a package yielding none, warns.
	ruleConstraints, warnings := selfCheckRuleConstraints(sem, in.RulePackages, constraints, in.Sources)
	findings = append(findings, warnings...)
	constraints = append(constraints, ruleConstraints...)

	var ctx *runtime.Context
	canEvaluate := err == nil
	if canEvaluate && len(constraints) > 0 {
		ctx, err = in.Runtime()
		if err != nil {
			canEvaluate = false
			counts.EvaluationErrors++
			findings = append(findings, errorFinding(in.Package, "", err))
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
			counts.Checks++
			constraintName := selfCheckName(constraint.symbol)
			counts.Applications[constraintName]++
			value := runtime.NewMetaobject(element.symbol, element.metaclass)
			holds, invokeErr := ctx.InvokePredicate(constraint.symbol, []runtime.Value{value}, constraint.symbol.OwnerScope)
			location := selfCheckLocation(element.symbol, in.Sources)
			elementName := selfCheckElementName(element)
			applied := Finding{Subject: constraintName + " for " + elementName, Location: location,
				Constraint: constraintName, Element: elementName, Err: invokeErr}
			switch {
			case invokeErr == nil && holds:
			case invokeErr == nil:
				counts.Violations++
				applied.Status = Fails
				findings = append(findings, applied)
			case selfCheckUnevaluable(invokeErr):
				counts.Unevaluated++
				applied.Status = Unevaluated
				findings = append(findings, applied)
			default:
				counts.EvaluationErrors++
				applied.Status = Error
				findings = append(findings, applied)
			}
		}
	}
	return Report{Findings: findings, Counts: counts}
}

// RulePackage is one distinct -self-check-package name resolved to
// every package symbol declaring it — a package may be re-opened across files.
type RulePackage struct {
	name    string
	symbols []*symbols.Symbol
}

// ResolvePackages resolves each distinct name to every package symbol
// it declares, in argument order. Quoted segments are read the way the notation
// reads them, and a name the notation cannot read counts as missing.
func ResolvePackages(idx *symbols.Index, pkgs []string) ([]RulePackage, error) {
	var groups []RulePackage
	seen := make(map[string]bool)
	var missing []string
	for _, name := range pkgs {
		segs, ok := source.QualifiedNameSegments(name)
		if !ok {
			missing = append(missing, name)
			continue
		}
		key := fmt.Sprintf("%q", segs)
		if seen[key] {
			continue
		}
		seen[key] = true
		group := RulePackage{name: name}
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
			strings.Join(missing, ", "), ErrPackageNotFound)
	}
	return groups, nil
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
func selfCheckRuleConstraints(sem *semantics.Model, groups []RulePackage, bundled []selfCheckConstraint,
	sources map[string]*source.SourceFile) ([]selfCheckConstraint, []Finding) {
	seen := make(map[symbols.ElementKey]bool, len(bundled))
	applicable := make(map[symbols.ElementKey]bool, len(bundled))
	for _, c := range bundled {
		seen[symbols.KeyOf(c.symbol)] = true
		applicable[symbols.KeyOf(c.symbol)] = true
	}
	var constraints []selfCheckConstraint
	var warnings []Finding
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
					warnings = append(warnings, Finding{Status: Warning, Subject: fqn, Location: selfCheckLocation(sym, sources), Message: fqn + " is skipped: it has no typed in parameter"})
				case meta == nil || !symbols.SameElement(meta, paramType):
					warnings = append(warnings, Finding{Status: Warning, Subject: fqn, Location: selfCheckLocation(sym, sources), Message: fqn + " is skipped: its first in parameter is not typed by a SysML or KerML metaclass"})
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
			warnings = append(warnings, Finding{Status: Warning, Subject: selfCheckName(group.symbols[0]),
				Message: fmt.Sprintf("self-check package %s has no applicable constraint definitions", selfCheckName(group.symbols[0]))})
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

func selfCheckElements(idx *symbols.Index, sem *semantics.Model, exclude []RulePackage) []selfCheckElement {
	w := &selfCheckWalker{sem: sem, seenSymbols: make(map[*symbols.Symbol]bool), seenScopes: make(map[*symbols.Scope]bool)}
	// A rule package's elements are checked by nothing, at any depth.
	for _, group := range exclude {
		for _, pkg := range group.symbols {
			w.seenSymbols[pkg] = true
			w.seenScopes[pkg.Scope] = true
		}
	}
	for _, doc := range idx.WorkspaceDocuments() {
		w.walk(idx.DocumentRoot(doc))
	}
	return w.out
}

// selfCheckWalker gathers the elements of the workspace's documents in source
// order, each once, with the metaclass each is checked as.
type selfCheckWalker struct {
	sem         *semantics.Model
	seenSymbols map[*symbols.Symbol]bool
	seenScopes  map[*symbols.Scope]bool
	out         []selfCheckElement
}

// walk visits a scope's members and unowned child scopes in source order.
func (w *selfCheckWalker) walk(scope *symbols.Scope) {
	if scope == nil || w.seenScopes[scope] {
		return
	}
	w.seenScopes[scope] = true
	for _, visit := range selfCheckVisits(scope) {
		if visit.member != nil {
			w.collect(visit.member)
			w.walk(visit.member.Scope)
			continue
		}
		w.walk(visit.child)
	}
}

// collect records a symbol not yet seen and the relationships implied for it.
func (w *selfCheckWalker) collect(sym *symbols.Symbol) {
	if w.seenSymbols[sym] {
		return
	}
	w.seenSymbols[sym] = true
	if meta := w.sem.MetaclassOf(sym); meta != nil {
		w.out = append(w.out, selfCheckElement{symbol: sym, metaclass: meta})
	}
	for _, rel := range w.sem.ImplicitRelationships(sym) {
		if meta := w.sem.MetaclassOf(rel); meta != nil {
			w.out = append(w.out, selfCheckElement{symbol: rel, metaclass: meta})
		}
	}
}

// selfCheckVisits orders a scope's members, and the child scopes no member
// owns, by their offset in the source.
func selfCheckVisits(scope *symbols.Scope) []selfCheckVisit {
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
	return visits
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

// selfCheckElementName names a reflected relationship object by its metaclass
// and owning element, since the object itself is anonymous.
func selfCheckElementName(element selfCheckElement) string {
	if element.symbol.Implicit == nil {
		return selfCheckName(element.symbol)
	}
	meta := "relationship"
	if element.metaclass != nil && element.metaclass.Name != "" {
		meta = element.metaclass.Name
	}
	return fmt.Sprintf("%s of '%s'", meta, selfCheckName(element.symbol.Implicit.Owner))
}

func selfCheckLocation(sym *symbols.Symbol, sources map[string]*source.SourceFile) string {
	if file := sources[sym.DocName]; file != nil {
		pos := file.Lines().PosAt(sym.DeclSpan.Offset)
		return fmt.Sprintf("%s:%d:%d", sym.DocName, pos.Line, pos.Col)
	}
	return sym.DocName
}

func selfCheckUnevaluable(err error) bool {
	var violation *runtime.ViolationError
	if err == nil || errors.As(err, &violation) || errors.Is(err, runtime.ErrViolated) {
		return false
	}
	return errors.Is(err, runtime.ErrReflectiveFeatureUnsupported) || errors.Is(err, runtime.ErrNoMetaclass)
}
