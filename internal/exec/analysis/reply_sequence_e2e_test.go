package analysis

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// sequenceDriver performs Profile, a tool-computed action answering a multi-valued
// temperature output.
const sequenceDriver = `package Profiles {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;

	action def Profile {
		metadata ToolExecution {
			toolName = "Thermal";
			uri = "thermal://host/profile";
		}
		out temps : TemperatureValue[0..*] { @ToolVariable { name = "temps"; } }
	}

	action def ProfileOnce {
		out temps : TemperatureValue[0..*];
		action step : Profile;
		bind temps = step.temps;
	}
}`

// Every CSV data record is one item of the sequence the multi-valued parameter binds:
// the unit column shared by every item, the sequence rendered in row order.
func TestToolReplyReadsCSVAllRowsAsASequence(t *testing.T) {
	p := parseReplyProbe(t, sequenceDriver, "Profiles")
	reply := &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
		"temps": {Column: &Column{Name: "T"}, Row: &Row{Kind: RowAll}, UnitColumn: &Column{Name: "U"}},
	}}
	entry := ToolEntry{ToolName: "Thermal", Executable: toolreply(t), Variables: []string{"temps"},
		Invocation: &Invocation{Args: []string{"csv-all"}}, Reply: reply}
	out, _, err := p.perform(t, toolRegistry(t, manifestDir(t, entry)), "ProfileOnce")
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	if got, want := runtime.FormatValue(out["temps"]), "[300.0 [SI::K], 310.5 [SI::K], 341.2 [SI::K]]"; got != want {
		t.Fatalf("temps = %s, want %s", got, want)
	}
}

// The unit column names one unit for the whole sequence: a row disagreeing refuses the
// reply as malformed.
func TestToolReplyCSVAllRowsRefusesMixedUnits(t *testing.T) {
	p := parseReplyProbe(t, sequenceDriver, "Profiles")
	reply := &Reply{Format: ReplyCSV, Outputs: map[string]*ReplyOutput{
		"temps": {Column: &Column{Name: "T"}, Row: &Row{Kind: RowAll}, UnitColumn: &Column{Name: "U"}},
	}}
	entry := ToolEntry{ToolName: "Thermal", Executable: toolreply(t), Variables: []string{"temps"},
		Invocation: &Invocation{Args: []string{"csv-all-mixed-units"}}, Reply: reply}
	_, _, err := p.perform(t, toolRegistry(t, manifestDir(t, entry)), "ProfileOnce")
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed || !strings.Contains(err.Error(), "degC in row 1") {
		t.Fatalf("perform = %v, want malformed naming the disagreeing row", err)
	}
}
