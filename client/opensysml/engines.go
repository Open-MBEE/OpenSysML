package opensysml

import (
	"context"
	"fmt"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// EngineInfo is one analysis engine the service answers with, as ListEngines
// and `sysml -engines` report it.
type EngineInfo struct {
	// Name is the engine's name, what an engine option spells.
	Name string
	// Authority is the strongest strength the engine's answers may carry.
	Authority string
	// Answers are the question kinds the engine answers.
	Answers []string
	// Bounds are the names of the bounds the engine reports on its answers.
	Bounds []string
	// Process is the external process the engine runs, empty for none.
	Process string
	// ProcessFound is where that process was found, empty when it was not.
	ProcessFound string
	// Ready is whether the engine can answer now.
	Ready bool
	// Unavailable says why the engine cannot answer, empty when Ready.
	Unavailable string
}

// Bound is one bound an engine's answer ran under, with whether reaching it
// was what stopped the run.
type Bound struct {
	// Name is the bound's name as the budget spells it: "runs", "depth", "steps".
	Name string
	// Limit is the bound's value.
	Limit int64
	// Reached is true when the run stopped at the limit, which lowers the
	// answer's strength.
	Reached bool
}

// Standing is how strongly an answer stands: the engine that answered, the
// strength of its evidence and the bounds it ran under. Zero from a service
// without CapabilityEngines.
type Standing struct {
	// Engine is the engine that answered, as ListEngines names it.
	Engine string
	// Strength is "not covered", "observed", "witnessed", "bounded" or "proved".
	Strength string
	// Bounds are the bounds the answer ran under.
	Bounds []Bound
}

// The engine selections every call accepts beside an engine's name.
const (
	// EngineAuto lets the service pick the engine, which sending no engine does.
	EngineAuto = "auto"
	// EngineAll puts the question to every engine that covers it and composes
	// their answers.
	EngineAll = "all"
)

// WithEngine names the engine that answers a verification, as `sysml -engine`
// spells it: an engine's name, EngineAuto or EngineAll. Requires the engines
// capability, checked before anything is sent.
func WithEngine(engine string) VerifyOption {
	return func(o *verifyOptions) { o.engine = engine }
}

// Engine names the engine that answers an analysis, as WithEngine does for a
// verification. Requires the engines capability, checked before anything is
// sent.
func Engine(engine string) AnalysisOption {
	return func(o *analysisOptions) { o.engine = engine }
}

// engineField is the engine as sent: empty for auto, which every service
// reads as such.
func engineField(engine string) string {
	if engine == EngineAuto {
		return ""
	}
	return engine
}

// requireEngine refuses to send a named engine to a service without the
// engines capability, which would answer under auto rather than refuse it.
func (c *client) requireEngine(ctx context.Context, engine string) error {
	if engineField(engine) == "" {
		return nil
	}
	info, err := c.serverInfo(ctx)
	if err != nil {
		return err
	}
	if !info.Has(CapabilityEngines) {
		return &StatusError{
			Code:    CodeUnimplemented,
			Message: fmt.Sprintf("capability %q is unavailable", CapabilityEngines),
		}
	}
	return nil
}

func (c *client) ListEngines(ctx context.Context) ([]EngineInfo, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	resp, err := c.caller.listEngines(ctx, &pb.ListEnginesRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]EngineInfo, 0, len(resp.Engines))
	for _, engine := range resp.Engines {
		if engine == nil {
			continue
		}
		out = append(out, EngineInfo{
			Name:         engine.Name,
			Authority:    engine.Authority,
			Answers:      append([]string(nil), engine.Answers...),
			Bounds:       append([]string(nil), engine.Bounds...),
			Process:      engine.Process,
			ProcessFound: engine.ProcessFound,
			Ready:        engine.Ready,
			Unavailable:  engine.Unavailable,
		})
	}
	return out, nil
}

func standingFromProto(engine, strength string, bounds []*pb.Bound) Standing {
	out := Standing{Engine: engine, Strength: strength}
	for _, bound := range bounds {
		if bound == nil {
			continue
		}
		out.Bounds = append(out.Bounds, Bound{Name: bound.Name, Limit: bound.Limit, Reached: bound.Reached})
	}
	return out
}
