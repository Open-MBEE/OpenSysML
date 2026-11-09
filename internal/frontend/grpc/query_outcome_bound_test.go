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
const detailBudget = 2048

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
