package view

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func writeRunTimelineText(b *strings.Builder, r *Rendering) {
	if len(r.Lanes) == 0 {
		b.WriteString(r.EmptyReason() + "\n")
		return
	}
	for i, lane := range r.Lanes {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(lane.Name + "\n")
		width := 0
		times := make([]string, len(lane.Spans))
		for i, span := range lane.Spans {
			times[i] = runInstant(span.From) + " .. " + runInstant(span.To)
			width = max(width, utf8.RuneCountInString(times[i]))
		}
		for i, span := range lane.Spans {
			fmt.Fprintf(b, "  %s%s  %s", times[i], strings.Repeat(" ", width-utf8.RuneCountInString(times[i])), span.State)
			if len(span.Triggers) > 0 {
				fmt.Fprintf(b, " (%s)", strings.Join(span.Triggers, ", "))
			}
			if len(span.Through) > 0 {
				fmt.Fprintf(b, "  via %s", strings.Join(span.Through, ", "))
			}
			if span.Open {
				b.WriteString("  (held at the end)")
			}
			b.WriteString("\n")
		}
		if len(lane.Transitions) > 0 {
			b.WriteString("  transitions:\n")
			for _, transition := range lane.Transitions {
				fmt.Fprintf(b, "    t=%s %s -> %s", runInstant(transition.At), transition.From, transition.To)
				if transition.Event != "" {
					fmt.Fprintf(b, " (%s)", transition.Event)
				}
				b.WriteString("\n")
			}
		}
		if len(lane.Marks) > 0 {
			b.WriteString("  noted:\n")
			for _, mark := range lane.Marks {
				fmt.Fprintf(b, "    t=%s %s\n", runInstant(mark.At), mark.Text)
			}
		}
	}
}

func (r *Rendering) runTimelineMermaid(options Options) string {
	var b strings.Builder
	r.writeMermaidFrontmatter(&b, labelsOf(nil, false, nil), options, portView{})
	fmt.Fprintf(&b, "%%%% run — %s rendering", r.Kind)
	if r.Stated != "" {
		fmt.Fprintf(&b, " (%s)", r.Stated)
	}
	b.WriteString("\n")
	notices := append([]string(nil), r.Notices...)
	notices = append(notices, r.mermaidTimelineNotices()...)
	for _, notice := range notices {
		fmt.Fprintf(&b, "%%%% not represented: %s\n", notice)
	}
	if len(r.Lanes) == 0 {
		fmt.Fprintf(&b, "%%%% %s\n", r.EmptyReason())
	} else {
		for _, lane := range r.Lanes {
			for _, mark := range lane.Marks {
				text := strings.NewReplacer("\r", " ", "\n", " ").Replace(mark.Text)
				fmt.Fprintf(&b, "%%%% t=%s %s\n", runInstant(mark.At), text)
			}
		}
	}
	b.WriteString("gantt\n")
	b.WriteString("    dateFormat x\n")
	b.WriteString("    axisFormat %M:%S.%L\n")
	b.WriteString("    todayMarker off\n")
	for _, lane := range r.Lanes {
		fmt.Fprintf(&b, "    section %s\n", ganttText(lane.Name))
		for i, span := range lane.Spans {
			if span.To <= span.From {
				continue
			}
			label := timelineStateLabel(span)
			if span.Open {
				fmt.Fprintf(&b, "        %s :active, %ss%d, %d, %d\n",
					ganttText(label), lane.ID, i, runMilliseconds(span.From), runMilliseconds(span.To))
			} else {
				fmt.Fprintf(&b, "        %s :%ss%d, %d, %d\n",
					ganttText(label), lane.ID, i, runMilliseconds(span.From), runMilliseconds(span.To))
			}
		}
	}
	return b.String()
}

func (r *Rendering) mermaidTimelineNotices() []string {
	var notices []string
	zero := 0
	fractional := !wholeMillisecond(r.RunUntil)
	for _, lane := range r.Lanes {
		for _, span := range lane.Spans {
			if span.To <= span.From {
				zero++
			}
			fractional = fractional || !wholeMillisecond(span.From) || !wholeMillisecond(span.To)
		}
		for _, transition := range lane.Transitions {
			fractional = fractional || !wholeMillisecond(transition.At)
		}
		for _, mark := range lane.Marks {
			fractional = fractional || !wholeMillisecond(mark.At)
		}
	}
	if zero > 0 {
		verb := "are"
		if zero == 1 {
			verb = "is"
		}
		notices = append(notices, fmt.Sprintf("%d %s held for no time %s listed in the text form",
			zero, plural(zero, "state", "states"), verb))
	}
	if fractional {
		notices = append(notices, "instants are rounded to the nearest millisecond")
	}
	if r.RunUntil >= 3600 {
		notices = append(notices, "the axis reads minutes and seconds; it wraps past an hour")
	}
	for _, lane := range r.Lanes {
		if len(lane.Marks) > 0 {
			notices = append(notices, "Mermaid gantt draws no note; choice and guard records are comments")
			break
		}
	}
	return notices
}

func (r *Rendering) runTimelinePlantUML() string {
	var b strings.Builder
	b.WriteString("@startuml\n")
	fmt.Fprintf(&b, "' run — %s rendering", r.Kind)
	if r.Stated != "" {
		fmt.Fprintf(&b, " (%s)", r.Stated)
	}
	b.WriteString("\n")
	for _, notice := range r.Notices {
		fmt.Fprintf(&b, "' not represented: %s\n", strings.ReplaceAll(notice, "\n", " "))
	}
	if len(r.Lanes) == 0 {
		fmt.Fprintf(&b, "' %s\n@enduml\n", r.EmptyReason())
		return b.String()
	}
	fmt.Fprintf(&b, "scale 1 as %s pixels\n", strconv.FormatFloat(timelineScale(r.Lanes, r.RunUntil), 'f', -1, 64))
	for _, lane := range r.Lanes {
		fmt.Fprintf(&b, "concise %s as %s\n", plantumlQuote(plantumlText(lane.Name)), lane.ID)
	}
	var changes []timelineChange
	until := r.RunUntil
	for _, lane := range r.Lanes {
		for i, span := range lane.Spans {
			changes = append(changes, timelineChange{span.From, fmt.Sprintf("%s is %s", lane.ID,
				plantumlQuote(plantumlText(timelineStateLabel(span))))})
			if !span.Open && i == len(lane.Spans)-1 && span.To < until {
				changes = append(changes, timelineChange{span.To, lane.ID + " is {hidden}"})
			}
		}
		for _, mark := range lane.Marks {
			changes = append(changes, timelineChange{mark.At, fmt.Sprintf("note top of %s : %s", lane.ID, plantumlText(mark.Text))})
		}
	}
	for _, lane := range r.Lanes {
		changes = append(changes, timelineChange{at: until, line: lane.ID + " is {hidden}"})
	}
	changes = append(changes, timelineChange{at: until})
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].at < changes[j].at })
	last := math.NaN()
	for _, item := range changes {
		if math.IsNaN(last) || item.at != last {
			fmt.Fprintf(&b, "@%s\n", runInstant(item.at))
			last = item.at
		}
		if item.line != "" {
			b.WriteString(item.line + "\n")
		}
	}
	b.WriteString("@enduml\n")
	return b.String()
}

type timelineChange struct {
	at   float64
	line string
}

func timelineScale(lanes []Lane, until float64) float64 {
	const (
		minimumSpanWidth = 140.0
		characterWidth   = 7.0
		labelPadding     = 24.0
		maximumWidth     = 4000.0
	)
	duration := math.Max(0, until)
	scale := 1.0
	for _, lane := range lanes {
		for _, span := range lane.Spans {
			spanDuration := span.To - span.From
			if spanDuration > 0 {
				duration = math.Max(duration, span.To)
				labelWidth := float64(utf8.RuneCountInString(timelineStateLabel(span)))*characterWidth + labelPadding
				spanWidth := math.Max(minimumSpanWidth, labelWidth)
				scale = math.Max(scale, math.Ceil(spanWidth/spanDuration))
			}
		}
	}
	if duration > 0 && scale*duration > maximumWidth {
		scale = maximumWidth / duration
	}
	return scale
}

func runTimelineMermaidLeftPadding(lanes []Lane) int {
	const (
		defaultPadding = 75
		characterWidth = 7
		labelPadding   = 24
	)
	padding := defaultPadding
	for _, lane := range lanes {
		padding = max(padding, utf8.RuneCountInString(lane.Name)*characterWidth+labelPadding)
	}
	return padding
}

func timelineStateLabel(span Span) string {
	label := span.State
	if len(span.Triggers) > 0 {
		label += " (" + strings.Join(span.Triggers, ", ") + ")"
	}
	return label
}

func runInstant(instant float64) string {
	return strconv.FormatFloat(instant, 'g', -1, 64)
}

func runMilliseconds(instant float64) int64 {
	return int64(math.Round(instant * 1000))
}

func wholeMillisecond(instant float64) bool {
	scaled := instant * 1000
	return math.Abs(scaled-math.Round(scaled)) < 1e-7
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func ganttText(text string) string {
	return strings.NewReplacer("\r", " ", "\n", " ", ":", "#58;", "#", "#35;", ";", "#59;").Replace(text)
}
