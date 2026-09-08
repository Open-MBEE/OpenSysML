package runtime

import (
	"container/heap"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// Token represents a control token in action execution. It carries no values of
// its own: the action's features are one space every token shares (see ActionExecutor.Data).
type Token struct {
	ID       int64    // Unique token ID
	Location ast.Node // Current node position

	// Via is the succession the token travelled to reach Location; the zero edge for a
	// token no succession delivered (a flow's first, or the one a synchronized node performs with).
	Via lower.ActionEdge

	// moved is the sweep of its flow that last moved the token along a succession
	// (ActionExecutor.sweep): until that sweep ends, it has not arrived.
	moved uint64

	// Wait records that this token is parked at an accept node: the accept
	// found no message it could consume, so the action is suspended there
	// until one arrives. It is nil for every token that is free to advance.
	Wait *AcceptWait

	// frame is the activation of a nested action node's own flow this token is
	// running in, nil for a token in the action's own flow (action_subflow.go).
	frame *actionFrame

	// body is the work of this token's step a breakpoint paused, resumed by the
	// next step (action_body_run.go); nil for a token with none pending.
	body *bodyRun
}

// travel moves the token along a succession in the given sweep, recording the one it arrived over.
func (t *Token) travel(edge lower.ActionEdge, sweep uint64) {
	t.Location = edge.Target
	t.Via = edge
	t.moved = sweep
}

// AcceptWait describes the message a parked token is waiting for. It is the
// accept's lowered shape plus the step the token parked at, which is what lets
// a blocked run report which accept is waiting for what rather than only that
// it is stuck.
type AcceptWait struct {
	ParamName  string // the accept parameter the message will bind to
	SignalType string // the type awaited, empty when the accept named none
	ViaPort    string // the port awaited on, empty when the accept named none
	Since      int    // the step during which the token parked, numbered as the trace numbers steps
	// Trigger describes the time or change event awaited instead of a message
	// (`accept when x > 1`), empty when the accept waits for a message.
	Trigger string
}

// String describes what a parked token is waiting for, and since when, for
// error messages and for the REPL's view of a suspended executor.
func (w AcceptWait) String() string {
	if w.Trigger != "" {
		return fmt.Sprintf("%s waiting since step %d for its event", w.Trigger, w.Since)
	}
	return fmt.Sprintf("accept %s waiting since step %d for a message of type %s%s",
		w.ParamName, w.Since, orAny(w.SignalType), viaSuffix(w.ViaPort))
}

// ExecutionState tracks executor state.
type ExecutionState int

const (
	StateReady     ExecutionState = iota // Not started
	StateRunning                         // In progress
	StateCompleted                       // Reached terminal state
	StateSuspended                       // Paused for debugging
	StateWaiting                         // Every remaining token is parked at an accept
)

func (s ExecutionState) String() string {
	switch s {
	case StateReady:
		return "Ready"
	case StateRunning:
		return "Running"
	case StateCompleted:
		return "Completed"
	case StateSuspended:
		return "Suspended"
	case StateWaiting:
		return "Waiting"
	default:
		return "Unknown"
	}
}

// EventType identifies event kinds for state machines.
type EventType int

const (
	EventTime   EventType = iota // TimeEvent - fires after duration
	EventChange                  // ChangeEvent - fires when condition true
	EventAccept                  // AcceptEvent - fires when signal received
	EventCall                    // CallEvent - fires when operation invoked
)

func (t EventType) String() string {
	switch t {
	case EventTime:
		return "TimeEvent"
	case EventChange:
		return "ChangeEvent"
	case EventAccept:
		return "AcceptEvent"
	case EventCall:
		return "CallEvent"
	default:
		return "Unknown"
	}
}

// Event represents a state machine event.
type Event struct {
	ID        int64       // Unique event ID
	Type      EventType   // Event type
	Timestamp float64     // Virtual time when event fires
	Payload   interface{} // Event-specific data
}

func (e Event) String() string {
	return fmt.Sprintf("Event{ID:%d, Type:%s, Time:%s}", e.ID, e.Type, semantics.FormatReal(e.Timestamp))
}

// EventQueue is a priority queue of events sorted by timestamp (min-heap).
type EventQueue struct {
	events eventHeap
}

// NewEventQueue creates an empty event queue.
func NewEventQueue() *EventQueue {
	q := &EventQueue{events: make(eventHeap, 0)}
	heap.Init(&q.events)
	return q
}

// Push adds an event to the queue.
func (q *EventQueue) Push(e Event) {
	heap.Push(&q.events, e)
}

// Pop removes and returns the earliest event.
func (q *EventQueue) Pop() Event {
	if len(q.events) == 0 {
		return Event{} // Return zero Event if empty
	}
	return heap.Pop(&q.events).(Event)
}

// Peek returns the earliest event without removing it.
func (q *EventQueue) Peek() Event {
	if len(q.events) == 0 {
		return Event{}
	}
	return q.events[0]
}

// Len returns the number of pending events.
func (q *EventQueue) Len() int {
	return len(q.events)
}

// Withdraw drops every pending event the predicate accepts, which cancels an
// occurrence a state no longer waits for.
func (q *EventQueue) Withdraw(drop func(Event) bool) {
	kept := q.events[:0]
	for _, event := range q.events {
		if !drop(event) {
			kept = append(kept, event)
		}
	}
	q.events = kept
	heap.Init(&q.events)
}

// eventHeap implements heap.Interface for Event.
type eventHeap []Event

func (h eventHeap) Len() int { return len(h) }

func (h eventHeap) Less(i, j int) bool {
	if h[i].Timestamp != h[j].Timestamp {
		return h[i].Timestamp < h[j].Timestamp
	}
	// A completion event is dispatched before the pool events queued for the same
	// instant: a state that has completed leaves before it reacts to a signal.
	if isCompletionEvent(h[i]) != isCompletionEvent(h[j]) {
		return isCompletionEvent(h[i])
	}
	// Otherwise events are dispatched in arrival order: IDs are handed out in that
	// order and a deferred event keeps its ID when it is recalled, so it still
	// comes before whatever arrived after it.
	return h[i].ID < h[j].ID
}

// isCompletionEvent reports whether an event carries a completion transition,
// the transition a state takes once it is done rather than on a trigger.
func isCompletionEvent(event Event) bool {
	if event.Type != EventTime {
		return false
	}
	switch payload := event.Payload.(type) {
	case *lower.Transition:
		return payload.Trigger == nil
	case *ast.TransitionEdge:
		return payload.Trigger == nil
	default:
		return false
	}
}

func (h eventHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *eventHeap) Push(x interface{}) {
	*h = append(*h, x.(Event))
}

func (h *eventHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
