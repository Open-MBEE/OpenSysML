// Package program is the execution-owned intermediate representation: the
// identity every lowered record carries, independent of the syntax tree it
// was lowered from. Ids are indices into the owning graph's tables, dense and
// assigned in declaration order, so two lowerings of one model agree on them;
// zero is "none". A graph's ids are local to it, and a GraphID tells graphs apart.
//
// The package reads no syntax: lower is the one package that reads the tree and
// writes these records, and executors consume them.
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

// GraphID identifies a lowered graph by the digest of its canonical encoding, so
// two lowerings of one model yield one id and an edit to the model moves it. The
// encoder that computes it is not written yet; a zero GraphID is "not computed".
type GraphID [16]byte

// IsZero reports whether the id was never computed.
func (id GraphID) IsZero() bool { return id == GraphID{} }

// String renders the id as hex.
func (id GraphID) String() string { return hex.EncodeToString(id[:]) }
