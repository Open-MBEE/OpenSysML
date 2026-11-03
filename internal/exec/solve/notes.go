package solve

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Note kinds, named the same on every surface that reports a verdict.
const (
	// NoteUncovered marks an object-holding feature whose objects' own
	// assertions the query does not state.
	NoteUncovered = "uncovered"
	// NoteAssumedOne marks a feature read as one value or one object while only
	// the default `0..*` governs it.
	NoteAssumedOne = "assumed_one"
)

// Note is an assumption or omission a verdict about the query rests on, worded
// once for every surface: the REPL prints Text, a gRPC verdict carries all three.
type Note struct {
	// Kind is NoteUncovered or NoteAssumedOne.
	Kind string
	// Subject names the feature the note is about, as the report quotes it.
	Subject string
	// Text is the note as a sentence.
	Text string
}

// Notes lists the query's notes in report order: what the query leaves
// uncovered, then what it assumes holds one.
func (q *Query) Notes() []Note {
	notes := make([]Note, 0, len(q.Uncovered)+len(q.AssumedOne))
	for _, r := range q.Uncovered {
		notes = append(notes, Note{
			Kind:    NoteUncovered,
			Subject: r.Path,
			Text:    fmt.Sprintf("Not in the query: what %s asserts about its own objects, as %s; instantiate and validate an object to check it", r.Path, r.Reason),
		})
	}
	for _, a := range q.AssumedOne {
		what := "value"
		if a.Object {
			what = "object"
		}
		name := source.QualifiedNameText(a.Name)
		notes = append(notes, Note{
			Kind:    NoteAssumedOne,
			Subject: name,
			Text:    fmt.Sprintf("Assuming %s holds one %s: it declares no multiplicity, so the default %s governs it", name, what, a.Multiplicity.Text()),
		})
	}
	return notes
}
