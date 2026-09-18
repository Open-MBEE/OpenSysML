package main

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/grpc/codes"
)

// TestOnlySetFieldsAreNormalized verifies a field left at its default is absent
// from the tree, since that is what it is on the wire.
func TestOnlySetFieldsAreNormalized(t *testing.T) {
	tree := newNormalizer("").normalize((&pb.EvaluateResponse{
		Result: &pb.Value{Kind: &pb.Value_IntValue{IntValue: 4}},
	}).ProtoReflect())
	if _, ok := tree["error"]; ok {
		t.Errorf("error is present, want it absent when unset: %v", tree)
	}
	if got, _ := lookup(tree, "result.int_value"); got != integer(4) {
		t.Errorf("result.int_value = %v (%T), want the integer 4", got, got)
	}
}

// TestIntegralFieldsStayIntegral verifies an int field does not become a
// float64, which would compare it within the Real tolerance.
func TestIntegralFieldsStayIntegral(t *testing.T) {
	tree := newNormalizer("").normalize((&pb.DiagnosticsResponse{
		Diagnostics: []*pb.Diagnostic{{Span: &pb.Span{StartLine: 7}}},
	}).ProtoReflect())
	got, _ := lookup(tree, "diagnostics.0.span.start_line")
	if _, isFloat := got.(float64); isFloat {
		t.Errorf("start_line = %v is a float64, want an integer", got)
	}
	if got != integer(7) {
		t.Errorf("start_line = %v (%T), want the integer 7", got, got)
	}
}

// TestInstanceIDsAreLabelledInOrderOfAppearance verifies two feature values
// naming one object still name one object after normalization, without the
// scenario knowing which ids the call assigned.
func TestInstanceIDsAreLabelledInOrderOfAppearance(t *testing.T) {
	response := &pb.InstantiateResponse{
		Instance: &pb.Instance{
			Id: 41,
			FeatureValues: map[string]*pb.FeatureValue{
				"engine": {Value: &pb.Value{Kind: &pb.Value_InstanceId{InstanceId: 77}}},
				"scale":  {Value: &pb.Value{Kind: &pb.Value_Function{Function: &pb.Function{CalcId: "T::scale", SelfId: 41}}}},
			},
		},
		Instances: []*pb.Instance{{Id: 41}, {Id: 77}},
	}
	tree := newNormalizer("").normalize(response.ProtoReflect())
	if got, _ := lookup(tree, "instance.id"); got != "@1" {
		t.Errorf("instance.id = %v, want @1", got)
	}
	if got, _ := lookup(tree, "instance.feature_values.engine.value.instance_id"); got != "@2" {
		t.Errorf("nested instance_id = %v, want @2", got)
	}
	if got, _ := lookup(tree, "instance.feature_values.scale.value.function.self_id"); got != "@1" {
		t.Errorf("function self_id = %v, want the owner's label @1", got)
	}
	if got, _ := lookup(tree, "instances.1.id"); got != "@2" {
		t.Errorf("instances.1.id = %v, want the same label @2", got)
	}
}

// TestTheVersionAndTheModelHashAreReplaced verifies neither a build version nor
// a hash is compared literally.
func TestTheVersionAndTheModelHashAreReplaced(t *testing.T) {
	info := newNormalizer("").normalize((&pb.ServerInfoResponse{Version: "0.9.1-dev"}).ProtoReflect())
	if got := info["version"]; got != versionPlaceholder {
		t.Errorf("version = %v, want %s", got, versionPlaceholder)
	}
	parsed := newNormalizer("abc123").normalize((&pb.ParseFileResponse{ModelHash: "abc123"}).ProtoReflect())
	if got := parsed["model_hash"]; got != modelHashPlaceholder {
		t.Errorf("model_hash = %v, want %s", got, modelHashPlaceholder)
	}
}

// TestAbsolutePathsAreReplaced verifies a span's file does not pin the machine
// the service ran on, while a relative name is kept.
func TestAbsolutePathsAreReplaced(t *testing.T) {
	tree := newNormalizer("").normalize((&pb.DiagnosticsResponse{Diagnostics: []*pb.Diagnostic{
		{Severity: "error", Span: &pb.Span{File: "/tmp/model.sysml"}},
		{Severity: "error", Span: &pb.Span{File: "model.sysml"}},
	}}).ProtoReflect())
	if got, _ := lookup(tree, "diagnostics.0.span.file"); got != pathPlaceholder {
		t.Errorf("absolute file = %v, want %s", got, pathPlaceholder)
	}
	if got, _ := lookup(tree, "diagnostics.1.span.file"); got != "model.sysml" {
		t.Errorf("relative file = %v, want it kept", got)
	}
}

// TestEnumsNormalizeToTheirNames verifies a scenario names an enum value as the
// schema spells it rather than by number.
func TestEnumsNormalizeToTheirNames(t *testing.T) {
	tree := newNormalizer("").normalize((&pb.ApplyEditsResponse{
		Failure: pb.EditFailure_EDIT_FAILURE_UNKNOWN_TARGET,
	}).ProtoReflect())
	if got := tree["failure"]; got != "EDIT_FAILURE_UNKNOWN_TARGET" {
		t.Errorf("failure = %v, want EDIT_FAILURE_UNKNOWN_TARGET", got)
	}
}

// TestStatusNamesAreCanonical verifies a scenario spells a status as gRPC does,
// not as Go's String() does.
func TestStatusNamesAreCanonical(t *testing.T) {
	if got := statusName(codes.NotFound); got != "NOT_FOUND" {
		t.Errorf("statusName(NotFound) = %q, want NOT_FOUND", got)
	}
	if !sameCode("INVALID_ARGUMENT", codes.InvalidArgument) {
		t.Error("INVALID_ARGUMENT does not match the code it names")
	}
	if sameCode("NOT_FOUND", codes.InvalidArgument) {
		t.Error("NOT_FOUND matched InvalidArgument")
	}
}
