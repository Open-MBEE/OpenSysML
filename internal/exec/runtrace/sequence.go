package runtrace

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

type sendRef struct {
	index  int
	record runtime.TraceRecord
	target int64
	used   bool
}

type sendQueue struct {
	items  []*sendRef
	cursor int
}

type runMessage struct {
	at    float64
	index int
	from  participant
	to    participant
	label string
}

type participant struct {
	object   *runtime.Instance
	behavior *symbols.Symbol
}

type participantKey struct {
	object   int64
	behavior *symbols.Symbol
	ambient  bool
}

type participantSet struct {
	ids     map[participantKey]string
	roots   []*view.Node
	labelFn func(*runtime.Instance) string
}

func sequence(trace *runtime.TraceRecorder, options Options) *view.Rendering {
	out := runRendering(view.KindSequence, options)
	if trace == nil {
		return out
	}
	records := trace.Records()
	queues := make(map[string]map[int64]*sendQueue)
	var sends []*sendRef
	var messages []runMessage
	for index, record := range records {
		switch record.Kind {
		case runtime.TraceSend:
			target := int64(0)
			if record.Target != nil {
				target = record.Target.ID
			}
			ref := &sendRef{index: index, record: record, target: target}
			sends = append(sends, ref)
			if queues[record.Event] == nil {
				queues[record.Event] = make(map[int64]*sendQueue)
			}
			queue := queues[record.Event][target]
			if queue == nil {
				queue = &sendQueue{}
				queues[record.Event][target] = queue
			}
			queue.items = append(queue.items, ref)
		case runtime.TraceAccept:
			var paired *sendRef
			objectID := int64(0)
			if record.Origin.Object != nil {
				objectID = record.Origin.Object.ID
			}
			if byTarget := queues[record.Event]; byTarget != nil {
				broadcast := byTarget[0]
				targeted := byTarget[objectID]
				paired = earliestSend(broadcast, targeted)
				if paired != nil {
					paired.used = true
					consumeSend(paired, broadcast, targeted)
				}
			}
			if paired == nil {
				messages = append(messages, runMessage{
					at: record.Origin.At, index: index,
					from: participant{}, to: participant{object: record.Origin.Object, behavior: record.Origin.Behavior},
					label: messageLabel(record.Origin.At, 0, record.Event, record.PayloadTexts(), false, false),
				})
				continue
			}
			payload := paired.record.PayloadTexts()
			messages = append(messages, runMessage{
				at: paired.record.Origin.At, index: paired.index,
				from:  participant{object: paired.record.Origin.Object, behavior: paired.record.Origin.Behavior},
				to:    participant{object: record.Origin.Object, behavior: record.Origin.Behavior},
				label: messageLabel(paired.record.Origin.At, record.Origin.At, paired.record.Event, payload, true, false),
			})
		}
	}
	for _, send := range sends {
		if send.used {
			continue
		}
		to := participant{}
		if send.record.Target != nil {
			to.object = send.record.Target
		}
		messages = append(messages, runMessage{
			at: send.record.Origin.At, index: send.index,
			from:  participant{object: send.record.Origin.Object, behavior: send.record.Origin.Behavior},
			to:    to,
			label: messageLabel(send.record.Origin.At, 0, send.record.Event, send.record.PayloadTexts(), false, true),
		})
	}
	sort.SliceStable(messages, func(i, j int) bool {
		if messages[i].at != messages[j].at {
			return messages[i].at < messages[j].at
		}
		return messages[i].index < messages[j].index
	})
	limit := effectiveLimit(options.Limit)
	if len(messages) > limit {
		later := messages[limit:]
		verb := "are"
		if len(later) == 1 {
			verb = "is"
		}
		first, last := runInstant(later[0].at), runInstant(later[len(later)-1].at)
		if first == last {
			out.Notices = append(out.Notices, fmt.Sprintf("%d later %s, at t = %s, %s not drawn (at most %d are)",
				len(later), plural(len(later), "message", "messages"), first, verb, limit))
		} else {
			out.Notices = append(out.Notices, fmt.Sprintf("%d later %s, t = %s to %s, %s not drawn (at most %d are)",
				len(later), plural(len(later), "message", "messages"), first, last, verb, limit))
		}
		messages = messages[:limit]
	}
	if count, upTo := trace.Dropped(); count > 0 {
		out.Notices = append(out.Notices, fmt.Sprintf("%d earlier records up to t = %s were dropped; senders or acceptors of earlier messages may be missing",
			count, runInstant(upTo)))
	}
	participants := participantSet{ids: make(map[participantKey]string), labelFn: options.Label}
	for _, message := range messages {
		from := participants.add(message.from)
		to := participants.add(message.to)
		out.Edges = append(out.Edges, view.Edge{From: from, To: to, Kind: view.EdgeFlow, Label: message.label})
	}
	out.Roots = participants.roots
	return out
}

func earliestSend(first, second *sendQueue) *sendRef {
	var a, b *sendRef
	if first != nil && first.cursor < len(first.items) {
		a = first.items[first.cursor]
	}
	if second != nil && second.cursor < len(second.items) {
		b = second.items[second.cursor]
	}
	if a == nil {
		return b
	}
	if b == nil || a.index < b.index {
		return a
	}
	return b
}

func consumeSend(send *sendRef, first, second *sendQueue) {
	if first != nil && first.cursor < len(first.items) && first.items[first.cursor] == send {
		first.cursor++
		return
	}
	if second != nil && second.cursor < len(second.items) && second.items[second.cursor] == send {
		second.cursor++
	}
}

func messageLabel(at, acceptedAt float64, event string, payload []string, accepted, unmatched bool) string {
	time := runInstant(at)
	if accepted && acceptedAt > at {
		time += ".." + runInstant(acceptedAt)
	}
	label := "t=" + time + " " + event
	if len(payload) > 0 {
		label += " (" + strings.Join(payload, ", ") + ")"
	}
	if unmatched {
		label += " (not accepted)"
	}
	return label
}

func (set *participantSet) add(participant participant) string {
	key := participantKey{}
	node := &view.Node{}
	switch {
	case participant.object != nil:
		key.object = participant.object.ID
		node.Kind = "object"
		node.Name = instanceName(participant.object, set.labelFn)
		if participant.object.Type != nil {
			node.Type = participant.object.Type.Name
		}
	case participant.behavior != nil:
		key.behavior = participant.behavior
		node.Kind = "behavior"
		node.Name = participant.behavior.Name
	default:
		key.ambient = true
		node.Kind = "environment"
		node.Name = "environment"
	}
	if id, ok := set.ids[key]; ok {
		return id
	}
	node.ID = fmt.Sprintf("n%d", len(set.roots))
	set.ids[key] = node.ID
	set.roots = append(set.roots, node)
	return node.ID
}
