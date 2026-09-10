package opensysml_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

func engineNames(engines []opensysml.EngineInfo) []string {
	names := make([]string, 0, len(engines))
	for _, engine := range engines {
		names = append(names, engine.Name)
	}
	return names
}

func TestListEnginesNamesEveryEngineInOrder(t *testing.T) {
	client := newClient(t)
	engines, err := client.ListEngines(context.Background())
	if err != nil {
		t.Fatalf("ListEngines: %v", err)
	}
	names := engineNames(engines)
	if !slices.IsSorted(names) {
		t.Errorf("engines = %v, want name order", names)
	}
	for _, want := range []string{"explore", "run", "solve", "sweep"} {
		if !slices.Contains(names, want) {
			t.Errorf("engines = %v, want %s among them", names, want)
		}
	}
	for _, engine := range engines {
		if engine.Authority == "" || len(engine.Answers) == 0 {
			t.Errorf("%s: authority %q answers %v, want both reported", engine.Name, engine.Authority, engine.Answers)
		}
		if engine.Name == "run" && (engine.Authority != "observed" || !engine.Ready) {
			t.Errorf("run: authority %q ready %v, want observed and ready", engine.Authority, engine.Ready)
		}
	}
}

func TestListEnginesAnswersTheSameRemotely(t *testing.T) {
	remote := dialClient(t, startService(t))
	got, err := remote.ListEngines(context.Background())
	if err != nil {
		t.Fatalf("remote ListEngines: %v", err)
	}
	want, err := newClient(t).ListEngines(context.Background())
	if err != nil {
		t.Fatalf("in-process ListEngines: %v", err)
	}
	if !slices.Equal(engineNames(got), engineNames(want)) {
		t.Errorf("remote engines = %v, in-process %v", engineNames(got), engineNames(want))
	}
}

func TestTheClientNamesTheEnginesCapability(t *testing.T) {
	client := newClient(t)
	info, err := client.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if !info.Has(opensysml.CapabilityEngines) {
		t.Errorf("capabilities = %v, want %s", info.Capabilities, opensysml.CapabilityEngines)
	}
}

// Every verdict carries the standing of its answer: which engine answered,
// how strongly, and the bounds it ran under.
func TestVerdictsCarryTheirStanding(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, verificationSource)

	verification, err := client.VerifyConstraint(ctx, model, "Demo::Vehicle::massLight",
		opensysml.Against("Demo::sedan"))
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if got := verification.Verdict.Standing; got.Engine != "run" || got.Strength != "observed" {
		t.Errorf("constraint standing = %+v, want run at observed", got)
	}

	satisfaction, err := client.VerifySatisfaction(ctx, model, "Demo::analysis")
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if len(satisfaction.Verdicts) == 0 {
		t.Fatal("satisfaction reports no verdicts")
	}
	for _, verdict := range satisfaction.Verdicts {
		if verdict.Standing.Engine != "run" || verdict.Standing.Strength == "" {
			t.Errorf("%s: standing = %+v, want run with a strength", verdict.Element, verdict.Standing)
		}
	}

	calculation, err := client.EvaluateCalc(ctx, model, "Demo::add", opensysml.Real(1), opensysml.Real(2))
	if err != nil {
		t.Fatalf("EvaluateCalc: %v", err)
	}
	if got := calculation.Standing; got.Engine != "run" || got.Strength != "observed" {
		t.Errorf("calculation standing = %+v, want run at observed", got)
	}
	for _, bound := range calculation.Standing.Bounds {
		if bound.Reached {
			t.Errorf("bound %s (%d) reported reached on a run that finished", bound.Name, bound.Limit)
		}
	}
}

// WithEngine and Engine put the question to the engine named; auto and the
// default select the same engine the service would.
func TestANamedEngineAnswers(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, verificationSource)

	for _, engine := range []string{"", opensysml.EngineAuto, "run"} {
		verification, err := client.VerifyConstraint(ctx, model, "Demo::Vehicle::massLight",
			opensysml.Against("Demo::sedan"), opensysml.WithEngine(engine))
		if err != nil {
			t.Fatalf("VerifyConstraint under %q: %v", engine, err)
		}
		if !verification.Verdict.Holds || verification.Verdict.Standing.Engine != "run" {
			t.Errorf("under %q: holds %v by %q, want holds by run",
				engine, verification.Verdict.Holds, verification.Verdict.Standing.Engine)
		}
	}
	analysis, err := client.RunAnalysis(ctx, parse(t, client, verdictSource), "Demo::checkBound", opensysml.Engine("run"))
	if err != nil {
		t.Fatalf("RunAnalysis under run: %v", err)
	}
	if analysis.Standing.Engine != "run" || analysis.Standing.Strength == "" {
		t.Errorf("analysis standing = %+v, want run with a strength", analysis.Standing)
	}
}

// An engine the service does not register is refused as an invalid argument
// naming it, on every path that selects one.
func TestAnUnknownEngineIsInvalidArgument(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, verificationSource)

	_, err := client.VerifyConstraint(ctx, model, "Demo::Vehicle::massLight",
		opensysml.Against("Demo::sedan"), opensysml.WithEngine("oracle"))
	if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), "oracle") {
		t.Errorf("VerifyConstraint: err = %v, want CodeInvalidArgument naming oracle", err)
	}
	_, err = client.VerifyRequirement(ctx, model, "Demo::Vehicle::lightEnough",
		opensysml.Against("Demo::sedan"), opensysml.WithEngine("oracle"))
	if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), "oracle") {
		t.Errorf("VerifyRequirement: err = %v, want CodeInvalidArgument naming oracle", err)
	}
	_, err = client.VerifySatisfaction(ctx, model, "Demo::analysis", opensysml.WithEngine("oracle"))
	if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), "oracle") {
		t.Errorf("VerifySatisfaction: err = %v, want CodeInvalidArgument naming oracle", err)
	}
	_, err = client.RunAnalysis(ctx, parse(t, client, verdictSource), "Demo::checkBound", opensysml.Engine("oracle"))
	if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), "oracle") {
		t.Errorf("RunAnalysis: err = %v, want CodeInvalidArgument naming oracle", err)
	}
}

// A satisfaction verification has no subject of its own to name.
func TestVerifySatisfactionRefusesASubject(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, verificationSource)
	_, err := client.VerifySatisfaction(context.Background(), model, "Demo::analysis",
		opensysml.Against("Demo::sedan"))
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("err = %v, want CodeInvalidArgument", err)
	}
}
