package record

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Placement is where a case's records go: the package, the case's declared
// name, and what the package already holds of the shape Generate must fit.
type Placement struct {
	Package  string
	CaseName string
	Existing Existing
}

// Place finds the package the records of the case fqn names go into — into
// when named, else Records beside the package enclosing the case's own — and
// what it already holds.
func Place(idx *symbols.Index, fqn, into string) (Placement, error) {
	pkg, err := targetPackage(idx, fqn, into)
	if err != nil {
		return Placement{}, err
	}
	caseSym := caseSymbol(idx, fqn)
	existing, err := existingIn(idx, caseSym, fqn, pkg)
	if err != nil {
		return Placement{}, err
	}
	caseName := fqn
	if caseSym != nil {
		caseName = caseSym.Name
	}
	return Placement{Package: pkg, CaseName: caseName, Existing: existing}, nil
}

// SubjectOf names the object a run was made on: its usage when one usage
// alone declares it, else its text in the context it lives in, else label.
func SubjectOf(idx *symbols.Index, inst *runtime.Instance, label string, contexts map[*runtime.Context]bool, text func(*runtime.Context, runtime.Value) string) Subject {
	if inst == nil {
		return Subject{}
	}
	for ctx := range contexts {
		if _, ok := ctx.Instance(inst.ID); ok {
			if usage := ctx.OccurrenceUsage(inst); usage != "" && len(idx.LookupQualified(usage)) == 1 {
				return Subject{Usage: usage, Text: usage}
			}
			return Subject{Text: text(ctx, runtime.Value{Kind: runtime.ValInstance, Instance: inst.ID})}
		}
	}
	return Subject{Text: label}
}

// recordRuns generates the record declarations for runs of the case fqn names,
// submits them, and returns what was generated.

// targetPackage is the package a case's records go into: the one into names,
// or Records beside the package enclosing the case's own package.
func targetPackage(idx *symbols.Index, fqn, into string) (string, error) {
	if into != "" {
		segs, ok := source.QualifiedNameSegments(into)
		if !ok {
			return "", fmt.Errorf("%q does not name a package", into)
		}
		for i := 1; i <= len(segs); i++ {
			prefix := strings.Join(segs[:i], "::")
			for _, sym := range idx.LookupQualified(prefix) {
				if sym.Kind != symbols.SymbolPackage {
					return "", fmt.Errorf("%s names a %s, not a package", prefix, sym.Kind)
				}
			}
		}
		return into, nil
	}
	for _, sym := range idx.LookupQualified(fqn) {
		pkg := enclosingPackage(sym)
		if pkg == nil {
			continue
		}
		parent := enclosingPackage(pkg.Owner())
		if parent == nil {
			return "Records", nil
		}
		return idx.GetFQN(parent) + "::Records", nil
	}
	return "Records", nil
}

// enclosingPackage walks a symbol's owners to the nearest package, nil when
// there is none.
func enclosingPackage(sym *symbols.Symbol) *symbols.Symbol {
	for s := sym; s != nil; s = s.Owner() {
		if s.Kind == symbols.SymbolPackage {
			return s
		}
	}
	return nil
}

// caseSymbol is the analysis case symbol fqn names, nil when the index
// names none or several.
func caseSymbol(idx *symbols.Index, fqn string) *symbols.Symbol {
	syms := idx.LookupQualified(fqn)
	if len(syms) == 1 {
		return syms[0]
	}
	return nil
}

// existingIn is what the target package already declares of the shape
// Generate must fit: the package itself, the case's record definition and the
// record numbers already taken. The stem the records are named from is the
// case's name, owner-prefixed when a definition of the same name belongs to a
// sibling case.
func existingIn(idx *symbols.Index, caseSym *symbols.Symbol, fqn, pkg string) (Existing, error) {
	var existing Existing
	if len(idx.LookupQualified(pkg)) > 0 {
		existing.Package = true
	}
	short := ShortName(fqn)
	if caseSym != nil {
		short = caseSym.Name
	}
	stems := recordStems(caseSym, short)
	if err := recordDefinition(idx, &existing, stems, fqn, pkg); err != nil {
		return existing, err
	}
	stem := existing.Stem
	if stem == "" {
		stem = short
	}
	existing.Taken = takenRecordNumbers(idx, pkg, stem)
	return existing, nil
}

// recordStems lists the stems a case's records may be named from: the case's
// name, then each owner up the chain prefixed.
func recordStems(caseSym *symbols.Symbol, short string) []string {
	stems := []string{short}
	if caseSym == nil {
		return stems
	}
	var owners []string
	for cur := caseSym.Owner(); cur != nil && cur.Name != ""; cur = cur.Owner() {
		owners = append([]string{cur.Name}, owners...)
		stems = append(stems, strings.Join(append(append([]string{}, owners...), short), "_"))
	}
	return stems
}

// recordDefinition settles which stem the records take: the first whose
// definition the package lacks, or whose definition is unowned or this case's
// own, which is reused. A definition of another case's blocks the last stem.
func recordDefinition(idx *symbols.Index, existing *Existing, stems []string, fqn, pkg string) error {
	var sem *semantics.Model
	for _, stem := range stems {
		def := pkg + "::" + upperFirst(stem) + "Run"
		defSyms := idx.LookupQualified(def)
		if len(defSyms) == 0 {
			existing.Stem = stem
			return nil
		}
		runSyms := idx.LookupQualified("AnalysisRecords::AnalysisRun")
		if len(runSyms) == 0 {
			return fmt.Errorf("the AnalysisRecords library is not loaded")
		}
		if sem == nil {
			resolver := resolve.New(idx)
			sem = semantics.NewModel(resolver)
			resolver.SetModel(sem)
		}
		if !specializesOne(sem, defSyms[0], runSyms[0]) {
			return fmt.Errorf("%s is not an analysis record definition", def)
		}
		owner := recordDefOwner(defSyms[0])
		if owner == "" || owner == fqn {
			existing.Definition = true
			existing.Attributes = Attributes(idx, sem, defSyms[0])
			existing.Stem = stem
			return nil
		}
		if stem == stems[len(stems)-1] {
			return fmt.Errorf("record definition %s belongs to %s; record into another package with `into`", def, owner)
		}
	}
	return nil
}

// takenRecordNumbers lists the numbers of the records named from stem the
// package already holds.
func takenRecordNumbers(idx *symbols.Index, pkg, stem string) map[int]bool {
	taken := map[int]bool{}
	prefix := stem + "_run"
	for _, sym := range idx.LookupQualified(pkg) {
		if sym.Scope == nil {
			continue
		}
		for _, m := range sym.Scope.Members() {
			tail, ok := strings.CutPrefix(m.Name, prefix)
			if !ok {
				continue
			}
			if n, err := strconv.Atoi(tail); err == nil {
				taken[n] = true
			}
		}
	}
	return taken
}

// recordDefOwner is the case a record definition's caseName marks it for,
// "" when it carries none (a hand-written definition, reused by any case).
func recordDefOwner(def *symbols.Symbol) string {
	if def.Scope == nil {
		return ""
	}
	m, ok := def.Scope.LookupLocal("caseName")
	if !ok {
		return ""
	}
	u, ok := m.Decl.(*ast.Usage)
	if !ok {
		return ""
	}
	lit, ok := u.Value.(*ast.LiteralString)
	if !ok {
		return ""
	}
	return source.StringValue(lit.Value)
}

// specializesOne reports whether def specializes want among its supertypes.
func specializesOne(sem *semantics.Model, def, want *symbols.Symbol) bool {
	for _, sup := range sem.AllSupertypes(def) {
		if sup == want {
			return true
		}
	}
	return def == want
}

// Attributes are the features a record definition declares, by name,
// each with whether it is a reference and the name of its declared type.
func Attributes(idx *symbols.Index, sem *semantics.Model, def *symbols.Symbol) map[string]Feature {
	attrs := map[string]Feature{}
	if def.Scope == nil {
		return attrs
	}
	for _, m := range def.Scope.Members() {
		if m.Name == "" || !m.IsFeature() {
			continue
		}
		f := Feature{}
		if u, ok := m.Decl.(*ast.Usage); ok && u.IsReference {
			f.Ref = true
		}
		if types := sem.DeclaredFeatureTypes(m); len(types) > 0 {
			f.TypeFQN = idx.GetFQN(types[0])
		}
		f.Multi = !sem.GoverningMultiplicityOf(m).AtMostOne()
		f.Unique = sem.IsUnique(m)
		attrs[m.Name] = f
	}
	return attrs
}

// recordSpelling renders the values the records cannot spell themselves, each
// through the context it was made in.

func SpellingIn(idx *symbols.Index, contexts map[*runtime.Context]bool, fallback *runtime.Context, text func(*runtime.Context, runtime.Value) string) Spelling {
	contextOf := func(v runtime.Value) *runtime.Context {
		if v.Kind == runtime.ValInstance || v.Kind == runtime.ValVariant {
			for ctx := range contexts {
				if _, ok := ctx.Instance(v.Instance); ok {
					return ctx
				}
			}
		}
		for ctx := range contexts {
			return ctx
		}
		return fallback
	}
	return Spelling{
		ObjectUsage: func(v runtime.Value) string {
			ctx := contextOf(v)
			if ctx == nil {
				return ""
			}
			var usage string
			if inst, ok := ctx.Instance(v.Instance); ok {
				usage = ctx.OccurrenceUsage(inst)
			} else if v.Kind == runtime.ValVariant {
				usage = idx.GetFQN(v.Variant())
			}
			if usage == "" || len(idx.LookupQualified(usage)) != 1 {
				return ""
			}
			return usage
		},
		Text: func(v runtime.Value) string {
			if ctx := contextOf(v); ctx != nil {
				return text(ctx, v)
			}
			return runtime.FormatValue(v)
		},
		Unset: func(v runtime.Value) bool {
			if ctx := contextOf(v); ctx != nil {
				return ctx.HoldsNoValue(v)
			}
			return v.Kind == runtime.ValNull
		},
	}
}

// ShortName is the last segment of a qualified name.
func ShortName(fqn string) string {
	segs, ok := source.QualifiedNameSegments(fqn)
	if !ok || len(segs) == 0 {
		return fqn
	}
	return segs[len(segs)-1]
}
