package view

import (
	"strings"
	"testing"
)

// annotationNodesUnder collects the comment, documentation and textual
// representation nodes anywhere under roots.
func annotationNodesUnder(roots []*Node) []*Node {
	var out []*Node
	var walk func(node *Node)
	walk = func(node *Node) {
		if annotationKind(node.Kind) {
			out = append(out, node)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	return out
}

func anchorEdges(r *Rendering) int {
	n := 0
	for _, edge := range r.Edges {
		if edge.Kind == EdgeAnchor {
			n++
		}
	}
	return n
}

// A tree draws each comment, doc and rep as a note joined to every element it
// annotates (§8.2.3.4), inside the view frame (§8.2.3.26), in every form.
func TestAnnotationNodesAndLinksAreDrawn(t *testing.T) {
	rendering := render(t, "annotations.sysml", "AnnotationViews::tree")
	kinds := map[string]int{}
	for _, node := range annotationNodesUnder(rendering.Roots) {
		kinds[node.Kind]++
	}
	if kinds[commentKind] != 2 || kinds[docKind] != 2 || kinds[repKind] != 1 {
		t.Fatalf("annotation nodes by kind = %v, want 2 comments, 2 docs, 1 rep", kinds)
	}
	// Pump's doc, two comments and rep, Safety to Tank as well, and Tank's doc.
	if got := anchorEdges(rendering); got != 6 {
		t.Errorf("annotation links = %d, want 6: %+v", got, rendering.Edges)
	}
	f := rendering.Frame
	if f == nil || f.Name != "AnnotationViews::tree" || f.Type != "" || len(f.Exposes) != 2 || len(f.Filters) != 0 || len(f.Annotations) != 1 {
		t.Fatalf("frame = %+v, want the view's name, two exposes and its doc", f)
	}
	for form, wants := range map[Form][]string{
		FormDot:      {"shape=note", "«comment»", "«doc»", "«rep»", "style=dashed", dotFrameCluster, "«view»", `"frame_tr"`, `"frame_br"`},
		FormPlantUML: {"note as ", "mainframe ", "«view»", "expose Annotated::Pump"},
		FormMermaid:  {"notch-rect", "title: "},
		FormD2:       {"class: annotation", "frame_name:", "frame_tr:", "frame_br:"},
		FormDrawio:   {"shape=note", "shape=umlFrame", `id="frame_tr"`},
		FormText:     {`comment "Sized for the nominal flow."`, `comment Safety (locale "en")`, `rep inline (language "plantuml")`, `doc Capacity (locale "en")`},
	} {
		out, err := rendering.Write(form)
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("%s lacks %q:\n%s", form, want, out)
			}
		}
	}
}

// A rendering with showAnnotations = false draws no annotation node, no
// annotation link and no layout note anchored to a hidden annotation.
func TestAnnotationsHiddenByTheRendering(t *testing.T) {
	bare := render(t, "annotations.sysml", "AnnotationViews::bare").showing(Shown{})
	if nodes := annotationNodesUnder(bare.Roots); len(nodes) != 0 {
		t.Errorf("hidden annotations are drawn: %+v", nodes)
	}
	if got := anchorEdges(bare); got != 0 {
		t.Errorf("annotation links = %d, want none: %+v", got, bare.Edges)
	}
	ids := map[string]bool{}
	var walk func(node *Node)
	walk = func(node *Node) {
		ids[node.ID] = true
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range bare.Roots {
		walk(root)
	}
	for _, note := range bare.Notes {
		if note.Anchor != "" && !ids[note.Anchor] {
			t.Errorf("note %q is anchored to the hidden node %q", note.Text, note.Anchor)
		}
	}
	shown := render(t, "annotations.sysml", "AnnotationViews::tree")
	if on, off := strings.Join(shown.Shown.Names(), ","), strings.Join(render(t, "annotations.sysml", "AnnotationViews::bare").Shown.Names(), ","); !strings.Contains(on, "annotations") || strings.Contains(off, "annotations") {
		t.Errorf("shown = %v and %v; annotations are on by default and off by the rendering", shown.Shown, bare.Shown)
	}
}

// A view's element filter admits or rejects annotations like any member.
func TestAnnotationsObeyViewFilters(t *testing.T) {
	filtered := render(t, "annotations.sysml", "AnnotationViews::filtered")
	if nodes := annotationNodesUnder(filtered.Roots); len(nodes) != 0 {
		t.Errorf("filter @SysML::PartDefinition lets annotations through: %+v", nodes)
	}
	if got := anchorEdges(filtered); got != 0 {
		t.Errorf("annotation links = %d, want none", got)
	}
}

// A requirement's documentation is drawn in full as a doc node; the clipped
// excerpt in the requirement's detail does not stand in for it.
func TestRequirementDocumentationIsDrawn(t *testing.T) {
	rendering := render(t, "general.sysml", "GeneralViews::requirementView")
	docs := 0
	for _, node := range annotationNodesUnder(rendering.Roots) {
		if node.Kind == docKind {
			docs++
		}
	}
	if docs == 0 {
		t.Fatalf("the requirement graph draws no doc node: %+v", rendering.Roots)
	}
}

// A typed view's frame names its view definition and carries its filters in
// the bottom-left info compartment; the Cameo style keeps its own frame header.
func TestViewFrameCompartments(t *testing.T) {
	typed := render(t, "annotations.sysml", "AnnotationViews::definitions")
	f := typed.Frame
	if f == nil || f.Type != "GeneralView" || len(f.Exposes) != 1 || strings.Join(f.Filters, ";") != "filter @SysML::PartDefinition" {
		t.Fatalf("frame = %+v, want GeneralView with one expose and one filter", f)
	}
	if got, want := f.NameLine(), "«view» AnnotationViews::definitions : GeneralView"; got != want {
		t.Errorf("name line = %q, want %q", got, want)
	}
	dot, err := typed.DOT()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"frame_tr"`, `"frame_bl"`, "filter @SysML::PartDefinition", "<b>AnnotationViews::definitions</b> : GeneralView"} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	cameo, err := typed.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cameo, "frame_") || !strings.Contains(cameo, cameoFrameColor) {
		t.Errorf("the Cameo frame is not Cameo's own:\n%s", cameo)
	}
}

// withoutViewFrame drops the DOT lines of the view frame: its cluster and the
// info compartments at its corners.
func withoutViewFrame(dot string) string {
	var kept []string
	for _, line := range strings.Split(dot, "\n") {
		if strings.Contains(line, dotFrameCluster) || strings.Contains(line, `"frame_`) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
