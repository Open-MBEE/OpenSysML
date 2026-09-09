package grpc

import (
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// nestedObjectsModelSource is a case whose outputs name objects only within
// structured values: a set, an array, a sequence of sequences, and a function
// read off a part.
const nestedObjectsModelSource = `package Nested {
	private import ScalarValues::*;
	private import Collections::*;

	part def Engine { attribute mass : Real; calc weigh { in x : Real; return : Real = mass * x; } }
	part a : Engine { attribute :>> mass = 30.0; }
	part b : Engine { attribute :>> mass = 10.0; }
	part c : Engine { attribute :>> mass = 20.0; }
	part d : Engine { attribute :>> mass = 40.0; }
	part e : Engine { attribute :>> mass = 50.0; }
	part f : Engine { attribute :>> mass = 60.0; }
	part g : Engine { attribute :>> mass = 70.0; }
	attribute pool : Set { :>> elements = (a, b); }
	attribute grid : Array { :>> dimensions = (1, 2); :>> elements = (c, d); }

	analysis def Grouping {
		subject s : Engine;
		in attribute k : Real = 1.0;
		out attribute pooled : Engine[*] = pool.elements;
		out attribute gridded : Array = grid;
		out attribute nested : Engine[*] = ((e), (s, (f)));
		out attribute weigher = g.weigh;
		return : Real = s.mass * k;
	}
	analysis grouping : Grouping { subject s = a; }
}
`

// idsOf collects the instance ids a wire value refers to, within any nesting.
func idsOf(v *pb.Value) []int64 {
	switch kind := v.GetKind().(type) {
	case *pb.Value_InstanceId:
		return []int64{kind.InstanceId}
	case *pb.Value_Function:
		if kind.Function.GetSelfId() != 0 {
			return []int64{kind.Function.GetSelfId()}
		}
	case *pb.Value_Sequence:
		var ids []int64
		for _, elem := range kind.Sequence.GetElements() {
			ids = append(ids, idsOf(elem)...)
		}
		return ids
	case *pb.Value_Set:
		var ids []int64
		for _, elem := range kind.Set.GetElements() {
			ids = append(ids, idsOf(elem)...)
		}
		return ids
	case *pb.Value_Array:
		var ids []int64
		for _, elem := range kind.Array.GetElements() {
			ids = append(ids, idsOf(elem)...)
		}
		return ids
	}
	return nil
}

// wantInstancesResolve checks the outputs cross in their structured arms, that
// every object they refer to is in the instance table once, and that together
// they name the expected number of distinct objects.
func wantInstancesResolve(t *testing.T, outputs []*pb.CalcOutput, instances []*pb.Instance, wantDistinct int) {
	t.Helper()
	var ids []int64
	for _, out := range outputs {
		switch out.Name {
		case "pooled":
			if out.GetValue().GetSet() == nil {
				t.Errorf("pooled crossed as %v, want a set", out.GetValue())
			}
		case "gridded":
			if out.GetValue().GetArray() == nil {
				t.Errorf("gridded crossed as %v, want an array", out.GetValue())
			}
		case "weigher":
			if out.GetValue().GetFunction().GetSelfId() == 0 {
				t.Errorf("weigher crossed as %v, want a function read off an object", out.GetValue())
			}
		}
		ids = append(ids, idsOf(out.GetValue())...)
	}
	slices.Sort(ids)
	if distinct := slices.Compact(slices.Clone(ids)); len(distinct) != wantDistinct {
		t.Errorf("outputs name %d distinct objects %v, want %d", len(distinct), distinct, wantDistinct)
	}
	for _, id := range ids {
		n := 0
		for _, inst := range instances {
			if inst.Id == id {
				n++
				if !strings.HasPrefix(inst.TypeSymbolId, "Nested::") {
					t.Errorf("instance %d is a %q, want one of the model's parts", id, inst.TypeSymbolId)
				}
			}
		}
		if n != 1 {
			t.Errorf("object %d appears %d time(s) among the instances, want once", id, n)
		}
	}
}

// TestRunAnalysisReportsNestedObjects verifies objects named only within a
// set, an array, a nested sequence or a function value in the outputs are among
// the response's instances, each once.
func TestRunAnalysisReportsNestedObjects(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, nestedObjectsModelSource, "nested-objects")

	resp := runAnalysis(t, srv, &pb.RunAnalysisRequest{ModelHash: hash, SymbolId: "Nested::grouping"})
	if resp.Error != "" {
		t.Fatalf("RunAnalysis reported %q", resp.Error)
	}
	if len(resp.Outputs) != 5 || realOutput(t, resp, "result") != 30.0 {
		t.Fatalf("outputs = %v, want pooled, gridded, nested, weigher and result 30.0", resp.Outputs)
	}
	// a (the subject too), b, c, d, e, f and g.
	wantInstancesResolve(t, resp.Outputs, resp.Instances, 7)
}

// TestRunSweepReportsNestedObjects verifies the same for every row of a sweep,
// whose instance table is shared by the rows.
func TestRunSweepReportsNestedObjects(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, nestedObjectsModelSource, "nested-objects-sweep")

	resp := runSweep(t, srv, &pb.RunSweepRequest{
		ModelHash: hash, SymbolId: "Nested::grouping",
		Ranges: []*pb.SweepRange{{Parameter: "k", Start: realProto(1), End: realProto(2), Step: realProto(1)}},
	})
	if resp.Error != "" || len(resp.Rows) != 2 {
		t.Fatalf("RunSweep = %q with %d row(s); want two rows: %s", resp.Error, len(resp.Rows), rowText(resp))
	}
	for i, row := range resp.Rows {
		if row.Error != "" || len(row.Outputs) != 5 {
			t.Fatalf("row %d = %v; want five outputs and no error", i, row)
		}
		wantInstancesResolve(t, row.Outputs, resp.Instances, 7)
	}
}
