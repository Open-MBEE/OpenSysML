package grpc_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

const nodeParameterQueryModel = `package P {
    attribute def Sig;
    action def Base {
        attribute count : ScalarValues::Integer;
        attribute total : ScalarValues::Integer;
        action nested;
        action assignCount assign count := count + 1;
        action whileAct while count < 5 { assign count := count + 1; }
        action forAct for i in (1, 2, 3) { assign count := i; }
        action ifAct if total > 3 { assign count := 1; } else { assign count := 2; }
        action termArg terminate nested;
        action termBare terminate;
        action acc1 accept r : Sig;
        action s send new Sig() to nested;
    }
}`

// A query names the parameter an action's notation states as an evaluation names it: by its role,
// under its action. None reads empty, as the expression that states it, or drops its property.
func TestOSLCQueryNamesTheParametersAnActionNodeStates(t *testing.T) {
	srv := grpc.MustNewServiceForTest(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: nodeParameterQueryModel},
	})
	if err != nil {
		t.Fatal(err)
	}
	session := repl.NewSession()
	if result := session.Submit(nodeParameterQueryModel); len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	const terminate = "Actions::TerminateAction::"
	for _, row := range []struct {
		node, properties string
		want             map[string]string
	}{
		{"assignCount", "parameter,input,feature,ownedFeature", map[string]string{
			"parameter": "P::Base::assignCount::target", "input": "P::Base::assignCount::target",
			"feature": "P::Base::assignCount::target", "ownedFeature": "P::Base::assignCount::target",
		}},
		{"forAct", "parameter,input,feature", map[string]string{
			"parameter": "P::Base::forAct::i", "input": "P::Base::forAct::i", "feature": "P::Base::forAct::i",
		}},
		{"whileAct", "parameter,input,feature", map[string]string{
			"parameter": "P::Base::whileAct::whileTest", "input": "P::Base::whileAct::whileTest",
			"feature": "P::Base::whileAct::whileTest",
		}},
		{"ifAct", "parameter,feature", map[string]string{
			"parameter": "P::Base::ifAct::ifTest", "feature": "P::Base::ifAct::ifTest",
		}},
		{"termArg", "parameter,input,ownedFeature,feature", map[string]string{
			"parameter": "P::Base::termArg::terminatedOccurrence", "input": "P::Base::termArg::terminatedOccurrence",
			"ownedFeature": "P::Base::termArg::terminatedOccurrence", "feature": "P::Base::termArg::terminatedOccurrence",
		}},
		{"termBare", "parameter,input,ownedFeature,feature", map[string]string{
			"parameter": terminate + "terminatedOccurrence", "input": terminate + "terminatedOccurrence",
			"feature": terminate + "terminateOccurrence",
		}},
		{"acc1", "parameter,input,output", map[string]string{
			"parameter": "P::Base::acc1::r", "input": "P::Base::acc1::r",
		}},
		{"s", "parameter,input,feature,ownedFeature", map[string]string{
			"parameter": "P::Base::s::payload", "input": "P::Base::s::payload",
			"feature": "P::Base::s::payload", "ownedFeature": "P::Base::s::payload",
		}},
	} {
		names := strings.Split(row.properties, ",")
		text := fmt.Sprintf(`oslc.where=sysml:name=%q&oslc.select=sysml:%s`, row.node, strings.Join(names, ",sysml:"))
		resp, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: parsed.ModelHash, OslcQuery: text})
		if err != nil {
			t.Fatalf("%s: Query: %v", row.node, err)
		}
		lines, err := session.Query(text)
		if err != nil {
			t.Fatalf("%s: REPL query: %v", row.node, err)
		}
		if len(resp.Elements) != 1 || len(lines) != 1 {
			t.Fatalf("%s: %d elements, %d lines, want one of each", row.node, len(resp.Elements), len(lines))
		}
		line := fmt.Sprintf("%s  %s", resp.Elements[0].Id, resp.Elements[0].Type)
		for _, name := range names {
			got, answered := resp.Elements[0].Properties[name]
			if want, expected := row.want[name]; got != want || answered != expected {
				t.Errorf("%s: Query %s = %q (answered %v), want %q (answered %v)", row.node, name, got, answered, want, expected)
			}
			if answered {
				line += fmt.Sprintf("  sysml:%s=%s", name, got)
			}
		}
		if lines[0] != line {
			t.Errorf("%s: REPL line = %q, gRPC line = %q", row.node, lines[0], line)
		}
	}
}
