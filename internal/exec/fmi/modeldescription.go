package fmi

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
)

// Version is an FMI version as fmiVersion spells it: major.minor only.
type Version string

// FMI versions this package reads.
const (
	Version1 Version = "1.0"
	Version2 Version = "2.0"
	Version3 Version = "3.0"
)

// Causality is a variable's causality attribute.
type Causality string

// The causalities a variable may declare.
const (
	CausalityParameter           Causality = "parameter"
	CausalityCalculatedParameter Causality = "calculatedParameter"
	CausalityInput               Causality = "input"
	CausalityOutput              Causality = "output"
	CausalityLocal               Causality = "local"
	CausalityIndependent         Causality = "independent"
	CausalityStructuralParameter Causality = "structuralParameter"
)

// Variability is a variable's variability attribute.
type Variability string

// The variabilities a variable may declare.
const (
	VariabilityConstant   Variability = "constant"
	VariabilityFixed      Variability = "fixed"
	VariabilityTunable    Variability = "tunable"
	VariabilityDiscrete   Variability = "discrete"
	VariabilityContinuous Variability = "continuous"
)

// Kind is a variable's type normalized across the FMI versions: the type
// element's name as FMI 2.0 writes it, with the FMI 3.0 scalar types folded
// onto it.
type Kind string

// The kinds a variable's type reads as.
const (
	KindReal        Kind = "Real"
	KindInteger     Kind = "Integer"
	KindBoolean     Kind = "Boolean"
	KindString      Kind = "String"
	KindEnumeration Kind = "Enumeration"
	KindBinary      Kind = "Binary"
	KindClock       Kind = "Clock"
)

// Variable is one scalar variable of the model.
type Variable struct {
	Name, Description string
	ValueReference    uint32
	Causality         Causality   // FMI 2/3 default "local"
	Variability       Variability // default "continuous" for Real, else "discrete"
	Kind              Kind
	FMIType           string // the element name as written (Float64, Real, Int32…)
	Start             string // raw start text, "" when none
	HasStart          bool
	Unit              string // unit attribute, or the declaredType's unit when the variable has none
	DeclaredType      string
	Min, Max          string
	// Dimensions are the variable's array dimensions in document order; empty
	// for a scalar. A Dimension with HasStart is fixed; HasVR names the
	// valueReference of a structural parameter.
	Dimensions []Dimension
}

// BaseUnit is the SI base-unit exponents of a Unit and the factor and offset
// converting its values to the SI unit.
type BaseUnit struct {
	Kg, M, S, A, K, Mol, Cd, Rad int
	Factor, Offset               float64 // Factor defaults 1
}

// Unit is one declared unit.
type Unit struct {
	Name string
	Base *BaseUnit
}

// Experiment is a DefaultExperiment element: the experiment the FMU's
// documentation suggests. Each field is meaningful only when its Has flag is set.
type Experiment struct {
	StartTime, StopTime, Tolerance, StepSize     float64
	HasStart, HasStop, HasTolerance, HasStepSize bool
}

// Interface is one FMI interface the FMU provides.
type Interface struct {
	ModelIdentifier string
	NeedsExecutionTool, CanHandleVariableCommunicationStepSize,
	CanGetAndSetFMUState, CanSerializeFMUState, ProvidesDirectionalDerivatives bool
}

// Description is the model description of one FMU.
type Description struct {
	FMIVersion                                                                     Version
	ModelName, Description, Author, Version, GenerationTool, GenerationDateAndTime string
	InstantiationToken                                                             string     // FMI3 instantiationToken, FMI2 guid
	CoSimulation, ModelExchange, ScheduledExecution                                *Interface // nil when absent
	Units                                                                          []Unit
	Variables                                                                      []Variable // document order
	DefaultExperiment                                                              *Experiment
	// Platforms are the sorted names of binaries/<platform>/ directories that
	// hold at least one file; empty for a source-only or description-only archive.
	Platforms []string
}

// Variable returns the variable named, and whether the description holds one.
func (d *Description) Variable(name string) (Variable, bool) {
	for _, v := range d.Variables {
		if v.Name == name {
			return v, true
		}
	}
	return Variable{}, false
}

// Unit returns the unit named, and whether the description declares one.
func (d *Description) Unit(name string) (Unit, bool) {
	for _, u := range d.Units {
		if u.Name == name {
			return u, true
		}
	}
	return Unit{}, false
}

// Inputs are the variables settable before or at initialization: causality
// input, parameter or structuralParameter, in document order.
func (d *Description) Inputs() []Variable {
	var out []Variable
	for _, v := range d.Variables {
		switch v.Causality {
		case CausalityInput, CausalityParameter, CausalityStructuralParameter:
			out = append(out, v)
		}
	}
	return out
}

// Outputs are the readable variables: causality output, calculatedParameter or
// local, in document order.
func (d *Description) Outputs() []Variable {
	var out []Variable
	for _, v := range d.Variables {
		switch v.Causality {
		case CausalityOutput, CausalityCalculatedParameter, CausalityLocal:
			out = append(out, v)
		}
	}
	return out
}

// ErrNotFMU is the typed error for data that is not an FMU: not a zip archive,
// or a zip with no modelDescription.xml at its root.
var ErrNotFMU = errors.New("not a Functional Mock-up Unit")

// ErrUnsupportedVersion is the typed error for an fmiVersion this package does
// not read.
var ErrUnsupportedVersion = errors.New("unsupported FMI version")

// UnsupportedVersionError reports a model description of a version other than
// 1.0, 2.0 or 3.0 — including pre-release spellings such as "3.0-beta.1".
type UnsupportedVersionError struct {
	Version string
}

// Error names the version read.
func (e *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("fmiVersion %q is not supported: expected 1.0, 2.0 or 3.0", e.Version)
}

// Is matches ErrUnsupportedVersion.
func (e *UnsupportedVersionError) Is(target error) bool { return target == ErrUnsupportedVersion }

// ErrModelDescription is the typed error for a malformed modelDescription.xml.
var ErrModelDescription = errors.New("malformed modelDescription.xml")

// ModelDescriptionError reports a model description that does not read.
type ModelDescriptionError struct {
	Detail string
	Err    error
}

// Error gives the detail, then the cause.
func (e *ModelDescriptionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("fmi: %s: %v", e.Detail, e.Err)
	}
	return "fmi: " + e.Detail
}

// Is matches ErrModelDescription.
func (e *ModelDescriptionError) Is(target error) bool { return target == ErrModelDescription }

// Unwrap returns the cause.
func (e *ModelDescriptionError) Unwrap() error { return e.Err }

// wireRoot is the XML shape every version shares enough of to read fmiVersion.
type wireRoot struct {
	XMLName   xml.Name `xml:"fmiModelDescription"`
	Version   string   `xml:"fmiVersion,attr"`
	ModelName string   `xml:"modelName,attr"`
	// modelIdentifier is an FMI 1.0 model-exchange identifier on the root.
	RootModelIdentifier string `xml:"modelIdentifier,attr"`
}

// ParseModelDescription reads the XML of one modelDescription.xml.
func ParseModelDescription(data []byte) (*Description, error) {
	var probe wireRoot
	if err := xml.Unmarshal(data, &probe); err != nil {
		return nil, &ModelDescriptionError{Detail: "modelDescription.xml does not read as XML", Err: err}
	}
	switch Version(probe.Version) {
	case Version1:
		return parse1(data, probe)
	case Version2:
		return parse23(data, probe, false)
	case Version3:
		return parse23(data, probe, true)
	}
	return nil, &UnsupportedVersionError{Version: probe.Version}
}

// wireUnit is one <Unit> element.
type wireUnit struct {
	Name string `xml:"name,attr"`
	Base *struct {
		Kg     int      `xml:"kg,attr"`
		M      int      `xml:"m,attr"`
		S      int      `xml:"s,attr"`
		A      int      `xml:"A,attr"`
		K      int      `xml:"K,attr"`
		Mol    int      `xml:"mol,attr"`
		Cd     int      `xml:"cd,attr"`
		Rad    int      `xml:"rad,attr"`
		Factor *float64 `xml:"factor,attr"`
		Offset *float64 `xml:"offset,attr"`
	} `xml:"BaseUnit"`
}

// wireExperiment is a DefaultExperiment element.
type wireExperiment struct {
	StartTime *float64 `xml:"startTime,attr"`
	StopTime  *float64 `xml:"stopTime,attr"`
	Tolerance *float64 `xml:"tolerance,attr"`
	StepSize  *float64 `xml:"stepSize,attr"`
}

// experiment reads a wire element, nil for none.
func (w *wireExperiment) experiment() *Experiment {
	if w == nil {
		return nil
	}
	e := &Experiment{}
	if w.StartTime != nil {
		e.StartTime, e.HasStart = *w.StartTime, true
	}
	if w.StopTime != nil {
		e.StopTime, e.HasStop = *w.StopTime, true
	}
	if w.Tolerance != nil {
		e.Tolerance, e.HasTolerance = *w.Tolerance, true
	}
	if w.StepSize != nil {
		e.StepSize, e.HasStepSize = *w.StepSize, true
	}
	return e
}

// wireInterface is a ModelExchange, CoSimulation or ScheduledExecution element.
type wireInterface struct {
	ModelIdentifier                        string `xml:"modelIdentifier,attr"`
	NeedsExecutionTool                     bool   `xml:"needsExecutionTool,attr"`
	CanHandleVariableCommunicationStepSize bool   `xml:"canHandleVariableCommunicationStepSize,attr"`
	CanGetAndSetFMUState                   bool   `xml:"canGetAndSetFMUState,attr"`
	CanSerializeFMUState                   bool   `xml:"canSerializeFMUState,attr"`
	ProvidesDirectionalDerivatives         bool   `xml:"providesDirectionalDerivative,attr"`
	ProvidesDirectionalDerivatives3        bool   `xml:"providesDirectionalDerivatives,attr"`
}

// iface reads a wire element as an Interface, nil for none.
func (w *wireInterface) iface() *Interface {
	if w == nil {
		return nil
	}
	return &Interface{ModelIdentifier: w.ModelIdentifier,
		NeedsExecutionTool:                     w.NeedsExecutionTool,
		CanHandleVariableCommunicationStepSize: w.CanHandleVariableCommunicationStepSize,
		CanGetAndSetFMUState:                   w.CanGetAndSetFMUState,
		CanSerializeFMUState:                   w.CanSerializeFMUState,
		ProvidesDirectionalDerivatives:         w.ProvidesDirectionalDerivatives || w.ProvidesDirectionalDerivatives3}
}

// wireTypedVariable is one FMI 3.0 variable element: the element name is the
// type, so it is unmarshalled through its XMLName.
type wireTypedVariable struct {
	XMLName        xml.Name
	Name           string  `xml:"name,attr"`
	ValueReference *int64  `xml:"valueReference,attr"`
	Description    string  `xml:"description,attr"`
	Causality      string  `xml:"causality,attr"`
	Variability    string  `xml:"variability,attr"`
	Start          *string `xml:"start,attr"`
	Unit           string  `xml:"unit,attr"`
	DeclaredType   string  `xml:"declaredType,attr"`
	Min            string  `xml:"min,attr"`
	Max            string  `xml:"max,attr"`
	Dimensions     []struct {
		Start          *string `xml:"start,attr"`
		ValueReference *int64  `xml:"valueReference,attr"`
	} `xml:"Dimension"`
}

// wireScalar is one FMI 1.0/2.0 ScalarVariable element.
type wireScalar struct {
	Name           string `xml:"name,attr"`
	ValueReference *int64 `xml:"valueReference,attr"`
	Description    string `xml:"description,attr"`
	Causality      string `xml:"causality,attr"`
	Variability    string `xml:"variability,attr"`
	DeclaredType   string `xml:"declaredType,attr"`
	Type           *struct {
		XMLName xml.Name
		Start   *string `xml:"start,attr"`
		Unit    string  `xml:"unit,attr"`
		Min     string  `xml:"min,attr"`
		Max     string  `xml:"max,attr"`
	} `xml:",any"`
}

// kindOf normalizes an FMI type element name to a Kind: the FMI 2.0 names read
// as-is, the FMI 3.0 scalar types fold onto them.
func kindOf(element string) (Kind, bool) {
	switch element {
	case "Real", "Float32", "Float64":
		return KindReal, true
	case "Integer", "Enumeration",
		"Int8", "UInt8", "Int16", "UInt16", "Int32", "UInt32", "Int64", "UInt64":
		if element == "Enumeration" {
			return KindEnumeration, true
		}
		return KindInteger, true
	case "Boolean":
		return KindBoolean, true
	case "String":
		return KindString, true
	case "Binary":
		return KindBinary, true
	case "Clock":
		return KindClock, true
	}
	return "", false
}

// defaultVariability fills the version-independent variability default.
func defaultVariability(v *Variable) {
	if v.Variability == "" {
		if v.Kind == KindReal {
			v.Variability = VariabilityContinuous
		} else {
			v.Variability = VariabilityDiscrete
		}
	}
}

// dedupe refuses the second variable of a name, the description read so far
// naming it.
func dedupe(vars []Variable, name string) error {
	for _, v := range vars {
		if v.Name == name {
			return &ModelDescriptionError{Detail: fmt.Sprintf("duplicate variable name %q", name)}
		}
	}
	return nil
}

// sortUnits orders the units by name for a stable description; the document's
// own order is meaningless to a lookup.
func sortUnits(units []Unit) {
	sort.Slice(units, func(i, j int) bool { return units[i].Name < units[j].Name })
}

// modelDescriptionName is the entry every FMU's root holds.
const modelDescriptionName = "modelDescription.xml"

// firstOf returns the first of its non-empty arguments.
func firstOf(first, second string) string {
	if first != "" {
		return first
	}
	return second
}

// drainClose reads a file entry of the archive fully.
func drainClose(rc io.ReadCloser) ([]byte, error) {
	defer rc.Close() // the read below is the only use; a close error cannot lose data
	return io.ReadAll(rc)
}
