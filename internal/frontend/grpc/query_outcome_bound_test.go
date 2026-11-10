package grpc

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
)

// detailBudget is what a QueryOutcome detail may take of the 8 KiB status
// trailer gRPC allows, leaving the status and message their share.
const detailBudget = 4096

// TestQueryFailuresBoundTheFrontier: a frontier naming hundreds of features is
// cut to the spellings nearest the step's argument, with the whole count kept,
// so the detail fits a status trailer whatever the metamodel's size.
func TestQueryFailuresBoundTheFrontier(t *testing.T) {
	features := make([]queryexec.FeatureRef, 0, 400)
	for i := 0; i < 400; i++ {
		features = append(features, queryexec.FeatureRef{Name: fmt.Sprintf("property%03d", i)})
	}
	features = append(features, queryexec.FeatureRef{Name: "color", Type: "String"})
	failures := queryFailures([]queryexec.Failure{{
		Step:     queryexec.Step{Kind: queryexec.StepFeature, Argument: "colour"},
		Expected: "a property of the metamodel",
		Frontier: queryexec.Frontier{Features: features, Nearest: "color"},
	}})
	frontier := failures[0].Frontier
	if len(frontier.Features) != frontierLimit || frontier.FeaturesTotal != 401 {
		t.Fatalf("features = %d (total %d), want %d of 401", len(frontier.Features), frontier.FeaturesTotal, frontierLimit)
	}
	if frontier.Features[0].Name != "color" || frontier.Features[0].Type != "String" {
		t.Fatalf("nearest spelling is not first: %v", frontier.Features)
	}
	outcome := queryOutcomeMessage(queryexec.Outcome{Path: &queryexec.PathError{Failures: []queryexec.Failure{{
		Step:     queryexec.Step{Kind: queryexec.StepFeature, Argument: "colour"},
		Frontier: queryexec.Frontier{Features: features},
	}}}})
	if size := proto.Size(outcome); size > detailBudget {
		t.Fatalf("detail is %d bytes, over the %d budget", size, detailBudget)
	}
}

// TestQueryOutcomeBoundsTheFanOut: a missing feature's fan-out records one
// failure per failing element; the wire form keeps the first failureLimit
// with the whole count, and a modelError's findings and path failures alike.
func TestQueryOutcomeBoundsTheFanOut(t *testing.T) {
	failures := make([]queryexec.Failure, 0, 1000)
	findings := make([]queryexec.Finding, 0, 1000)
	for i := 0; i < 1000; i++ {
		features := make([]queryexec.FeatureRef, 0, 40)
		for j := 0; j < 40; j++ {
			features = append(features, queryexec.FeatureRef{Name: fmt.Sprintf("feature%02d", j), Type: "Real"})
		}
		failures = append(failures, queryexec.Failure{
			Step:     queryexec.Step{Kind: queryexec.StepFeature, Argument: "mass"},
			Expected: "a feature named mass on the element",
			Frontier: queryexec.Frontier{At: fmt.Sprintf("Lander::lander::part%04d", i), Features: features},
		})
		findings = append(findings, queryexec.Finding{Step: queryexec.Step{Kind: queryexec.StepFeature, Argument: "mass"}, Element: fmt.Sprintf("Lander::lander::part%04d", i), Observed: "none"})
	}
	path := queryOutcomeMessage(queryexec.Outcome{Path: &queryexec.PathError{Failures: failures}})
	if len(path.Failures) != failureLimit || path.FailuresTotal != 1000 || path.Failures[0].Frontier.At != "Lander::lander::part0000" {
		t.Fatalf("failures = %d of %d, first at %q", len(path.Failures), path.FailuresTotal, path.Failures[0].Frontier.At)
	}
	if size := proto.Size(path); size > detailBudget {
		t.Fatalf("pathError detail is %d bytes, over the %d budget", size, detailBudget)
	}
	model := queryOutcomeMessage(queryexec.Outcome{Model: &queryexec.ModelError{Findings: findings, PathFailures: failures}})
	if len(model.Findings) != failureLimit || model.FindingsTotal != 1000 || len(model.PathFailures) != failureLimit || model.PathFailuresTotal != 1000 {
		t.Fatalf("findings = %d of %d, path failures = %d of %d", len(model.Findings), model.FindingsTotal, len(model.PathFailures), model.PathFailuresTotal)
	}
	if size := proto.Size(model); size > detailBudget {
		t.Fatalf("modelError detail is %d bytes, over the %d budget", size, detailBudget)
	}
}

// TestQueryFailuresKeepSameNamedMembers: a member declared under several
// metaclasses keeps every (name, metaclass) pair the record holds.
func TestQueryFailuresKeepSameNamedMembers(t *testing.T) {
	members := []queryexec.Member{{Name: "engine", Metaclass: "PartUsage"}, {Name: "engine", Metaclass: "PartDefinition"}, {Name: "wheel", Metaclass: "PartUsage"}}
	for i := 0; i < 20; i++ {
		members = append(members, queryexec.Member{Name: fmt.Sprintf("other%02d", i), Metaclass: "PartUsage"})
	}
	frontier := queryFailures([]queryexec.Failure{{
		Step:     queryexec.Step{Kind: queryexec.StepNamed, Argument: "engin"},
		Frontier: queryexec.Frontier{At: "Lander", Members: members, Nearest: "engine"},
	}})[0].Frontier
	if len(frontier.Members) != frontierLimit || frontier.MembersTotal != 23 {
		t.Fatalf("members = %d of %d", len(frontier.Members), frontier.MembersTotal)
	}
	if frontier.Members[0].Name != "engine" || frontier.Members[0].Metaclass != "PartUsage" || frontier.Members[1].Name != "engine" || frontier.Members[1].Metaclass != "PartDefinition" {
		t.Fatalf("same-named members are not kept together first: %v", frontier.Members)
	}
}

// TestQueryUnknownPropertyDetailFitsTheTrailer: the Query RPC's unknown-property
// detail, whose frontier is the whole metamodel's property table, stays within
// the trailer budget and still names the nearest spelling.
func TestQueryUnknownPropertyDetailFitsTheTrailer(t *testing.T) {
	srv, hash, _ := runQueryOnSource(t, `package Paint { part def Can { attribute color : ScalarValues::String; } }`, &pb.Query{})
	for _, c := range []struct {
		request *pb.QueryRequest
		nearest string
	}{
		{&pb.QueryRequest{ModelHash: hash, Query: &pb.Query{OrderBy: []string{"colour"}}}, ""},
		{&pb.QueryRequest{ModelHash: hash, OslcQuery: `oslc.where=sysml:nmae="x"`}, "sysml:name"},
	} {
		q := c.request
		_, err := srv.Query(context.Background(), q)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%v: code = %v", q, connect.CodeOf(err))
		}
		outcome := queryOutcomeOf(t, err)
		frontier := outcome.Failures[0].Frontier
		if size := proto.Size(outcome); size > detailBudget {
			t.Fatalf("detail is %d bytes, over the %d budget: %d of %d features", size, detailBudget, len(frontier.Features), frontier.FeaturesTotal)
		}
		if len(frontier.Features) > frontierLimit || frontier.FeaturesTotal <= frontierLimit || frontier.Nearest != c.nearest {
			t.Fatalf("frontier = %d of %d features, nearest %q", len(frontier.Features), frontier.FeaturesTotal, frontier.Nearest)
		}
	}
}
