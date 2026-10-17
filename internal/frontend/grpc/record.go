package grpc

import (
	"context"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/record"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/nsmerge"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// RecordRun runs an analysis case once, over a sweep's rows or as a Monte
// Carlo sample, and records the runs into the model as AnalysisRecords
// declarations, the way `sysml -record-run` does.
func (s *Service) RecordRun(ctx context.Context, req *pb.RecordRunRequest) (*pb.RecordRunResponse, error) {
	if err := s.requireCapability(CapabilityRecordRun); err != nil {
		return nil, err
	}
	if req.AnalysisSymbolId == "" {
		return nil, statusError(connect.CodeInvalidArgument, "analysis_symbol_id names the case to record")
	}
	if req.Runs < 0 {
		return nil, statusErrorf(connect.CodeInvalidArgument, "runs must not be negative, got %d", req.Runs)
	}
	if req.Runs > 0 && len(req.Ranges) > 0 {
		return nil, statusError(connect.CodeInvalidArgument, "ranges and runs are two ways to run the case; give one")
	}
	if req.Runs > 0 && req.Seed == nil {
		return nil, statusError(connect.CodeInvalidArgument, "the runs draw at random; set seed")
	}
	v, err := s.newVerifyContext(ctx, req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()
	if err := acceptsDocuments(v.cached, req.AcceptDocuments); err != nil {
		return nil, err
	}
	sym, err := v.lookup(req.AnalysisSymbolId)
	if err != nil {
		return &pb.RecordRunResponse{Error: err.Error()}, nil
	}
	fqn := v.cached.Index.GetFQN(sym)

	resp := &pb.RecordRunResponse{}
	var runs []record.Run
	var kind record.Kind
	switch {
	case len(req.Ranges) > 0:
		kind = record.KindSweep
		resp.Sweep, runs, resp.Skipped, err = s.recordSweep(ctx, v, sym, req)
		if resp.Sweep != nil {
			resp.Error, resp.FailureReason = resp.Sweep.Error, resp.Sweep.FailureReason
		}
	case req.Runs > 0:
		kind = record.KindRuns
		resp.Sample, runs, resp.Skipped, resp.Reasons, err = s.recordSample(ctx, v, sym, req)
		if resp.Sample != nil {
			resp.Error, resp.FailureReason = resp.Sample.Error, resp.Sample.FailureReason
		}
	default:
		resp.Run, runs, kind, err = s.recordOne(ctx, v, sym, req)
		if resp.Run != nil {
			resp.Error, resp.FailureReason = resp.Run.Error, resp.Run.FailureReason
		}
	}
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		if resp.Error == "" {
			resp.Error = "nothing recorded: no run completed"
		}
		return resp, nil
	}
	placed, err := record.Place(v.cached.Index, fqn, req.RecordInto)
	if err != nil {
		resp.Error = err.Error()
		return resp, nil
	}
	res, err := record.Generate(record.Request{
		Package:    placed.Package,
		Case:       fqn,
		CaseName:   placed.CaseName,
		Provenance: record.Provenance{RunAt: time.Now(), Tool: s.version, Kind: kind},
		Runs:       runs,
		Existing:   placed.Existing,
	})
	if err != nil {
		resp.Error = err.Error()
		return resp, nil
	}
	resp.Package, resp.Definition, resp.Records, resp.Source = placed.Package, res.Definition, res.Records, res.Source
	if err := s.mergeRecord(v.cached, res.Source, resp); err != nil {
		resp.Error = err.Error()
		resp.Content, resp.Documents, resp.ModelHash = "", nil, ""
	}
	return resp, nil
}

// recordOne runs the case once and makes its record: a trade study's when the
// run evaluated calcs, a plain run's otherwise.
func (s *Service) recordOne(ctx context.Context, v *verifyContext, sym *symbols.Symbol, req *pb.RecordRunRequest) (*pb.RunAnalysisResponse, []record.Run, record.Kind, error) {
	if err := v.runtime.RequireAnalysisCase(sym); err != nil {
		return &pb.RunAnalysisResponse{Error: err.Error(), FailureReason: failureReason(err)}, nil, record.KindRun, nil
	}
	args, resp, err := v.analysisArgs(&pb.RunAnalysisRequest{
		SubjectSymbolId: req.SubjectSymbolId, Arguments: req.Arguments, NamedArguments: req.NamedArguments,
	})
	if resp != nil || err != nil {
		return resp, nil, record.KindRun, err
	}
	run, plan, err := v.runCase(ctx, sym, args)
	if gone := callerGone(ctx, err); gone != nil {
		return nil, nil, record.KindRun, gone
	}
	resp = v.caseResponse(run, plan, err)
	if err != nil {
		return resp, nil, record.KindRun, nil
	}
	kind := record.KindRun
	if len(run.result.Evaluations) > 0 {
		kind = record.KindTrade
	}
	idx := v.cached.Index
	contexts := map[*runtime.Context]bool{v.runtime: true}
	rec := record.Run{
		Subject:       record.SubjectOf(idx, run.result.Subject, req.SubjectSymbolId, contexts, objectText),
		Inputs:        run.result.Inputs,
		Outputs:       run.result.Outputs,
		Verdicts:      run.result.Verdicts,
		Evaluations:   run.result.Evaluations,
		Verifications: run.verdicts(),
		Spell:         record.SpellingIn(idx, contexts, v.runtime, objectText),
		Tools:         plan.ToolTexts(),
	}
	return resp, []record.Run{rec}, kind, nil
}

// recordSweep runs the case once per row of the ranges and makes a record of
// each completed row, numbered in table order; a failed row is skipped.
func (s *Service) recordSweep(ctx context.Context, v *verifyContext, sym *symbols.Symbol, req *pb.RecordRunRequest) (*pb.RunSweepResponse, []record.Run, int64, error) {
	// A sweep row carries no verification verdict, so a verification case is
	// recorded one run at a time, as RunSweep refuses it too.
	if runtime.IsVerificationCaseSymbol(sym) {
		return sweepFailure(fmt.Errorf("%w: %s is a verification case, which a sweep does not run; record it without ranges",
			runtime.ErrNotAnAnalysis, req.AnalysisSymbolId)), nil, 0, nil
	}
	if !runtime.IsRunnableCaseSymbol(sym) {
		return &pb.RunSweepResponse{
			Error:         fmt.Sprintf("%s is not an analysis case", req.AnalysisSymbolId),
			FailureReason: pb.FailureReason_FAILURE_REASON_WRONG_KIND,
		}, nil, 0, nil
	}
	sweep := &pb.RunSweepRequest{
		ModelHash: req.ModelHash, SymbolId: req.AnalysisSymbolId, SubjectSymbolId: req.SubjectSymbolId,
		Arguments: req.Arguments, NamedArguments: req.NamedArguments, Ranges: req.Ranges, Engine: req.Engine,
	}
	table, answered, resp, err := s.sweepTable(ctx, v, sym, true, sweep)
	if resp != nil || err != nil {
		return resp, nil, 0, err
	}
	resp = v.sweepResponse(table, s.standingOf(answered))
	idx := v.cached.Index
	var runs []record.Run
	var skipped int64
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
			Subject:     record.SubjectOf(idx, row.Subject, req.SubjectSymbolId, own, objectText),
			Inputs:      row.Inputs,
			Outputs:     row.Outputs,
			Verdicts:    row.Verdicts,
			Evaluations: row.Evaluations,
			Spell:       record.SpellingIn(idx, own, nil, objectText),
			Tools:       answered.ToolTextsIn(row.Context),
		})
	}
	return resp, runs, skipped, nil
}

// recordSample runs the Monte Carlo case the request's number of times and
// makes a record of each completed run and one of the sample's conclusion.
func (s *Service) recordSample(ctx context.Context, v *verifyContext, sym *symbols.Symbol, req *pb.RecordRunRequest) (*pb.RunRunsResponse, []record.Run, int64, []string, error) {
	runsReq := &pb.RunRunsRequest{
		ModelHash: req.ModelHash, AnalysisSymbolId: req.AnalysisSymbolId, SubjectSymbolId: req.SubjectSymbolId,
		Arguments: req.Arguments, NamedArguments: req.NamedArguments, Runs: req.Runs, Seed: req.Seed, Engine: req.Engine,
	}
	plan, seedRow := s.runsPlan(runsReq)
	resp, sample, err := s.monteCarloRuns(ctx, v, runsReq, plan, seedRow)
	if err != nil || sample == nil {
		return resp, nil, 0, nil, err
	}
	idx := v.cached.Index
	var runs []record.Run
	var reasons []string
	var skipped int64
	for _, run := range sample.completed {
		// A run that left a measurement unread has no value to record, so it
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
			Subject:   record.SubjectOf(idx, run.Subject, req.SubjectSymbolId, own, objectText),
			Inputs:    run.Inputs,
			Outputs:   run.Outputs,
			Verdicts:  run.Verdicts,
			Spell:     record.SpellingIn(idx, own, nil, objectText),
			Tools:     sample.answered.ToolTextsIn(run.Context()),
		})
	}
	// The sample's own record carries what the run rows cannot: the statistics,
	// the result and the sample's checks of the case's conclusion.
	switch {
	case sample.concludeErr != nil:
		reasons = append(reasons, fmt.Sprintf("sample not recorded: %s", sample.concludeErr))
	case len(sample.completed) > 0:
		last := sample.completed[len(sample.completed)-1]
		own := map[*runtime.Context]bool{last.Context(): true}
		runs = append(runs, record.Run{
			Kind:        record.KindSample,
			Subject:     record.SubjectOf(idx, last.Subject, req.SubjectSymbolId, own, objectText),
			Inputs:      last.Inputs,
			Outputs:     sample.concluded.Outputs,
			Verdicts:    sample.concluded.Verdicts,
			Evaluations: sample.concluded.Evaluations,
			Spell:       record.SpellingIn(idx, own, nil, objectText),
			Tools:       sample.answered.ToolTexts(),
		})
	}
	skipped += int64(len(sample.table.Rows) - len(sample.completed))
	return resp, runs, skipped, reasons, nil
}

// mergeRecord puts the generated declarations into the model: into the
// document already holding the deepest prefix of the target package's nesting,
// or as a document of their own, then analyzes the result and refuses one that
// errors or drops a declaration, so the model is left unchanged.
func (s *Service) mergeRecord(cached *CachedModel, src string, resp *pb.RecordRunResponse) error {
	describe := func(n ast.Node) string {
		m := libs.TopMemberOf(n)
		return strings.TrimSpace(m.Kind + " " + m.Name)
	}
	root := parser.New(source.New("<record>", []byte(src))).ParseFile()
	inputs := make([]sourceInput, 0, len(cached.Documents)+1)
	for _, doc := range cached.Documents {
		inputs = append(inputs, sourceInput{name: doc.Source.Name(), content: string(doc.Source.Bytes()), kind: doc.Source.Kind()})
	}
	target, depth := -1, -1
	var oldDecl nsmerge.Decl
	newDecl, ok := nsmerge.SoleNamespace(src, root, describe)
	if ok && len(newDecl.Members) > 0 {
		chain := nsmerge.Chain(src, newDecl)
		for i, in := range inputs {
			cand, ok := nsmerge.Named(in.content, newDecl.Name, in.kind, describe)
			if !ok || cand.Header != newDecl.Header {
				continue
			}
			if d := nsmerge.Depth(in.content, cand, chain); d > depth {
				target, depth, oldDecl = i, d, cand
			}
		}
	}
	var edited *pb.EditedDocument
	if target >= 0 {
		edits, lost, gone := nsmerge.MergeEdits(inputs[target].content, oldDecl, src, newDecl, newDecl.Name, describe)
		if drops := append(append([]string{}, lost...), gone...); len(drops) > 0 {
			return fmt.Errorf("the model was left unchanged: recording would drop %s", strings.Join(drops, ", "))
		}
		merged, _ := nsmerge.Apply(inputs[target].content, edits)
		inputs[target].content = merged
		edited = &pb.EditedDocument{Name: inputs[target].name, Content: merged}
	} else {
		name := recordDocumentName(inputs, newDecl.Name)
		inputs = append(inputs, sourceInput{name: name, content: src, kind: source.KindSysML})
		edited = &pb.EditedDocument{Name: name, Content: src}
	}
	before := errorCounts(s.modelDiagnostics(cached))
	// The edited model is staged, and cached only once it is known to be valid,
	// so a refused recording evicts nothing.
	hash, model := s.parseModelKeeping(inputs, cached.Mode, true, false)
	after := s.modelDiagnostics(model)
	if problems := newErrors(before, after); len(problems) > 0 {
		resp.Diagnostics = after
		return fmt.Errorf("the model was left unchanged: %s", strings.Join(problems, "; "))
	}
	s.cache.Add(hash, model)
	resp.ModelHash = hash
	resp.Documents = []*pb.EditedDocument{edited}
	if len(cached.Documents) == 1 && target == 0 {
		resp.Content = edited.Content
	}
	return nil
}

// errorCounts tallies a model's error diagnostics by message.
func errorCounts(diags []*pb.Diagnostic) map[string]int {
	counts := make(map[string]int)
	for _, d := range diags {
		if strings.EqualFold(d.Severity, "error") {
			counts[d.Message]++
		}
	}
	return counts
}

// newErrors reports the errors a recording raised that the model did not
// already report: a message reported before is new only once its count grows.
func newErrors(before map[string]int, after []*pb.Diagnostic) []string {
	seen := make(map[string]int)
	var problems []string
	for _, d := range after {
		if !strings.EqualFold(d.Severity, "error") {
			continue
		}
		seen[d.Message]++
		if seen[d.Message] > before[d.Message] {
			problems = append(problems, d.Message)
		}
	}
	return problems
}

// recordDocumentName names the document the records make when no document
// holds their package: the package's top namespace with the notation's
// extension, numbered past any document already so named.
func recordDocumentName(inputs []sourceInput, top string) string {
	taken := make(map[string]bool, len(inputs))
	for _, in := range inputs {
		taken[in.name] = true
	}
	stem := strings.Trim(strings.ReplaceAll(top, "'", ""), " ")
	if stem == "" {
		stem = "Records"
	}
	name := stem + ".sysml"
	for n := 2; taken[name]; n++ {
		name = fmt.Sprintf("%s%d.sysml", stem, n)
	}
	return name
}
