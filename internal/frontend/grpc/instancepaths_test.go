package grpc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
)

// instancePathsModel holds one port object under several paths: the tank's
// `fuelOut`, the interface end `supplier` referencing it, and both again below
// the reference `pump` to the tank.
const instancePathsModel = `package Fuel {
	port def FuelPort { out item fuel; }
	interface def FuelLine { end supplier : FuelPort; end consumer : ~FuelPort; }
	part def Tank { port fuelOut : FuelPort; }
	part def Engine { port fuelIn : ~FuelPort; }
	part def Powertrain { part tank : Tank; part engine : Engine; }
	part def Wheel;
	part vehicle {
		part powertrain : Powertrain;
		part wheels : Wheel[2];
		ref part pump : Tank = powertrain.tank;
		interface fuelLine : FuelLine connect supplier ::> powertrain.tank.fuelOut to consumer ::> powertrain.engine.fuelIn;
	}
}`

func parseAndInstantiate(t *testing.T, srv *Service, content, symbolID string) (string, *pb.InstantiateResponse) {
	t.Helper()
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: content}})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	resp, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{ModelHash: parseResp.ModelHash, SymbolId: symbolID})
	if err != nil || resp.Error != "" {
		t.Fatalf("Instantiate: %v %s", err, resp.GetError())
	}
	return parseResp.ModelHash, resp
}

func pathTexts(resp *pb.InstancePathsResponse) []string {
	texts := make([]string, 0, len(resp.Paths))
	for _, path := range resp.Paths {
		texts = append(texts, path.GetRoot().GetSymbolId()+"|"+path.Path)
	}
	return texts
}

// Every instance of the graph but the root says how the traversal reached it:
// the holder, the feature, its index within a multi-valued one and the path
// from the root, in the documented order.
func TestInstantiate_InstancesSayHowTheyWereReached(t *testing.T) {
	srv := mustNewService(t, 10)
	_, resp := parseAndInstantiate(t, srv, instancePathsModel, "Fuel::vehicle")
	if resp.Instance.ReachedBy != nil {
		t.Errorf("root reached by %v", resp.Instance.ReachedBy)
	}
	want := []string{"", "fuelLine", "fuelLine.consumer", "fuelLine.consumer.fuel", "fuelLine.supplier", "fuelLine.supplier.fuel",
		"powertrain", "powertrain.engine", "powertrain.tank", "wheels[1]", "wheels[2]"}
	var got []string
	byPath := map[string]*pb.Instance{}
	for _, inst := range resp.Instances {
		got = append(got, inst.GetReachedBy().GetPath())
		byPath[inst.GetReachedBy().GetPath()] = inst
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("paths = %q, want %q", got, want)
	}
	supplier := byPath["fuelLine.supplier"].ReachedBy
	if supplier.InstanceId != byPath["fuelLine"].Id || supplier.FeatureName != "supplier" || supplier.FeatureSymbolId != "Fuel::FuelLine::supplier" || supplier.Index != 0 {
		t.Errorf("supplier reached by %v", supplier)
	}
	wheel := byPath["wheels[2]"].ReachedBy
	if wheel.InstanceId != resp.Instance.Id || wheel.FeatureName != "wheels" || wheel.FeatureSymbolId != "Fuel::vehicle::wheels" || wheel.Index != 2 {
		t.Errorf("second wheel reached by %v", wheel)
	}
	if tank := byPath["powertrain.tank"]; tank.FeatureValues["fuelOut"].GetValue().GetInstanceId() != byPath["fuelLine.supplier"].Id {
		t.Errorf("the tank's fuelOut is not the supplier end's object")
	}
}

// InstancePaths lists every path to an object from every root the population
// holds, by id or by path; the root is reached by the empty path alone.
func TestInstancePaths_ListsEveryAlias(t *testing.T) {
	srv := mustNewService(t, 10)
	hash, resp := parseAndInstantiate(t, srv, instancePathsModel, "Fuel::vehicle")
	var port int64
	for _, inst := range resp.Instances {
		if inst.GetReachedBy().GetPath() == "fuelLine.supplier" {
			port = inst.Id
		}
	}
	paths, err := srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash, Object: &pb.DocumentObject{InstanceId: port}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Fuel::vehicle|fuelLine.supplier", "Fuel::vehicle|powertrain.tank.fuelOut", "Fuel::vehicle|pump.fuelOut"}
	if got := pathTexts(paths); strings.Join(got, " ") != strings.Join(want, " ") || paths.Truncated {
		t.Errorf("paths = %q (truncated %v), want %q", got, paths.Truncated, want)
	}
	steps := paths.Paths[1].Steps
	if len(steps) != 3 || steps[0].FeatureName != "powertrain" || steps[1].FeatureSymbolId != "Fuel::Powertrain::tank" || steps[2].InstanceId != port || steps[2].Index != 0 {
		t.Errorf("steps = %v", steps)
	}

	byPath, err := srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash, Object: &pb.DocumentObject{Path: "Fuel::vehicle.pump.fuelOut"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := pathTexts(byPath); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("by path: %q, want %q", got, want)
	}

	root, err := srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash, Object: &pb.DocumentObject{InstanceId: resp.Instance.Id}})
	if err != nil {
		t.Fatal(err)
	}
	if got := pathTexts(root); strings.Join(got, " ") != "Fuel::vehicle|" || root.Paths[0].Root.InstanceId != resp.Instance.Id || len(root.Paths[0].Steps) != 0 {
		t.Errorf("root: %q", got)
	}

	wheel, err := srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash, Object: &pb.DocumentObject{Path: "Fuel::vehicle.wheels[2]"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := pathTexts(wheel); strings.Join(got, " ") != "Fuel::vehicle|wheels[2]" || wheel.Paths[0].Steps[0].Index != 2 {
		t.Errorf("wheel: %q", got)
	}
}

// A second root reaches an object by its own paths too, listed after the
// first root's in id order.
func TestInstancePaths_ListsEveryRoot(t *testing.T) {
	srv := mustNewService(t, 10)
	hash, resp := parseAndInstantiate(t, srv, instancePathsModel, "Fuel::vehicle")
	if again, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{ModelHash: hash, SymbolId: "Fuel::vehicle"}); err != nil || again.Error != "" {
		t.Fatalf("Instantiate: %v %s", err, again.GetError())
	}
	paths, err := srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash, Object: &pb.DocumentObject{Path: fmt.Sprintf("#%d.powertrain.tank", resp.Instance.Id)}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"|powertrain.tank", "|pump"}
	if got := pathTexts(paths); strings.Join(got, " ") != strings.Join(want, " ") || paths.Paths[0].Root.InstanceId != resp.Instance.Id {
		t.Errorf("paths = %q, want %q from the displaced root", got, want)
	}
}

// One limit covers every root: a root that would add paths past it makes the
// answer truncated, never longer; one that reaches the object no more leaves
// a full answer complete.
func TestInstancePaths_OneLimitOverEveryRoot(t *testing.T) {
	srv := mustNewService(t, 10)
	hash, resp := parseAndInstantiate(t, srv, instancePathsModel, "Fuel::vehicle")
	cached, _ := srv.cache.Get(hash)
	held := srv.objects(cached)
	defer held.lock(context.Background())()
	root, _, err := held.resolve("object", &pb.DocumentObject{InstanceId: resp.Instance.Id})
	if err != nil {
		t.Fatal(err)
	}
	port, _, err := held.resolve("object", &pb.DocumentObject{Path: "Fuel::vehicle.pump.fuelOut"})
	if err != nil {
		t.Fatal(err)
	}
	depth := protoconv.DefaultGraphBounds().Depth
	if paths, truncated := pathsFromRoots(held.rt, []*runtime.Instance{root}, port, depth, 3); len(paths) != 3 || truncated {
		t.Errorf("three allowed from one root: %d paths, truncated %v", len(paths), truncated)
	}
	if paths, truncated := pathsFromRoots(held.rt, []*runtime.Instance{root, root}, port, depth, 3); len(paths) != 3 || !truncated {
		t.Errorf("three allowed from two roots: %d paths, truncated %v", len(paths), truncated)
	}
	wheel, _, err := held.resolve("object", &pb.DocumentObject{Path: "Fuel::vehicle.wheels[1]"})
	if err != nil {
		t.Fatal(err)
	}
	if paths, truncated := pathsFromRoots(held.rt, []*runtime.Instance{root, wheel}, port, depth, 3); len(paths) != 3 || truncated {
		t.Errorf("a root past the limit that reaches the object no more: %d paths, truncated %v", len(paths), truncated)
	}
}

// An object nothing names is not found, an empty request is invalid, and a
// service without the capability refuses the call.
func TestInstancePaths_Errors(t *testing.T) {
	srv := mustNewService(t, 10)
	hash, _ := parseAndInstantiate(t, srv, instancePathsModel, "Fuel::vehicle")
	_, err := srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash, Object: &pb.DocumentObject{InstanceId: 999}})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown id: %v", err)
	}
	_, err = srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash, Object: &pb.DocumentObject{Path: "Fuel::nothing"}})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown name: %v", err)
	}
	_, err = srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash, Object: &pb.DocumentObject{Path: "Fuel::vehicle.nowhere"}})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("unknown path: %v", err)
	}
	_, err = srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no object: %v", err)
	}
	_, err = srv.InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: "nope", Object: &pb.DocumentObject{InstanceId: 1}})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown model: %v", err)
	}
	_, err = mustNewServiceWithout(t, CapabilityInstancePaths).InstancePaths(context.Background(), &pb.InstancePathsRequest{ModelHash: hash, Object: &pb.DocumentObject{InstanceId: 1}})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityInstancePaths) {
		t.Errorf("without the capability: %v", err)
	}
	without := mustNewServiceWithout(t, CapabilityInstancePaths)
	if _, resp := parseAndInstantiate(t, without, instancePathsModel, "Fuel::vehicle"); resp.Instances[1].ReachedBy != nil {
		t.Errorf("without the capability an instance says how it was reached: %v", resp.Instances[1].ReachedBy)
	}
}
