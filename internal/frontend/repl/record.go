package repl

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/record"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const recordUsage = "usage: %record <name>[(<args>)] [<object>] [into <package>]"

// doRecord carries out %record at the prompt: the run %analysis would make,
// recorded into the model beside the case or into the package `into` names.
func (s *Session) doRecord(tail string) ([]string, bool, error) {
	inv, into, err := splitRecordArgs(tail)
	if err != nil {
		return []string{errPrefix + err.Error(), recordUsage}, false, nil
	}
	if inv.name == "" {
		return []string{recordUsage}, false, nil
	}
	return s.withTrace(s.recordAnalysisInv(inv, into, "%record "+strings.TrimSpace(tail))).Lines, false, nil
}

// splitRecordArgs takes apart %record's tail: the invocation %analysis takes,
// then `into <package>` written last when it is.
func splitRecordArgs(tail string) (analysisInvocation, string, error) {
	if i := lastTopLevelInto(tail); i >= 0 {
		into := strings.TrimSpace(tail[i+len(" into "):])
		if _, ok := source.QualifiedNameSegments(into); !ok {
			return analysisInvocation{}, "", fmt.Errorf("%q does not name a package", into)
		}
		inv, err := splitAnalysisArgs(tail[:i])
		return inv, into, err
	}
	inv, err := splitAnalysisArgs(tail)
	return inv, "", err
}

// lastTopLevelInto is the index of the last ` into ` token written at top
// level — outside quoted names, string literals and parentheses — or -1.
func lastTopLevelInto(tail string) int {
	last, depth := -1, 0
	var quote byte
	escaped := false
	for i := 0; i < len(tail); i++ {
		c := tail[i]
		switch {
		case escaped:
			escaped = false
		case c == '\\':
			escaped = true
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case depth == 0 && strings.HasPrefix(tail[i:], " into "):
			last = i
		}
	}
	return last
}

// RecordAnalysis runs the invocation as %analysis does and records the run
// into the model as AnalysisRecords elements, into the package named or a
// Records package beside the case's. command is the invocation text the
// record's provenance carries.
func (s *Session) RecordAnalysis(invocation, into, command string) Verdict {
	defer s.enter()()
	inv, err := splitAnalysisArgs(invocation)
	if err != nil {
		return s.withTrace(unresolvedVerdict(invocation, err.Error()))
	}
	return s.withTrace(s.recordAnalysisInv(inv, into, command))
}

// recordAnalysisInv is RecordAnalysis on an invocation already parsed.
func (s *Session) recordAnalysisInv(inv analysisInvocation, into, command string) Verdict {
	run, err := s.runAnalysis(inv)
	verdict := s.caseVerdict(inv, run, err)
	if err != nil {
		return verdict
	}
	_, fqn, serr := s.analysisSymbol(inv)
	if serr != nil {
		return s.recordFailed(verdict, serr)
	}
	kind := record.KindRun
	if len(run.result.Evaluations) > 0 {
		kind = record.KindTrade
	}
	contexts := map[*runtime.Context]bool{}
	if s.rtCtx != nil {
		contexts[s.rtCtx] = true
	}
	inst := run.subject
	if inst == nil {
		inst = run.result.Subject
	}
	rec := record.Run{
		Subject:       s.recordSubject(inst, run.label, contexts),
		Inputs:        run.result.Inputs,
		Outputs:       run.result.Outputs,
		Verdicts:      run.result.Verdicts,
		Evaluations:   run.result.Evaluations,
		Verifications: run.verdicts,
		Spell:         s.recordSpelling(contexts),
	}
	var tools []string
	if run.plan != nil {
		tools = run.plan.ToolTexts()
	}
	res, rerr := s.recordRuns(fqn, kind, into, command, tools, []record.Run{rec})
	return s.recorded(verdict, res, rerr, 0, nil)
}

// RecordSweep runs the invocation once per row of the ranges as RunSweep does
// and records each completed row, numbered in table order.
func (s *Session) RecordSweep(invocation string, ranges []string, into, command string) Verdict {
	defer s.enter()()
	inv, trailing, err := splitSweepTail(invocation)
	if err != nil {
		return s.withTrace(unresolvedVerdict(invocation, err.Error()))
	}
	specs := make([]sweepSpec, 0, len(ranges)+len(trailing))
	specs = append(specs, trailing...)
	for _, text := range ranges {
		spec, err := parseSweepSpec(text)
		if err != nil {
			return s.withTrace(unresolvedVerdict(invocation, err.Error()))
		}
		specs = append(specs, spec)
	}
	return s.withTrace(s.recordSweepInv(inv, specs, into, command))
}

// recordSweepInv is RecordSweep on an invocation and its specs already parsed.
func (s *Session) recordSweepInv(inv analysisInvocation, specs []sweepSpec, into, command string) Verdict {
	sym, fqn, err := s.lookupSymbolOfKinds(inv.name,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolCalcDef, symbols.SymbolCalcUsage)
	if err == nil && !runtime.IsRunnableCaseSymbol(sym) {
		err = fmt.Errorf("%s is a calc, not a case", inv.name)
	}
	if err != nil {
		return unresolvedVerdict(sweepLabel(inv, sweepDraws{}), err.Error())
	}
	table, plan, err := s.runSweep(inv, specs, sweepDraws{})
	if err != nil {
		return standing(unresolvedVerdict(sweepLabel(inv, sweepDraws{}), err.Error()), plan)
	}
	verdict := standing(s.sweepReport(inv, table, sweepDraws{}), plan)
	var runs []record.Run
	skipped := 0
	for i, row := range table.Rows {
		if row.Err != nil {
			skipped++
			continue
		}
		// Each row's values spell in its own context: instance ids restart per
		// row, so a value means nothing read through another row's.
		own := map[*runtime.Context]bool{row.Context: true}
		runs = append(runs, record.Run{
			Iteration:   i + 1,
			Subject:     s.recordSubject(row.Subject, inv.object, own),
			Inputs:      row.Inputs,
			Outputs:     row.Outputs,
			Verdicts:    row.Verdicts,
			Evaluations: row.Evaluations,
			Spell:       s.recordSpelling(own),
		})
	}
	if len(runs) == 0 {
		return s.recorded(verdict, record.Result{}, nil, skipped, []string{"nothing recorded: every row failed"})
	}
	var tools []string
	if plan != nil {
		tools = plan.ToolTexts()
	}
	res, rerr := s.recordRuns(fqn, record.KindSweep, into, command, tools, runs)
	return s.recorded(verdict, res, rerr, skipped, nil)
}

// RecordMonteCarlo runs the invocation count times as RunMonteCarlo does and
// records each completed run, numbered by its run number.
func (s *Session) RecordMonteCarlo(invocation string, count int64, seed *uint64, into, command string) Verdict {
	defer s.enter()()
	inv, err := splitAnalysisArgs(invocation)
	if err != nil {
		return s.withTrace(unresolvedVerdict(invocation, err.Error()))
	}
	return s.withTrace(s.recordMonteCarloInv(inv, count, seed, into, command))
}

// recordMonteCarloInv is RecordMonteCarlo on an invocation already parsed.
func (s *Session) recordMonteCarloInv(inv analysisInvocation, count int64, seed *uint64, into, command string) Verdict {
	sample, answered, err := s.monteCarloSample(inv, count, seed)
	verdict := s.monteCarloReport(inv, sample, answered, err)
	if err != nil || sample == nil {
		return verdict
	}
	fqn := sampleRunsCase(sample)
	var runs []record.Run
	var reasons []string
	skipped := 0
	concluded, cerr := sample.conclusion()
	for _, run := range sample.completed {
		// Outputs that are the sample's were already left out of Unread; what
		// is left failed the iteration.
		if len(run.Unread) > 0 {
			missing := make([]string, 0, len(run.Unread))
			for name := range run.Unread {
				missing = append(missing, name)
			}
			sort.Strings(missing)
			skipped++
			reasons = append(reasons, fmt.Sprintf("run %d not recorded: %s", run.Number, run.Unread[missing[0]]))
			continue
		}
		own := map[*runtime.Context]bool{run.Context(): true}
		runs = append(runs, record.Run{
			Iteration: int(run.Number),
			Subject:   s.recordSubject(run.Subject, inv.object, own),
			Inputs:    run.Inputs,
			Outputs:   run.Outputs,
			Verdicts:  run.Verdicts,
			Spell:     s.recordSpelling(own),
		})
	}
	// The sample's own record carries what the run rows cannot: the statistics,
	// the result and the sample's checks of the case's conclusion.
	switch {
	case cerr != nil:
		reasons = append(reasons, fmt.Sprintf("sample not recorded: %s", cerr))
	case len(sample.completed) > 0:
		last := sample.last()
		own := map[*runtime.Context]bool{last.Context(): true}
		runs = append(runs, record.Run{
			Kind:        record.KindSample,
			Subject:     s.recordSubject(last.Subject, inv.object, own),
			Inputs:      last.Inputs,
			Outputs:     concluded.Outputs,
			Verdicts:    concluded.Verdicts,
			Evaluations: concluded.Evaluations,
			Spell:       s.recordSpelling(own),
		})
	}
	skipped += len(sample.table.Rows) - len(sample.completed)
	if len(runs) == 0 {
		return s.recorded(verdict, record.Result{}, nil, skipped, append(reasons, "nothing recorded: no run completed"))
	}
	var tools []string
	if answered != nil {
		tools = answered.ToolTexts()
	}
	res, rerr := s.recordRuns(fqn, record.KindRuns, into, command, tools, runs)
	return s.recorded(verdict, res, rerr, skipped, reasons)
}

// sampleRunsCase is the qualified name of the case a sample's runs were made
// of, from any completed run's report.
func sampleRunsCase(sample *monteCarloRuns) string {
	for _, run := range sample.completed {
		return run.Case
	}
	return ""
}

// recordSubject is how a run's object is recorded: its usage in the model when
// that resolves to exactly one element, and its text for subjectName.
func (s *Session) recordSubject(inst *runtime.Instance, label string, contexts map[*runtime.Context]bool) record.Subject {
	if inst == nil {
		return record.Subject{}
	}
	for ctx := range contexts {
		if _, ok := ctx.Instance(inst.ID); ok {
			if usage := ctx.OccurrenceUsage(inst); usage != "" && len(s.symbolIndex().LookupQualified(usage)) == 1 {
				return record.Subject{Usage: usage, Text: usage}
			}
			return record.Subject{Text: objectText(ctx, runtime.Value{Kind: runtime.ValInstance, Instance: inst.ID})}
		}
	}
	return record.Subject{Text: label}
}

// recordRuns generates the record declarations for runs of the case fqn names,
// submits them, and returns what was generated. tools is what every external tool
// call the runs made ran, in call order.
func (s *Session) recordRuns(fqn string, kind record.Kind, into, command string, tools []string, runs []record.Run) (record.Result, error) {
	pkg, err := s.recordPackage(fqn, into)
	if err != nil {
		return record.Result{}, err
	}
	caseSym := s.recordCaseSymbol(fqn)
	existing, err := s.recordExisting(caseSym, fqn, pkg)
	if err != nil {
		return record.Result{}, err
	}
	caseName := fqn
	if caseSym != nil {
		caseName = caseSym.Name
	}
	res, err := record.Generate(record.Request{
		Package:  pkg,
		Case:     fqn,
		CaseName: caseName,
		Provenance: record.Provenance{
			RunAt:   s.now(),
			Tool:    s.toolVersion,
			Command: command,
			Kind:    kind,
			Tools:   tools,
		},
		Runs:     runs,
		Existing: existing,
	})
	if err != nil {
		return record.Result{}, err
	}
	if err := s.submitRecord(res.Source); err != nil {
		return record.Result{}, err
	}
	return res, nil
}

// recordPackage is the package a case's records go into: the one into names,
// or Records beside the package enclosing the case's own package.
func (s *Session) recordPackage(fqn, into string) (string, error) {
	if into != "" {
		segs, ok := source.QualifiedNameSegments(into)
		if !ok {
			return "", fmt.Errorf("%q does not name a package", into)
		}
		for i := 1; i <= len(segs); i++ {
			prefix := strings.Join(segs[:i], "::")
			for _, sym := range s.symbolIndex().LookupQualified(prefix) {
				if sym.Kind != symbols.SymbolPackage {
					return "", fmt.Errorf("%s names a %s, not a package", prefix, sym.Kind)
				}
			}
		}
		return into, nil
	}
	idx := s.symbolIndex()
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

// recordCaseSymbol is the analysis case symbol fqn names, nil when the index
// names none or several.
func (s *Session) recordCaseSymbol(fqn string) *symbols.Symbol {
	syms := s.symbolIndex().LookupQualified(fqn)
	if len(syms) == 1 {
		return syms[0]
	}
	return nil
}

// recordExisting is what the target package already declares of the shape
// Generate must fit: the package itself, the case's record definition and the
// record numbers already taken. The stem the records are named from is the
// case's name, owner-prefixed when a definition of the same name belongs to a
// sibling case.
func (s *Session) recordExisting(caseSym *symbols.Symbol, fqn, pkg string) (record.Existing, error) {
	idx := s.symbolIndex()
	var existing record.Existing
	if len(idx.LookupQualified(pkg)) > 0 {
		existing.Package = true
	}
	short := shortName(fqn)
	if caseSym != nil {
		short = caseSym.Name
	}
	// Candidate stems: the case's name, then each owner up the chain prefixed.
	stems := []string{short}
	if caseSym != nil {
		var owners []string
		for cur := caseSym.Owner(); cur != nil && cur.Name != ""; cur = cur.Owner() {
			owners = append([]string{cur.Name}, owners...)
			stems = append(stems, strings.Join(append(append([]string{}, owners...), short), "_"))
		}
	}
	var sem *semantics.Model
	for _, stem := range stems {
		def := pkg + "::" + upperFirst(stem) + "Run"
		defSyms := idx.LookupQualified(def)
		if len(defSyms) == 0 {
			existing.Stem = stem
			break
		}
		runSyms := idx.LookupQualified("AnalysisRecords::AnalysisRun")
		if len(runSyms) == 0 {
			return existing, fmt.Errorf("the AnalysisRecords library is not loaded")
		}
		if sem == nil {
			resolver := resolve.New(idx)
			sem = semantics.NewModel(resolver)
			resolver.SetModel(sem)
		}
		if !specializesOne(sem, defSyms[0], runSyms[0]) {
			return existing, fmt.Errorf("%s is not an analysis record definition", def)
		}
		owner := recordDefOwner(defSyms[0])
		switch {
		case owner == "" || owner == fqn:
			// Unowned or this case's own: reused.
			existing.Definition = true
			existing.Attributes = recordAttributes(idx, sem, defSyms[0])
			existing.Stem = stem
		default:
			if stem == stems[len(stems)-1] {
				return existing, fmt.Errorf("record definition %s belongs to %s; record into another package with `into`", def, owner)
			}
		}
		if existing.Stem != "" {
			break
		}
	}
	stem := existing.Stem
	if stem == "" {
		stem = short
	}
	existing.Taken = map[int]bool{}
	for _, sym := range idx.LookupQualified(pkg) {
		if sym.Scope == nil {
			continue
		}
		prefix := stem + "_run"
		for _, m := range sym.Scope.Members() {
			tail, ok := strings.CutPrefix(m.Name, prefix)
			if !ok {
				continue
			}
			if n, err := strconv.Atoi(tail); err == nil {
				existing.Taken[n] = true
			}
		}
	}
	return existing, nil
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

// recordAttributes are the features a record definition declares, by name,
// each with whether it is a reference and the name of its declared type.
func recordAttributes(idx *symbols.Index, sem *semantics.Model, def *symbols.Symbol) map[string]record.Feature {
	attrs := map[string]record.Feature{}
	if def.Scope == nil {
		return attrs
	}
	for _, m := range def.Scope.Members() {
		if m.Name == "" || !m.IsFeature() {
			continue
		}
		f := record.Feature{}
		if u, ok := m.Decl.(*ast.Usage); ok && u.IsReference {
			f.Ref = true
		}
		if types := sem.DeclaredFeatureTypes(m); len(types) > 0 {
			f.TypeFQN = idx.GetFQN(types[0])
		}
		attrs[m.Name] = f
	}
	return attrs
}

// recordSpelling renders the values the records cannot spell themselves, each
// through the context it was made in.
func (s *Session) recordSpelling(contexts map[*runtime.Context]bool) record.Spelling {
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
		return s.rtCtx
	}
	return record.Spelling{
		ObjectUsage: func(v runtime.Value) string {
			ctx := contextOf(v)
			if ctx == nil {
				return ""
			}
			var usage string
			if inst, ok := ctx.Instance(v.Instance); ok {
				usage = ctx.OccurrenceUsage(inst)
			} else if v.Kind == runtime.ValVariant {
				usage = s.symbolIndex().GetFQN(v.Variant())
			}
			if usage == "" || len(s.symbolIndex().LookupQualified(usage)) != 1 {
				return ""
			}
			return usage
		},
		Text: func(v runtime.Value) string {
			if ctx := contextOf(v); ctx != nil {
				return objectText(ctx, v)
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

// recorded finishes a record verdict: the run's own report, then what the
// model gained, the count of rows skipped, or the failure that left it
// untouched.
func (s *Session) recorded(verdict Verdict, res record.Result, err error, skipped int, reasons []string) Verdict {
	for _, reason := range reasons {
		verdict.Lines = append(verdict.Lines, "  "+reason)
	}
	if err != nil {
		return s.recordFailed(verdict, err)
	}
	switch len(res.Records) {
	case 0:
		verdict.Lines = append(verdict.Lines, "  nothing was recorded")
	case 1:
		verdict.Lines = append(verdict.Lines, fmt.Sprintf("  recorded %s (%s)", res.Records[0], res.Definition))
	default:
		verdict.Lines = append(verdict.Lines, fmt.Sprintf("  recorded %d runs as %s … %s", len(res.Records), res.Records[0], shortName(res.Records[len(res.Records)-1])))
	}
	if skipped > 0 {
		verdict.Lines = append(verdict.Lines, fmt.Sprintf("  %d failed run(s) were not recorded", skipped))
	}
	return verdict
}

// recordFailed fails a run's verdict with the reason its record was not made.
func (s *Session) recordFailed(verdict Verdict, err error) Verdict {
	verdict.Status = VerdictFails
	verdict.Lines = append(verdict.Lines, errPrefix+"recording the run failed: "+err.Error())
	return verdict
}

// shortName is the last segment of a qualified name.
func shortName(fqn string) string {
	segs, ok := source.QualifiedNameSegments(fqn)
	if !ok || len(segs) == 0 {
		return fqn
	}
	return segs[len(segs)-1]
}

// upperFirst capitalizes a name's leading letter.
func upperFirst(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// submitRecord applies generated declarations as one submission: the record
// merge flag lets it fold into the package a loaded file declared, and any
// loss or error the submission makes restores the buffer as it was.
func (s *Session) submitRecord(src string) error {
	before := append([]snippet{}, s.snippets...)
	beforeErrors := s.errorCounts()
	s.recordMerge = true
	res, _, _ := s.submitEach([]SourceFile{{Text: src}})
	s.recordMerge = false

	problems := newProblems(beforeErrors, res.Diagnostics)
	for _, drop := range s.recordDrops {
		for _, name := range append(append([]string{}, drop.lost...), drop.gone...) {
			problems = append(problems, fmt.Sprintf("recording would drop %s", name))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	s.rollbackSubmit(before)
	return fmt.Errorf("the model was left unchanged: %s", strings.Join(problems, "; "))
}

// newProblems reports the error diagnostics a submission raised that the
// model did not already report: a message the model reported before is new
// only once its count grows.
func newProblems(before map[string]int, diagnostics []diag.Diagnostic) []string {
	var problems []string
	for _, d := range diagnostics {
		if d.Severity != diag.SeverityError {
			continue
		}
		if before[d.Message] > 0 {
			before[d.Message]--
		} else {
			problems = append(problems, d.Message)
		}
	}
	return problems
}

// errorCounts keys the error diagnostics the buffer already reports, by
// message: a merge can move the offsets an old error sits at, and a record's
// diagnostic is new only once it outnumbers what the model already reported.
func (s *Session) errorCounts() map[string]int {
	counts := map[string]int{}
	for _, d := range s.diagnostics() {
		if d.Severity == diag.SeverityError {
			counts[d.Message]++
		}
	}
	return counts
}

// rollbackSubmit restores the snippets a failed record submission replaced and
// rebuilds over them as a submission does, so what the session holds — its
// objects and debugging sessions — is carried into the restored document.
func (s *Session) rollbackSubmit(before []snippet) {
	s.snippets = before
	s.version++
	s.rebuildOver(nil)
	s.idxVersion = 0
	s.names = nil
}
