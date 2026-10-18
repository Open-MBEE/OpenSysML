package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/record"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
	if run.plan != nil {
		rec.Tools = run.plan.ToolTexts()
	}
	res, rerr := s.recordRuns(fqn, kind, into, command, []record.Run{rec})
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
		rec := record.Run{
			Iteration:   i + 1,
			Subject:     s.recordSubject(row.Subject, inv.object, own),
			Inputs:      row.Inputs,
			Outputs:     row.Outputs,
			Verdicts:    row.Verdicts,
			Evaluations: row.Evaluations,
			Spell:       s.recordSpelling(own),
		}
		if plan != nil {
			rec.Tools = plan.ToolTextsIn(row.Context)
		}
		runs = append(runs, rec)
	}
	if len(runs) == 0 {
		return s.recorded(verdict, record.Result{}, nil, skipped, []string{"nothing recorded: every row failed"})
	}
	res, rerr := s.recordRuns(fqn, record.KindSweep, into, command, runs)
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
		rec := record.Run{
			Iteration: int(run.Number),
			Subject:   s.recordSubject(run.Subject, inv.object, own),
			Inputs:    run.Inputs,
			Outputs:   run.Outputs,
			Verdicts:  run.Verdicts,
			Spell:     s.recordSpelling(own),
		}
		if answered != nil {
			// The conclusion's calls ran in the last row's context: toolMark bounds
			// each row's own.
			rec.Tools = answered.ToolTextsInBefore(run.Context(), sample.toolMark)
		}
		runs = append(runs, rec)
	}
	// The sample's own record carries what the run rows cannot: the statistics,
	// the result and the sample's checks of the case's conclusion.
	switch {
	case cerr != nil:
		reasons = append(reasons, fmt.Sprintf("sample not recorded: %s", cerr))
	case len(sample.completed) > 0:
		last := sample.last()
		own := map[*runtime.Context]bool{last.Context(): true}
		rec := record.Run{
			Kind:        record.KindSample,
			Subject:     s.recordSubject(last.Subject, inv.object, own),
			Inputs:      last.Inputs,
			Outputs:     concluded.Outputs,
			Verdicts:    concluded.Verdicts,
			Evaluations: concluded.Evaluations,
			Spell:       s.recordSpelling(own),
		}
		if answered != nil {
			rec.Tools = answered.ToolTexts()
		}
		runs = append(runs, rec)
	}
	skipped += len(sample.table.Rows) - len(sample.completed)
	if len(runs) == 0 {
		return s.recorded(verdict, record.Result{}, nil, skipped, append(reasons, "nothing recorded: no run completed"))
	}
	res, rerr := s.recordRuns(fqn, record.KindRuns, into, command, runs)
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
	return record.SubjectOf(s.symbolIndex(), inst, label, contexts, objectText)
}
func (s *Session) recordRuns(fqn string, kind record.Kind, into, command string, runs []record.Run) (record.Result, error) {
	placed, err := record.Place(s.symbolIndex(), fqn, into)
	if err != nil {
		return record.Result{}, err
	}
	res, err := record.Generate(record.Request{
		Package:  placed.Package,
		Case:     fqn,
		CaseName: placed.CaseName,
		Provenance: record.Provenance{
			RunAt:   s.now(),
			Tool:    s.toolVersion,
			Command: command,
			Kind:    kind,
		},
		Runs:     runs,
		Existing: placed.Existing,
	})
	if err != nil {
		return record.Result{}, err
	}
	if err := s.submitRecord(res.Source); err != nil {
		return record.Result{}, err
	}
	return res, nil
}

func (s *Session) recordSpelling(contexts map[*runtime.Context]bool) record.Spelling {
	return record.SpellingIn(s.symbolIndex(), contexts, s.rtCtx, objectText)
}
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
		verdict.Lines = append(verdict.Lines, fmt.Sprintf("  recorded %d runs as %s … %s", len(res.Records), res.Records[0], record.ShortName(res.Records[len(res.Records)-1])))
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
