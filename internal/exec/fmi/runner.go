package fmi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// runnerRequest is the JSON object written to the runner's standard input.
type runnerRequest struct {
	Protocol   int              `json:"protocol"`
	FMU        string           `json:"fmu"`
	Interface  string           `json:"interface"`
	Experiment runnerExperiment `json:"experiment"`
	Start      map[string]any   `json:"start"`
	Outputs    []string         `json:"outputs"`
}

// runnerExperiment is the request's experiment member: startTime and stopTime
// always, stepSize and tolerance only when the call or the DefaultExperiment
// gives one.
type runnerExperiment struct {
	StartTime float64  `json:"startTime"`
	StopTime  float64  `json:"stopTime"`
	StepSize  *float64 `json:"stepSize,omitempty"`
	Tolerance *float64 `json:"tolerance,omitempty"`
}

// runnerReply is the JSON object the runner writes: the simulation time and
// the outputs, or its own error.
type runnerReply struct {
	Protocol int                        `json:"protocol"`
	Time     *float64                   `json:"time"`
	Outputs  map[string]json.RawMessage `json:"outputs"`
	Error    *string                    `json:"error"`
}

// requestOf composes the request one call makes of the FMU at path: the
// interface the FMU provides (coSimulation first), the experiment from the
// call's reserved inputs over the DefaultExperiment, the start values typed by
// each variable's kind and the outputs to read at stopTime.
func requestOf(call *runtime.ToolCall, d *Description, fmu string) ([]byte, error) {
	iface := "modelExchange"
	switch {
	case d.CoSimulation != nil:
		iface = "coSimulation"
	case d.ModelExchange != nil:
		iface = "modelExchange"
	default:
		iface = "scheduledExecution"
	}
	request := runnerRequest{
		Protocol:  1,
		FMU:       fmu,
		Interface: iface,
		Start:     make(map[string]any),
	}
	startTime, stopTime := 0.0, 1.0
	if d.DefaultExperiment != nil {
		if d.DefaultExperiment.HasStart {
			startTime = d.DefaultExperiment.StartTime
		}
		if d.DefaultExperiment.HasStop {
			stopTime = d.DefaultExperiment.StopTime
		}
	}
	var stepSize, tolerance *float64
	if d.DefaultExperiment != nil {
		if d.DefaultExperiment.HasStepSize {
			s := d.DefaultExperiment.StepSize
			stepSize = &s
		}
		if d.DefaultExperiment.HasTolerance {
			t := d.DefaultExperiment.Tolerance
			tolerance = &t
		}
	}
	for _, in := range call.Inputs {
		if strings.HasPrefix(in.Variable, "fmi:") {
			n, err := reservedNumber(call, in)
			if err != nil {
				return nil, err
			}
			switch in.Variable {
			case "fmi:startTime":
				startTime = n
			case "fmi:stopTime":
				stopTime = n
			case "fmi:stepSize":
				s := n
				stepSize = &s
			case "fmi:tolerance":
				t := n
				tolerance = &t
			}
			continue
		}
		v, _ := d.Variable(in.Variable)
		value, err := inputValue(call, in, d, v)
		if err != nil {
			return nil, err
		}
		request.Start[v.Name] = value
	}
	for _, out := range call.Outputs {
		if reservedOutputs[out.Variable] {
			continue
		}
		v, _ := d.Variable(out.Variable)
		request.Outputs = append(request.Outputs, v.Name)
	}
	request.Experiment = runnerExperiment{StartTime: startTime, StopTime: stopTime,
		StepSize: stepSize, Tolerance: tolerance}
	return json.Marshal(request)
}

// reservedNumber is the number a reserved fmi: input carries: Real or Integer.
func reservedNumber(call *runtime.ToolCall, in runtime.ToolInput) (float64, error) {
	switch in.Value.Value.Kind {
	case semantics.ValReal:
		return in.Value.Value.Real, nil
	case semantics.ValInt:
		return float64(in.Value.Value.Int), nil
	}
	return 0, &runtime.ToolError{Tool: call.ToolName, Kind: runtime.ToolUnsentInput,
		Detail: fmt.Sprintf("%s (%s): %s is not a number", in.Variable, in.Parameter, displayValue(in.Value))}
}

// inputValue is the JSON value one call input carries for the FMU variable:
// a scalar typed by the variable's kind, converted to the variable's coherent
// unit when the value was sent measured, or an array for a fixed one-dimensional
// variable. A measured value against a variable resolving no unit, or a
// scalar/sequence against the wrong shape, fails before the process starts.
func inputValue(call *runtime.ToolCall, in runtime.ToolInput, d *Description, v Variable) (any, error) {
	fault := func(detail string) error {
		return &runtime.ToolError{Tool: call.ToolName, Kind: runtime.ToolUnsentInput,
			Detail: fmt.Sprintf("%s (%s): %s", in.Variable, in.Parameter, detail)}
	}
	if in.Value.Items != nil {
		if len(v.Dimensions) != 1 || !v.Dimensions[0].HasStart {
			return nil, fault(fmt.Sprintf("a sequence was sent for %s, which is not a fixed one-dimensional array", v.Name))
		}
		items := make([]any, 0, len(in.Value.Items))
		for _, item := range in.Value.Items {
			if item.Unit == "" {
				item.Unit = in.Value.Unit
			}
			value, err := scalarValue(call, in, d, v, item)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
		}
		return items, nil
	}
	if len(v.Dimensions) != 0 {
		return nil, fault(fmt.Sprintf("%s is an array but %s is not a sequence", v.Name, displayValue(in.Value)))
	}
	return scalarValue(call, in, d, v, in.Value)
}

// scalarValue is the JSON one scalar of the variable's kind holds, the number
// converted to the variable's coherent unit when the value was sent measured.
func scalarValue(call *runtime.ToolCall, in runtime.ToolInput, d *Description, v Variable, tv runtime.ToolValue) (any, error) {
	fault := func(detail string) error {
		return &runtime.ToolError{Tool: call.ToolName, Kind: runtime.ToolUnsentInput,
			Detail: fmt.Sprintf("%s (%s): %s", in.Variable, in.Parameter, detail)}
	}
	if tv.Unit != "" {
		base, ok := v.UnitExponents(d)
		if !ok {
			return nil, fault(fmt.Sprintf("%s is measured in %s but variable %s resolves no coherent unit", displayValue(tv), tv.Unit, v.Name))
		}
		num, err := call.ConvertInput(runtime.ToolInput{Variable: in.Variable, Parameter: in.Parameter, Value: tv}, base.SIExpression())
		if err != nil {
			return nil, err
		}
		tv.Value, tv.Unit = num, ""
	}
	switch v.Kind {
	case KindReal:
		switch tv.Value.Kind {
		case semantics.ValReal:
			return tv.Value.Real, nil
		case semantics.ValInt:
			return float64(tv.Value.Int), nil
		}
	case KindInteger, KindEnumeration:
		if tv.Value.Kind == semantics.ValInt {
			return tv.Value.Int, nil
		}
	case KindBoolean:
		if tv.Value.Kind == semantics.ValBool {
			return tv.Value.Bool, nil
		}
	case KindString:
		if tv.Value.Kind == semantics.ValInvalid {
			return tv.Text, nil
		}
	}
	return nil, fault(fmt.Sprintf("%s is not a %s", displayValue(tv), v.Kind))
}

// displayValue spells a ToolValue in an error.
func displayValue(v runtime.ToolValue) string {
	if v.Value.Kind == semantics.ValInvalid {
		return strconv.Quote(v.Text)
	}
	return semantics.FormatConst(v.Value)
}

// replyOf reads the runner's standard output as the reply, the outputs mapped
// to the call's variables as runtime.ToolValues the call binds.
func replyOf(call *runtime.ToolCall, d *Description, stdout, stderr []byte) (map[string]runtime.ToolValue, error) {
	malformed := func(detail string) error {
		return &runtime.ToolError{Tool: call.ToolName, Kind: runtime.ToolMalformed, Detail: detail}
	}
	var reply runnerReply
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &reply); err != nil {
		return nil, malformed(fmt.Sprintf("the runner wrote no JSON object: %v", err))
	}
	if reply.Protocol != 1 {
		return nil, malformed(fmt.Sprintf("the runner spoke protocol %d; the protocol is 1", reply.Protocol))
	}
	if reply.Error != nil {
		return nil, &RunnerError{Message: *reply.Error, Stderr: strings.TrimSpace(string(stderr))}
	}
	outputs := make(map[string]runtime.ToolValue, len(call.Outputs))
	for _, out := range call.Outputs {
		if out.Variable == "fmi:time" {
			if reply.Time == nil {
				return nil, malformed("the runner answered no time")
			}
			outputs[out.Variable] = runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValReal, Real: *reply.Time}}
			continue
		}
		v, _ := d.Variable(out.Variable)
		raw, ok := reply.Outputs[v.Name]
		if !ok {
			return nil, malformed(fmt.Sprintf("the runner answered no output %q", v.Name))
		}
		value, err := outputValue(d, v, raw)
		if err != nil {
			return nil, malformed(err.Error())
		}
		outputs[out.Variable] = value
	}
	return outputs, nil
}

// outputValue is the JSON the runner answered for one variable as a ToolValue,
// typed by the variable's kind and measured in its coherent unit when it
// resolves one; a sequence for a scalar or a scalar for an array, a wrongly
// typed value, or an array of the wrong length is a protocol break.
func outputValue(d *Description, v Variable, raw json.RawMessage) (runtime.ToolValue, error) {
	text := strings.TrimSpace(string(raw))
	unit := ""
	if b, ok := v.UnitExponents(d); ok {
		unit = b.SIExpression()
	}
	if len(v.Dimensions) != 0 {
		var elems []json.RawMessage
		if err := json.Unmarshal(raw, &elems); err != nil {
			return runtime.ToolValue{}, fmt.Errorf("output %q is %s, not an array", v.Name, text)
		}
		if dim := v.Dimensions[0]; dim.HasStart && uint64(len(elems)) != dim.Start {
			return runtime.ToolValue{}, fmt.Errorf("output %q answered %d elements, its dimension holds %d", v.Name, len(elems), dim.Start)
		}
		items := make([]runtime.ToolValue, 0, len(elems))
		for _, elem := range elems {
			item, err := scalarOutput(v, elem)
			if err != nil {
				return runtime.ToolValue{}, err
			}
			items = append(items, item)
		}
		// Elements bind as plain numbers: a sequence of measured values has no
		// SysML form, so an array parameter types as Real and carries no unit.
		return runtime.ToolValue{Items: items}, nil
	}
	if strings.HasPrefix(text, "[") {
		return runtime.ToolValue{}, fmt.Errorf("output %q is %s, an array the variable does not declare", v.Name, text)
	}
	value, err := scalarOutput(v, raw)
	if err != nil {
		return runtime.ToolValue{}, err
	}
	value.Unit = unit
	return value, nil
}

// scalarOutput is the JSON of one scalar as a ToolValue, typed by the variable's
// kind; a wrongly typed value is a protocol break.
func scalarOutput(v Variable, raw json.RawMessage) (runtime.ToolValue, error) {
	text := strings.TrimSpace(string(raw))
	switch v.Kind {
	case KindReal:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return runtime.ToolValue{}, fmt.Errorf("output %q is %s, not a real", v.Name, text)
		}
		return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValReal, Real: f}}, nil
	case KindInteger, KindEnumeration:
		if strings.ContainsAny(text, ".eE") {
			return runtime.ToolValue{}, fmt.Errorf("output %q is %s, not an integer", v.Name, text)
		}
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return runtime.ToolValue{}, fmt.Errorf("output %q is %s, not an integer", v.Name, text)
		}
		return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValInt, Int: n}}, nil
	case KindBoolean:
		b, err := strconv.ParseBool(text)
		if err != nil {
			return runtime.ToolValue{}, fmt.Errorf("output %q is %s, not a boolean", v.Name, text)
		}
		return runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValBool, Bool: b}}, nil
	case KindString:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return runtime.ToolValue{}, fmt.Errorf("output %q is %s, not a string", v.Name, text)
		}
		return runtime.ToolValue{Text: s}, nil
	}
	return runtime.ToolValue{}, fmt.Errorf("output %q is a %s, which the reply cannot carry", v.Name, v.Kind)
}
