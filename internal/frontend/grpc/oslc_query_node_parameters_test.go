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
    part def Q { attribute q : ScalarValues::Integer; }
    action def Base {
        part p : Q;
        part other : Q;
        action assignChain assign p.q := 1;
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
        action s send new Sig() to other;
    }
}`

// A query reads the parameter an action's notation states as the expression written for it, as it reads a
// send's payload, and names one stated with none by its role under its action: none reads empty or is dropped.
func TestOSLCQueryReadsTheParametersAnActionNodeStates(t *testing.T) {
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
		{"assignChain", "parameter,feature", map[string]string{"parameter": "p", "feature": "p"}},
		{"forAct", "parameter,input,feature", map[string]string{
			"parameter": "P::Base::forAct::i", "input": "P::Base::forAct::i", "feature": "P::Base::forAct::i",
		}},
		{"whileAct", "parameter,input,feature", map[string]string{
			"parameter": "count < 5", "input": "count < 5", "feature": "count < 5",
		}},
		{"ifAct", "parameter,feature", map[string]string{
			"parameter": "total > 3", "feature": "total > 3",
		}},
		{"termArg", "parameter,input,ownedFeature,feature", map[string]string{
			"parameter": "nested", "input": "nested", "ownedFeature": "nested", "feature": "nested",
		}},
		{"termBare", "parameter,input,ownedFeature,feature", map[string]string{
			"parameter": terminate + "terminatedOccurrence", "input": terminate + "terminatedOccurrence",
			"feature": terminate + "terminateOccurrence",
		}},
		{"acc1", "parameter,input,output", map[string]string{
			"parameter": "P::Base::acc1::r", "input": "P::Base::acc1::r",
		}},
		{"s", "parameter,input,feature,ownedFeature", map[string]string{
			"parameter": "new Sig()", "input": "new Sig()", "feature": "new Sig()", "ownedFeature": "new Sig()",
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

// A filter matches such a parameter by the text a query reads it as, in a body no name reaches included.
func TestOSLCQueryFiltersByTheParametersAnActionNodeStates(t *testing.T) {
	srv := grpc.MustNewServiceForTest(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: nodeParameterQueryModel},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ parameter, element, read string }{
		{"P::Base::assignCount::target", "P::Base::assignCount", "P::Base::assignCount::target"},
		{"P::Base::forAct::i", "P::Base::forAct", "P::Base::forAct::i"},
		{"new Sig()", "P::Base::s", "new Sig()"},
		{"nested", "P::Base::termArg", "nested"},
		{"p", "P::Base::assignChain", "p"},
		{"P::Base::forAct::@0::@0::target", "P::Base::forAct::@0::@0", "P::Base::forAct::@0::@0::target"},
	} {
		text := fmt.Sprintf(`oslc.where=sysml:parameter=%q&oslc.select=sysml:parameter`, row.parameter)
		resp, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: parsed.ModelHash, OslcQuery: text})
		if err != nil {
			t.Fatalf("%s: Query: %v", row.parameter, err)
		}
		if len(resp.Elements) != 1 || resp.Elements[0].Id != row.element || resp.Elements[0].Properties["parameter"] != row.read {
			t.Errorf("parameter %q selects %v, want %s reading %q", row.parameter, resp.Elements, row.element, row.read)
		}
	}
}

// An assignment no name reaches, in a loop or conditional body or after a `then`, names its target under its
// own positional identity: none reads an empty value.
func TestOSLCQueryNamesTheTargetOfAnUnnamedAssignment(t *testing.T) {
	const model = `package U {
    private import ScalarValues::*;
    action def A {
        attribute n : Integer := 0;
        first start;
        then action w while n < 3 { assign n := n + 1; }
        then assign n := 7;
        then done;
        action l loop { assign n := 4; } until n > 3;
        action f for i in (1, 2) { assign n := i; }
        action cond if n > 1 { assign n := 2; } else { assign n := 3; }
    }
}`
	srv := grpc.MustNewServiceForTest(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: model},
	})
	if err != nil {
		t.Fatal(err)
	}
	session := repl.NewSession()
	if result := session.Submit(model); len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	const text = `oslc.where=rdf:type="AssignmentActionUsage"&oslc.select=sysml:parameter,sysml:input,sysml:feature,sysml:ownedFeature`
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: parsed.ModelHash, OslcQuery: text})
	if err != nil {
		t.Fatal(err)
	}
	lines, err := session.Query(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Elements) != 6 || len(lines) != 6 {
		t.Fatalf("%d elements, %d lines, want the six assignments: %v", len(resp.Elements), len(lines), lines)
	}
	for i, element := range resp.Elements {
		target := element.Id + "::target"
		line := element.Id + "  " + element.Type
		for _, name := range []string{"parameter", "input", "feature", "ownedFeature"} {
			if got := element.Properties[name]; got != target {
				t.Errorf("%s: Query %s = %q, want %q", element.Id, name, got, target)
			}
			line += "  sysml:" + name + "=" + target
		}
		if lines[i] != line {
			t.Errorf("REPL line = %q, want %q", lines[i], line)
		}
	}
}
