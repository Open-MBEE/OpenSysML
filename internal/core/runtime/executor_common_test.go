package runtime

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

func TestToken_Creation(t *testing.T) {
	token := Token{
		ID:       1,
		Location: nil, // Will use actual AST node in real usage
		Data:     make(map[string]Value),
	}
	
	if token.ID != 1 {
		t.Errorf("expected ID 1, got %d", token.ID)
	}
	
	if token.Data == nil {
		t.Error("expected non-nil Data map")
	}
}

func TestToken_DataStorage(t *testing.T) {
	token := Token{
		ID:   1,
		Data: make(map[string]Value),
	}
	
	token.Data["x"] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 42}}
	
	val, exists := token.Data["x"]
	if !exists {
		t.Error("expected data key 'x' to exist")
	}
	if val.Const.Int != 42 {
		t.Errorf("expected value 42, got %d", val.Const.Int)
	}
}

func TestEventQueue_PushPop(t *testing.T) {
	queue := NewEventQueue()
	
	e1 := Event{ID: 1, Timestamp: 5.0}
	e2 := Event{ID: 2, Timestamp: 2.0}
	e3 := Event{ID: 3, Timestamp: 8.0}
	
	queue.Push(e1)
	queue.Push(e2)
	queue.Push(e3)
	
	if queue.Len() != 3 {
		t.Errorf("expected length 3, got %d", queue.Len())
	}
	
	// Should pop in timestamp order (min-heap)
	first := queue.Pop()
	if first.ID != 2 || first.Timestamp != 2.0 {
		t.Errorf("expected event 2 at 2.0s, got %d at %fs", first.ID, first.Timestamp)
	}
	
	second := queue.Pop()
	if second.ID != 1 || second.Timestamp != 5.0 {
		t.Errorf("expected event 1 at 5.0s, got %d at %fs", second.ID, second.Timestamp)
	}
	
	third := queue.Pop()
	if third.ID != 3 || third.Timestamp != 8.0 {
		t.Errorf("expected event 3 at 8.0s, got %d at %fs", third.ID, third.Timestamp)
	}
	
	if queue.Len() != 0 {
		t.Errorf("expected empty queue, got length %d", queue.Len())
	}
}

func TestEventQueue_Peek(t *testing.T) {
	queue := NewEventQueue()
	
	e1 := Event{ID: 1, Timestamp: 5.0}
	e2 := Event{ID: 2, Timestamp: 2.0}
	
	queue.Push(e1)
	queue.Push(e2)
	
	peeked := queue.Peek()
	if peeked.ID != 2 {
		t.Errorf("expected peek to return event 2, got %d", peeked.ID)
	}
	
	if queue.Len() != 2 {
		t.Error("peek should not remove event")
	}
}

func TestEventQueue_PopEmpty(t *testing.T) {
	queue := NewEventQueue()
	
	// Pop from empty queue should return zero Event (not panic)
	event := queue.Pop()
	if event.ID != 0 {
		t.Errorf("expected zero Event from empty queue, got %+v", event)
	}
	
	if event.Timestamp != 0.0 {
		t.Errorf("expected zero timestamp, got %f", event.Timestamp)
	}
}
