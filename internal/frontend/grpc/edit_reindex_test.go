package grpc

import (
	"context"
	"reflect"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// editedModel is the model the edit equivalence tests change: it resolves
// against the library, so an edit's analysis reaches the library index.
const editedModel = `package Rig {
	private import ISQ::*;
	part def Sensor {
		attribute reading : ScalarValues::Real;
	}
	part def Frame;
	part sensor : Sensor;
	attribute margin = 1.0;
}`

func addMember(owner, kind, name, typ string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_AddMember{
		AddMember: &pb.AddMemberEdit{Owner: owner, Kind: kind, Name: name, Type: typ},
	}}
}

func setValue(target, value string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_SetValue{
		SetValue: &pb.SetValueEdit{Target: target, Value: value},
	}}
}

func rename(target, newName string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_Rename{
		Rename: &pb.RenameEdit{Target: target, NewName: newName},
	}}
}

// applyEdits edits the cached model named by hash, failing the test if the call
// itself fails: a refused edit is a response, not a call failure.
func applyEdits(t *testing.T, svc *Service, hash string, ops ...*pb.EditOperation) *pb.ApplyEditsResponse {
	t.Helper()
	resp, err := svc.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, Operations: ops,
	})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	return resp
}

// appliedText renders what each operation changed, without the operation index:
// a request of one operation numbers its own, and the same change made in one
// request of N carries N different indexes.
func appliedText(applied []*pb.AppliedEdit) []string {
	out := make([]string, 0, len(applied))
	for _, a := range applied {
		out = append(out, a.Target+" -> "+a.NewText)
	}
	return out
}

// oneAtATime applies each operation in a request of its own, reparsing between
// them, which is what an intermediate state costs a client that does not batch.
// It returns the final notation and the response of the last call.
func oneAtATime(t *testing.T, svc *Service, content string, ops []*pb.EditOperation) (string, *pb.ApplyEditsResponse, []string) {
	t.Helper()
	var last *pb.ApplyEditsResponse
	var applied []string
	for _, op := range ops {
		resp, _ := parseContent(t, svc, content)
		last = applyEdits(t, svc, resp.ModelHash, op)
		if last.Error != "" {
			return content, last, applied
		}
		applied = append(applied, appliedText(last.Applied)...)
		content = last.Content
	}
	return content, last, applied
}

var editScenarios = []struct {
	name string
	ops  []*pb.EditOperation
}{{
	// Every later add owns an earlier one, which is the reparsing path: the
	// operation after this one has to see the member this one wrote.
	name: "adds nest into what earlier operations added",
	ops: []*pb.EditOperation{
		addMember("Rig", "part def", "Mount", ""),
		addMember("Rig::Mount", "attribute", "height", "ScalarValues::Real"),
		addMember("Rig::Mount", "attribute", "width", "ScalarValues::Real"),
		addMember("Rig::Frame", "attribute", "mass", "ScalarValues::Real"),
	},
}, {
	name: "adds and edits mix",
	ops: []*pb.EditOperation{
		addMember("Rig", "part def", "Bracket", ""),
		addMember("Rig::Bracket", "attribute", "span", "ScalarValues::Real"),
		setValue("Rig::margin", "2.5"),
		rename("Rig::Frame", "Chassis"),
	},
}, {
	name: "an add is deleted again and a referenced name is renamed",
	ops: []*pb.EditOperation{
		addMember("Rig", "part def", "Spare", ""),
		addMember("Rig::Spare", "attribute", "mass", "ScalarValues::Real"),
		{Operation: &pb.EditOperation_Delete{Delete: &pb.DeleteEdit{Target: "Rig::Spare::mass"}}},
		rename("Rig::Sensor", "Probe"),
	},
}, {
	name: "deep nesting",
	ops: []*pb.EditOperation{
		addMember("Rig", "part def", "A", ""),
		addMember("Rig::A", "part def", "B", ""),
		addMember("Rig::A::B", "attribute", "depth", "ScalarValues::Real"),
	},
}}

// TestApplyEditsMatchesApplyingOneAtATime proves the request-local index does
// not move semantics: a request of N operations gives the same notation, the
// same diagnostics and the same whole-index qualified lookups as N requests of
// one operation, which each build an index of their own.
func TestApplyEditsMatchesApplyingOneAtATime(t *testing.T) {
	for _, sc := range editScenarios {
		t.Run(sc.name, func(t *testing.T) {
			svc := prewarmedService(t, 8)
			defer svc.Close()

			resp, _ := parseContent(t, svc, editedModel)
			batch := applyEdits(t, svc, resp.ModelHash, sc.ops...)
			if batch.Error != "" {
				t.Fatalf("the edit was refused: %s", batch.Error)
			}
			step, _, stepApplied := oneAtATime(t, svc, editedModel, sc.ops)

			if batch.Content != step {
				t.Fatalf("one request of %d operations wrote different notation than %d requests of one:\n%s\nwant:\n%s",
					len(sc.ops), len(sc.ops), batch.Content, step)
			}
			if got, want := appliedText(batch.Applied), stepApplied; !reflect.DeepEqual(got, want) {
				t.Fatalf("applied edits differ:\n%v\nwant:\n%v", got, want)
			}

			// The edited notation is read back the way the original was, so the
			// diagnostics and the lookups both models are analysed against are
			// comparable.
			batchParse, batchModel := parseContent(t, svc, batch.Content)
			stepParse, stepModel := parseContent(t, svc, step)
			if got, want := diagLines(batchParse.Diagnostics), diagLines(stepParse.Diagnostics); !reflect.DeepEqual(got, want) {
				t.Fatalf("diagnostics differ:\n%v\nwant:\n%v", got, want)
			}
			if got, want := lookupLines(batchModel.Index), lookupLines(stepModel.Index); !reflect.DeepEqual(got, want) {
				t.Fatalf("qualified lookups differ: %d vs %d names", len(got), len(want))
			}
		})
	}
}

// TestApplyEditsRefusesWhereOneAtATimeDoes proves an operation that depends on
// an earlier one's intermediate state is judged the same in a batch: the
// refusal, its kind and its message are those the same operation gets on its
// own, and a refused request writes nothing.
func TestApplyEditsRefusesWhereOneAtATimeDoes(t *testing.T) {
	cases := []struct {
		name string
		ops  []*pb.EditOperation
		// differentMessage marks a refusal a batch words differently because it
		// can name the pair of operations that collide.
		differentMessage bool
	}{{
		name:             "second add takes the name the first wrote",
		differentMessage: true,
		ops: []*pb.EditOperation{
			addMember("Rig", "part def", "Mount", ""),
			addMember("Rig::Mount", "attribute", "height", "ScalarValues::Real"),
			addMember("Rig::Mount", "attribute", "height", "ScalarValues::Nat"),
		},
	}, {
		name: "add names a type nothing declares",
		ops: []*pb.EditOperation{
			addMember("Rig", "part def", "Mount", ""),
			addMember("Rig::Mount", "attribute", "height", "Nowhere::Missing"),
		},
	}, {
		name: "add owns something no operation created",
		ops: []*pb.EditOperation{
			addMember("Rig", "part def", "Mount", ""),
			addMember("Rig::Absent", "attribute", "height", "ScalarValues::Real"),
		},
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := prewarmedService(t, 8)
			defer svc.Close()

			resp, _ := parseContent(t, svc, editedModel)
			batch := applyEdits(t, svc, resp.ModelHash, tc.ops...)
			if batch.Error == "" {
				t.Fatalf("the batch was applied: %s", batch.Content)
			}
			if batch.Content != "" {
				t.Fatalf("a refused request wrote notation: %s", batch.Content)
			}
			_, step, _ := oneAtATime(t, svc, editedModel, tc.ops)
			if step.Error == "" {
				t.Fatal("applying the operations one at a time was not refused")
			}
			if batch.Failure != step.Failure || (!tc.differentMessage && batch.Error != step.Error) {
				t.Fatalf("refusals differ: %v %q, one at a time %v %q",
					batch.Failure, batch.Error, step.Failure, step.Error)
			}
			if got, want := diagLines(batch.Diagnostics), diagLines(step.Diagnostics); !reflect.DeepEqual(got, want) {
				t.Fatalf("refusal diagnostics differ:\n%v\nwant:\n%v", got, want)
			}
		})
	}
}

// TestApplyEditsLeavesOtherModelsAndTheSharedBaseAlone proves the request-local
// index is exactly that: the edited model's own index and another model's index
// are unchanged by an edit, neither model sees the other's documents, and a
// fresh overlay over the shared base carries no edited notation.
func TestApplyEditsLeavesOtherModelsAndTheSharedBaseAlone(t *testing.T) {
	svc := prewarmedService(t, 8)
	defer svc.Close()

	other := `package Other {
	part def Widget;
}`
	edited, editedModelCached := parseContent(t, svc, editedModel)
	_, otherCached := parseContent(t, svc, other)

	baseBefore := lookupLines(sharedBase(svc))
	editedBefore := lookupLines(editedModelCached.Index)
	otherBefore := lookupLines(otherCached.Index)

	resp := applyEdits(t, svc, edited.ModelHash,
		addMember("Rig", "part def", "Mount", ""),
		addMember("Rig::Mount", "attribute", "height", "ScalarValues::Real"),
		addMember("Rig::Mount", "attribute", "width", "ScalarValues::Real"))
	if resp.Error != "" {
		t.Fatalf("the edit was refused: %s", resp.Error)
	}

	if got := lookupLines(sharedBase(svc)); !reflect.DeepEqual(got, baseBefore) {
		t.Fatalf("the shared base changed: %d names, was %d", len(got), len(baseBefore))
	}
	if got := lookupLines(editedModelCached.Index); !reflect.DeepEqual(got, editedBefore) {
		t.Fatalf("the edited model's own index changed: %d names, was %d", len(got), len(editedBefore))
	}
	if got := lookupLines(otherCached.Index); !reflect.DeepEqual(got, otherBefore) {
		t.Fatalf("another model's index changed: %d names, was %d", len(got), len(otherBefore))
	}
	for _, line := range lookupLines(otherCached.Index) {
		if strings.HasPrefix(line, "Rig") {
			t.Fatalf("the edited model is visible in another model: %s", line)
		}
	}
	for _, line := range lookupLines(sharedBase(svc)) {
		if strings.HasPrefix(line, "Rig") || strings.HasPrefix(line, "Other") {
			t.Fatalf("a model is visible in the shared base: %s", line)
		}
	}
}

// TestApplyEditsTakesOneIndexPerRequest locks the defect this slice fixed: a
// request costs one index however many operations it carries, and none at all
// when it carries a batch the engine applies without reparsing.
func TestApplyEditsTakesOneIndexPerRequest(t *testing.T) {
	svc := prewarmedService(t, 8)
	defer svc.Close()

	resp, _ := parseContent(t, svc, editedModel)
	ops := []*pb.EditOperation{
		addMember("Rig", "part def", "Mount", ""),
		addMember("Rig::Mount", "attribute", "a", "ScalarValues::Real"),
		addMember("Rig::Mount", "attribute", "b", "ScalarValues::Real"),
		addMember("Rig::Mount", "attribute", "c", "ScalarValues::Real"),
		addMember("Rig::Mount", "attribute", "d", "ScalarValues::Real"),
	}
	before := svc.libIndexes.snapshot()
	if got := applyEdits(t, svc, resp.ModelHash, ops...); got.Error != "" {
		t.Fatalf("the edit was refused: %s", got.Error)
	}
	after := svc.libIndexes.snapshot()
	if got := (after.Shared + after.Inline) - (before.Shared + before.Inline); got != 1 {
		t.Fatalf("a request of %d operations took %d indexes, want 1", len(ops), got)
	}
}
