//go:build !js && !wasip1

package route

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

// goldenScene is one scene of testdata/parity, recorded by the panel's
// avoidRoutes through editors/vscode/tools/route-goldens.mjs.
type goldenScene struct {
	Name   string `json:"name"`
	Shapes []struct {
		ID    string `json:"id"`
		Box   goldenBox
		Ports []struct {
			ID     string  `json:"id"`
			Side   Side    `json:"side"`
			Offset float64 `json:"offset"`
			Flush  bool    `json:"flush"`
		} `json:"ports"`
	} `json:"shapes"`
	Edges []struct {
		Index    int    `json:"index"`
		From     string `json:"from"`
		To       string `json:"to"`
		FromPort string `json:"fromPort"`
		ToPort   string `json:"toPort"`
	} `json:"edges"`
	Bounds *goldenBox               `json:"bounds"`
	Routes map[string][]goldenPoint `json:"routes"`
}

type goldenBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type goldenPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (b goldenBox) box() Box { return Box{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height} }

func (s *goldenScene) inputs() ([]Shape, []Edge, *Box) {
	shapes := make([]Shape, 0, len(s.Shapes))
	for _, shape := range s.Shapes {
		ports := make([]Port, 0, len(shape.Ports))
		for _, port := range shape.Ports {
			ports = append(ports, Port{ID: port.ID, Side: port.Side, Offset: port.Offset, Flush: port.Flush})
		}
		shapes = append(shapes, Shape{ID: shape.ID, Box: shape.Box.box(), Ports: ports})
	}
	edges := make([]Edge, 0, len(s.Edges))
	for _, edge := range s.Edges {
		edges = append(edges, Edge{Index: edge.Index, From: edge.From, To: edge.To, FromPort: edge.FromPort, ToPort: edge.ToPort})
	}
	var bounds *Box
	if s.Bounds != nil {
		b := s.Bounds.box()
		bounds = &b
	}
	return shapes, edges, bounds
}

func loadGoldens(t testing.TB) []goldenScene {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "parity", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no parity goldens: %v", err)
	}
	sort.Strings(files)
	var scenes []goldenScene
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var golden struct {
			Scenes []goldenScene `json:"scenes"`
		}
		if err := json.Unmarshal(data, &golden); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		scenes = append(scenes, golden.Scenes...)
	}
	return scenes
}

// TestParityWithPanel routes every recorded scene and expects the panel's
// routes point for point; regenerate the goldens with
// `node tools/route-goldens.mjs` in editors/vscode after changing avoid.ts.
func TestParityWithPanel(t *testing.T) {
	scenes := loadGoldens(t)
	compared := 0
	for _, scene := range scenes {
		shapes, edges, bounds := scene.inputs()
		routes, err := Routes(shapes, edges, bounds)
		if err != nil {
			t.Errorf("%s: %v", scene.Name, err)
			continue
		}
		got := make(map[string][]Point, len(routes))
		for _, route := range routes {
			got[strconv.Itoa(route.Index)] = route.Points
		}
		if len(got) != len(scene.Routes) {
			t.Errorf("%s: routed %d edges, panel routed %d", scene.Name, len(got), len(scene.Routes))
		}
		for index, want := range scene.Routes {
			points, ok := got[index]
			if !ok {
				t.Errorf("%s edge %s: no route; panel: %v", scene.Name, index, want)
				continue
			}
			compared++
			if len(points) != len(want) {
				t.Errorf("%s edge %s: %d points, panel %d\n got %v\nwant %v", scene.Name, index, len(points), len(want), points, want)
				continue
			}
			for i := range want {
				if points[i].X != want[i].X || points[i].Y != want[i].Y {
					t.Errorf("%s edge %s point %d: got %v, panel %v", scene.Name, index, i, points[i], want[i])
					break
				}
			}
		}
	}
	if compared == 0 {
		t.Fatal("no routes compared")
	}
	t.Logf("%d scenes, %d routes compared with the panel", len(scenes), compared)
}

// BenchmarkRouteParityScenes routes every golden scene once per iteration.
func BenchmarkRouteParityScenes(b *testing.B) {
	scenes := loadGoldens(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scene := scenes[i%len(scenes)]
		shapes, edges, bounds := scene.inputs()
		if _, err := Routes(shapes, edges, bounds); err != nil {
			b.Fatal(err)
		}
	}
}
