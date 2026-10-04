// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Package engine is an in-process engine over SysML text: it serves the
// execution RPCs of SysMLService — ParseSources, Evaluate, Instantiate,
// ExecuteAction and ExecuteState — plus the engine-only RenderView call, with
// the same answers as default-capabilities sysml-grpc for those RPCs, but shaped
// and marshalled as the proto3 JSON (protojson) of the api/proto messages rather
// than as protobuf. WebAssembly clients decode the
// results with the generated types they already hold, and no protobuf runtime
// is linked in, which is what keeps the js/wasm build small.
package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	fsyntax "github.com/Open-MBEE/OpenSysML/internal/frontend/syntax"
)

// I64 is a proto3 int64/uint64 under protojson: on the wire a decimal string.
// Input accepts the string or, as protojson does, the bare number.
type I64 int64

// MarshalJSON writes the decimal string protojson writes.
func (v I64) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(strconv.FormatInt(int64(v), 10))), nil
}

// UnmarshalJSON reads the decimal string, or the bare number.
func (v *I64) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int64 %q", s)
		}
		*v = I64(n)
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("invalid int64 %s", data)
	}
	*v = I64(n)
	return nil
}

// F64 is a proto3 double under protojson: a number, or the strings "NaN",
// "Infinity" and "-Infinity" that protojson uses where JSON has no spelling.
type F64 float64

// MarshalJSON writes the number, or the protojson string for a non-finite one.
func (v F64) MarshalJSON() ([]byte, error) {
	switch f := float64(v); {
	case math.IsNaN(f):
		return []byte(`"NaN"`), nil
	case math.IsInf(f, 1):
		return []byte(`"Infinity"`), nil
	case math.IsInf(f, -1):
		return []byte(`"-Infinity"`), nil
	default:
		return json.Marshal(f)
	}
}

// UnmarshalJSON reads the number, or one of the protojson non-finite spellings.
func (v *F64) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		switch s {
		case "NaN":
			*v = F64(math.NaN())
		case "Infinity":
			*v = F64(math.Inf(1))
		case "-Infinity":
			*v = F64(math.Inf(-1))
		default:
			return fmt.Errorf("invalid double %q", s)
		}
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("invalid double %s", data)
	}
	*v = F64(f)
	return nil
}

// The messages below carry the protojson field names of api/proto: oneof arms
// under their field names, every default-valued field omitted, exactly as
// protojson.Marshal emits them.

// JValue is the Value message: a oneof, so exactly one arm is ever set.
type JValue struct {
	IntValue       *I64             `json:"intValue,omitempty"`
	BigIntValue    *string          `json:"bigIntValue,omitempty"`
	RealValue      *F64             `json:"realValue,omitempty"`
	BoolValue      *bool            `json:"boolValue,omitempty"`
	StringValue    *string          `json:"stringValue,omitempty"`
	InstanceId     *I64             `json:"instanceId,omitempty"`
	Sequence       *JValueSequence  `json:"sequence,omitempty"`
	Null           *string          `json:"null,omitempty"`
	Quantity       *JQuantity       `json:"quantity,omitempty"`
	EnumLiteral    *JEnumLiteral    `json:"enumLiteral,omitempty"`
	Unset          *bool            `json:"unset,omitempty"`
	Complex        *JComplex        `json:"complex,omitempty"`
	Array          *JArray          `json:"array,omitempty"`
	Vector         *JVector         `json:"vector,omitempty"`
	VectorQuantity *JVectorQuantity `json:"vectorQuantity,omitempty"`
	MeasurementRef *JMeasurementRef `json:"measurementRef,omitempty"`
	Infinity       *bool            `json:"infinity,omitempty"`
	Function       *JFunction       `json:"function,omitempty"`
	Set            *JValueSet       `json:"set,omitempty"`
	TensorQuantity *JTensorQuantity `json:"tensorQuantity,omitempty"`
	Metaobject     *JMetaobject     `json:"metaobject,omitempty"`
	Undetermined   *JUndetermined   `json:"undetermined,omitempty"`
}

// UnmarshalJSON rejects a value setting two arms of the oneof, as protojson
// rejects it: a request naming more than one arm is ambiguous, not a pick.
func (v *JValue) UnmarshalJSON(data []byte) error {
	type alias JValue
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*v = JValue(a)
	return oneofArms("value", map[string]bool{
		"intValue": v.IntValue != nil, "bigIntValue": v.BigIntValue != nil,
		"realValue": v.RealValue != nil,
		"boolValue": v.BoolValue != nil, "stringValue": v.StringValue != nil,
		"instanceId": v.InstanceId != nil, "sequence": v.Sequence != nil,
		"null": v.Null != nil, "quantity": v.Quantity != nil,
		"enumLiteral": v.EnumLiteral != nil, "unset": v.Unset != nil,
		"complex": v.Complex != nil, "array": v.Array != nil,
		"vector": v.Vector != nil, "vectorQuantity": v.VectorQuantity != nil,
		"measurementRef": v.MeasurementRef != nil, "infinity": v.Infinity != nil,
		"function": v.Function != nil, "set": v.Set != nil,
		"tensorQuantity": v.TensorQuantity != nil, "metaobject": v.Metaobject != nil,
		"undetermined": v.Undetermined != nil,
	})
}

// oneofArms reports the first two arms set when more than one is, the
// conflict protojson reports for a message's oneof written twice.
func oneofArms(message string, arms map[string]bool) error {
	var set []string
	for name, present := range arms {
		if present {
			set = append(set, name)
		}
	}
	if len(set) > 1 {
		sort.Strings(set)
		return fmt.Errorf("%s sets both %s and %s", message, set[0], set[1])
	}
	return nil
}

type JValueSequence struct {
	Elements []*JValue `json:"elements,omitempty"`
}

type JValueSet struct {
	Elements []*JValue `json:"elements,omitempty"`
}

// JQuantity is the Quantity message; Magnitude is its oneof.
type JQuantity struct {
	IntMagnitude    *I64       `json:"intMagnitude,omitempty"`
	RealMagnitude   *F64       `json:"realMagnitude,omitempty"`
	Unit            string     `json:"unit,omitempty"`
	UnitTerm        *JUnitTerm `json:"unitTerm,omitempty"`
	BigIntMagnitude *string    `json:"bigIntMagnitude,omitempty"`
}

// UnmarshalJSON rejects a quantity setting both magnitude arms of its oneof.
func (q *JQuantity) UnmarshalJSON(data []byte) error {
	type alias JQuantity
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*q = JQuantity(a)
	return oneofArms("quantity", map[string]bool{
		"intMagnitude": q.IntMagnitude != nil, "realMagnitude": q.RealMagnitude != nil,
		"bigIntMagnitude": q.BigIntMagnitude != nil,
	})
}

type JUnitTerm struct {
	ScaleNum F64            `json:"scaleNum,omitempty"`
	ScaleDen F64            `json:"scaleDen,omitempty"`
	Factors  []*JUnitFactor `json:"factors,omitempty"`
}

type JUnitFactor struct {
	UnitId   string `json:"unitId,omitempty"`
	Exponent F64    `json:"exponent,omitempty"`
}

type JEnumLiteral struct {
	LiteralId     string  `json:"literalId,omitempty"`
	EnumerationId string  `json:"enumerationId,omitempty"`
	Name          string  `json:"name,omitempty"`
	Value         *JValue `json:"value,omitempty"`
}

type JComplex struct {
	Real      F64 `json:"real,omitempty"`
	Imaginary F64 `json:"imaginary,omitempty"`
}

type JArray struct {
	Dimensions []I64     `json:"dimensions,omitempty"`
	Elements   []*JValue `json:"elements,omitempty"`
}

type JVector struct {
	Components []*JValue `json:"components,omitempty"`
}

type JVectorQuantity struct {
	Components []*JQuantity `json:"components,omitempty"`
}

type JTensorQuantity struct {
	Dimensions []I64        `json:"dimensions,omitempty"`
	Components []*JQuantity `json:"components,omitempty"`
}

type JMeasurementRef struct {
	Unit     string     `json:"unit,omitempty"`
	UnitTerm *JUnitTerm `json:"unitTerm,omitempty"`
	UnitId   string     `json:"unitId,omitempty"`
}

type JFunction struct {
	CalcId string `json:"calcId,omitempty"`
	SelfId I64    `json:"selfId,omitempty"`
}

type JMetaobject struct {
	ElementId   string `json:"elementId,omitempty"`
	MetaclassId string `json:"metaclassId,omitempty"`
}

type JUndetermined struct {
	Reason string             `json:"reason,omitempty"`
	Count  *JMultiplicityInfo `json:"count,omitempty"`
}

type JMultiplicityInfo struct {
	Lower string `json:"lower,omitempty"`
	Upper string `json:"upper,omitempty"`
}

// JFeatureValue is the FeatureValue message.
type JFeatureValue struct {
	FeatureName  string    `json:"featureName,omitempty"`
	Value        *JValue   `json:"value,omitempty"`
	Values       []*JValue `json:"values,omitempty"`
	Materialized bool      `json:"materialized,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// JInstance is the Instance message.
type JInstance struct {
	Id            I64                       `json:"id,omitempty"`
	TypeSymbolId  string                    `json:"typeSymbolId,omitempty"`
	FeatureValues map[string]*JFeatureValue `json:"featureValues,omitempty"`
}

// JDiagnostic is the Diagnostic message, shaped in frontend/syntax where the
// syntactic command answers with it too.
type JDiagnostic = fsyntax.Diagnostic

// JSpan is the Span message.
type JSpan = fsyntax.Span

// JSourceDocument is the SourceDocument message; Source is its oneof.
type JSourceDocument struct {
	FilePath *string `json:"filePath,omitempty"`
	Content  *string `json:"content,omitempty"`
	Language string  `json:"language,omitempty"`
	Name     string  `json:"name,omitempty"`
}

// UnmarshalJSON rejects a document naming both of its source arms.
func (d *JSourceDocument) UnmarshalJSON(data []byte) error {
	type alias JSourceDocument
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*d = JSourceDocument(a)
	return oneofArms("source document", map[string]bool{
		"filePath": d.FilePath != nil, "content": d.Content != nil,
	})
}

// The request and response messages of the methods the engine serves.

type JParseSourcesRequest struct {
	Documents         []*JSourceDocument `json:"documents,omitempty"`
	StrictConformance bool               `json:"strictConformance,omitempty"`
}

type JParseSourcesResponse struct {
	ModelHash   string         `json:"modelHash,omitempty"`
	Diagnostics []*JDiagnostic `json:"diagnostics,omitempty"`
}

type JRenderViewRequest struct {
	ModelHash string `json:"modelHash,omitempty"`
	View      string `json:"view,omitempty"`
	Ports     string `json:"ports,omitempty"`
}

type JRenderViewResponse struct {
	View    string         `json:"view"`
	Kind    string         `json:"kind"`
	Stated  string         `json:"stated"`
	Notices []string       `json:"notices"`
	Canvas  *JRenderCanvas `json:"canvas,omitempty"`
	Nodes   []JRenderNode  `json:"nodes"`
	Edges   []JRenderEdge  `json:"edges"`
	Columns []string       `json:"columns,omitempty"`
	Rows    []JRenderRow   `json:"rows,omitempty"`
}

type JRenderCanvas struct {
	Unit   string   `json:"unit,omitempty"`
	Width  *float64 `json:"width,omitempty"`
	Height *float64 `json:"height,omitempty"`
}

type JRenderNode struct {
	ID              string        `json:"id"`
	Kind            string        `json:"kind"`
	Name            string        `json:"name"`
	NameSynthesized bool          `json:"nameSynthesized,omitempty"`
	Type            string        `json:"type"`
	Detail          string        `json:"detail"`
	Parent          string        `json:"parent,omitempty"`
	Fill            string        `json:"fill,omitempty"`
	Border          string        `json:"border,omitempty"`
	Style           *JRenderStyle `json:"style,omitempty"`
	X               *float64      `json:"x,omitempty"`
	Y               *float64      `json:"y,omitempty"`
	Width           *float64      `json:"width,omitempty"`
	Height          *float64      `json:"height,omitempty"`
	Collapsed       bool          `json:"collapsed,omitempty"`
	Ports           []JRenderPort `json:"ports,omitempty"`
}

type JRenderPort struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Direction string `json:"direction,omitempty"`
}

type JRenderStyle struct {
	Fill     string  `json:"fill,omitempty"`
	Line     string  `json:"line,omitempty"`
	Text     string  `json:"text,omitempty"`
	Font     string  `json:"font,omitempty"`
	FontSize float64 `json:"fontSize,omitempty"`
	Bold     bool    `json:"bold,omitempty"`
	Italic   bool    `json:"italic,omitempty"`
}

type JRenderEdge struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	FromPort string         `json:"fromPort,omitempty"`
	ToPort   string         `json:"toPort,omitempty"`
	Label    string         `json:"label"`
	Kind     string         `json:"kind"`
	Style    *JRenderStyle  `json:"style,omitempty"`
	Route    []JRenderPoint `json:"route,omitempty"`
}

type JRenderPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type JRenderRow struct {
	Cells []string `json:"cells"`
}

type JEvaluateRequest struct {
	ModelHash       string `json:"modelHash,omitempty"`
	Expression      string `json:"expression,omitempty"`
	ContextSymbolId string `json:"contextSymbolId,omitempty"`
	SubjectSymbolId string `json:"subjectSymbolId,omitempty"`
}

type JEvaluateResponse struct {
	Result      *JValue        `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	Diagnostics []*JDiagnostic `json:"diagnostics,omitempty"`
}

type JInstantiateRequest struct {
	ModelHash string `json:"modelHash,omitempty"`
	SymbolId  string `json:"symbolId,omitempty"`
}

type JInstantiateResponse struct {
	Instance    *JInstance     `json:"instance,omitempty"`
	Error       string         `json:"error,omitempty"`
	Diagnostics []*JDiagnostic `json:"diagnostics,omitempty"`
	Instances   []*JInstance   `json:"instances,omitempty"`
}

type JExecuteActionRequest struct {
	ModelHash         string             `json:"modelHash,omitempty"`
	ActionSymbolId    string             `json:"actionSymbolId,omitempty"`
	Inputs            map[string]*JValue `json:"inputs,omitempty"`
	Schedule          string             `json:"schedule,omitempty"`
	PerformerSymbolId string             `json:"performerSymbolId,omitempty"`
}

type JExecuteActionResponse struct {
	Outputs             map[string]*JValue `json:"outputs,omitempty"`
	Error               string             `json:"error,omitempty"`
	Diagnostics         []*JDiagnostic     `json:"diagnostics,omitempty"`
	FinalTime           F64                `json:"finalTime,omitempty"`
	PerformerAttributes map[string]*JValue `json:"performerAttributes,omitempty"`
}

type JExecuteStateRequest struct {
	ModelHash            string   `json:"modelHash,omitempty"`
	StateMachineSymbolId string   `json:"stateMachineSymbolId,omitempty"`
	Events               []string `json:"events,omitempty"`
	Schedule             string   `json:"schedule,omitempty"`
	PerformerSymbolId    string   `json:"performerSymbolId,omitempty"`
	Trace                bool     `json:"trace,omitempty"`
}

type JExecuteStateResponse struct {
	StatesVisited []string           `json:"statesVisited,omitempty"`
	FinalContext  map[string]*JValue `json:"finalContext,omitempty"`
	Error         string             `json:"error,omitempty"`
	Diagnostics   []*JDiagnostic    `json:"diagnostics,omitempty"`
	FinalTime     F64                `json:"finalTime,omitempty"`
	Trace         []JTraceEvent      `json:"trace,omitempty"`
	TraceDropped  int                `json:"traceDropped,omitempty"`
}

// JTraceEvent is one documented state-machine execution record.
type JTraceEvent struct {
	Kind         string   `json:"kind"`
	At           F64      `json:"at"`
	Object       string   `json:"object,omitempty"`
	Machine      string   `json:"machine,omitempty"`
	State        string   `json:"state,omitempty"`
	From         string   `json:"from,omitempty"`
	To           string   `json:"to,omitempty"`
	Target       string   `json:"target,omitempty"`
	Event        string   `json:"event,omitempty"`
	Payload      []string `json:"payload,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
	Taken        string   `json:"taken,omitempty"`
	Text         string   `json:"text"`
}
