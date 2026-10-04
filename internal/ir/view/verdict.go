package view

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Overlay is what a rendering draws over the model's structure; "" draws none.
type Overlay string

// OverlayVerdicts runs the verification cases verifying each requirement of a
// requirement rendering and colours and labels the requirement by their verdicts.
const OverlayVerdicts Overlay = "verdicts"

// Overlays are the overlays there are.
func Overlays() []Overlay { return []Overlay{OverlayVerdicts} }

// OverlayNames spells the overlays there are, for a message naming them.
func OverlayNames() string {
	names := make([]string, 0, len(Overlays()))
	for _, o := range Overlays() {
		names = append(names, string(o))
	}
	return strings.Join(names, ", ")
}

// ParseOverlay is the overlay named; "" is none, which is valid.
func ParseOverlay(name string) (Overlay, bool) {
	if name == "" {
		return "", true
	}
	for _, o := range Overlays() {
		if string(o) == name {
			return o, true
		}
	}
	return "", false
}

// SupportsOverlay reports whether a rendering of the kind draws o: verdicts
// are drawn on a requirement rendering alone.
func (k Kind) SupportsOverlay(o Overlay) bool {
	return o == "" || o == OverlayVerdicts && k == KindRequirement
}

// Verdict is the outcome of one verification case run on a requirement.
type Verdict struct {
	// Case is the verification case, by qualified name.
	Case string
	// Kind is pass, fail, inconclusive or error.
	Kind string
	// Detail is why a run was inconclusive or failed to run, "" otherwise.
	Detail string
}

// Verdicts answers the verdicts of the verification cases verifying a
// requirement, in a stable order; none for a requirement nothing verifies.
type Verdicts func(requirement *symbols.Symbol) []Verdict

// SetVerdicts overlays each requirement of a requirement rendering with the
// verdicts v answers; nil, the default, draws the rendering structurally.
func (r *Renderer) SetVerdicts(v Verdicts) { r.verdicts = v }

// The verdict kinds, worst last, and the colour each is drawn in: the
// Okabe-Ito bluish green, yellow, vermillion and reddish purple.
var verdictOrder = []string{"pass", "inconclusive", "fail", "error"}

var verdictColors = map[string]string{
	"pass": "#009E73", "inconclusive": "#F0E442", "fail": "#D55E00", "error": "#CC79A7",
}

// overlayVerdicts labels a requirement node with each verdict on it and
// colours it by the worst, unless the view styles the node itself.
func (g *generalGraph) overlayVerdicts(sym *symbols.Symbol, node *Node) {
	if g.r.verdicts == nil {
		return
	}
	verdicts := g.r.verdicts(sym)
	if len(verdicts) == 0 {
		return
	}
	worst := -1
	parts := make([]string, 0, len(verdicts))
	for _, v := range verdicts {
		rank := verdictRank(v.Kind)
		worst = max(worst, rank)
		part := v.Kind + " by " + v.Case
		if detail := strings.Join(strings.Fields(v.Detail), " "); detail != "" && v.Kind != "pass" && v.Kind != "fail" {
			part += " (" + detail + ")"
		}
		parts = append(parts, part)
	}
	if worst < 0 {
		return
	}
	node.Verdict = verdictOrder[worst]
	node.Detail = detailWith(node.Detail, "verdict "+strings.Join(parts, ", "))
	if node.Style == nil {
		color := verdictColors[node.Verdict]
		node.Style = &Style{Fill: paletteFill(color, true), Line: color}
		node.verdictStyled = true
	}
}

// verdictRank orders a verdict kind by severity, -1 for one not known.
func verdictRank(kind string) int {
	for i, k := range verdictOrder {
		if k == kind {
			return i
		}
	}
	return -1
}
