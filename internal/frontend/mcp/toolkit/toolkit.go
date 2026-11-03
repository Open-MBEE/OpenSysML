// Package toolkit holds what every sysml-mcp tool group shares: the annotations
// a tool declares about itself and the coordinate conventions tool arguments
// use, so the groups agree without importing one another.
package toolkit

import (
	"math"
	"unicode/utf16"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.lsp.dev/protocol"
)

// ReadOnly annotates a tool that inspects the model or the workspace and
// changes nothing; title is what a client shows for it.
func ReadOnly(title string) *mcp.ToolAnnotations {
	closed := false
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, OpenWorldHint: &closed}
}

// Mutating annotates a tool that changes server state or files on disk;
// idempotent says whether repeating a call with the same arguments changes
// nothing further.
func Mutating(title string, idempotent bool) *mcp.ToolAnnotations {
	closed, destructive := false, true
	return &mcp.ToolAnnotations{Title: title, IdempotentHint: idempotent, DestructiveHint: &destructive, OpenWorldHint: &closed}
}

// Position is a place in a document as an editor shows it: 1-based line and
// column, the column counted in characters. Tool arguments use it rather than
// the protocol's 0-based UTF-16 positions, which agents get wrong.
type Position struct {
	Line   int `json:"line" jsonschema:"1-based line number"`
	Column int `json:"column" jsonschema:"1-based column number, counted in characters"`
}

// ToProtocol converts p to the protocol's 0-based position; lineText is the
// text of the line, needed because the protocol counts columns in UTF-16 units.
func (p Position) ToProtocol(lineText string) protocol.Position {
	line := max(p.Line-1, 0)
	want := max(p.Column-1, 0)
	var units uint32
	for i, r := range []rune(lineText) {
		if i >= want {
			break
		}
		// #nosec G115 -- RuneLen is 1 or 2 for a rune decoded from a string.
		units += uint32(utf16.RuneLen(r))
	}
	return protocol.Position{Line: lineNumber(line), Character: units}
}

// lineNumber is a 0-based line as the protocol counts it; no document has
// more lines than the protocol can number, so the clamp is never reached.
func lineNumber(line int) uint32 {
	if line > math.MaxUint32 {
		return math.MaxUint32
	}
	// #nosec G115 -- clamped above.
	return uint32(line)
}

// FromProtocol converts a protocol position on lineText to a 1-based Position.
func FromProtocol(pos protocol.Position, lineText string) Position {
	var units uint32
	chars := 0
	for _, r := range lineText {
		if units >= pos.Character {
			break
		}
		// #nosec G115 -- see above.
		units += uint32(utf16.RuneLen(r))
		chars++
	}
	return Position{Line: int(pos.Line) + 1, Column: chars + 1}
}
