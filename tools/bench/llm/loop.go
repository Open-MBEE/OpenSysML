package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// DefaultMaxRounds is how many repair rounds follow the first generation.
const DefaultMaxRounds = 3

// DefaultMaxTokens is the completion limit a run asks for unless -max-tokens says otherwise.
const DefaultMaxTokens = 4096

// MaxToolCallsPerRound bounds the tool-call turns a model may take before it
// must answer with a document.
const MaxToolCallsPerRound = 8

// Arm is one experimental condition.
type Arm string

const (
	// ArmCompiler feeds the model compiler diagnostics only.
	ArmCompiler Arm = "compiler"
	// ArmMCP feeds the model compiler diagnostics and offers sysml-mcp tools.
	ArmMCP Arm = "mcp"
)

// Arms are both conditions in order.
var Arms = []Arm{ArmCompiler, ArmMCP}

// Cell is one (task, arm, sample) of a model's run.
type Cell struct {
	Task   string `json:"task"`
	Arm    Arm    `json:"arm"`
	Sample int    `json:"sample"`
}

// Attempt is one generation or repair and the compiler's verdict on it.
type Attempt struct {
	Round        int              `json:"round"`
	SourceDigest string           `json:"sourceDigest"`
	SourceBytes  int              `json:"sourceBytes"`
	OK           bool             `json:"ok"`
	Diagnostics  []Diagnostic     `json:"diagnostics,omitempty"`
	Unresolved   []string         `json:"unresolved,omitempty"`
	ToolCalls    []ToolCallRecord `json:"toolCalls,omitempty"`
	Usage        Usage            `json:"usage"`
	// Model is the identifier the provider reported for this attempt, if any.
	Model string `json:"model,omitempty"`
}

// CellResult is everything measured about one cell.
type CellResult struct {
	Cell
	Family     string    `json:"family"`
	Difficulty string    `json:"difficulty"`
	Attempts   []Attempt `json:"attempts"`
	// CompiledRound0 is whether the first generation validated.
	CompiledRound0 bool `json:"compiledRound0"`
	// CompiledFinal is whether any attempt validated within the rounds.
	CompiledFinal bool `json:"compiledFinal"`
	// RepairRounds is how many repairs were needed to compile; MaxRounds when
	// the cell never compiled.
	RepairRounds int `json:"repairRounds"`
	// Fidelity is the Jaccard overlap of the final document's structure with
	// the gold's, 0 when the final document did not compile.
	Fidelity float64 `json:"fidelity"`
	// Hallucinated are the unresolved names of the first generation.
	Hallucinated []string `json:"hallucinated,omitempty"`
	// PilotAccepted is the pilot's verdict on the final compiled document; nil
	// when the pilot did not run.
	PilotAccepted *bool  `json:"pilotAccepted,omitempty"`
	PilotOutput   string `json:"pilotOutput,omitempty"`
	ToolCalls     int    `json:"toolCalls"`
	// Error is set when the cell could not be measured (provider failure).
	Error string `json:"error,omitempty"`
	// FinalSource is the last document the model wrote; not written to baselines.
	FinalSource string `json:"-"`
	Elapsed     string `json:"elapsed"`
}

// Runner drives the generate–validate–repair loop for one provider.
type Runner struct {
	Provider  Provider
	Toolchain Toolchain
	Corpus    *Manifest
	// MaxRounds is how many repair rounds follow the first generation; zero
	// makes the first generation final.
	MaxRounds   int
	Temperature float64
	MaxTokens   int
	// MCP is the in-process server for the MCP arm; nil disables that arm.
	MCP *MCPArm
	// Tools are the specs offered to the model in the MCP arm; nil offers the
	// server's own. A replay sets the fixture's, so the request is the one the
	// model answered even after the server's descriptions changed.
	Tools []ToolSpec
	// Pilot runs the pilot validator over every final compiled document.
	Pilot bool
	// Log receives one line per event when non-nil.
	Log func(format string, args ...any)
}

func (r *Runner) tools() []ToolSpec {
	if r.Tools != nil {
		return r.Tools
	}
	return r.MCP.Specs()
}

func (r *Runner) logf(format string, args ...any) {
	if r.Log != nil {
		r.Log(format, args...)
	}
}

// RunCell runs one cell to completion.
func (r *Runner) RunCell(ctx context.Context, task Task, arm Arm, sample int) (res CellResult) {
	start := time.Now()
	cell := Cell{Task: task.ID, Arm: arm, Sample: sample}
	ctx = WithCell(ctx, cell)
	res = CellResult{Cell: cell, Family: task.Family, Difficulty: task.Difficulty}
	defer func() { res.Elapsed = time.Since(start).Round(time.Millisecond).String() }()
	maxRounds := r.MaxRounds
	if maxRounds < 0 {
		res.Error = fmt.Sprintf("negative repair rounds %d", maxRounds)
		return res
	}
	res.RepairRounds = maxRounds

	req := Request{System: SystemPrompt(), Temperature: r.Temperature, MaxTokens: r.MaxTokens}
	if arm == ArmMCP {
		if r.MCP == nil {
			res.Error = "mcp arm asked for without a server"
			return res
		}
		req.Tools = r.tools()
	}
	req.Messages = []Message{{Role: "user", Content: Primer(task)}}

	for round := 0; round <= maxRounds; round++ {
		source, attempt, err := r.generate(ctx, &req, round)
		if err != nil {
			res.Error = fmt.Sprintf("round %d: %v", round, err)
			r.logf("%s: %s", CellKey(cell), res.Error)
			return res
		}
		verdict, err := r.Toolchain.Validate(ctx, source)
		if err != nil {
			res.Error = fmt.Sprintf("round %d: %v", round, err)
			return res
		}
		attempt.OK = verdict.OK
		attempt.Diagnostics = verdict.Diagnostics
		attempt.Unresolved = verdict.Unresolved()
		res.Attempts = append(res.Attempts, attempt)
		res.FinalSource = source
		res.ToolCalls += len(attempt.ToolCalls)
		if round == 0 {
			res.CompiledRound0 = verdict.OK
			res.Hallucinated = attempt.Unresolved
		}
		if verdict.OK {
			res.CompiledFinal = true
			res.RepairRounds = round
			break
		}
		if round == maxRounds {
			break
		}
		req.Messages = append(req.Messages,
			Message{Role: "assistant", Content: source},
			Message{Role: "user", Content: Feedback(verdict.Errors())})
	}
	r.measure(ctx, task, &res)
	r.logf("%s: compiled=%v round0=%v repairs=%d fidelity=%.2f tools=%d", CellKey(cell), res.CompiledFinal, res.CompiledRound0, res.RepairRounds, res.Fidelity, res.ToolCalls)
	return res
}

// generate asks for one document, serving tool calls until the model answers
// with text.
func (r *Runner) generate(ctx context.Context, req *Request, round int) (string, Attempt, error) {
	attempt := Attempt{Round: round}
	exhausted := false
	for calls := 0; ; {
		resp, err := r.Provider.Complete(ctx, *req)
		if err != nil {
			return "", attempt, err
		}
		attempt.Usage.InputTokens += resp.Usage.InputTokens
		attempt.Usage.OutputTokens += resp.Usage.OutputTokens
		if resp.Model != "" {
			attempt.Model = resp.Model
		}
		if len(resp.ToolCalls) == 0 {
			source := ExtractModel(resp.Content)
			if source == "" {
				return "", attempt, errors.New("empty answer")
			}
			sum := sha256.Sum256([]byte(source))
			attempt.SourceDigest = hex.EncodeToString(sum[:])
			attempt.SourceBytes = len(source)
			return source, attempt, nil
		}
		if r.MCP == nil {
			return "", attempt, fmt.Errorf("model called tool %q in an arm without tools", resp.ToolCalls[0].Name)
		}
		if exhausted {
			return "", attempt, fmt.Errorf("model kept calling tools after its budget of %d calls", MaxToolCallsPerRound)
		}
		req.Messages = append(req.Messages, Message{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls})
		for _, call := range resp.ToolCalls {
			var text string
			var rec ToolCallRecord
			if calls < MaxToolCallsPerRound {
				text, rec = r.MCP.Call(ctx, round, call)
			} else {
				text, rec = r.MCP.Refuse(round, call)
			}
			calls++
			attempt.ToolCalls = append(attempt.ToolCalls, rec)
			req.Messages = append(req.Messages, Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: text})
		}
		if calls >= MaxToolCallsPerRound {
			exhausted = true
			req.Messages = append(req.Messages, Message{Role: "user", Content: "No more tool calls are available this round. Answer now with the complete document only."})
			req.Tools = nil
			defer func() { req.Tools = r.tools() }()
		}
	}
}

// measure scores the final document against the gold and, when asked, the pilot.
func (r *Runner) measure(ctx context.Context, task Task, res *CellResult) {
	if !res.CompiledFinal {
		return
	}
	gold, err := r.Corpus.GoldSource(task)
	if err != nil {
		res.Error = err.Error()
		return
	}
	goldJSON, err := r.Toolchain.Export(ctx, gold)
	if err != nil {
		res.Error = "gold export: " + err.Error()
		return
	}
	genJSON, err := r.Toolchain.Export(ctx, res.FinalSource)
	if err != nil {
		res.Error = "export: " + err.Error()
		return
	}
	goldS, err := ExtractStructure(goldJSON)
	if err != nil {
		res.Error = err.Error()
		return
	}
	genS, err := ExtractStructure(genJSON)
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Fidelity = Jaccard(goldS, genS)
	if r.Pilot && r.Toolchain.Pilot != "" {
		ok, out, err := r.Toolchain.PilotAccepts(ctx, res.FinalSource)
		if err != nil {
			res.Error = "pilot: " + err.Error()
			return
		}
		res.PilotAccepted = &ok
		res.PilotOutput = out
	}
}
