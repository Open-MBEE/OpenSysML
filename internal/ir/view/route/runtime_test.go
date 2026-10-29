package route

import (
	"errors"
	"reflect"
	"testing"
)

var threeShapes = []Shape{
	{ID: "a", Box: Box{X: 0, Y: 0, Width: 100, Height: 60}},
	{ID: "b", Box: Box{X: 300, Y: 200, Width: 100, Height: 60}},
	{ID: "c", Box: Box{X: 150, Y: 80, Width: 80, Height: 80}},
}

var twoEdges = []Edge{{Index: 0, From: "a", To: "b"}, {Index: 1, From: "b", To: "a"}}

// TestRouteIsDeterministic routes the same scene twice and expects identical routes.
func TestRouteIsDeterministic(t *testing.T) {
	router, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	first, err := router.Route(threeShapes, twoEdges, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(twoEdges) {
		t.Fatalf("routed %d of %d edges: %+v", len(first), len(twoEdges), first)
	}
	for _, route := range first {
		if len(route.Points) < 2 || !orthogonal(route.Points) {
			t.Errorf("edge %d: route %+v is not an orthogonal polyline", route.Index, route.Points)
		}
	}
	second, err := router.Route(threeShapes, twoEdges, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("routes differ between runs:\n%+v\n%+v", first, second)
	}
}

// TestRouteSkipsEdgesWithoutShapes leaves an edge to an unknown shape unrouted.
func TestRouteSkipsEdgesWithoutShapes(t *testing.T) {
	router, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	routes, err := router.Route(threeShapes, []Edge{{Index: 7, From: "a", To: "missing"}, {Index: 8, From: "a", To: "c"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].Index != 8 {
		t.Fatalf("expected only edge 8 routed, got %+v", routes)
	}
}

// TestNewRouterRejectsCorruptModule reports a ModuleError naming the source.
func TestNewRouterRejectsCorruptModule(t *testing.T) {
	_, err := NewRouter([]byte("\x00asm\x01\x00\x00\x00garbage"), "corrupt.wasm")
	var moduleErr *ModuleError
	if !errors.As(err, &moduleErr) {
		t.Fatalf("expected *ModuleError, got %T %v", err, err)
	}
	if moduleErr.Source != "corrupt.wasm" {
		t.Errorf("source = %q", moduleErr.Source)
	}
}

// TestNewRouterRequiresBindings refuses a valid module that is not libavoid.
func TestNewRouterRequiresBindings(t *testing.T) {
	// The smallest valid module: magic and version, no sections.
	_, err := NewRouter([]byte("\x00asm\x01\x00\x00\x00"), "empty.wasm")
	var moduleErr *ModuleError
	if !errors.As(err, &moduleErr) {
		t.Fatalf("expected *ModuleError, got %T %v", err, err)
	}
}

// TestModuleBytesHonoursEnvironment loads the module OPENSYSML_LIBAVOID names.
func TestModuleBytesHonoursEnvironment(t *testing.T) {
	t.Setenv(EnvVar, t.TempDir()+"/missing.wasm")
	_, _, err := moduleBytes()
	var moduleErr *ModuleError
	if !errors.As(err, &moduleErr) {
		t.Fatalf("expected *ModuleError for a missing file, got %T %v", err, err)
	}
	t.Setenv(EnvVar, "")
	data, source, err := moduleBytes()
	if err != nil || source != "embedded" || len(data) != len(embeddedModule) {
		t.Fatalf("embedded fallback: source=%q len=%d err=%v", source, len(data), err)
	}
}
