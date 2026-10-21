package view

import (
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultAnimationStep is how long the animated SVG form spends on each step of a run
// (a timeline instant where a state changes, a message) and on the final state.
const DefaultAnimationStep = time.Second

// svgNamespace is the SVG document namespace, the only reference the form writes.
const svgNamespace = "http://www.w3.org/2000/svg"

// svgNoticePrefix opens each notice line under the title; svgGroupEnd closes a drawn group.
const (
	svgNoticePrefix = "not represented: "
	svgGroupEnd     = "</g>\n"
)

// The SVG form's geometry, in user units at default zoom.
const (
	svgMargin         = 16.0
	svgFontSize       = 12.0
	svgCharWidth      = 7.0
	svgLineHeight     = 16.0
	svgLaneHeight     = 28.0
	svgLaneGap        = 10.0
	svgAxisHeight     = 32.0
	svgChartMinWidth  = 320.0
	svgChartMaxWidth  = 1200.0
	svgTickLabelGap   = 28.0
	svgHeadHeight     = 36.0
	svgMessageRow     = 40.0
	svgColumnMinWidth = 140.0
	svgSelfLoopWidth  = 32.0
	svgDimOpacity     = 0.25
)

// svgWriter writes a run rendering as a self-contained SVG document: no script,
// no style sheet, no reference outside it but the links asked for, ids by drawing order.
type svgWriter struct {
	b        strings.Builder
	r        *Rendering
	options  Options
	animated bool
	// step is the animation step; total the loop, play the part of it replaying the run.
	step, play, total time.Duration
}

// SVGWith is the rendering as an SVG drawing, animated replaying the run with SMIL;
// the base attributes stay the complete, final picture for a viewer without SMIL.
func (r *Rendering) SVGWith(options Options, animated bool) string {
	w := &svgWriter{r: r, options: options, animated: animated, step: options.AnimationStep}
	if w.step <= 0 {
		w.step = DefaultAnimationStep
	}
	switch r.Kind {
	case KindTimeline:
		w.timeline()
	default:
		w.sequence()
	}
	return w.b.String()
}

// open writes the document head for a drawing of width × height.
func (w *svgWriter) open(width, height float64) {
	fmt.Fprintf(&w.b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(&w.b, "<svg xmlns=%q width=%q height=%q viewBox=\"0 0 %s %s\" font-family=\"Helvetica, Arial, sans-serif\" font-size=%q>\n",
		svgNamespace, svgNumber(width), svgNumber(height), svgNumber(width), svgNumber(height), svgNumber(svgFontSize))
	w.b.WriteString("<title>" + svgText(w.title()) + "</title>\n")
	if w.r.Stated != "" {
		w.b.WriteString("<desc>" + svgText(w.r.Stated) + "</desc>\n")
	}
	fmt.Fprintf(&w.b, "<rect width=%q height=%q fill=\"#ffffff\"/>\n", svgNumber(width), svgNumber(height))
}

func (w *svgWriter) title() string {
	title := string(w.r.Kind) + " of a run"
	if w.r.RunUntil > 0 || len(w.r.Lanes) > 0 || len(w.r.Edges) > 0 {
		title += " to t = " + runInstant(w.r.RunUntil)
	}
	if w.animated {
		title += " (animated)"
	}
	return title
}

// heading writes the title line and the notices under it, returning the y
// the drawing proper starts at.
func (w *svgWriter) heading(notices []string) float64 {
	y := svgMargin + svgFontSize
	fmt.Fprintf(&w.b, "<text x=%q y=%q font-weight=\"bold\">%s</text>\n", svgNumber(svgMargin), svgNumber(y), svgText(w.title()))
	for _, notice := range notices {
		y += svgLineHeight
		fmt.Fprintf(&w.b, "<text x=%q y=%q fill=\"#666666\">%s%s</text>\n", svgNumber(svgMargin), svgNumber(y), svgNoticePrefix, svgText(notice))
	}
	return y + svgLineHeight
}

// headingHeight is what heading adds for the notices, measured ahead of
// writing so the document head can state the drawing's size.
func headingHeight(notices []string) float64 {
	return svgMargin + svgFontSize + float64(len(notices))*svgLineHeight + svgLineHeight
}

func (w *svgWriter) empty() {
	notices := append([]string(nil), w.r.Notices...)
	reason := w.r.EmptyReason()
	width := svgMargin*2 + math.Max(svgTextWidth(w.title())+4*svgCharWidth, svgTextWidth(reason))
	for _, notice := range notices {
		width = math.Max(width, svgMargin*2+svgTextWidth(svgNoticePrefix+notice))
	}
	height := headingHeight(notices) + svgLineHeight + svgMargin
	w.open(width, height)
	y := w.heading(notices)
	fmt.Fprintf(&w.b, "<text x=%q y=%q fill=\"#666666\">%s</text>\n", svgNumber(svgMargin), svgNumber(y), svgText(reason))
	w.b.WriteString("</svg>\n")
}

// timeline draws one lane per machine with its state spans over a shared time axis;
// animated, a cursor sweeps the axis and the state under it brightens.
func (w *svgWriter) timeline() {
	r := w.r
	if len(r.Lanes) == 0 {
		w.empty()
		return
	}
	notices := append([]string(nil), r.Notices...)
	if notice := timelineZeroSpanNotice(r.Lanes); notice != "" {
		notices = append(notices, notice)
	}
	until := r.RunUntil
	labelWidth := 0.0
	for _, lane := range r.Lanes {
		labelWidth = math.Max(labelWidth, svgTextWidth(lane.Name))
	}
	chartLeft := svgMargin + labelWidth + 2*svgCharWidth
	chartWidth := svgChartMinWidth
	scale := 0.0
	if until > 0 {
		chartWidth = math.Min(math.Max(until*timelineScale(r.Lanes, until), svgChartMinWidth), svgChartMaxWidth)
		scale = chartWidth / until
	}
	chartRight := chartLeft + chartWidth
	width := chartRight + svgMargin + 2*svgCharWidth
	width = math.Max(width, svgMargin*2+svgTextWidth(w.title()))
	for _, notice := range notices {
		width = math.Max(width, svgMargin*2+svgTextWidth(svgNoticePrefix+notice))
	}
	lanesTop := headingHeight(notices) + svgLineHeight/2
	axisTop := lanesTop + float64(len(r.Lanes))*(svgLaneHeight+svgLaneGap)
	height := axisTop + svgAxisHeight + svgMargin
	w.open(width, height)
	w.heading(notices)
	w.timelineClock(until)
	x := func(t float64) float64 { return chartLeft + t*scale }

	w.timelineAxis(x, chartLeft, chartRight, axisTop, until)
	for i, lane := range r.Lanes {
		w.timelineLane(lane, x, lanesTop+float64(i)*(svgLaneHeight+svgLaneGap), chartLeft, chartWidth, until)
	}

	cursorX := x(until)
	fmt.Fprintf(&w.b, "<line id=\"cursor\" x1=%q y1=%q x2=%q y2=%q stroke=\"#c0392b\" stroke-width=\"1.5\">",
		svgNumber(cursorX), svgNumber(lanesTop-svgLaneGap/2), svgNumber(cursorX), svgNumber(axisTop))
	if w.animated && w.play > 0 {
		for _, attribute := range []string{"x1", "x2"} {
			fmt.Fprintf(&w.b, "<animate attributeName=%q values=\"%s;%s;%s\" keyTimes=\"0;%s;1\" dur=%q repeatCount=\"indefinite\"/>",
				attribute, svgNumber(chartLeft), svgNumber(chartRight), svgNumber(chartRight), svgFraction(w.fraction(until, until)), svgDuration(w.total))
		}
	}
	w.b.WriteString("</line>\n</svg>\n")
}

// timelineAxis draws the time axis with a tick at each instant, labelling the
// ticks that leave room for their label.
func (w *svgWriter) timelineAxis(x func(float64) float64, chartLeft, chartRight, axisTop, until float64) {
	fmt.Fprintf(&w.b, "<line x1=%q y1=%q x2=%q y2=%q stroke=\"#333333\"/>\n",
		svgNumber(chartLeft), svgNumber(axisTop), svgNumber(chartRight), svgNumber(axisTop))
	lastLabel := math.Inf(-1)
	instants := timelineInstants(w.r.Lanes, until)
	for i, t := range instants {
		tx := x(t)
		fmt.Fprintf(&w.b, "<line x1=%q y1=%q x2=%q y2=%q stroke=\"#333333\"/>\n",
			svgNumber(tx), svgNumber(axisTop), svgNumber(tx), svgNumber(axisTop+4))
		label := "t = " + runInstant(t)
		last := i == len(instants)-1
		if !last && (tx-lastLabel < svgTickLabelGap || x(until)-tx < svgTextWidth(label)) {
			continue
		}
		fmt.Fprintf(&w.b, "<text x=%q y=%q text-anchor=\"middle\" fill=\"#333333\">%s</text>\n",
			svgNumber(tx), svgNumber(axisTop+svgFontSize+6), svgText(label))
		lastLabel = tx
	}
}

// timelineLane draws one machine's lane at top: its name, its state spans and its marks.
func (w *svgWriter) timelineLane(lane Lane, x func(float64) float64, top, chartLeft, chartWidth, until float64) {
	fmt.Fprintf(&w.b, "<g id=%q>\n", lane.ID)
	fmt.Fprintf(&w.b, "<rect x=%q y=%q width=%q height=%q fill=\"#f4f6f8\"/>\n",
		svgNumber(chartLeft), svgNumber(top), svgNumber(chartWidth), svgNumber(svgLaneHeight))
	w.linked(lane.Origin, func() {
		fmt.Fprintf(&w.b, "<text x=%q y=%q>%s</text>", svgNumber(svgMargin), svgNumber(top+svgLaneHeight/2+svgFontSize/2-1), svgText(lane.Name))
	})
	w.b.WriteString("\n")
	for j, span := range lane.Spans {
		w.timelineSpan(fmt.Sprintf("%ss%d", lane.ID, j), span, x, top, until)
	}
	for _, mark := range lane.Marks {
		mx, my := x(mark.At), top+svgLaneHeight/2
		fmt.Fprintf(&w.b, "<g><title>%s</title>", svgText(mark.Kind+": "+mark.Text+"\nt = "+runInstant(mark.At)))
		fmt.Fprintf(&w.b, "<path d=\"M%s %sl5 5l-5 5l-5 -5z\" fill=\"#f1c40f\" stroke=\"#7d6608\"/>",
			svgNumber(mx), svgNumber(my-5))
		w.appearAt(mark.At, until, "opacity")
		w.b.WriteString(svgGroupEnd)
	}
	w.b.WriteString(svgGroupEnd)
}

// timelineSpan draws one state span in a lane: a bar between its instants (a
// line where it was held for no time), marking an entry made through other states.
func (w *svgWriter) timelineSpan(id string, span Span, x func(float64) float64, top, until float64) {
	label := timelineStateLabel(span)
	when := "t = " + runInstant(span.From)
	if span.To > span.From {
		when += " to " + runInstant(span.To)
	} else {
		when += ", held for no time"
	}
	if span.Open {
		when += " (open at the end of the run)"
	}
	if span.To <= span.From {
		tx := x(span.From)
		fmt.Fprintf(&w.b, "<g id=%q><title>%s</title>", id, svgText(label+"\n"+when))
		fmt.Fprintf(&w.b, "<line x1=%q y1=%q x2=%q y2=%q stroke=\"#c0392b\" stroke-width=\"2\"/>",
			svgNumber(tx), svgNumber(top), svgNumber(tx), svgNumber(top+svgLaneHeight))
		w.appearAt(span.From, until, "opacity")
		w.b.WriteString(svgGroupEnd)
		return
	}
	left, right := x(span.From), x(span.To)
	fmt.Fprintf(&w.b, "<g id=%q><title>%s</title>", id, svgText(label+"\n"+when))
	w.linked(span.Origin, func() {
		fmt.Fprintf(&w.b, "<rect x=%q y=%q width=%q height=%q rx=\"3\" fill=\"#8fb6e0\" stroke=\"#3b6ea5\">",
			svgNumber(left), svgNumber(top+3), svgNumber(right-left), svgNumber(svgLaneHeight-6))
		w.activeBetween(span.From, span.To, until)
		w.b.WriteString("</rect>")
		if text := svgFit(label, right-left-2*svgCharWidth); text != "" {
			fmt.Fprintf(&w.b, "<text x=%q y=%q text-anchor=\"middle\">%s</text>",
				svgNumber((left+right)/2), svgNumber(top+svgLaneHeight/2+svgFontSize/2-1), svgText(text))
		}
	})
	w.b.WriteString(svgGroupEnd)
	if len(span.Through) > 0 {
		fmt.Fprintf(&w.b, "<g><title>%s</title>", svgText("through "+strings.Join(span.Through, ", ")+"\nt = "+runInstant(span.From)))
		fmt.Fprintf(&w.b, "<line x1=%q y1=%q x2=%q y2=%q stroke=\"#c0392b\" stroke-width=\"2\"/>",
			svgNumber(left), svgNumber(top), svgNumber(left), svgNumber(top+svgLaneHeight))
		w.appearAt(span.From, until, "opacity")
		w.b.WriteString(svgGroupEnd)
	}
}

// timelineClock sets the replay's duration: one step per instant at which
// something changes, then one more holding the final state.
func (w *svgWriter) timelineClock(until float64) {
	if !w.animated || until <= 0 {
		return
	}
	steps := len(timelineInstants(w.r.Lanes, until)) - 1
	if steps < 1 {
		steps = 1
	}
	w.play = time.Duration(steps) * w.step
	w.total = w.play + w.step
}

// timelineInstants are the instants at which some lane changes, ascending,
// from 0 to the end of the run.
func timelineInstants(lanes []Lane, until float64) []float64 {
	seen := map[float64]bool{0: true, until: true}
	for _, lane := range lanes {
		for _, span := range lane.Spans {
			seen[span.From] = true
			if span.To <= until {
				seen[span.To] = true
			}
		}
		for _, mark := range lane.Marks {
			seen[mark.At] = true
		}
	}
	instants := make([]float64, 0, len(seen))
	for t := range seen {
		if t >= 0 && t <= until {
			instants = append(instants, t)
		}
	}
	sort.Float64s(instants)
	return instants
}

// fraction is where instant t of a run to until falls in the loop, the run
// replayed over play of the total.
func (w *svgWriter) fraction(t, until float64) float64 {
	if until <= 0 || w.total <= 0 {
		return 0
	}
	return math.Min(t/until, 1) * float64(w.play) / float64(w.total)
}

// activeBetween brightens the enclosing shape while the clock is between from
// and to, dimming it otherwise; a state open at the end stays bright.
func (w *svgWriter) activeBetween(from, to, until float64) {
	if !w.animated || w.total <= 0 {
		return
	}
	start, end := w.fraction(from, until), w.fraction(to, until)
	values, keyTimes := []string{}, []string{}
	if start > 0 {
		values, keyTimes = append(values, svgNumber(svgDimOpacity)), append(keyTimes, "0")
	}
	values, keyTimes = append(values, "1"), append(keyTimes, svgFraction(start))
	if to < until {
		values, keyTimes = append(values, svgNumber(svgDimOpacity)), append(keyTimes, svgFraction(end))
	}
	if len(values) < 2 {
		return
	}
	fmt.Fprintf(&w.b, "<animate attributeName=\"fill-opacity\" calcMode=\"discrete\" values=%q keyTimes=%q dur=%q repeatCount=\"indefinite\"/>",
		strings.Join(values, ";"), strings.Join(keyTimes, ";"), svgDuration(w.total))
}

// appearAt shows the enclosing shape from instant t of the run on.
func (w *svgWriter) appearAt(t, until float64, attribute string) {
	if !w.animated || w.total <= 0 {
		return
	}
	at := w.fraction(t, until)
	if at <= 0 {
		return
	}
	fmt.Fprintf(&w.b, "<animate attributeName=%q calcMode=\"discrete\" values=\"0;1\" keyTimes=\"0;%s\" dur=%q repeatCount=\"indefinite\"/>",
		attribute, svgFraction(at), svgDuration(w.total))
}

// sequence draws heads over lifelines and the messages between them as arrows
// down the page; animated, each arrow appears in turn with a marker travelling it.
func (w *svgWriter) sequence() {
	r := w.r
	if len(r.Roots) == 0 || len(r.Edges) == 0 {
		w.empty()
		return
	}
	notices := append([]string(nil), r.Notices...)
	column := svgColumnMinWidth
	index := map[string]int{}
	for i, node := range r.Roots {
		index[node.ID] = i
		column = math.Max(column, svgTextWidth(runParticipantLabel(node))+4*svgCharWidth)
	}
	for _, edge := range r.Edges {
		from, to := index[edge.From], index[edge.To]
		if from == to {
			column = math.Max(column, svgSelfLoopWidth+svgTextWidth(edge.Label)+3*svgCharWidth)
		} else {
			span := float64(abs(from - to))
			column = math.Max(column, (svgTextWidth(edge.Label)+2*svgCharWidth)/span)
		}
	}
	width := svgMargin*2 + column*float64(len(r.Roots))
	for _, edge := range r.Edges {
		if edge.From == edge.To {
			right := svgMargin + column*(float64(index[edge.From])+0.5) + svgSelfLoopWidth + svgCharWidth + svgTextWidth(edge.Label)
			width = math.Max(width, right+svgMargin)
		}
	}
	width = math.Max(width, svgMargin*2+svgTextWidth(w.title()))
	for _, notice := range notices {
		width = math.Max(width, svgMargin*2+svgTextWidth(svgNoticePrefix+notice))
	}
	headTop := headingHeight(notices) + svgLineHeight/2
	messagesTop := headTop + svgHeadHeight + svgMessageRow
	bottom := messagesTop + float64(len(r.Edges)-1)*svgMessageRow + svgMessageRow/2
	height := bottom + svgMargin
	w.open(width, height)
	w.heading(notices)
	if w.animated {
		w.play = time.Duration(len(r.Edges)) * w.step
		w.total = w.play + w.step
	}
	w.b.WriteString("<defs><marker id=\"arrow\" viewBox=\"0 0 10 10\" refX=\"9\" refY=\"5\" markerWidth=\"8\" markerHeight=\"8\" orient=\"auto-start-reverse\"><path d=\"M0 0L10 5L0 10z\" fill=\"#333333\"/></marker></defs>\n")
	x := func(i int) float64 { return svgMargin + column*(float64(i)+0.5) }
	for i, node := range r.Roots {
		cx := x(i)
		label := runParticipantLabel(node)
		fmt.Fprintf(&w.b, "<g id=%q>\n", node.ID)
		fmt.Fprintf(&w.b, "<line x1=%q y1=%q x2=%q y2=%q stroke=\"#999999\" stroke-dasharray=\"4 3\"/>\n",
			svgNumber(cx), svgNumber(headTop+svgHeadHeight), svgNumber(cx), svgNumber(bottom))
		w.linked(node.Origin, func() {
			fmt.Fprintf(&w.b, "<rect x=%q y=%q width=%q height=%q rx=\"3\" fill=\"#eef3f8\" stroke=\"#3b6ea5\"/>",
				svgNumber(cx-column/2+svgCharWidth), svgNumber(headTop), svgNumber(column-2*svgCharWidth), svgNumber(svgHeadHeight))
			fmt.Fprintf(&w.b, "<text x=%q y=%q text-anchor=\"middle\">%s</text>",
				svgNumber(cx), svgNumber(headTop+svgHeadHeight/2+svgFontSize/2-1), svgText(label))
		})
		w.b.WriteString("\n</g>\n")
	}
	for i, edge := range r.Edges {
		y := messagesTop + float64(i)*svgMessageRow
		from, to := x(index[edge.From]), x(index[edge.To])
		fmt.Fprintf(&w.b, "<g id=\"m%d\">", i)
		if edge.Label != "" {
			w.b.WriteString("<title>" + svgText(edge.Label) + "</title>")
		}
		var travel [][2]float64
		if edge.From == edge.To {
			travel = [][2]float64{{from, y - 6}, {from + svgSelfLoopWidth, y - 6}, {from + svgSelfLoopWidth, y + 6}, {from + 2, y + 6}}
			fmt.Fprintf(&w.b, "<path d=\"M%s %sh%sv%sh-%s\" fill=\"none\" stroke=\"#333333\" marker-end=\"url(#arrow)\"/>",
				svgNumber(from), svgNumber(y-6), svgNumber(svgSelfLoopWidth), svgNumber(12), svgNumber(svgSelfLoopWidth-2))
			fmt.Fprintf(&w.b, "<text x=%q y=%q>%s</text>", svgNumber(from+svgSelfLoopWidth+svgCharWidth), svgNumber(y+svgFontSize/2-1), svgText(edge.Label))
		} else {
			travel = [][2]float64{{from, y}, {to, y}}
			fmt.Fprintf(&w.b, "<line x1=%q y1=%q x2=%q y2=%q stroke=\"#333333\" marker-end=\"url(#arrow)\"/>",
				svgNumber(from), svgNumber(y), svgNumber(to), svgNumber(y))
			fmt.Fprintf(&w.b, "<text x=%q y=%q text-anchor=\"middle\">%s</text>", svgNumber((from+to)/2), svgNumber(y-5), svgText(edge.Label))
		}
		if w.animated {
			start, end := float64(i)/float64(len(r.Edges)+1), float64(i+1)/float64(len(r.Edges)+1)
			if start > 0 {
				fmt.Fprintf(&w.b, "<animate attributeName=\"opacity\" calcMode=\"discrete\" values=\"0;1\" keyTimes=\"0;%s\" dur=%q repeatCount=\"indefinite\"/>",
					svgFraction(start), svgDuration(w.total))
			}
			w.marker(travel, start, end)
		}
		w.b.WriteString(svgGroupEnd)
	}
	w.b.WriteString("</svg>\n")
}

// marker is a sequence message's travelling marker: hidden until start, then
// moving along the arrow's points to end, where it hides again.
func (w *svgWriter) marker(points [][2]float64, start, end float64) {
	last := points[len(points)-1]
	fmt.Fprintf(&w.b, "<circle cx=%q cy=%q r=\"4\" fill=\"#c0392b\" opacity=\"0\">", svgNumber(last[0]), svgNumber(last[1]))
	var xs, ys, times []string
	if start > 0 {
		xs, ys, times = append(xs, svgNumber(points[0][0])), append(ys, svgNumber(points[0][1])), append(times, "0")
	}
	for i, p := range points {
		at := start + (end-start)*float64(i)/float64(len(points)-1)
		xs, ys, times = append(xs, svgNumber(p[0])), append(ys, svgNumber(p[1])), append(times, svgFraction(at))
	}
	xs, ys, times = append(xs, svgNumber(last[0])), append(ys, svgNumber(last[1])), append(times, "1")
	fmt.Fprintf(&w.b, "<animate attributeName=\"cx\" values=%q keyTimes=%q dur=%q repeatCount=\"indefinite\"/>", strings.Join(xs, ";"), strings.Join(times, ";"), svgDuration(w.total))
	fmt.Fprintf(&w.b, "<animate attributeName=\"cy\" values=%q keyTimes=%q dur=%q repeatCount=\"indefinite\"/>", strings.Join(ys, ";"), strings.Join(times, ";"), svgDuration(w.total))
	if start > 0 {
		fmt.Fprintf(&w.b, "<animate attributeName=\"opacity\" calcMode=\"discrete\" values=\"0;1;0\" keyTimes=\"0;%s;%s\" dur=%q repeatCount=\"indefinite\"/>",
			svgFraction(start), svgFraction(end), svgDuration(w.total))
	} else {
		fmt.Fprintf(&w.b, "<animate attributeName=\"opacity\" calcMode=\"discrete\" values=\"1;0\" keyTimes=\"0;%s\" dur=%q repeatCount=\"indefinite\"/>",
			svgFraction(end), svgDuration(w.total))
	}
	w.b.WriteString("</circle>")
}

// linked writes what draw writes inside an <a> to the origin's source when
// the options link and the URL is one an SVG href carries verbatim.
func (w *svgWriter) linked(origin Origin, draw func()) {
	url, ok := w.options.Links.URL(origin)
	if !ok || !safeLinkURL(url) {
		draw()
		return
	}
	fmt.Fprintf(&w.b, "<a href=%q>", svgAttribute(url))
	draw()
	w.b.WriteString("</a>")
}

// svgText is text with XML's special characters escaped.
func svgText(text string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(text)); err != nil {
		return ""
	}
	return b.String()
}

// svgAttribute is an attribute value escaped; %q around it supplies the quotes.
func svgAttribute(text string) string {
	return strings.NewReplacer(`"`, "&quot;", "&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}

// svgFit is label shortened with an ellipsis to fit width, empty when not even
// one character does.
func svgFit(label string, width float64) string {
	runes := []rune(label)
	fit := int(width / svgCharWidth)
	if len(runes) <= fit {
		return label
	}
	if fit < 2 {
		return ""
	}
	return string(runes[:fit-1]) + "…"
}

func svgTextWidth(text string) float64 {
	return float64(len([]rune(text))) * svgCharWidth
}

func svgNumber(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

func svgFraction(v float64) string {
	return strconv.FormatFloat(math.Round(v*10000)/10000, 'f', -1, 64)
}

func svgDuration(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + "s"
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
