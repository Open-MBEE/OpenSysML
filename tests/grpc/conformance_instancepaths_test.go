package grpc_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// checkReachPaths checks the graph Instantiate answers says how each object
// was reached: the paths in graph order, and each holder the object before the
// last step of its path, holding it under the feature the path ends with.
func checkReachPaths(t *testing.T, resp *pb.InstantiateResponse, want []string) {
	t.Helper()
	byID := make(map[int64]*pb.Instance, len(resp.Instances))
	var got []string
	for _, inst := range resp.Instances {
		byID[inst.Id] = inst
		if inst.ReachedBy == nil {
			got = append(got, "")
			continue
		}
		got = append(got, inst.ReachedBy.Path)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("reach paths = %q, want %q", got, want)
	}
	if resp.Instance.ReachedBy != nil {
		t.Errorf("the root says it was reached by %v, want nothing", resp.Instance.ReachedBy)
	}
	for _, inst := range resp.Instances {
		reach := inst.ReachedBy
		if reach == nil {
			continue
		}
		holder, ok := byID[reach.InstanceId]
		if !ok {
			t.Errorf("%s: holder #%d is not in the graph", reach.Path, reach.InstanceId)
			continue
		}
		step := reach.FeatureName
		if reach.Index > 0 {
			step = fmt.Sprintf("%s[%d]", reach.FeatureName, reach.Index)
		}
		wantPath := step
		if holder.ReachedBy != nil {
			wantPath = holder.ReachedBy.Path + "." + step
		}
		if reach.Path != wantPath {
			t.Errorf("%s: the holder's path and the step make %q", reach.Path, wantPath)
		}
		if reach.FeatureSymbolId == "" {
			t.Errorf("%s: no feature symbol id", reach.Path)
		}
		fv, ok := holder.FeatureValues[reach.FeatureName]
		if !ok {
			t.Errorf("%s: holder #%d has no feature %q", reach.Path, holder.Id, reach.FeatureName)
			continue
		}
		held := fv.Value
		if reach.Index > 0 {
			if reach.Index > int32(len(fv.Values)) {
				t.Errorf("%s: holder's %q has %d values", reach.Path, reach.FeatureName, len(fv.Values))
				continue
			}
			held = fv.Values[reach.Index-1]
		}
		if held.GetInstanceId() != inst.Id {
			t.Errorf("%s: holder's %q holds %v, not #%d", reach.Path, step, held, inst.Id)
		}
	}
}

// runInstancePathsCase instantiates the case's roots, then asks for every
// path to the object object_path names and checks them in order, each as
// "<root>.<path>" (the root by the name it was instantiated as, or by "#<id>"
// once another object took the name; the root alone when the object is it).
func runInstancePathsCase(t *testing.T, srv *grpc.Service, ctx context.Context, modelHash string, tc conformanceCase) {
	t.Helper()
	instantiateAll(t, srv, ctx, modelHash, tc.Instantiate)
	resp, err := srv.InstancePaths(ctx, &pb.InstancePathsRequest{
		ModelHash: modelHash,
		Object:    &pb.DocumentObject{Path: tc.ObjectPath},
	})
	if tc.ExpectedError != "" {
		if err == nil {
			t.Fatalf("InstancePaths answered %d paths, want an error containing %q", len(resp.Paths), tc.ExpectedError)
		}
		if !strings.Contains(err.Error(), tc.ExpectedError) {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), tc.ExpectedError)
		}
		return
	}
	if err != nil {
		t.Fatalf("InstancePaths: %v", err)
	}
	if resp.Truncated {
		t.Errorf("the paths are truncated")
	}
	var got []string
	for _, path := range resp.Paths {
		root := path.Root.SymbolId
		if root == "" {
			root = fmt.Sprintf("#%d", path.Root.InstanceId)
		}
		if path.Path != "" {
			root += "." + path.Path
		}
		got = append(got, root)
		var steps []string
		for _, step := range path.Steps {
			name := step.FeatureName
			if step.Index > 0 {
				name = fmt.Sprintf("%s[%d]", name, step.Index)
			}
			steps = append(steps, name)
			if step.FeatureSymbolId == "" || step.InstanceId == 0 {
				t.Errorf("%s: step %q lacks its feature symbol id or object", root, name)
			}
		}
		if strings.Join(steps, ".") != path.Path {
			t.Errorf("%s: the steps spell %q", root, strings.Join(steps, "."))
		}
	}
	if strings.Join(got, "\n") != strings.Join(tc.ExpectedPaths, "\n") {
		t.Errorf("paths = %q, want %q", got, tc.ExpectedPaths)
	}
}
