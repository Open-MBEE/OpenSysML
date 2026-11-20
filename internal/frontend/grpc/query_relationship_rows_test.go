package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const verifyQueryModel = `package Ver {
	requirement def MassReq;
	requirement massReq : MassReq;
	requirement other;
	part def Vehicle;
	part vehicle : Vehicle { satisfy massReq by vehicle; }
	verification def MassTest {
		subject v : Vehicle;
		objective {
			verify massReq;
			verify requirement declared : MassReq;
			verify missing;
		}
	}
	verification massTest : MassTest {
		objective obj { verify other; }
	}
	requirement loose { verify other; }
}`

// rowsByID indexes a query's rows by @id, failing on a repeated one.
func rowsByID(t *testing.T, resp *pb.QueryResponse) map[string]*pb.QueryResultElement {
	t.Helper()
	rows := map[string]*pb.QueryResultElement{}
	for _, element := range resp.Elements {
		if _, seen := rows[element.Id]; seen {
			t.Fatalf("row %s answered twice", element.Id)
		}
		rows[element.Id] = element
	}
	return rows
}

// A `verify` answers the requirement it verifies and the verification case whose objective
// holds it, in one query; a `satisfy` stays on its own pair and an unresolved end is absent.
func TestQueryAnswersVerifyEnds(t *testing.T) {
	_, _, resp := runQueryOnSource(t, verifyQueryModel, &pb.Query{
		Select: []string{"verifiedRequirement", "verifyingCase", QueryPropSatisfiedRequirement},
	})
	type ends struct{ requirement, verifier, satisfied string }
	got := map[string]ends{}
	for id, row := range rowsByID(t, resp) {
		if len(row.Properties) > 0 {
			got[id] = ends{row.Properties["verifiedRequirement"], row.Properties["verifyingCase"], row.Properties[QueryPropSatisfiedRequirement]}
		}
	}
	objective := ""
	for id, e := range got {
		if e.requirement == "Ver::massReq" && e.verifier == "Ver::MassTest" {
			objective = strings.TrimSuffix(id, "::massReq")
		}
	}
	if objective == "" {
		t.Fatalf("rows = %v, want a verify of Ver::massReq by Ver::MassTest", got)
	}
	want := map[string]ends{
		"Ver::MassTest":             {requirement: "Ver::massReq"},
		objective + "::massReq":     {requirement: "Ver::massReq", verifier: "Ver::MassTest"},
		objective + "::declared":    {requirement: objective + "::declared", verifier: "Ver::MassTest"},
		objective + "::missing":     {verifier: "Ver::MassTest"},
		"Ver::massTest":             {requirement: "Ver::other"},
		"Ver::massTest::obj::other": {requirement: "Ver::other", verifier: "Ver::massTest"},
		"Ver::loose::other":         {requirement: "Ver::other"},
	}
	for id, e := range got {
		if e.satisfied != "" {
			if e != (ends{satisfied: "Ver::massReq"}) {
				t.Fatalf("%s = %+v, want a satisfy on satisfiedRequirement alone", id, e)
			}
			continue
		}
		if want[id] != e {
			t.Fatalf("%s = %+v, want %+v", id, e, want[id])
		}
		delete(want, id)
	}
	if len(want) != 0 {
		t.Fatalf("rows lacking verify ends: %v (answered %v)", want, got)
	}
}

// The verifier half of a consistency check is one predicate over either end.
func TestQueryFiltersByVerifyEnds(t *testing.T) {
	srv, hash, resp := runQueryOnSource(t, verifyQueryModel, &pb.Query{
		Where:  primitive("verifyingCase", pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL, false, "Ver::massTest"),
		Select: []string{"verifiedRequirement"},
	})
	if len(resp.Elements) != 1 || resp.Elements[0].Id != "Ver::massTest::obj::other" ||
		resp.Elements[0].Properties["verifiedRequirement"] != "Ver::other" {
		t.Fatalf("verifies of Ver::massTest = %v, want Ver::massTest::obj::other verifying Ver::other", resp.Elements)
	}
	verifiers := map[string]bool{}
	for _, element := range oslcQuery(t, srv, hash, `oslc.where=sysml:verifiedRequirement="Ver::other"&oslc.select=sysx:verifyingCase`).Elements {
		if verifier, ok := element.Properties["verifyingCase"]; ok {
			verifiers[verifier] = true
		}
	}
	if len(verifiers) != 1 || !verifiers["Ver::massTest"] {
		t.Fatalf("cases verifying Ver::other = %v, want Ver::massTest alone", verifiers)
	}
}

const portGraphModel = `package Net {
	port def P;
	port def Q;
	part def Box { port p : P; port q : Q; port spare : P; }
	part sys {
		part a : Box;
		part b : Box;
		connection ok connect a.p to b.p;
		connection crossed connect a.q to b.p;
		interface wired connect a.p to b.q;
		bind a.spare = b.spare;
	}
}`

// The port graph is one scoped query: each connector row reports the port either end
// reaches and that port's definition, so a mismatch is a predicate over the answer.
func TestQueryAnswersThePortGraph(t *testing.T) {
	_, _, resp := runQueryOnSource(t, portGraphModel, &pb.Query{
		Scope: []string{"Net::sys"},
		Where: primitive("@type", pb.PrimitiveOperator_PRIMITIVE_OPERATOR_INSTANCE_OF, false, "Connector"),
		Select: []string{"sourceFeature", "sourceFeature.featureTarget", "sourceFeature.featureTarget.type",
			"targetFeature", "targetFeature.featureTarget", "targetFeature.featureTarget.type"},
	})
	type edge struct{ typ, from, fromPort, to, toPort string }
	var edges []edge
	mismatched := map[string]bool{}
	for _, row := range resp.Elements {
		p := row.Properties
		edges = append(edges, edge{row.Type, p["sourceFeature"], p["sourceFeature.featureTarget"], p["targetFeature"], p["targetFeature.featureTarget"]})
		if p["sourceFeature.featureTarget.type"] != p["targetFeature.featureTarget.type"] {
			mismatched[row.Id] = true
		}
	}
	want := []edge{
		{"ConnectionUsage", "a.p", "Net::Box::p", "b.p", "Net::Box::p"},
		{"ConnectionUsage", "a.q", "Net::Box::q", "b.p", "Net::Box::p"},
		{"InterfaceUsage", "a.p", "Net::Box::p", "b.q", "Net::Box::q"},
		{"BindingConnectorAsUsage", "a.spare", "Net::Box::spare", "b.spare", "Net::Box::spare"},
	}
	if len(edges) != len(want) {
		t.Fatalf("edges = %+v, want %+v", edges, want)
	}
	for i := range want {
		if edges[i] != want[i] {
			t.Fatalf("edge %d = %+v, want %+v", i, edges[i], want[i])
		}
	}
	if len(mismatched) != 2 || !mismatched["Net::sys::crossed"] || !mismatched["Net::sys::wired"] {
		t.Fatalf("mismatched = %v, want crossed and wired", mismatched)
	}
}

// The connectors reaching one port are a predicate too, whichever end reaches it.
func TestQuerySelectsTheConnectorsReachingAPort(t *testing.T) {
	_, _, resp := runQueryOnSource(t, portGraphModel, &pb.Query{
		Where: primitive("relatedFeature.featureTarget", pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL, false, "Net::Box::q"),
	})
	var reaching []string
	for _, row := range resp.Elements {
		reaching = append(reaching, row.Id)
	}
	if strings.Join(reaching, " ") != "Net::sys::crossed Net::sys::wired" {
		t.Fatalf("connectors reaching Net::Box::q = %v, want crossed and wired", reaching)
	}
}

const machineQueryModel = `package SM {
	state def Machine {
		entry; then a;
		state a;
		state b;
		state c;
		transition t1 first a if true then b;
		transition t2 first b then c;
		transition first c then a;
		transition t4 first a then c;
	}
	state def Other {
		state x;
		state y;
		transition first x then y;
	}
}`

// A transition is a row of its own with identity, owner, source and target, so a machine's
// cyclomatic number E − N + 2P comes from one answer.
func TestQueryAnswersTransitionRows(t *testing.T) {
	_, _, resp := runQueryOnSource(t, machineQueryModel, &pb.Query{
		Where: composite(pb.CompositeOperator_COMPOSITE_OPERATOR_OR,
			primitive("@type", pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL, false, "TransitionUsage"),
			primitive("@type", pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL, false, "StateUsage")),
		Select: []string{QueryPropOwner, QueryPropName, "source", "target"},
	})
	edges, nodes := map[string]int{}, map[string]int{}
	ids := map[string]bool{}
	for id, row := range rowsByID(t, resp) {
		owner := row.Properties[QueryPropOwner]
		if row.Type == "StateUsage" {
			nodes[owner]++
			continue
		}
		if id == "" || row.Properties["source"] == "" || row.Properties["target"] == "" {
			t.Fatalf("transition %q = %v, want an @id, a source and a target", id, row.Properties)
		}
		ids[id] = true
		edges[owner]++
	}
	t1 := rowsByID(t, resp)["SM::Machine::t1"]
	if t1 == nil || t1.Properties[QueryPropName] != "t1" || t1.Properties["source"] != "SM::Machine::a" || t1.Properties["target"] != "SM::Machine::b" {
		t.Fatalf("t1 = %v, want a named transition from a to b (transitions %v)", t1, ids)
	}
	if edges["SM::Machine"] != 4 || nodes["SM::Machine"] != 3 || edges["SM::Other"] != 1 || nodes["SM::Other"] != 2 {
		t.Fatalf("edges = %v, nodes = %v, want Machine 4 over 3 and Other 1 over 2", edges, nodes)
	}
	if got := edges["SM::Machine"] - nodes["SM::Machine"] + 2; got != 3 {
		t.Fatalf("cyclomatic number of SM::Machine = %d, want 3", got)
	}
}

// A name outside the grown set is still refused, a near miss of a new name included.
func TestQueryRefusesNamesOutsideTheRelationshipProperties(t *testing.T) {
	srv, hash, _ := runQueryOnSource(t, verifyQueryModel, &pb.Query{})
	for _, name := range []string{"verifyingCas", "verifiedBy", "attachment", "reachingConnector"} {
		_, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, Query: &pb.Query{Select: []string{name}}})
		var cerr *connect.Error
		if !errors.As(err, &cerr) || cerr.Code() != connect.CodeInvalidArgument || !strings.Contains(cerr.Message(), "unknown query property") {
			t.Fatalf("select %q: err = %v, want an unknown query property refusal", name, err)
		}
	}
}
