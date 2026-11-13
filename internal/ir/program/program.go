// Package program is the execution-owned IR: the identity every lowered record
// carries independent of the syntax tree, assigned by lower and read by executors.
package program

import "encoding/hex"

// NodeID indexes a node of an action graph.
type NodeID uint32

// EdgeID indexes a succession of an action graph.
type EdgeID uint32

// StmtID indexes a statement of an action graph.
type StmtID uint32

// ExprID indexes an expression, shared by the action, state machine and calculation graphs.
type ExprID uint32

// StateID indexes a state of a state machine graph.
type StateID uint32

// RegionID indexes a region of a state machine graph.
type RegionID uint32

// PseudoID indexes a pseudostate of a state machine graph.
type PseudoID uint32

// TransID indexes a transition of a state machine graph.
type TransID uint32

// GraphID is the digest of a graph's canonical encoding, the same for two lowerings
// of one model; its encoder is not written yet, and a zero GraphID is "not computed".
type GraphID [16]byte

// IsZero reports whether the id was never computed.
func (id GraphID) IsZero() bool { return id == GraphID{} }

// String renders the id as hex.
func (id GraphID) String() string { return hex.EncodeToString(id[:]) }
