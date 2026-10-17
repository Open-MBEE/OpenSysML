package runtimetools

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// CheckBehaviorInput searches every schedule of a behavior.
type CheckBehaviorInput struct {
	ModelInput
	ActionSymbolID       string                    `json:"actionSymbolId,omitempty" jsonschema:"qualified name of the action definition or usage to check; give this or stateMachineSymbolId"`
	StateMachineSymbolID string                    `json:"stateMachineSymbolId,omitempty" jsonschema:"qualified name of the state definition or usage to check; give this or actionSymbolId"`
	Inputs               map[string]map[string]any `json:"inputs,omitempty" jsonschema:"input parameter name to value, each a protojson Value. Example arms: {\"intValue\":\"7\"}, {\"realValue\":1.5}, {\"boolValue\":true}, {\"stringValue\":\"x\"}"`
	PerformerSymbolID    string                    `json:"performerSymbolId,omitempty" jsonschema:"qualified name of a part/usage, or a path from one, whose object performs the behavior; empty performs it outside any object"`
	CheckProperty        []string                  `json:"checkProperty,omitempty" jsonschema:"constraints or requirements evaluated at every stable state; one false at a state is a violation (-check-property)"`
	CheckDiverge         []string                  `json:"checkDiverge,omitempty" jsonschema:"features whose final value the schedule may decide (-check-diverge); the action's own bare, a node's as node.pin, the performer's as this.<name>"`
	CheckInput           []string                  `json:"checkInput,omitempty" jsonschema:"inputs left free for the symbolic engine to range over (-check-input)"`
	CheckAssume          []string                  `json:"checkAssume,omitempty" jsonschema:"constraints or requirements the symbolic engine assumes (-check-assume)"`
	CheckWitness         bool                      `json:"checkWitness,omitempty" jsonschema:"carry each violation's and divergent value's witness file in the answer, for a replay schedule to follow (-check-witness)"`
	CheckDepth           int32                     `json:"checkDepth,omitempty" jsonschema:"moves one schedule may make; 0 is the engine's default (-check-depth)"`
	CheckStates          int32                     `json:"checkStates,omitempty" jsonschema:"distinct states the search may visit; 0 is the engine's default (-check-states)"`
	CheckUnroll          int32                     `json:"checkUnroll,omitempty" jsonschema:"loop iterations the symbolic engine unrolls; 0 is its default (-check-unroll)"`
	CheckTimeout         float64                   `json:"checkTimeout,omitempty" jsonschema:"seconds the search and each solver query may take; 0 is none (-check-timeout)"`
	Engine               string                    `json:"engine,omitempty" jsonschema:"the engine asked: check (the default), smt, or one list_engines names"`
}

func (g *group) registerCheck(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "check_behavior",
		Description: "Search every schedule of an action or state machine (the CheckBehavior RPC, the REPL's %check-property/%check-diverge " +
			"settings about %action/%state): a property false at a stable state, a deadlock, a failure or a feature whose final value the " +
			"schedule decides is witnessed and replayed. `verdict` is holds, fails or undecided; `violations`, `divergences` and `finals` " +
			"carry each witness as its replay schedule, draws and choices, and the witness file when asked for. With checkInput or " +
			"checkAssume the symbolic engine answers with `claim`, the inputs it ranged over and its witnesses. Bounded by checkDepth, " +
			"checkStates and checkTimeout. The result is the CheckBehaviorResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Check behavior"),
	}, func(ctx context.Context, call *mcp.CallToolRequest, in CheckBehaviorInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.CheckBehaviorRequest{}
		var resp *pb.CheckBehaviorResponse
		subject := in.ActionSymbolID
		if subject == "" {
			subject = in.StateMachineSymbolID
		}
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			err := withProgress(ctx, call, "check_behavior "+subject, func() (err error) {
				resp, err = g.svc.CheckBehavior(ctx, req)
				return err
			})
			return resp, err
		}, func() string {
			var b strings.Builder
			b.WriteString(errorText(resp.GetError()))
			for _, line := range resp.GetLines() {
				b.WriteString(line)
				b.WriteByte('\n')
			}
			if resp.GetReason() != "" && len(resp.GetLines()) == 0 {
				b.WriteString(resp.GetReason())
				b.WriteByte('\n')
			}
			return b.String()
		})
	})
}
