package runtime

import (
	"container/heap"
	"fmt"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// Token represents a control/data token in action execution.
type Token struct {
	ID       int64            // Unique token ID
	Location ast.Node         // Current node position
	Data     map[string]Value // Token data (parameters, flow values)
}

// ExecutionState tracks executor state.
type ExecutionState int

const (
	StateReady      ExecutionState = iota // Not started
	StateRunning                          // In progress
	StateCompleted                        // Reached terminal state
	StateSuspended                        // Paused for debugging
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
	return fmt.Sprintf("Event{ID:%d, Type:%s, Time:%.2f}", e.ID, e.Type, e.Timestamp)
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

// eventHeap implements heap.Interface for Event.
type eventHeap []Event

func (h eventHeap) Len() int { return len(h) }

func (h eventHeap) Less(i, j int) bool {
	return h[i].Timestamp < h[j].Timestamp
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
