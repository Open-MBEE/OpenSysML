package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const compareModelSource = `package Cfg {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	part def Probe {
		attribute total : Real = 1.0;
		action shaky {
			attribute k : Integer = uniformInteger(0, 1);
			first start;
			then assign total := 10.0 / k;
			then done;
		}
		action steady { first start; then assign total := 4.0; then done; }
	}
	action def 'Group 0' { part target : Probe; perform action run ::> target.shaky; }
	action def 'Group 1' { part target : Probe; perform action run ::> target.steady; }
}`

const compareSidecar = `{"source":"probe.xmi","configurations":[
 {"id":"_c1","name":"Cfg::'Group 1'","runs":2,"target":"target","behavior":"run","resultLocation":"Results",
  "observables":["total"],"snapshots":[{"id":"_r1","values":{"total":4.0}},{"id":"_r2","values":{"total":6.0}}]},
 {"id":"_c0","name":"Cfg::'Group 0'","runs":3,"target":"target","behavior":"run","observables":["total"],
  "snapshots":[{"id":"_s1","values":{"total":10.0}}]},
 {"id":"_cn","name":"Cfg::Nothing","observables":[],"snapshots":[],"notes":["migrated without a behavior"]}]}`

func compareResults(t *testing.T, srv *Service, req *pb.CompareResultsRequest) *pb.CompareResultsResponse {
	t.Helper()
	resp, err := srv.CompareResults(context.Background(), req)
	if err != nil {
		t.Fatalf("CompareResults: %v", err)
	}
	return resp
}

func TestCompareResultsTablesTheToolBesideTheRuns(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, compareModelSource)
	seed := uint64(7)
	resp := compareResults(t, srv, &pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Seed: &seed, Actions: []string{"Group 1"}})
	if resp.Error != "" || resp.Source != "probe.xmi" || len(resp.Configurations) != 1 {
		t.Fatalf("response = %v; want the one configuration of probe.xmi", resp)
	}
	c := resp.Configurations[0]
	if c.Status != "holds" || c.Name != "Cfg::'Group 1'" || c.Runs != 2 || c.Completed != 2 || c.StoredRuns != 2 || c.Seed == nil || *c.Seed != 7 || len(c.Rows) != 2 {
		t.Fatalf("configuration = %v; want 2 completed runs of 2 seeded from 7 beside 2 stored", c)
	}
	if len(c.Observables) != 1 {
		t.Fatalf("observables = %v; want total", c.Observables)
	}
	o := c.Observables[0]
	if o.Stored != "total" || o.Feature != "target.total" || !o.Compared || o.Tool == nil || o.Run == nil || o.Difference == nil {
		t.Fatalf("observable = %v; want total compared with target.total", o)
	}
	if o.Tool.Count != 2 || o.Tool.Mean != 5 || o.Run.Count != 2 || o.Run.Mean != 4 || o.Difference.Mean != "-20.0%" || o.Difference.Min != "+0.0%" {
		t.Errorf("tool %v, run %v, difference %v; want means 5 and 4, -20.0%%", o.Tool, o.Run, o.Difference)
	}
	text := strings.Join(c.Lines, "\n")
	for _, want := range []string{"compare Cfg::'Group 1' — 2 stored run(s) in Results; 2 run(s) by OpenSysML, draws random, seed 7", "| tool", "OpenSysML (target.total)", "difference"} {
		if !strings.Contains(text, want) {
			t.Errorf("lines:\n%s\nwant %q", text, want)
		}
	}
}

func TestCompareResultsReportsFailuresAndUnresolvedConfigurations(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, compareModelSource)
	seed := uint64(1)
	resp := compareResults(t, srv, &pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Seed: &seed, Runs: 4})
	if len(resp.Configurations) != 3 {
		t.Fatalf("configurations = %v; want three", resp.Configurations)
	}
	byName := map[string]*pb.ConfigurationComparison{}
	for _, c := range resp.Configurations {
		byName[c.Name] = c
	}
	if c := byName["Cfg::'Group 1'"]; c == nil || c.Status != "holds" || c.Runs != 4 || c.Completed != 4 {
		t.Errorf("Group 1 = %v; want 4 completed runs", c)
	}
	if c := byName["Cfg::'Group 0'"]; c == nil || c.Status != "fails" || len(c.Errors) == 0 || c.Completed == 4 {
		t.Errorf("Group 0 = %v; want a failing run dividing by zero", c)
	}
	if c := byName["Cfg::Nothing"]; c == nil || c.Status != "unresolved" || !strings.Contains(c.Unresolved, "performs no migrated behavior") || !strings.Contains(c.Unresolved, "migrated without a behavior") {
		t.Errorf("Nothing = %v; want it unresolved with its note", c)
	}
	unknown := compareResults(t, srv, &pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Seed: &seed, Actions: []string{"Missing"}})
	if len(unknown.Configurations) != 1 || unknown.Configurations[0].Status != "unresolved" || !strings.Contains(unknown.Configurations[0].Unresolved, "no configuration is named Missing") {
		t.Errorf("unknown action = %v; want it refused by name", unknown.Configurations)
	}
	unseeded := compareResults(t, srv, &pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Actions: []string{"Group 0"}})
	if c := unseeded.Configurations[0]; c.Status != "unresolved" || !strings.Contains(c.Unresolved, "draw at random") {
		t.Errorf("unseeded random runs = %v; want them refused", c)
	}
	fixed := compareResults(t, srv, &pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Actions: []string{"Group 1"}, Draws: "average"})
	if c := fixed.Configurations[0]; c.Status != "holds" || c.Draws != "average" || c.Seed != nil {
		t.Errorf("fixed draws = %v; want the runs made without a seed", c)
	}
}

func TestGRPCRobustnessCompareResults(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, compareModelSource)
	bad := -1.0
	cases := map[string]struct {
		req  *pb.CompareResultsRequest
		code connect.Code
	}{
		"no results":      {&pb.CompareResultsRequest{ModelHash: hash}, connect.CodeInvalidArgument},
		"invalid results": {&pb.CompareResultsRequest{ModelHash: hash, Results: "{"}, connect.CodeInvalidArgument},
		"negative runs":   {&pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Runs: -1}, connect.CodeInvalidArgument},
		"bad draws":       {&pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Draws: "sometimes"}, connect.CodeInvalidArgument},
		"bad clock step":  {&pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, ClockStep: &bad}, connect.CodeInvalidArgument},
		"empty observe":   {&pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Observe: []*pb.ObservablePair{{Feature: "x"}}}, connect.CodeInvalidArgument},
		"stale model":     {&pb.CompareResultsRequest{ModelHash: strings.Repeat("0", 64), Results: compareSidecar}, connect.CodeNotFound},
		"bad engine":      {&pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Engine: "nothing-of-the-kind"}, connect.CodeInvalidArgument},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := srv.CompareResults(context.Background(), tc.req)
			if connect.CodeOf(err) != tc.code {
				t.Fatalf("error %v; want code %v", err, tc.code)
			}
		})
	}
	t.Run("withheld capability", func(t *testing.T) {
		without := mustNewServiceWithout(t, CapabilityCompareResults)
		_, err := without.CompareResults(context.Background(), &pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar})
		if connect.CodeOf(err) != connect.CodeUnimplemented {
			t.Fatalf("error %v; want unimplemented", err)
		}
	})
	t.Run("a cancelled caller is told so", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		seed := uint64(1)
		_, err := srv.CompareResults(ctx, &pb.CompareResultsRequest{ModelHash: hash, Results: compareSidecar, Seed: &seed})
		if err == nil {
			t.Fatalf("a cancelled comparison answered")
		}
	})
}
