package grpc

import (
	"context"
	"errors"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// msgNoToolReached is the error of a dry run whose case called no external tool.
const msgNoToolReached = "no ToolExecution-annotated action was reached; nothing to preview"

// toolDryRun runs the case under the registry's dry runner, as `-tool-dry-run`
// does: the first external tool call ends the run and its preview is the answer.
// The run's own results are not kept, so the response carries the preview, the
// run's notes and, where the run failed otherwise, its error.
func (s *Service) toolDryRun(ctx context.Context, v *verifyContext, sym *symbols.Symbol, args runtime.AnalysisArgs) (*pb.RunAnalysisResponse, error) {
	// The preview runs on the request's own runtime, as the REPL's does: the
	// dry runner is set on that context, and an engine's run of its own would
	// not see it.
	runner := s.engines.DryRunner(v.engine)
	v.runtime.SetToolRunner(runner)
	_, err := v.runCaseOn(sym, args)
	if gone := callerGone(ctx, err); gone != nil {
		return nil, gone
	}
	resp := &pb.RunAnalysisResponse{}
	resp.Diagnostics = s.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(v.runtime.Notes(), v.cached))
	dry, reached := runner.Reached()
	if !reached {
		// An engine of its own may surface the preview as the run's error.
		var typed *analysis.ToolDryRunError
		if errors.As(err, &typed) {
			dry, reached = typed, true
		}
	}
	if reached && errors.Is(err, runtime.ErrToolDryRun) {
		err = nil
	}
	switch {
	case reached:
		resp.ToolDryRun = toolDryRunToProto(dry)
	case err == nil:
		resp.Error = msgNoToolReached
	}
	if err != nil {
		resp.Error = err.Error()
		resp.FailureReason = failureReason(err)
	}
	return resp, nil
}

// toolDryRunToProto spells the preview a tool call reached, the lines as the
// CLI prints them beside the typed fields.
func toolDryRunToProto(dry *analysis.ToolDryRunError) *pb.ToolDryRun {
	p := dry.Preview
	out := &pb.ToolDryRun{
		Action:      dry.Action,
		Tool:        p.Tool,
		Version:     p.Version,
		Manifest:    p.Manifest,
		Executable:  p.Executable,
		Protocol:    p.Protocol,
		Args:        p.Args,
		Env:         p.Env,
		Cwd:         p.Cwd,
		Stdin:       string(p.Stdin),
		StdinData:   string(p.StdinData),
		OutputDir:   p.OutputDir,
		ReplySource: p.ReplySource,
		Lines:       p.Lines(),
	}
	if f := p.InputFile; f != nil {
		out.InputFile = &pb.ToolDryRunFile{Name: f.Name, Format: string(f.Format), Data: string(f.Data)}
	}
	for _, in := range p.Inputs {
		out.Inputs = append(out.Inputs, &pb.ToolDryRunInput{
			Variable: in.Variable, Parameter: in.Parameter, Value: p.InputText(in), Unit: in.Value.Unit,
		})
	}
	for _, o := range p.Outputs {
		out.Outputs = append(out.Outputs, &pb.ToolDryRunOutput{Variable: o.Variable, Parameter: o.Parameter})
	}
	return out
}
