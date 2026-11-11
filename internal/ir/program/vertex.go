package program

// VertexKind tells which table of a state graph a VertexRef indexes.
type VertexKind uint8

const (
	VertexNone VertexKind = iota
	VertexState
	VertexPseudostate
	VertexTerminate
)

// String spells the kind the way messages name vertices.
func (k VertexKind) String() string {
	switch k {
	case VertexState:
		return "state"
	case VertexPseudostate:
		return "pseudostate"
	case VertexTerminate:
		return "terminate"
	}
	return "none"
}

// VertexRef names a vertex of a lowered state graph: a StateID, a PseudoID or a
// terminate action's 1-based ordinal in the graph's terminate table. The zero
// VertexRef names no vertex.
type VertexRef struct {
	Kind  VertexKind
	Index uint32
}

// StateVertex refers to the state id, or to no vertex when id is 0.
func StateVertex(id StateID) VertexRef {
	if id == 0 {
		return VertexRef{}
	}
	return VertexRef{Kind: VertexState, Index: uint32(id)}
}

// PseudoVertex refers to the pseudostate id, or to no vertex when id is 0.
func PseudoVertex(id PseudoID) VertexRef {
	if id == 0 {
		return VertexRef{}
	}
	return VertexRef{Kind: VertexPseudostate, Index: uint32(id)}
}

// TerminateVertex refers to the terminate action at ordinal, or to no vertex when ordinal is 0.
func TerminateVertex(ordinal uint32) VertexRef {
	if ordinal == 0 {
		return VertexRef{}
	}
	return VertexRef{Kind: VertexTerminate, Index: ordinal}
}

// IsZero reports whether the reference names no vertex.
func (r VertexRef) IsZero() bool { return r == VertexRef{} }

// State is the state the reference names, 0 when it names another kind of vertex.
func (r VertexRef) State() StateID {
	if r.Kind != VertexState {
		return 0
	}
	return StateID(r.Index)
}

// Pseudo is the pseudostate the reference names, 0 when it names another kind of vertex.
func (r VertexRef) Pseudo() PseudoID {
	if r.Kind != VertexPseudostate {
		return 0
	}
	return PseudoID(r.Index)
}

// Terminate is the ordinal of the terminate action the reference names, 0 when
// it names another kind of vertex.
func (r VertexRef) Terminate() uint32 {
	if r.Kind != VertexTerminate {
		return 0
	}
	return r.Index
}
