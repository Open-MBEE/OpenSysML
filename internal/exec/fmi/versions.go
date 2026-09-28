package fmi

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// wireRoot1 is the FMI 1.0 document shape.
type wireRoot1 struct {
	Description           string          `xml:"description,attr"`
	Author                string          `xml:"author,attr"`
	Version               string          `xml:"version,attr"`
	GenerationTool        string          `xml:"generationTool,attr"`
	GenerationDateAndTime string          `xml:"generationDateAndTime,attr"`
	UnitDefinitions       []wireUnit      `xml:"UnitDefinitions>Unit"`
	TypeDefinitions       []wireType1     `xml:"TypeDefinitions>Type"`
	DefaultExperiment     *wireExperiment `xml:"DefaultExperiment"`
	Implementation        *struct {
		StandAlone *wireInterface `xml:"CoSimulation_StandAlone"`
		Tool       *wireInterface `xml:"CoSimulation_Tool"`
	} `xml:"Implementation"`
	Variables []wireScalar `xml:"ModelVariables>ScalarVariable"`
}

// wireType1 is an FMI 1.0 <Type> element.
type wireType1 struct {
	Name string `xml:"name,attr"`
	Type *struct {
		Unit string `xml:"unit,attr"`
	} `xml:",any"`
}

// parse1 reads an FMI 1.0 model description: ScalarVariable elements with the
// type as a child, and a causality of input, output, internal or none —
// internal and none both read as local.
func parse1(data []byte, probe wireRoot) (*Description, error) {
	var root wireRoot1
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, &ModelDescriptionError{Detail: "modelDescription.xml does not read as FMI 1.0", Err: err}
	}
	d := &Description{
		FMIVersion:            Version1,
		ModelName:             probe.ModelName,
		Description:           root.Description,
		Author:                root.Author,
		Version:               root.Version,
		GenerationTool:        root.GenerationTool,
		GenerationDateAndTime: root.GenerationDateAndTime,
		DefaultExperiment:     root.DefaultExperiment.experiment(),
	}
	typeUnits := make(map[string]string, len(root.TypeDefinitions))
	for _, t := range root.TypeDefinitions {
		if t.Type != nil && t.Type.Unit != "" {
			typeUnits[t.Name] = t.Type.Unit
		}
	}
	for _, u := range root.UnitDefinitions {
		d.Units = append(d.Units, unitOf(u))
	}
	sortUnits(d.Units)
	if root.Implementation != nil && (root.Implementation.StandAlone != nil || root.Implementation.Tool != nil) {
		d.CoSimulation = root.Implementation.StandAlone.iface()
		if d.CoSimulation == nil {
			d.CoSimulation = root.Implementation.Tool.iface()
		}
	} else {
		d.ModelExchange = &Interface{ModelIdentifier: probe.RootModelIdentifier}
	}
	for _, sv := range root.Variables {
		v, err := variableOfScalar(sv, typeUnits, causality1)
		if err != nil {
			return nil, err
		}
		if err := dedupe(d.Variables, v.Name); err != nil {
			return nil, err
		}
		d.Variables = append(d.Variables, v)
	}
	return d, nil
}

// causality1 maps an FMI 1.0 causality to the FMI 2/3 spelling.
func causality1(name, causality string) (Causality, error) {
	switch causality {
	case "input":
		return CausalityInput, nil
	case "output":
		return CausalityOutput, nil
	case "internal", "none":
		return CausalityLocal, nil
	}
	return "", &ModelDescriptionError{Detail: fmt.Sprintf("variable %q has unknown causality %q", name, causality)}
}

// variableOfScalar reads a ScalarVariable: the type child names Kind and carries
// start, unit, min and max.
func variableOfScalar(sv wireScalar, typeUnits map[string]string, causality func(string, string) (Causality, error)) (Variable, error) {
	if sv.Name == "" {
		return Variable{}, &ModelDescriptionError{Detail: "a model variable has no name"}
	}
	if sv.Type == nil {
		return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q declares no type", sv.Name)}
	}
	kind, ok := kindOf(sv.Type.XMLName.Local)
	if !ok {
		return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q is of unknown type %q", sv.Name, sv.Type.XMLName.Local)}
	}
	v := Variable{
		Name:         sv.Name,
		Description:  sv.Description,
		Causality:    Causality(sv.Causality),
		Variability:  Variability(sv.Variability),
		Kind:         kind,
		FMIType:      sv.Type.XMLName.Local,
		DeclaredType: sv.DeclaredType,
	}
	if sv.ValueReference == nil || *sv.ValueReference < 0 || *sv.ValueReference > math.MaxUint32 {
		return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q has no valueReference", sv.Name)}
	}
	v.ValueReference = uint32(*sv.ValueReference)
	if causality != nil {
		c, err := causality(v.Name, string(v.Causality))
		if err != nil {
			return Variable{}, err
		}
		v.Causality = c
	} else {
		switch v.Causality {
		case "":
			v.Causality = CausalityLocal
		case CausalityParameter, CausalityCalculatedParameter, CausalityInput, CausalityOutput,
			CausalityLocal, CausalityIndependent, CausalityStructuralParameter:
		default:
			return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q has unknown causality %q", v.Name, v.Causality)}
		}
	}
	if sv.Type.Start != nil {
		v.Start, v.HasStart = *sv.Type.Start, true
	}
	v.Unit = sv.Type.Unit
	if v.Unit == "" && sv.DeclaredType != "" {
		v.Unit = typeUnits[sv.DeclaredType]
	}
	v.Min, v.Max = sv.Type.Min, sv.Type.Max
	defaultVariability(&v)
	return v, nil
}

// wireType23 is a declared type of FMI 2.0 (<SimpleType name><Real .../></SimpleType>)
// or 3.0 (<Float64Type name .../>): the unit the type declares is what its users
// fall back to.
type wireType23 struct {
	XMLName xml.Name
	Name    string `xml:"name,attr"`
	Unit    string `xml:"unit,attr"`
	Type    *struct {
		Unit string `xml:"unit,attr"`
	} `xml:",any"`
}

// wireRoot23 is the FMI 2.0 and 3.0 document shape; the two differ only in how
// variables spell themselves, which the ModelVariables read separates.
type wireRoot23 struct {
	Description           string         `xml:"description,attr"`
	Author                string         `xml:"author,attr"`
	Version               string         `xml:"version,attr"`
	GenerationTool        string         `xml:"generationTool,attr"`
	GenerationDateAndTime string         `xml:"generationDateAndTime,attr"`
	GUID                  string         `xml:"guid,attr"`
	InstantiationToken    string         `xml:"instantiationToken,attr"`
	ModelExchange         *wireInterface `xml:"ModelExchange"`
	CoSimulation          *wireInterface `xml:"CoSimulation"`
	ScheduledExecution    *wireInterface `xml:"ScheduledExecution"`
	UnitDefinitions       []wireUnit     `xml:"UnitDefinitions>Unit"`
	TypeDefinitions       *struct {
		Items []wireType23 `xml:",any"`
	} `xml:"TypeDefinitions"`
	DefaultExperiment *wireExperiment `xml:"DefaultExperiment"`
	Scalars           []wireScalar    `xml:"ModelVariables>ScalarVariable"`
}

// parse23 reads an FMI 2.0 (v3 false, ScalarVariable elements) or 3.0 (v3 true,
// typed elements) model description.
func parse23(data []byte, probe wireRoot, v3 bool) (*Description, error) {
	var root wireRoot23
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, &ModelDescriptionError{Detail: fmt.Sprintf("modelDescription.xml does not read as FMI %s", probe.Version), Err: err}
	}
	d := &Description{
		FMIVersion:            Version(probe.Version),
		ModelName:             probe.ModelName,
		Description:           root.Description,
		Author:                root.Author,
		Version:               root.Version,
		GenerationTool:        root.GenerationTool,
		GenerationDateAndTime: root.GenerationDateAndTime,
		InstantiationToken:    firstOf(root.InstantiationToken, root.GUID),
		CoSimulation:          root.CoSimulation.iface(),
		ModelExchange:         root.ModelExchange.iface(),
		ScheduledExecution:    root.ScheduledExecution.iface(),
		DefaultExperiment:     root.DefaultExperiment.experiment(),
	}
	typeUnits := make(map[string]string)
	if root.TypeDefinitions != nil {
		for _, t := range root.TypeDefinitions.Items {
			unit := t.Unit
			if unit == "" && t.Type != nil {
				unit = t.Type.Unit
			}
			if unit != "" {
				typeUnits[t.Name] = unit
			}
		}
	}
	for _, u := range root.UnitDefinitions {
		d.Units = append(d.Units, unitOf(u))
	}
	sortUnits(d.Units)
	if v3 {
		vars, err := typedVariables(data, typeUnits)
		if err != nil {
			return nil, err
		}
		d.Variables = vars
	} else {
		for _, sv := range root.Scalars {
			v, err := variableOfScalar(sv, typeUnits, nil)
			if err != nil {
				return nil, err
			}
			if err := dedupe(d.Variables, v.Name); err != nil {
				return nil, err
			}
			d.Variables = append(d.Variables, v)
		}
	}
	return d, nil
}

// typedVariables reads the ModelVariables element of an FMI 3.0 description:
// each variable's element name is its type. The root unmarshal skips them
// because they share no element name; here they are read again through a
// ModelVariables-only view of the same document.
func typedVariables(data []byte, typeUnits map[string]string) ([]Variable, error) {
	var model struct {
		Variables *struct {
			Items []wireTypedVariable `xml:",any"`
		} `xml:"ModelVariables"`
	}
	if err := xml.Unmarshal(data, &model); err != nil {
		return nil, &ModelDescriptionError{Detail: "modelDescription.xml does not read as FMI 3.0", Err: err}
	}
	var vars []Variable
	if model.Variables == nil {
		return vars, nil
	}
	for _, tv := range model.Variables.Items {
		v, err := variableOfTyped(tv, typeUnits)
		if err != nil {
			return nil, err
		}
		if err := dedupe(vars, v.Name); err != nil {
			return nil, err
		}
		vars = append(vars, v)
	}
	return vars, nil
}

// variableOfTyped reads one FMI 3.0 variable element.
func variableOfTyped(tv wireTypedVariable, typeUnits map[string]string) (Variable, error) {
	if tv.Name == "" {
		return Variable{}, &ModelDescriptionError{Detail: "a model variable has no name"}
	}
	kind, ok := kindOf(tv.XMLName.Local)
	if !ok {
		return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q is of unknown type %q", tv.Name, tv.XMLName.Local)}
	}
	v := Variable{
		Name:         tv.Name,
		Description:  tv.Description,
		Causality:    Causality(tv.Causality),
		Variability:  Variability(tv.Variability),
		Kind:         kind,
		FMIType:      tv.XMLName.Local,
		DeclaredType: tv.DeclaredType,
		Min:          tv.Min,
		Max:          tv.Max,
	}
	for _, dim := range tv.Dimensions {
		var d Dimension
		switch {
		case dim.Start != nil:
			n, err := strconv.ParseUint(strings.TrimSpace(*dim.Start), 10, 64)
			if err != nil {
				return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q has a dimension start %q that is not a count", v.Name, *dim.Start)}
			}
			d.Start, d.HasStart = n, true
		case dim.ValueReference != nil:
			vr := *dim.ValueReference
			if vr < 0 || vr > math.MaxUint32 {
				return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q has a dimension valueReference %d out of range", v.Name, vr)}
			}
			d.ValueReference, d.HasVR = uint32(vr), true
		default:
			return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q has a dimension with neither start nor valueReference", v.Name)}
		}
		v.Dimensions = append(v.Dimensions, d)
	}
	if tv.ValueReference == nil || *tv.ValueReference < 0 || *tv.ValueReference > math.MaxUint32 {
		return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q has no valueReference", tv.Name)}
	}
	v.ValueReference = uint32(*tv.ValueReference)
	switch v.Causality {
	case CausalityParameter, CausalityCalculatedParameter, CausalityInput, CausalityOutput,
		CausalityLocal, CausalityIndependent, CausalityStructuralParameter:
	case "":
		v.Causality = CausalityLocal
	default:
		return Variable{}, &ModelDescriptionError{Detail: fmt.Sprintf("variable %q has unknown causality %q", v.Name, tv.Causality)}
	}
	if tv.Start != nil {
		v.Start, v.HasStart = *tv.Start, true
	}
	v.Unit = tv.Unit
	if v.Unit == "" && tv.DeclaredType != "" {
		v.Unit = typeUnits[tv.DeclaredType]
	}
	defaultVariability(&v)
	return v, nil
}

// unitOf reads a wire Unit element.
func unitOf(u wireUnit) Unit {
	out := Unit{Name: u.Name}
	if u.Base != nil {
		factor, offset := 1.0, 0.0
		if u.Base.Factor != nil {
			factor = *u.Base.Factor
		}
		if u.Base.Offset != nil {
			offset = *u.Base.Offset
		}
		out.Base = &BaseUnit{Kg: u.Base.Kg, M: u.Base.M, S: u.Base.S, A: u.Base.A,
			K: u.Base.K, Mol: u.Base.Mol, Cd: u.Base.Cd, Rad: u.Base.Rad,
			Factor: factor, Offset: offset}
	}
	return out
}
