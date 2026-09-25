package analysis

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// Every data record is one item of the output's sequence, read in order; a unit column is
// read per row and must agree.
func TestReplyReadsCSVAllRows(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
		"T_max": {Column: &Column{Name: "T_max"}, Row: &Row{Kind: RowAll}, UnitColumn: &Column{Name: "U"}},
	}})
	source := "T_max,U\n300.0,K\n310.5,K\n341.2,K\n"
	out, err := readReply(t, r, entry, source)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := out["T_max"]
	if got.Unit != "K" || len(got.Items) != 3 {
		t.Fatalf("T_max = %+v, want three items measured in K", got)
	}
	for i, want := range []float64{300.0, 310.5, 341.2} {
		if got.Items[i].Value.Kind != semantics.ValReal || got.Items[i].Value.Real != want || got.Items[i].Unit != "" {
			t.Errorf("item %d = %+v, want %v with no unit of its own", i, got.Items[i], want)
		}
	}

	// A fixed unit stands in for a unit column.
	r, entry = replyOf(t, &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
		"T_max": {Column: &Column{Name: "T_max"}, Row: &Row{Kind: RowAll}, Unit: "K"},
	}})
	out, err = readReply(t, r, entry, source)
	if err != nil {
		t.Fatalf("read with fixed unit: %v", err)
	}
	if got := out["T_max"]; got.Unit != "K" || len(got.Items) != 3 {
		t.Errorf("T_max = %+v, want three items under unit K", got)
	}

	// No data record is the empty sequence, not a missing output.
	out, err = readReply(t, r, entry, "T_max,U\n")
	if err != nil {
		t.Fatalf("read empty: %v", err)
	}
	if got := out["T_max"]; got.Items == nil || len(got.Items) != 0 {
		t.Errorf("T_max = %+v, want an empty sequence", got)
	}

	// The unit column names one unit for every row.
	r, entry = replyOf(t, &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
		"T_max": {Column: &Column{Name: "T_max"}, Row: &Row{Kind: RowAll}, UnitColumn: &Column{Name: "U"}},
	}})
	_, err = readReply(t, r, entry, "T_max,U\n300.0,K\n310.5,degC\n")
	if replyKind(err) != runtime.ToolMalformed || !strings.Contains(err.Error(), "unit column U is K in row 0 but degC in row 1") {
		t.Errorf("mixed units: %v, want malformed naming the disagreeing rows", err)
	}
	_, err = readReply(t, r, entry, "T_max,U\n300.0,K\n,degC\n")
	if replyKind(err) != runtime.ToolMissingOutput || !strings.Contains(err.Error(), "row 1: an empty cell") {
		t.Errorf("empty cell: %v, want missing output naming row 1", err)
	}
	_, err = readReply(t, r, entry, "T_max,U\n300.0,\n")
	if replyKind(err) != runtime.ToolMalformed || !strings.Contains(err.Error(), "row 0 is empty") {
		t.Errorf("empty unit: %v, want malformed naming row 0", err)
	}
}

// A JSON array reads as a sequence of its scalars, every element the same wire kind.
func TestReplyReadsJSONArray(t *testing.T) {
	r, entry := replyOf(t, &Reply{Format: ReplyJSON, Outputs: map[string]*ReplyOutput{
		"T_max": {Path: jsonPath("/results/temps"), UnitPath: "/units/T_max"},
	}})
	source := `{"results":{"temps":[300,310.5,341.2]},"units":{"T_max":"K"}}`
	out, err := readReply(t, r, entry, source)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := out["T_max"]
	if got.Unit != "K" || len(got.Items) != 3 {
		t.Fatalf("T_max = %+v, want three items measured in K", got)
	}
	if got.Items[0].Value.Kind != semantics.ValInt || got.Items[0].Value.Int != 300 ||
		got.Items[1].Value.Kind != semantics.ValReal || got.Items[1].Value.Real != 310.5 {
		t.Errorf("items = %+v", got.Items)
	}

	cases := map[string]struct {
		array  string
		detail string
	}{
		"nested array": {`[[1,2],3]`, "element 0 is an array, not a number, boolean or string"},
		"object":       {`[1,{"x":2}]`, "element 1 is an object, not a number, boolean or string"},
		"null":         {`[1,null]`, "element 1 is null, not a number, boolean or string"},
		"mixed kinds":  {`[1,true]`, "element 1 is a boolean after element 0 is a number"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := strings.Replace(source, "[300,310.5,341.2]", tc.array, 1)
			_, err := readReply(t, r, entry, src)
			if replyKind(err) != runtime.ToolMalformed || !strings.Contains(err.Error(), tc.detail) ||
				!strings.Contains(err.Error(), "/results/temps") {
				t.Errorf("read: %v, want malformed naming %q at the path", err, tc.detail)
			}
		})
	}
}

// The object protocol reads `"value": [...]` as a sequence under the shared unit.
func TestReplyObjectReadsAnArrayValue(t *testing.T) {
	out, err := ToolReplyOf("Thermal", []byte(`{"outputs":{"temps":{"value":[300,310.5],"unit":"K"}}}`))
	if err != nil {
		t.Fatalf("ToolReplyOf: %v", err)
	}
	got := out["temps"]
	if got.Unit != "K" || len(got.Items) != 2 || got.Items[1].Value.Real != 310.5 {
		t.Fatalf("temps = %+v, want two items measured in K", got)
	}

	for name, tc := range map[string]struct {
		value  string
		detail string
	}{
		"nested":          {`[[1],2]`, "element 0 is an array"},
		"mixed kinds":     {`[1,"x"]`, "element 1 is a string after element 0 is a number"},
		"unit on strings": {`["a","b"]`, "a string has no unit"},
	} {
		t.Run(name, func(t *testing.T) {
			src := `{"outputs":{"temps":{"value":` + tc.value + `,"unit":"K"}}}`
			_, err := ToolReplyOf("Thermal", []byte(src))
			if replyKind(err) != runtime.ToolMalformed || !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("read: %v, want malformed naming %q", err, tc.detail)
			}
		})
	}
}

// A sequence reply renders its elements as scalars under the shared unit.
func TestRenderReplySequence(t *testing.T) {
	reply := map[string]runtime.ToolValue{
		"speeds": {Unit: "km/h", Items: []runtime.ToolValue{
			{Value: semantics.Value{Kind: semantics.ValReal, Real: 36}},
			{Value: semantics.Value{Kind: semantics.ValInt, Int: 72}},
		}},
		"note": {Text: "ok"},
	}
	if got, want := renderReply(reply), `note="ok" speeds=(36.0, 72) [km/h]`; got != want {
		t.Errorf("renderReply = %q, want %q", got, want)
	}
}
