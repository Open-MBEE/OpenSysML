package ast

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestActionNodeConstruction(t *testing.T) {
	span := source.Span{Offset: 0, Len: 10}

	nodes := []Node{
		&InitialNode{NodeBase: NodeBase{NodeSpan: span}, First: &QualifiedName{Parts: []NameSegment{{Text: "start"}}}},
		&FinalNode{NodeBase: NodeBase{NodeSpan: span}},
		&ForkNode{NodeBase: NodeBase{NodeSpan: span}, Name: "split"},
		&JoinNode{NodeBase: NodeBase{NodeSpan: span}, Name: "sync"},
		&MergeNode{NodeBase: NodeBase{NodeSpan: span}, Name: "merge"},
		&DecisionNode{NodeBase: NodeBase{NodeSpan: span}, Name: "decide"},
		&ActionExecutionNode{NodeBase: NodeBase{NodeSpan: span}, Name: "exec"},
	}

	for i, node := range nodes {
		if node.Span().Offset != 0 || node.Span().Len != 10 {
			t.Errorf("node %d: span mismatch", i)
		}
	}
}

func TestStateNodeConstruction(t *testing.T) {
	span := source.Span{Offset: 0, Len: 20}

	state := &StateNode{
		NodeBase: NodeBase{NodeSpan: span},
		Name:     "Active",
		Entry:    []Node{},
		Do:       []Node{},
		Exit:     []Node{},
	}

	if state.Name != "Active" {
		t.Errorf("expected name 'Active', got %q", state.Name)
	}
	if state.Span().Len != 20 {
		t.Error("span mismatch")
	}
}

func TestTriggerEventInterface(t *testing.T) {
	// Verify all 4 trigger types implement TriggerEvent interface
	var _ TriggerEvent = (*TimeEvent)(nil)
	var _ TriggerEvent = (*ChangeEvent)(nil)
	var _ TriggerEvent = (*AcceptEvent)(nil)
	var _ TriggerEvent = (*CallEvent)(nil)

	// Verify they also implement Node (via NodeBase)
	var _ Node = (*TimeEvent)(nil)
	var _ Node = (*ChangeEvent)(nil)
	var _ Node = (*AcceptEvent)(nil)
	var _ Node = (*CallEvent)(nil)
}

func TestEdgeConstruction(t *testing.T) {
	span := source.Span{Offset: 0, Len: 15}
	src := &QualifiedName{Parts: []NameSegment{{Text: "a"}}}
	target := &QualifiedName{Parts: []NameSegment{{Text: "b"}}}

	succEdge := &SuccessionEdge{
		NodeBase: NodeBase{NodeSpan: span},
		Source:   src,
		Target:   target,
	}

	if succEdge.Source.Parts[0].Text != "a" {
		t.Error("source mismatch")
	}
	if succEdge.Target.Parts[0].Text != "b" {
		t.Error("target mismatch")
	}
}

func TestStateNode_AllFields(t *testing.T) {
	span := source.Span{Offset: 0, Len: 25}

	substate := &StateNode{
		NodeBase: NodeBase{NodeSpan: span},
		Name:     "Substate1",
	}

	region := &StateRegion{
		NodeBase: NodeBase{NodeSpan: span},
		Name:     "Region1",
		States:   []Node{},
	}

	state := &StateNode{
		NodeBase:  NodeBase{NodeSpan: span},
		Name:      "CompositeFinal",
		Entry:     []Node{},
		Do:        []Node{},
		Exit:      []Node{},
		Substates: []Node{substate},
		Regions:   []*StateRegion{region},
	}

	if state.Name != "CompositeFinal" {
		t.Errorf("expected name 'CompositeFinal', got %q", state.Name)
	}
	if len(state.Substates) != 1 {
		t.Errorf("expected 1 substate, got %d", len(state.Substates))
	}
	if len(state.Regions) != 1 {
		t.Errorf("expected 1 region, got %d", len(state.Regions))
	}
	if state.Regions[0].Name != "Region1" {
		t.Errorf("expected region name 'Region1', got %q", state.Regions[0].Name)
	}
}

func TestActionExecutionNode_Fields(t *testing.T) {
	span := source.Span{Offset: 0, Len: 10}

	// Test ActionRef variant
	actionRef := &QualifiedName{Parts: []NameSegment{{Text: "nestedAction"}}}
	nodeWithRef := &ActionExecutionNode{
		NodeBase:  NodeBase{NodeSpan: span},
		Name:      "exec1",
		ActionRef: actionRef,
	}

	if nodeWithRef.ActionRef == nil {
		t.Error("expected ActionRef to be set")
	}
	if nodeWithRef.ActionRef.Parts[0].Text != "nestedAction" {
		t.Errorf("expected ActionRef 'nestedAction', got %q", nodeWithRef.ActionRef.Parts[0].Text)
	}
	if nodeWithRef.Expression != nil {
		t.Error("expected Expression to be nil when ActionRef is set")
	}

	// Test Expression variant
	expr := &LiteralInteger{NodeBase: NodeBase{NodeSpan: span}, Value: "42"}
	nodeWithExpr := &ActionExecutionNode{
		NodeBase:   NodeBase{NodeSpan: span},
		Name:       "exec2",
		Expression: expr,
	}

	if nodeWithExpr.Expression == nil {
		t.Error("expected Expression to be set")
	}
	if nodeWithExpr.ActionRef != nil {
		t.Error("expected ActionRef to be nil when Expression is set")
	}
}

func TestEdges_AllTypes(t *testing.T) {
	span := source.Span{Offset: 0, Len: 15}
	src := &QualifiedName{Parts: []NameSegment{{Text: "a"}}}
	target := &QualifiedName{Parts: []NameSegment{{Text: "b"}}}

	// ControlFlowEdge with guard
	guardExpr := &LiteralBool{NodeBase: NodeBase{NodeSpan: span}, Value: true}
	controlEdge := &ControlFlowEdge{
		NodeBase: NodeBase{NodeSpan: span},
		Source:   src,
		Target:   target,
		Guard:    guardExpr,
	}

	if controlEdge.Source.Parts[0].Text != "a" {
		t.Error("ControlFlowEdge source mismatch")
	}
	if controlEdge.Guard == nil {
		t.Error("expected Guard to be set")
	}

	// TransitionEdge with trigger
	trigger := &TimeEvent{
		NodeBase: NodeBase{NodeSpan: span},
		Duration: &LiteralInteger{NodeBase: NodeBase{NodeSpan: span}, Value: "5"},
	}
	transitionEdge := &TransitionEdge{
		NodeBase: NodeBase{NodeSpan: span},
		Source:   src,
		Target:   target,
		Trigger:  trigger,
	}

	if transitionEdge.Trigger == nil {
		t.Error("expected Trigger to be set")
	}
	timeEvt, ok := transitionEdge.Trigger.(*TimeEvent)
	if !ok {
		t.Error("expected Trigger to be *TimeEvent")
	}
	if timeEvt.Duration == nil {
		t.Error("expected Duration in TimeEvent")
	}

	// ObjectFlowEdge
	objectFlow := &ObjectFlowEdge{
		NodeBase: NodeBase{NodeSpan: span},
		Source:   src,
		Target:   target,
	}

	if objectFlow.Source.Parts[0].Text != "a" {
		t.Error("ObjectFlowEdge source mismatch")
	}
	if objectFlow.Target.Parts[0].Text != "b" {
		t.Error("ObjectFlowEdge target mismatch")
	}
}
