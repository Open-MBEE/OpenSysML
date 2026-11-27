package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// TestGRPCRobustness exercises failure modes: missing models, invalid symbols, parse errors.
// Each RPC must return typed errors, never panic.
func TestGRPCRobustness(t *testing.T) {
	service := mustNewService(t, 10) // cache size 10

	t.Run("parse_invalid_syntax", func(t *testing.T) {
		req := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test { invalid syntax (((",
			},
		}
		resp, err := service.ParseFile(context.Background(), req)
		if err != nil {
			t.Fatalf("ParseFile RPC failed: %v (should return diagnostics, not RPC error)", err)
		}
		if len(resp.Diagnostics) == 0 {
			t.Error("Expected diagnostics for invalid syntax, got none")
		}
	})

	t.Run("get_symbol_missing_model", func(t *testing.T) {
		req := &pb.GetSymbolRequest{
			ModelHash: "nonexistent_hash",
			SymbolId:  "test::Symbol",
		}
		_, err := service.GetSymbol(context.Background(), req)
		if err == nil {
			t.Error("Expected error for missing model, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}
	})

	t.Run("evaluate_missing_model", func(t *testing.T) {
		req := &pb.EvaluateRequest{
			ModelHash:  "nonexistent_hash",
			Expression: "2 + 2",
		}
		_, err := service.Evaluate(context.Background(), req)
		if err == nil {
			t.Error("Expected error for missing model, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}
	})

	t.Run("evaluate_parse_error", func(t *testing.T) {
		// First parse a model
		parseReq := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test {}",
			},
		}
		parseResp, err := service.ParseFile(context.Background(), parseReq)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		// Try to evaluate invalid expression
		evalReq := &pb.EvaluateRequest{
			ModelHash:  parseResp.ModelHash,
			Expression: "invalid syntax (((",
		}
		evalResp, err := service.Evaluate(context.Background(), evalReq)
		if err != nil {
			t.Fatalf("Evaluate RPC failed: %v (should return error field, not RPC error)", err)
		}
		if evalResp.Error == "" {
			t.Error("Expected error field for invalid expression, got empty")
		}
	})

	t.Run("instantiate_missing_symbol", func(t *testing.T) {
		// Parse a model
		parseReq := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test {}",
			},
		}
		parseResp, err := service.ParseFile(context.Background(), parseReq)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		// Try to instantiate non-existent symbol
		instReq := &pb.InstantiateRequest{
			ModelHash: parseResp.ModelHash,
			SymbolId:  "test::NonExistent",
		}
		instResp, err := service.Instantiate(context.Background(), instReq)
		if err != nil {
			t.Fatalf("Instantiate RPC failed: %v (should return error field, not RPC error)", err)
		}
		if instResp.Error == "" {
			t.Error("Expected error field for missing symbol, got empty")
		}
	})

	t.Run("execute_action_missing_symbol", func(t *testing.T) {
		// Parse a model
		parseReq := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test {}",
			},
		}
		parseResp, err := service.ParseFile(context.Background(), parseReq)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		// Try to execute non-existent action
		execReq := &pb.ExecuteActionRequest{
			ModelHash:      parseResp.ModelHash,
			ActionSymbolId: "test::NonExistent",
			Inputs:         map[string]*pb.Value{},
		}
		execResp, err := service.ExecuteAction(context.Background(), execReq)
		if err != nil {
			t.Fatalf("ExecuteAction RPC failed: %v (should return error field, not RPC error)", err)
		}
		if execResp.Error == "" {
			t.Error("Expected error field for missing action, got empty")
		}
	})

	t.Run("execute_state_missing_symbol", func(t *testing.T) {
		// Parse a model
		parseReq := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test {}",
			},
		}
		parseResp, err := service.ParseFile(context.Background(), parseReq)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		// Try to execute non-existent state machine
		execReq := &pb.ExecuteStateRequest{
			ModelHash:            parseResp.ModelHash,
			StateMachineSymbolId: "test::NonExistent",
			Events:               []string{},
		}
		execResp, err := service.ExecuteState(context.Background(), execReq)
		if err != nil {
			t.Fatalf("ExecuteState RPC failed: %v (should return error field, not RPC error)", err)
		}
		if execResp.Error == "" {
			t.Error("Expected error field for missing state machine, got empty")
		}
	})

	t.Run("parse_with_unavailable_standard_library", func(t *testing.T) {
		// A library that would not load leaves the index without it. The request
		// must still answer, reporting unresolved names as diagnostics.
		svc := mustNewService(t, 10)
		defer svc.Close()
		svc.libIndexes = newLibraryBase(func() (*symbols.Index, libs.Source) {
			idx := symbols.NewIndex()
			idx.Freeze()
			return idx, nil
		})

		resp, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test { attribute def A { attribute x : ScalarValues::Real; } }",
			},
		})
		if err != nil {
			t.Fatalf("ParseFile RPC failed: %v (should return diagnostics, not RPC error)", err)
		}
		if len(resp.Diagnostics) == 0 {
			t.Error("Expected diagnostics for library types that did not load, got none")
		}
		if _, ok := svc.cache.Get(resp.ModelHash); !ok {
			t.Error("Expected the model to be cached despite the missing library")
		}
	})
}

func TestGRPCAuthoringRobustness(t *testing.T) {
	service := mustNewService(t, 10)
	hash := mustParsedModel(t, service, `package Demo {
    part def Base;
    part 'use' : Base;
}`)

	t.Run("unknown_owner", func(t *testing.T) {
		resp, err := service.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
			ModelHash: hash,
			Operations: []*pb.EditOperation{{
				Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
					Owner: "Demo::Missing", Kind: "part def", Name: "Vehicle",
				}},
			}},
		})
		if err != nil {
			t.Fatalf("ApplyEdits failed the call: %v", err)
		}
		if resp.Failure != pb.EditFailure_EDIT_FAILURE_OWNER_UNKNOWN {
			t.Fatalf("failure = %s, want owner unknown", resp.Failure)
		}
		if resp.Content != "" || resp.Error == "" {
			t.Fatalf("refusal response = content %q, error %q", resp.Content, resp.Error)
		}
	})

	t.Run("delete_referenced_without_cascade", func(t *testing.T) {
		resp, err := service.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
			ModelHash: hash,
			Operations: []*pb.EditOperation{{
				Operation: &pb.EditOperation_Delete{Delete: &pb.DeleteEdit{
					Target: "Demo::Base",
				}},
			}},
		})
		if err != nil {
			t.Fatalf("ApplyEdits failed the call: %v", err)
		}
		if resp.Failure != pb.EditFailure_EDIT_FAILURE_DELETE_REFERENCED {
			t.Fatalf("failure = %s, want delete referenced", resp.Failure)
		}
		if len(resp.ReferringElements) == 0 {
			t.Fatal("refusal did not identify referring elements")
		}
	})
}

// TestGRPCObjectBindingRobustness: a document-query binding to an object the
// service does not hold fails with a typed status naming the parameter, never
// a panic — nothing held, an unknown id, a path reaching no object.
func TestGRPCObjectBindingRobustness(t *testing.T) {
	service := mustNewService(t, 10)
	hash := parseFixture(t, service, objectFixture)

	run := func(value *pb.DocumentValue) error {
		_, err := service.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Garage::Parts",
			Bindings: []*pb.DocumentQueryBinding{binding("root", value)},
		})
		return err
	}
	expect := func(t *testing.T, err error, code connect.Code, texts ...string) {
		t.Helper()
		if err == nil {
			t.Fatalf("got no error, want %v", code)
		}
		if connect.CodeOf(err) != code {
			t.Errorf("code = %v, want %v: %v", connect.CodeOf(err), code, err)
		}
		for _, text := range texts {
			if !strings.Contains(err.Error(), text) {
				t.Errorf("error %q lacks %q", err.Error(), text)
			}
		}
	}

	t.Run("nothing_held", func(t *testing.T) {
		expect(t, run(objectByID(1)), connect.CodeNotFound, "binding root", "holds no objects", "Instantiate")
		expect(t, run(objectByPath("car")), connect.CodeNotFound, "binding root", "holds no objects")
		expect(t, run(objectByPath("car.wheels[2]")), connect.CodeNotFound, "binding root", "holds no objects")
		// A malformed reference is refused the same way whether or not objects are held.
		expect(t, run(objectByPath("")), connect.CodeInvalidArgument, "binding root", "instance_id or by path")
		expect(t, run(objectByID(-1)), connect.CodeInvalidArgument, "binding root", "#-1 is not an object id")
		expect(t, run(objectByPath("car..wheels")), connect.CodeInvalidArgument, "binding root", "not an object reference")
		expect(t, run(objectByPath("car.wheels[0]")), connect.CodeInvalidArgument, "binding root", "counted from 1")
	})

	holdObject(t, service, hash, "Garage::car")

	t.Run("unknown_id", func(t *testing.T) {
		expect(t, run(objectByID(99)), connect.CodeNotFound, "binding root", "no object #99", "#1")
		expect(t, run(objectByPath("#99")), connect.CodeNotFound, "binding root", "no object #99")
		expect(t, run(objectByPath("#99.wheels[1]")), connect.CodeNotFound, "binding root", "no object #99")
		expect(t, run(objectByID(-1)), connect.CodeInvalidArgument, "binding root", "#-1 is not an object id")
	})

	t.Run("no_reference", func(t *testing.T) {
		expect(t, run(objectByPath("")), connect.CodeInvalidArgument, "binding root", "instance_id or by path")
		expect(t, run(objectByPath("car..wheels")), connect.CodeInvalidArgument, "binding root", "not an object reference")
		expect(t, run(objectByPath("car.wheels[0]")), connect.CodeInvalidArgument, "binding root", "not an object reference", "counted from 1")
		expect(t, run(objectByPath("Garage.car")), connect.CodeInvalidArgument, "binding root", "Garage is a package, not an object")
	})

	t.Run("name_not_instantiated", func(t *testing.T) {
		expect(t, run(objectByPath("spare")), connect.CodeNotFound, "binding root", `no instance of "Garage::spare"`, "Instantiate first")
		expect(t, run(objectByPath("Garage::spare.pressure")), connect.CodeNotFound, "binding root", `no instance of "Garage::spare"`)
		expect(t, run(objectByPath("nowhere")), connect.CodeNotFound, "binding root", "symbol not found: nowhere")
		expect(t, run(objectByPath("Garage::nowhere")), connect.CodeNotFound, "binding root", "symbol not found: Garage::nowhere")
		expect(t, run(objectByPath("Garage::nowhere.pressure")), connect.CodeNotFound, "binding root", "symbol not found: Garage::nowhere")
		expect(t, run(objectByPath("Garage::Car")), connect.CodeNotFound, "binding root", `no instance of "Garage::Car"`)
	})

	t.Run("unknown_member_of_package_in_definition", func(t *testing.T) {
		nested := mustNewService(t, 10)
		parsed, err := nested.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: `package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	part def Car {
		package Kit {
			part jack;
		}
	}
	part car : Car;
	part spare;
	calc def Parts :> Query {
		in root : Element;
		Descendants(source = root, maxDepth = 1)
	}
}`}})
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		// An unrelated held object must not turn the answer into a missing instance.
		holdObject(t, nested, parsed.ModelHash, "Garage::spare")
		_, err = nested.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
			ModelHash: parsed.ModelHash, QueryId: "Garage::Parts",
			Bindings: []*pb.DocumentQueryBinding{binding("root", objectByPath("Garage::Car::Kit::nowhere"))},
		})
		expect(t, err, connect.CodeNotFound, "binding root", "symbol not found: Garage::Car::Kit::nowhere")
		// A held object is walked from: a package it owns or inherits is not a feature.
		for _, held := range []string{"Garage::car", "Garage::Car"} {
			holdObject(t, nested, parsed.ModelHash, held)
			_, err = nested.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
				ModelHash: parsed.ModelHash, QueryId: "Garage::Parts",
				Bindings: []*pb.DocumentQueryBinding{binding("root", objectByPath(held+"::Kit::nowhere"))},
			})
			expect(t, err, connect.CodeInvalidArgument, "binding root", held+` has no feature "Kit"`)
		}
	})

	t.Run("package_member_under_unheld_declaration", func(t *testing.T) {
		const model = `package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	part def Car {
		package Kit {
			part jack;
			package Deep {
				part bolt;
			}
		}
		part engine;
	}
	part def Truck :> Car;
	part car : Car;
	part truck : Truck;
	part spare;
	calc def Parts :> Query {
		in root : Element;
		Descendants(source = root, maxDepth = 1)
	}
}`
		// Each root is looked up while it holds no object and another declaration does.
		roots := []string{"Garage::car", "Garage::truck", "Garage::Truck"}
		for _, held := range [][]string{
			{"Garage::car"}, {"Garage::Car"}, {"Garage::car", "Garage::Car"},
			{"Garage::truck"}, {"Garage::Truck"}, {"Garage::spare"},
		} {
			srv := mustNewService(t, 10)
			parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: model}})
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			for _, sym := range held {
				holdObject(t, srv, parsed.ModelHash, sym)
			}
			lookup := func(path string) error {
				_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
					ModelHash: parsed.ModelHash, QueryId: "Garage::Parts",
					Bindings: []*pb.DocumentQueryBinding{binding("root", objectByPath(path))},
				})
				return err
			}
			for _, root := range roots {
				if slices.Contains(held, root) {
					continue
				}
				// A package the declaration owns or inherits only encloses the unknown name.
				for _, rest := range []string{"::Kit::nowhere", "::Kit::Deep::nowhere"} {
					expect(t, lookup(root+rest), connect.CodeNotFound, "binding root", "symbol not found: "+root+rest)
				}
				// A member the package does declare is the declaration lacking an object.
				expect(t, lookup(root+"::Kit::jack"), connect.CodeNotFound, "binding root", `no instance of "Garage::Car::Kit::jack"`)
				expect(t, lookup(root+"::Kit::Deep::bolt::nowhere"), connect.CodeNotFound, "binding root", `no instance of "Garage::Car::Kit::Deep::bolt"`)
				expect(t, lookup(root+"::engine::nowhere"), connect.CodeNotFound, "binding root", `no instance of "Garage::Car::engine"`)
				// A name that is no member there still blames the root.
				expect(t, lookup(root+"::nowhere::bolt"), connect.CodeNotFound, "binding root", `no instance of "`+root+`"`)
			}
		}

		// An object held under the declared name is reached through the inheriting one.
		srv := mustNewService(t, 10)
		parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: model}})
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		holdObject(t, srv, parsed.ModelHash, "Garage::Car::Kit::jack")
		for _, root := range roots {
			if _, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
				ModelHash: parsed.ModelHash, QueryId: "Garage::Parts",
				Bindings: []*pb.DocumentQueryBinding{binding("root", objectByPath(root+"::Kit::jack"))},
			}); err != nil {
				t.Errorf("%s::Kit::jack with Garage::Car::Kit::jack held: %v", root, err)
			}
			_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
				ModelHash: parsed.ModelHash, QueryId: "Garage::Parts",
				Bindings: []*pb.DocumentQueryBinding{binding("root", objectByPath(root+"::Kit::jack::nowhere"))},
			})
			expect(t, err, connect.CodeInvalidArgument, "binding root", `Garage::Car::Kit::jack has no feature "nowhere"`)
		}
	})

	t.Run("inherited_member_beside_held_root_and_quoted_name", func(t *testing.T) {
		const model = `package Fleet {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	part def Drive {
		part motor;
	}
	part def Car {
		part drive : Drive;
		part 'drive::motor';
	}
	part car : Car;
	calc def Parts :> Query {
		in root : Element;
		Descendants(source = root, maxDepth = 1)
	}
}`
		lookup := func(path string, held ...string) error {
			srv := mustNewService(t, 10)
			parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: model}})
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			for _, sym := range held {
				holdObject(t, srv, parsed.ModelHash, sym)
			}
			_, err = srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
				ModelHash: parsed.ModelHash, QueryId: "Fleet::Parts",
				Bindings: []*pb.DocumentQueryBinding{binding("root", objectByPath(path))},
			})
			return err
		}
		// A quoted segment holding `::` is one member of Car, not a chain through drive.
		expect(t, lookup("Fleet::car::'drive::motor'", "Fleet::Drive::motor"), connect.CodeNotFound, "binding root", `no instance of "Fleet::Car::drive::motor"`)
		expect(t, lookup("Fleet::car::'drive::motor'::nowhere", "Fleet::Drive::motor"), connect.CodeNotFound, "binding root", `no instance of "Fleet::Car::drive::motor"`)
		expect(t, lookup("Fleet::car::drive::motor::nowhere", "Fleet::Drive::motor"), connect.CodeInvalidArgument, "binding root", `Fleet::Drive::motor has no feature "nowhere"`)
		// A held root as written is walked from, whatever is held under the inherited name.
		expect(t, lookup("Fleet::car::drive::motor::nowhere", "Fleet::car", "Fleet::Drive::motor"), connect.CodeInvalidArgument, "binding root", `Fleet::car.drive.motor has no feature "nowhere"`)
		expect(t, lookup("Fleet::car::'drive::motor'::nowhere", "Fleet::car", "Fleet::Drive::motor"), connect.CodeInvalidArgument, "binding root", `Fleet::car.'drive::motor' has no feature "nowhere"`)
	})

	t.Run("path_reaches_no_object", func(t *testing.T) {
		expect(t, run(objectByPath("car.wheels")), connect.CodeInvalidArgument, "binding root", "wheels of Garage::car holds 2 objects", "wheels[1] to wheels[2]")
		expect(t, run(objectByPath("car.wheels[3]")), connect.CodeInvalidArgument, "binding root", "wheels[3] names none")
		expect(t, run(objectByPath("car.engine.power")), connect.CodeInvalidArgument, "binding root", "power of Garage::car.engine holds a value (100), not an object")
		expect(t, run(objectByPath("car.hood")), connect.CodeInvalidArgument, "binding root", `Garage::car has no feature "hood"`, "engine, wheels")
		expect(t, run(objectByPath("#1.engine.power")), connect.CodeInvalidArgument, "binding root", "power of #1.engine holds a value (100), not an object")
	})

	t.Run("id_and_path_disagree", func(t *testing.T) {
		both := &pb.DocumentValue{Kind: &pb.DocumentValue_Object{Object: &pb.DocumentObject{InstanceId: 2, Path: "car"}}}
		expect(t, run(both), connect.CodeInvalidArgument, "binding root", "Garage::car is object #1, not #2")
	})

	t.Run("object_bound_to_scalar_parameter", func(t *testing.T) {
		_, err := service.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "Garage::Wheels",
			Bindings: []*pb.DocumentQueryBinding{binding("root", objectByID(1))},
		})
		expect(t, err, connect.CodeInvalidArgument, "unknown binding root")
	})

	t.Run("missing_model", func(t *testing.T) {
		_, err := service.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
			ModelHash: "nonexistent_hash", QueryId: "Garage::Parts",
			Bindings: []*pb.DocumentQueryBinding{binding("root", objectByID(1))},
		})
		expect(t, err, connect.CodeNotFound, "not found")
	})
}
