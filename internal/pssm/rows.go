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
	"SM7":  {"SM7", RowToolChoice, "Deferral against a transition elsewhere in the configuration"},
	"SM11": {"SM11", RowToolChoice, "What the completion of a composite state completes"},
	"SM15": {"SM15", RowDiffersByDesign, "A do activity and the machine competing for one occurrence"},
	"SM28": {"SM28", RowToolChoice, "History with nothing to restore"},
	"SM30": {"SM30", RowToolChoice, "Choice: guards read on arrival"},
	"SM32": {"SM32", RowToolChoice, "Junction with no path through"},
}

// TestRows maps a test to the note row its requirement lands on. It is written
// by hand from the note, never inferred from a run: a mapped test that fails is
// `differs-by-design` when its row is v2's verdict and stays `fail` when the
// row is a tool choice, with the row cited either way; a mapped test that passes
// is a `pass` reporting on the row.
var TestRows = map[string]string{
	// The deferring state's do activity accepts the occurrence the machine
	// would defer or take; PSSM gives it to one accepter, KerML to every scope.
	"Deferred 006 A": "SM15",
	"Deferred 006 B": "SM15",
	"Deferred 006 C": "SM15",
	// One region defers what a sibling region's transition takes.
	"Deferred 004 A": "SM7",
	"Deferred 004 B": "SM7",
	// A substate defers what the enclosing state's transition takes; released
	// when the deferring state is left, ahead of later arrivals.
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
	// A junction none of whose outgoing guards holds disables the whole compound
	// transition.
	"Junction 002": "SM32",
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
