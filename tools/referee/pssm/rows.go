package pssm

// RowKind is which verdict the alignment note reached on a row a test reports on.
type RowKind string

const (
	// RowDiffersByDesign: the note's "differs because v2 differs" — the v2
	// library or notation specifies otherwise, so a disagreement is v2's.
	RowDiffersByDesign RowKind = "differs because v2 differs"
	// RowToolChoice: the note's "differs, v2 silent" — v2 says nothing, the
	// runtime chose, and the test is a second opinion on that choice.
	RowToolChoice RowKind = "differs, v2 silent"
	// RowAgrees is the note's "agrees" — decided for, or always matching,
	// PSSM's rule; a failing test reporting on it stays `fail`.
	RowAgrees RowKind = "agrees"
)

// Row is one row of the alignment note a test's requirement lands on.
type Row struct {
	// ID is the row's label in the note, e.g. "SM15".
	ID    string
	Kind  RowKind
	Title string
}

// Rows are the note's rows the suite's tests report on, by row label.
var Rows = map[string]Row{
	"SM6":  {"SM6", RowDiffersByDesign, "Order of a kept signal's replay against occurrences already pooled"},
	"SM7":  {"SM7", RowDiffersByDesign, "Deferral against a transition elsewhere in the configuration"},
	"SM11": {"SM11", RowAgrees, "What the completion of a composite state completes"},
	"SM15": {"SM15", RowDiffersByDesign, "A do activity and the machine competing for one occurrence"},
	"SM28": {"SM28", RowAgrees, "History with nothing to restore"},
	"SM30": {"SM30", RowAgrees, "Choice: guards read on arrival"},
	"SM32": {"SM32", RowAgrees, "Junction or join with no path through"},
	"SM46": {"SM46", RowDiffersByDesign, "Junction on a nested default entry"},
	"SM34": {"SM34", RowToolChoice, "Join: where the owner is left"},
}

// TestRows maps a test to the note row its requirement lands on. It is written
// by hand from the note, never inferred from a run: a mapped failure stays
// `fail` for an agrees or tool-choice row, and moves only for a v2-difference
// row; every mapped result cites its row.
var TestRows = map[string]string{
	// The deferring state's exit action sends each kept occurrence to self, so
	// it arrives behind what the pool already holds; PSSM releases it ahead.
	"Deferred 001": "SM6",
	"Deferred 005": "SM6",
	// One region keeps what a sibling region's transition takes.
	"Deferred 004 A": "SM7",
	"Deferred 004 B": "SM7",
	// A substate keeps what the enclosing state's transition takes.
	"Deferred 003": "SM7",
	// A composite state's body reaching `done` completes the composite, not
	// the machine, and its own completion or triggered transition then fires.
	"Final001":    "SM11",
	"Event 016 A": "SM11",
	// A history entered with nothing recorded and no default transition; History
	// 001-A has a configuration to restore (the note's own-conformance findings).
	"History 002-C": "SM28",
	// A choice's guards read what the incoming transition's effect wrote.
	"Choice 001": "SM30",
	"Choice 002": "SM30",
	// PSSM reads a nested default-entry junction at the outer transition's selection;
	// v2 reads it when the composite's separate entry transition is taken (SM46).
	"Junction 002": "SM46",
	"Junction 004": "SM46",
	"Join003":      "SM32",
	// Join001 measures owner-exit timing. Transition 019's remaining mismatch
	// ties silent target-entry order to effects, not to when the owner is left.
	"Join001": "SM34",
}

// RowOf is the note row a test reports on, if the table maps it.
func RowOf(test string) (Row, bool) {
	id, ok := TestRows[test]
	if !ok {
		return Row{}, false
	}
	row, ok := Rows[id]
	return row, ok
}
