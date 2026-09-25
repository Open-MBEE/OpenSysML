package grpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// stateQueryModel is a lamp whose machine accepts Toggle, plus a bare part, so
// the state and event query refusals can each be reached over RunDocumentQuery.
const stateQueryModel = `package Lamps {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	attribute def Toggle;
	state def LampMachine {
		entry; then off;
		state off;
		transition off_on first off accept Toggle then on;
		state on;
	}
	part def Lamp { exhibit state lp : LampMachine; }
	part def Rock;
	part lamp : Lamp;
	part rock : Rock;

	calc def CurrentStates :> Query {
		in root : Element;
		Project(source = States(source = root), properties = ("machine", "statePath"))
	}
	calc def Off :> Query {
		InState(name = "off")
	}
	calc def Nowhere :> Query {
		InState(name = "orbit")
	}
	calc def Steps :> Query {
		in root : Element;
		Events(source = root)
	}
	calc def Window :> Query {
		in root : Element;
		in s : Real;
		in b : Real;
		Events(source = root, since = s, before = b)
	}
}`

// TestGRPCRobustnessDocumentQueryStatesEvents exercises the typed refusals of
// the state and event queries over the held population: each is a connect
// status, never a panic or a bare error.
func TestGRPCRobustnessDocumentQueryStatesEvents(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, stateQueryModel)
	holdObject(t, srv, hash, "Lamps::lamp")
	holdObject(t, srv, hash, "Lamps::rock")

	run := func(query string, bindings ...*pb.DocumentQueryBinding) error {
		t.Helper()
		_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: query, Bindings: bindings,
		})
		return err
	}
	code := func(err error) connect.Code {
		t.Helper()
		if err == nil {
			t.Fatal("RunDocumentQuery succeeded, want a typed refusal")
		}
		return connect.CodeOf(err)
	}

	t.Run("states_of_an_object_with_no_machine", func(t *testing.T) {
		if got := code(run("Lamps::CurrentStates", binding("root", objectByID(3)))); got != connect.CodeFailedPrecondition {
			t.Fatalf("code = %v, want FAILED_PRECONDITION", got)
		}
	})
	t.Run("in_state_names_a_state_no_machine_declares", func(t *testing.T) {
		if got := code(run("Lamps::Nowhere")); got != connect.CodeFailedPrecondition {
			t.Fatalf("code = %v, want FAILED_PRECONDITION", got)
		}
	})
	t.Run("events_empty_interval", func(t *testing.T) {
		err := run("Lamps::Window", binding("root", objectByID(1)),
			binding("s", &pb.DocumentValue{Kind: &pb.DocumentValue_RealValue{RealValue: 1}}),
			binding("b", &pb.DocumentValue{Kind: &pb.DocumentValue_RealValue{RealValue: 1}}))
		if got := code(err); got != connect.CodeFailedPrecondition {
			t.Fatalf("code = %v, want FAILED_PRECONDITION: %v", got, err)
		}
	})
	t.Run("events_over_the_unrun_population", func(t *testing.T) {
		// The population's run is traced from the first Instantiate, so Events
		// always reads a trace over gRPC; the lamp's start recorded its entry.
		resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Lamps::Steps", Bindings: []*pb.DocumentQueryBinding{binding("root", objectByID(1))},
		})
		if err != nil {
			t.Fatalf("RunDocumentQuery Steps: %v", err)
		}
		if len(resp.Rows) != 1 || resp.Rows[0].Element.GetEvent().GetKind() != "entry" {
			t.Fatalf("rows = %v, want the one entry record", resp.Rows)
		}
	})
	t.Run("source_bound_to_an_unknown_id", func(t *testing.T) {
		if got := code(run("Lamps::CurrentStates", binding("root", objectByID(99)))); got != connect.CodeNotFound {
			t.Fatalf("code = %v, want NOT_FOUND", got)
		}
	})
}
