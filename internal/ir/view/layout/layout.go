// Package layout places the nodes of a drawing with Graphviz's layered
// engine: a positions-only graph is laid out by `dot` and the boxes it
// settles on are read back, for the caller to route edges over and draw.
package layout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// Graph is a drawing to lay out: its nodes, nested where the drawing nests
// them, and the edges whose ends the layering keeps near each other.
type Graph struct {
	// Direction is the rank direction, Graphviz's rankdir: TB, LR, RL or BT;
	// empty leaves dot's top-to-bottom.
	Direction string
	// Nodes are the drawing's roots; a node with children is a container.
	Nodes []Node
	// Edges are the edges laid out; an end naming no node is skipped.
	Edges []Edge
	// Gap is the room left between neighbouring nodes and ranks, in pixels.
	Gap float64
	// Pad is the room a container keeps between its border and its members.
	Pad float64
}

// Node is one box to place. A leaf is placed at Width by Height; a container
// keeps a header Width by Height above its members and grows round them.
type Node struct {
	ID            string
	Width, Height float64
	Children      []Node
}

// Edge joins two nodes by ID, either of which may be a container.
type Edge struct {
	From, To string
}

// Box is a placed node: its top-left corner and size in pixels, y down.
type Box struct {
	X, Y, Width, Height float64
}

// Points is Graphviz's points to the inch, the pixel scale the DOT and
// draw.io forms position with.
const Points = 72.0

// Place lays g out with the dot at path and returns a box for every node,
// containers included, in pixels from the drawing's top-left corner.
func Place(path string, g Graph) (map[string]Box, error) {
	source := g.dot()
	cmd := exec.Command(path, "-Tjson0")
	cmd.Stdin = strings.NewReader(source)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, &Error{Path: path, Err: err, Stderr: strings.TrimSpace(stderr.String())}
	}
	boxes, err := parse(stdout.Bytes(), g.containers())
	if err != nil {
		return nil, &Error{Path: path, Err: err, Stderr: strings.TrimSpace(stderr.String())}
	}
	return boxes, nil
}

// Error is a dot run that failed: the process, or its answer.
type Error struct {
	Path   string
	Err    error
	Stderr string
}

func (e *Error) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("%s -Tjson0: %v: %s", e.Path, e.Err, e.Stderr)
	}
	return fmt.Sprintf("%s -Tjson0: %v", e.Path, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// containers are the IDs of the nodes with children, drawn as clusters.
func (g Graph) containers() map[string]bool {
	out := map[string]bool{}
	var walk func(nodes []Node)
	walk = func(nodes []Node) {
		for _, node := range nodes {
			if len(node.Children) > 0 {
				out[node.ID] = true
			}
			walk(node.Children)
		}
	}
	walk(g.Nodes)
	return out
}

// dot is the positions-only graph: every leaf a fixed-size box, every
// container a cluster whose label reserves its header, every edge drawn to
// a node or to the point anchoring a container, with no routing asked for.
func (g Graph) dot() string {
	var b strings.Builder
	b.WriteString("digraph layout {\n")
	fmt.Fprintf(&b, "  graph [splines=none, nodesep=%s, ranksep=%s", inches(g.Gap), inches(g.Gap))
	if g.Direction != "" {
		b.WriteString(", rankdir=" + g.Direction)
	}
	b.WriteString("];\n  node [shape=box, fixedsize=true, label=\"\"];\n")
	known := map[string]bool{}
	var collect func(nodes []Node)
	collect = func(nodes []Node) {
		for _, node := range nodes {
			known[node.ID] = true
			collect(node.Children)
		}
	}
	collect(g.Nodes)
	anchored := map[string]bool{}
	var edges []Edge
	for _, edge := range g.Edges {
		if !known[edge.From] || !known[edge.To] {
			continue
		}
		edges = append(edges, edge)
		anchored[edge.From], anchored[edge.To] = true, true
	}
	seen := map[string]bool{}
	for _, node := range g.Nodes {
		g.writeNode(&b, node, "  ", anchored, seen)
	}
	for _, edge := range edges {
		fmt.Fprintf(&b, "  %s -> %s;\n", quote(edge.From), quote(edge.To))
	}
	b.WriteString("}\n")
	return b.String()
}

// writeNode writes a node once, where it first appears: a leaf as a box, a
// container as a cluster holding its anchor point when an edge meets it.
func (g Graph) writeNode(b *strings.Builder, node Node, indent string, anchored, seen map[string]bool) {
	if seen[node.ID] {
		return
	}
	seen[node.ID] = true
	if len(node.Children) == 0 {
		fmt.Fprintf(b, "%s%s [width=%s, height=%s];\n", indent, quote(node.ID), inches(node.Width), inches(node.Height))
		return
	}
	fmt.Fprintf(b, "%ssubgraph %s {\n", indent, quote("cluster_"+node.ID))
	fmt.Fprintf(b, "%s  label=<<table border=\"0\" cellborder=\"0\" cellspacing=\"0\" cellpadding=\"0\"><tr><td fixedsize=\"true\" width=\"%s\" height=\"%s\"></td></tr></table>>;\n", indent, points(node.Width), points(node.Height))
	fmt.Fprintf(b, "%s  margin=%s;\n", indent, points(g.Pad))
	if anchored[node.ID] {
		fmt.Fprintf(b, "%s  %s [shape=point, style=invis, width=0, height=0];\n", indent, quote(node.ID))
	}
	for _, child := range node.Children {
		g.writeNode(b, child, indent+"  ", anchored, seen)
	}
	fmt.Fprintf(b, "%s}\n", indent)
}

// graph is the layout dot answers, as -Tjson0 writes it: the graph's bounds
// and every node and cluster with its place.
type graph struct {
	BB      string   `json:"bb"`
	Objects []object `json:"objects"`
}

type object struct {
	Name   string `json:"name"`
	BB     string `json:"bb"`
	Pos    string `json:"pos"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

// parse reads the boxes out of dot's answer, flipping Graphviz's y-up points
// into pixels from the top-left corner, whole: dot prints inches to five
// places, noise a drawing has no use for.
func parse(answer []byte, containers map[string]bool) (map[string]Box, error) {
	var g graph
	if err := json.Unmarshal(answer, &g); err != nil {
		return nil, fmt.Errorf("layout answer: %w", err)
	}
	_, _, _, top, err := bounds(g.BB)
	if err != nil {
		return nil, fmt.Errorf("layout answer: graph bb %q: %w", g.BB, err)
	}
	top = math.Ceil(top)
	boxes := make(map[string]Box, len(g.Objects))
	for _, o := range g.Objects {
		switch {
		case strings.HasPrefix(o.Name, "cluster_"):
			x1, y1, x2, y2, err := bounds(o.BB)
			if err != nil {
				return nil, fmt.Errorf("layout answer: cluster %s bb %q: %w", o.Name, o.BB, err)
			}
			x1, y1, x2, y2 = math.Floor(x1), math.Floor(y1), math.Ceil(x2), math.Ceil(y2)
			boxes[strings.TrimPrefix(o.Name, "cluster_")] = Box{X: x1, Y: top - y2, Width: x2 - x1, Height: y2 - y1}
		case containers[o.Name]:
			// A container's anchor point: its cluster places it.
		default:
			cx, cy, err := pair(o.Pos)
			if err != nil {
				return nil, fmt.Errorf("layout answer: node %s pos %q: %w", o.Name, o.Pos, err)
			}
			w, errW := strconv.ParseFloat(o.Width, 64)
			h, errH := strconv.ParseFloat(o.Height, 64)
			if errW != nil || errH != nil {
				return nil, fmt.Errorf("layout answer: node %s size %q by %q", o.Name, o.Width, o.Height)
			}
			w, h = math.Round(w*Points), math.Round(h*Points)
			boxes[o.Name] = Box{X: math.Round(cx - w/2), Y: math.Round(top - cy - h/2), Width: w, Height: h}
		}
	}
	return boxes, nil
}

func bounds(bb string) (x1, y1, x2, y2 float64, err error) {
	parts := strings.Split(bb, ",")
	if len(parts) != 4 {
		return 0, 0, 0, 0, fmt.Errorf("want four coordinates")
	}
	values := make([]float64, 4)
	for i, part := range parts {
		if values[i], err = strconv.ParseFloat(part, 64); err != nil {
			return 0, 0, 0, 0, err
		}
	}
	return values[0], values[1], values[2], values[3], nil
}

func pair(pos string) (x, y float64, err error) {
	parts := strings.Split(pos, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("want two coordinates")
	}
	if x, err = strconv.ParseFloat(parts[0], 64); err != nil {
		return 0, 0, err
	}
	if y, err = strconv.ParseFloat(parts[1], 64); err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

// inches is a pixel length as Graphviz's inch attributes take it.
func inches(px float64) string { return strconv.FormatFloat(px/Points, 'f', -1, 64) }

// points is a pixel length as Graphviz's point attributes take it, whole.
func points(px float64) string { return strconv.FormatFloat(math.Ceil(px), 'f', -1, 64) }

// quote is text as a DOT identifier.
func quote(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + replacer.Replace(text) + `"`
}
