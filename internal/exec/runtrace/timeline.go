package runtrace

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

type laneKey struct {
	object   int64
	behavior *symbols.Symbol
}

type timelineLane struct {
	rendering *view.Lane
	key       laneKey
	regions   map[string]int
	states    map[string]activeState
	first     map[string]int
	nextOrder int
}

type activeState struct {
	record runtime.TraceRecord
	order  int
}

type timelineGroup struct {
	at          float64
	states      []runtime.TraceRecord
	transitions []runtime.TraceRecord
}

type transientState struct {
	path   string
	record runtime.TraceRecord
	order  int
}

func timeline(trace *runtime.TraceRecorder, options Options) *view.Rendering {
	out := runRendering(view.KindTimeline, options)
	if trace == nil {
		return out
	}
	records := trace.Records()
	lanes := make([]*timelineLane, 0)
	byKey := make(map[laneKey]*timelineLane)
	for _, record := range records {
		if record.Kind != runtime.TraceEntry && record.Kind != runtime.TraceExit {
			continue
		}
		key := keyOf(record.Origin)
		if lane := byKey[key]; lane != nil {
			lane.regionOrder(record.Region)
			if record.Kind == runtime.TraceEntry {
				path := statePath(record)
				if _, seen := lane.first[path]; !seen {
					lane.first[path] = lane.nextOrder
					lane.nextOrder++
				}
			}
			continue
		}
		lane := &view.Lane{
			ID:   fmt.Sprintf("l%d", len(lanes)),
			Name: laneName(record, options.Label),
		}
		stateLane := &timelineLane{
			rendering: lane, key: key,
			regions: make(map[string]int), states: make(map[string]activeState), first: make(map[string]int),
		}
		stateLane.regionOrder(record.Region)
		if record.Kind == runtime.TraceEntry {
			path := statePath(record)
			stateLane.first[path] = stateLane.nextOrder
			stateLane.nextOrder++
		}
		lanes = append(lanes, stateLane)
		byKey[key] = stateLane
	}
	groups := make(map[*timelineLane]map[float64]*timelineGroup, len(lanes))
	for _, lane := range lanes {
		groups[lane] = make(map[float64]*timelineGroup)
	}
	for _, record := range records {
		if record.Kind != runtime.TraceEntry && record.Kind != runtime.TraceExit {
			continue
		}
		lane := byKey[keyOf(record.Origin)]
		if lane != nil {
			group := groups[lane][record.Origin.At]
			if group == nil {
				group = &timelineGroup{at: record.Origin.At}
				groups[lane][record.Origin.At] = group
			}
			group.states = append(group.states, record)
		}
	}
	for _, record := range records {
		key := keyOf(record.Origin)
		lane := byKey[key]
		switch record.Kind {
		case runtime.TraceTransition:
			if lane == nil {
				out.Notices = append(out.Notices, fmt.Sprintf("transition at t = %s has no state lane: %s -> %s",
					runInstant(record.Origin.At), record.From, record.To))
				continue
			}
			transition := view.LaneTransition{At: record.Origin.At, From: record.From, To: record.To, Event: record.Event}
			lane.rendering.Transitions = append(lane.rendering.Transitions, transition)
			if group := groups[lane][record.Origin.At]; group != nil {
				group.transitions = append(group.transitions, record)
			}
		case runtime.TraceChoice, runtime.TraceGuard:
			if lane == nil {
				lane = laneForObject(lanes, key.object)
			}
			if lane == nil {
				out.Notices = append(out.Notices, fmt.Sprintf("t=%s %s", runInstant(record.Origin.At), record.Text()))
				continue
			}
			lane.rendering.Marks = append(lane.rendering.Marks, view.Mark{
				At: record.Origin.At, Kind: record.Kind.String(), Text: record.Text(),
			})
		}
	}
	for _, lane := range lanes {
		ordered := make([]*timelineGroup, 0, len(groups[lane]))
		for _, group := range groups[lane] {
			ordered = append(ordered, group)
		}
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].at < ordered[j].at })
		currentSpan := -1
		for _, group := range ordered {
			through, changed := lane.apply(group.states)
			if !changed {
				continue
			}
			if currentSpan >= 0 {
				lane.rendering.Spans[currentSpan].To = group.at
				lane.rendering.Spans[currentSpan].Open = false
			}
			leaves := lane.leaves()
			if len(leaves) == 0 {
				currentSpan = -1
				continue
			}
			stateNames := make([]string, len(leaves))
			for i, leaf := range leaves {
				stateNames[i] = leaf.record.State
			}
			span := view.Span{State: strings.Join(stateNames, " | "), From: group.at, To: group.at}
			for _, item := range through {
				span.Through = append(span.Through, item.record.State)
			}
			for _, transition := range group.transitions {
				if transition.Event != "" {
					span.Triggers = append(span.Triggers, transition.Event)
				}
			}
			lane.rendering.Spans = append(lane.rendering.Spans, span)
			currentSpan = len(lane.rendering.Spans) - 1
		}
		if currentSpan >= 0 {
			span := &lane.rendering.Spans[currentSpan]
			span.To = max(options.Until, span.From)
			span.Open = true
		}
		sort.SliceStable(lane.rendering.Transitions, func(i, j int) bool {
			return lane.rendering.Transitions[i].At < lane.rendering.Transitions[j].At
		})
		sort.SliceStable(lane.rendering.Marks, func(i, j int) bool {
			return lane.rendering.Marks[i].At < lane.rendering.Marks[j].At
		})
		out.Lanes = append(out.Lanes, *lane.rendering)
	}
	if count, upTo := trace.Dropped(); count > 0 {
		out.Notices = append(out.Notices, fmt.Sprintf("%d earlier records up to t = %s were dropped; a lane starts at its first state entry kept",
			count, runInstant(upTo)))
	}
	applyTimelineLimit(out, options.Limit)
	return out
}

func (lane *timelineLane) apply(records []runtime.TraceRecord) ([]transientState, bool) {
	entered := make(map[string]runtime.TraceRecord)
	var exitedLeaves []transientState
	changed := false
	for _, record := range records {
		path := statePath(record)
		lane.regionOrder(record.Region)
		switch record.Kind {
		case runtime.TraceEntry:
			order, seen := lane.first[path]
			if !seen {
				order = lane.nextOrder
				lane.nextOrder++
				lane.first[path] = order
			}
			if current, ok := lane.states[path]; ok {
				order = current.order
			}
			lane.states[path] = activeState{record: record, order: order}
			entered[path] = record
			changed = true
		case runtime.TraceExit:
			current, ok := lane.states[path]
			if !ok {
				continue
			}
			if !lane.hasDescendant(path) {
				exitedLeaves = append(exitedLeaves, transientState{path: path, record: current.record, order: current.order})
			}
			delete(lane.states, path)
			changed = true
		}
	}
	through := make([]transientState, 0)
	for _, exited := range exitedLeaves {
		if _, wasEntered := entered[exited.path]; !wasEntered {
			continue
		}
		if _, active := lane.states[exited.path]; active {
			continue
		}
		through = append(through, exited)
	}
	sort.SliceStable(through, func(i, j int) bool {
		left, right := lane.regionOrder(through[i].record.Region), lane.regionOrder(through[j].record.Region)
		if left != right {
			return left < right
		}
		return through[i].order < through[j].order
	})
	filtered := through[:0]
	for _, item := range through {
		parent := false
		for _, other := range through {
			if strings.HasPrefix(other.path, item.path+".") {
				parent = true
				break
			}
		}
		if !parent {
			filtered = append(filtered, item)
		}
	}
	return filtered, changed
}

func (lane *timelineLane) leaves() []activeState {
	var leaves []activeState
	for path, state := range lane.states {
		if !lane.hasDescendant(path) {
			leaves = append(leaves, state)
		}
	}
	sort.SliceStable(leaves, func(i, j int) bool {
		left, right := lane.regionOrder(leaves[i].record.Region), lane.regionOrder(leaves[j].record.Region)
		if left != right {
			return left < right
		}
		return leaves[i].order < leaves[j].order
	})
	return leaves
}

func (lane *timelineLane) hasDescendant(path string) bool {
	prefix := path + "."
	for active := range lane.states {
		if strings.HasPrefix(active, prefix) {
			return true
		}
	}
	return false
}

func (lane *timelineLane) regionOrder(region string) int {
	if order, ok := lane.regions[region]; ok {
		return order
	}
	order := len(lane.regions)
	lane.regions[region] = order
	return order
}

func keyOf(origin runtime.TraceOrigin) laneKey {
	key := laneKey{behavior: origin.Behavior}
	if origin.Object != nil {
		key.object = origin.Object.ID
	}
	return key
}

func laneName(record runtime.TraceRecord, label func(*runtime.Instance) string) string {
	machine := record.Machine()
	if machine == "" && record.Origin.Behavior != nil {
		machine = record.Origin.Behavior.Name
	}
	if record.Origin.Object != nil {
		name := instanceName(record.Origin.Object, label)
		if machine != "" {
			return name + "." + machine
		}
		return name
	}
	if machine != "" {
		return machine
	}
	return "environment"
}

func instanceName(instance *runtime.Instance, label func(*runtime.Instance) string) string {
	if label != nil {
		if name := label(instance); name != "" {
			return name
		}
	}
	return fmt.Sprintf("#%d", instance.ID)
}

func laneForObject(lanes []*timelineLane, object int64) *timelineLane {
	if object == 0 {
		return nil
	}
	for _, lane := range lanes {
		if lane.key.object == object {
			return lane
		}
	}
	return nil
}

func statePath(record runtime.TraceRecord) string {
	if record.Path != "" {
		return record.Path
	}
	return record.State
}

type spanRef struct {
	lane  int
	index int
	from  float64
}

func applyTimelineLimit(rendering *view.Rendering, requested int) {
	limit := effectiveLimit(requested)
	var spans []spanRef
	for laneIndex, lane := range rendering.Lanes {
		for spanIndex, span := range lane.Spans {
			spans = append(spans, spanRef{lane: laneIndex, index: spanIndex, from: span.From})
		}
	}
	if len(spans) <= limit {
		return
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].from < spans[j].from })
	cutoff := spans[limit].from
	dropped := 0
	for laneIndex := range rendering.Lanes {
		lane := &rendering.Lanes[laneIndex]
		kept := lane.Spans[:0]
		for _, span := range lane.Spans {
			if span.From >= cutoff {
				dropped++
				continue
			}
			if span.To > cutoff || span.Open && span.To >= cutoff {
				span.To = cutoff
				span.Open = false
			}
			kept = append(kept, span)
		}
		lane.Spans = kept
		transitions := lane.Transitions[:0]
		for _, transition := range lane.Transitions {
			if transition.At <= cutoff {
				transitions = append(transitions, transition)
			}
		}
		lane.Transitions = transitions
		marks := lane.Marks[:0]
		for _, mark := range lane.Marks {
			if mark.At <= cutoff {
				marks = append(marks, mark)
			}
		}
		lane.Marks = marks
	}
	verb := "are"
	if dropped == 1 {
		verb = "is"
	}
	rendering.Notices = append(rendering.Notices, fmt.Sprintf("%d later state %s after t = %s %s not drawn (at most %d spans are)",
		dropped, plural(dropped, "change", "changes"), runInstant(cutoff), verb, limit))
}

func effectiveLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	return limit
}

func runInstant(instant float64) string {
	return strconv.FormatFloat(instant, 'g', -1, 64)
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
