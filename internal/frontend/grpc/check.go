package grpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// CheckBehavior searches every schedule of an action or a state machine, as the
// `sysml` command's -check-* flags do about -action/-state: a violated property, a
// deadlock, a failure or a divergent feature is witnessed, replayed and reported.
func (s *Service) CheckBehavior(ctx context.Context, req *pb.CheckBehaviorRequest) (*pb.CheckBehaviorResponse, error) {
	if err := s.requireCapability(CapabilityCheck); err != nil {
		return nil, err
	}
	action := req.GetActionSymbolId() != ""
	if action == (req.GetStateMachineSymbolId() != "") {
		return nil, statusError(connect.CodeInvalidArgument, "name exactly one behavior to check: action_symbol_id or state_machine_symbol_id")
	}
	if req.GetCheckDepth() < 0 || req.GetCheckStates() < 0 || req.GetCheckUnroll() < 0 {
		return nil, statusError(connect.CodeInvalidArgument, "check_depth, check_states and check_unroll are counts: 0 for the engine's default, else positive")
	}
	if timeout := req.GetCheckTimeout(); timeout < 0 || math.IsNaN(timeout) || math.IsInf(timeout, 0) {
		return nil, statusError(connect.CodeInvalidArgument, "check_timeout is a finite number of seconds, 0 for none")
	}
	if req.GetPerformerSymbolId() != "" {
		if err := s.requireCapability(CapabilityPerformer); err != nil {
			return nil, err
		}
	}
	// A check is the check engine's unless the request selects otherwise, as the
	// CLI's -check-* flags are read under -engine check, smt or all.
	selection := analysis.Only(analysis.CheckEngineName)
	if req.GetEngine() != "" {
		var err error
		if selection, err = s.engineSelection(req.GetEngine()); err != nil {
			return nil, err
		}
	}
	for _, pv := range req.GetInputs() {
		if err := s.requireValueCapabilities(pv); err != nil {
			return nil, err
		}
	}
	cached, ok := s.cache.Get(req.GetModelHash())
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.GetModelHash())
	}
	id := req.GetActionSymbolId()
	if !action {
		id = req.GetStateMachineSymbolId()
	}
	syms := cached.LookupNamedMember(id)
	if len(syms) == 0 {
		return &pb.CheckBehaviorResponse{Verdict: analysis.CheckUndecided.String(), Error: fmt.Sprintf("behavior not found: %s", id)}, nil
	}
	sym := syms[0]
	lookup := func(name string) (*symbols.Symbol, error) {
		found := cached.LookupNamedMember(name)
		if len(found) == 0 {
			return nil, fmt.Errorf("symbol not found: %s", name)
		}
		return found[0], nil
	}
	conditions, err := analysis.ResolveCheckConditions(lookup, cached.PrimaryRoot(), req.GetCheckProperty(), req.GetCheckAssume())
	if err != nil {
		return &pb.CheckBehaviorResponse{Verdict: analysis.CheckUndecided.String(), Error: err.Error()}, nil
	}

	witnessDir := ""
	if req.GetCheckWitness() {
		dir, err := os.MkdirTemp("", "sysml-check-witness-")
		if err != nil {
			return nil, statusError(connect.CodeInternal, err.Error())
		}
		defer os.RemoveAll(dir)
		witnessDir = dir
	}

	starter := func(rt *runtime.Context) (*runtime.Invocation, error) {
		inputs := make(map[string]runtime.Value, len(req.GetInputs()))
		for name, pv := range req.GetInputs() {
			val, cerr := protoconv.ProtoToRuntimeValue(rt, pv, cached.Index, rt.Semantics())
			if cerr != nil {
				return nil, fmt.Errorf("input %q could not be read: %v", name, cerr)
			}
			inputs[name] = val
		}
		self, err := s.performer(cached, rt, req.GetPerformerSymbolId())
		if err != nil {
			return nil, err
		}
		inv := &runtime.Invocation{}
		if action {
			exec, _, err := rt.StartActionPerformedBy(sym, self, inputs)
			if err != nil {
				return nil, fmt.Errorf("failed to create executor: %w", err)
			}
			inv.Actions = append(inv.Actions, exec)
			return inv, nil
		}
		exec, _, err := rt.StartStatePerformedBy(sym, self)
		if err != nil {
			return nil, fmt.Errorf("failed to create executor: %w", err)
		}
		inv.States = append(inv.States, exec)
		inv.Horizon = runtime.HorizonInitial()
		return inv, nil
	}
	run := func(rt *runtime.Context) (runtime.Outcome, error) {
		inv, err := starter(rt)
		if err != nil {
			return runtime.Outcome{}, err
		}
		for _, exec := range inv.Actions {
			if err := exec.RunToCompletion(); err != nil {
				return runtime.Outcome{}, err
			}
		}
		return inv.Outcome(), nil
	}
	check := &analysis.CheckAsk{
		Start:      starter,
		Performer:  req.GetPerformerSymbolId(),
		Diverge:    req.GetCheckDiverge(),
		WitnessDir: witnessDir,
		Properties: conditions.Properties,
	}
	var holds *analysis.HoldsAsk
	if action {
		holds = &analysis.HoldsAsk{
			Behavior: sym,
			Start: func(rt *runtime.Context) (*runtime.ActionExecutor, error) {
				inv, err := starter(rt)
				if err != nil {
					return nil, err
				}
				return inv.Actions[0], nil
			},
			Performer:  req.GetPerformerSymbolId(),
			Inputs:     req.GetCheckInput(),
			Diverge:    req.GetCheckDiverge(),
			WitnessDir: witnessDir,
			Conditions: conditions.Conditions,
			Scope:      conditions.Scope,
			Assume:     conditions.Assume,
		}
	}
	unroll := int(req.GetCheckUnroll())
	kind := analysis.CheckKind(check, holds, unroll)
	if selection == analysis.Only(analysis.SMTEngineName) && kind == analysis.Outcomes {
		kind = analysis.Holds
	}
	free := analysis.FreeNothing
	if len(req.GetCheckInput()) > 0 || len(req.GetCheckAssume()) > 0 {
		free = analysis.FreeInputs
	}
	policy := runtime.DefaultExploreSchedulePolicy
	budget := checkBudget(analysis.BudgetOf(s.budgets, policy, kind, s.jobs), selection, int(req.GetCheckDepth()), int(req.GetCheckStates()))
	budget.Unroll = unroll
	if timeout := req.GetCheckTimeout(); timeout > 0 {
		d := time.Duration(timeout * float64(time.Second))
		budget.Deadline = time.Now().Add(d)
		budget.Solver = d
	}
	plan, err := s.engines.Check(ctx, analysis.Request{
		Model:     s.model(cached),
		Subject:   id,
		Schedule:  policy,
		Budget:    budget,
		Selection: selection,
	}, kind, free, check, holds, run)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var setup *runtime.SetupError
		if errors.As(err, &setup) {
			return &pb.CheckBehaviorResponse{Verdict: analysis.CheckUndecided.String(), Error: err.Error()}, nil
		}
		return &pb.CheckBehaviorResponse{Verdict: analysis.CheckUndecided.String(), Reason: err.Error()}, nil
	}
	label := "Action " + id
	if req.StateMachineSymbolId != "" {
		label = "State machine " + id
	}
	return s.checkResponse(label, plan, witnessDir), nil
}

// checkResponse renders what the engines answered: the report as the command
// prints it and its parts as data.
func (s *Service) checkResponse(label string, plan analysis.Plan, witnessDir string) *pb.CheckBehaviorResponse {
	result := plan.Result
	st := s.standingOf(plan)
	status, lines, _ := analysis.CheckLines(label, result)
	resp := &pb.CheckBehaviorResponse{
		Verdict:  status.String(),
		Lines:    lines,
		Engine:   st.engine,
		Strength: st.strength,
		Bounds:   st.bounds,
		Reason:   result.Reason,
	}
	if checked := result.Check(); checked != nil && checked.Report != nil {
		report := checked.Report
		search := &pb.CheckSearch{
			States:      int32Clamp(report.States),
			Moves:       int32Clamp(report.Moves),
			MaxDepth:    int32Clamp(report.MaxDepth),
			BoundsHit:   report.BoundsHit,
			MassBounded: report.MassBounded,
			Notes:       observationNotes(report.Scope),
			Status:      report.Status(),
		}
		if at, ok := report.Horizon.Bound(); ok {
			search.Horizon, search.HorizonSet = at, true
		}
		for _, reason := range report.Scope {
			search.Scope = append(search.Scope, string(reason))
		}
		resp.Search = search
		for i, v := range report.Violations {
			violation := &pb.CheckViolation{
				Kind:        v.Kind.String(),
				Property:    v.Name,
				Depth:       int32Clamp(v.Depth),
				Probability: v.Mass,
				Witness:     checkWitness(v.Witness, witnessPathAt(checked.Violations, i)),
				Text:        v.String(),
			}
			if v.Err != nil {
				violation.Error = v.Err.Error()
			}
			resp.Violations = append(resp.Violations, violation)
		}
		for i, d := range report.Divergent {
			divergence := &pb.CheckDivergence{Feature: d.Feature}
			var paths []string
			if i < len(checked.Divergent) {
				paths = checked.Divergent[i]
			}
			for j, value := range d.Values {
				divergence.Values = append(divergence.Values, &pb.CheckDivergentValue{
					Value:   value.Value,
					Witness: checkWitness(value.Witness, witnessPathAt(paths, j)),
				})
			}
			resp.Divergences = append(resp.Divergences, divergence)
		}
		for _, final := range report.Finals {
			resp.Finals = append(resp.Finals, &pb.CheckFinal{
				Outcome: final.Outcome,
				Values:  final.Values,
				Witness: checkWitness(final.Witness, ""),
			})
		}
		return resp
	}
	resp.Claim = result.Claim.String()
	for _, in := range result.Inputs {
		resp.FreeInputs = append(resp.FreeInputs, in.String())
	}
	resp.Assumptions = result.Assumptions
	if result.Witness != nil {
		resp.Witness = analysisWitness(result.Witness)
	}
	if result.Contrast != nil {
		resp.Contrast = analysisWitness(result.Contrast)
	}
	return resp
}

func witnessPathAt(paths []string, i int) string {
	if i < len(paths) {
		return paths[i]
	}
	return ""
}

// checkWitness renders a search witness: its replay schedule, draws and choices,
// and the witness file written for it, read back so a caller without the
// service's filesystem can replay it.
func checkWitness(w runtime.Witness, path string) *pb.CheckWitness {
	witness := &pb.CheckWitness{Schedule: runtime.ReplayOf(w).String()}
	for _, d := range w.Draws {
		witness.Draws = append(witness.Draws, d.String())
	}
	for _, c := range w.Choices {
		witness.Choices = append(witness.Choices, c.String())
	}
	for _, in := range w.Inputs {
		witness.Inputs = append(witness.Inputs, in.String())
	}
	attachWitnessFile(witness, path)
	return witness
}

// analysisWitness renders a symbolic engine's witness.
func analysisWitness(w *analysis.Witness) *pb.CheckWitness {
	witness := &pb.CheckWitness{Schedule: w.Schedule.String()}
	for _, d := range w.Draws {
		witness.Draws = append(witness.Draws, d.String())
	}
	for _, c := range w.Choices {
		witness.Choices = append(witness.Choices, c.String())
	}
	for _, in := range w.Inputs {
		witness.Inputs = append(witness.Inputs, in.String())
	}
	attachWitnessFile(witness, w.Written)
	return witness
}

func attachWitnessFile(witness *pb.CheckWitness, path string) {
	if path == "" {
		return
	}
	content, err := os.ReadFile(path) // #nosec G304 -- the witness file the checker wrote under the request's own witness directory
	if err != nil {
		return
	}
	witness.FileName = filepath.Base(path)
	witness.FileContent = string(content)
}

// observationNotes spells the reasons the searched runs' coverage is narrower
// than the behavior admits, one per line as the CLI prints them.
func observationNotes(scope []runtime.ObservationReason) []string {
	notes := make([]string, 0, len(scope))
	for _, reason := range scope {
		notes = append(notes, string(reason))
	}
	return notes
}

// checkBudget bounds a check as the REPL does: the check or smt engine alone
// gets its own defaults, other selections the policy's, a set bound always stands.
func checkBudget(budget analysis.Budget, selection analysis.Selection, depth, states int) analysis.Budget {
	checkOnly := selection == analysis.Only(analysis.CheckEngineName)
	symbolic := selection == analysis.Only(analysis.SMTEngineName)
	switch {
	case depth > 0:
		budget.Depth = depth
	case checkOnly:
		budget.Depth = analysis.DefaultCheckDepth
	case symbolic:
		budget.Depth = 0
	}
	switch {
	case states > 0:
		budget.Runs = states
	case checkOnly:
		budget.Runs = analysis.DefaultCheckStates
	}
	return budget
}
