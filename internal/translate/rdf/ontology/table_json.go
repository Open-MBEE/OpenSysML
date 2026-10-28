package ontology

import "encoding/json"

// Table is the generated metamodel table in one record, the form -metamodel-table
// writes so tools outside this module read the same table the writers do.
type Table struct {
	// Metamodel is Version, the date in the ecore's namespace URI.
	Metamodel string `json:"metamodel"`
	// PilotTag and PilotCommit name the pilot release the table was generated from.
	PilotTag     string             `json:"pilotTag"`
	PilotCommit  string             `json:"pilotCommit"`
	Classes      []TableClass       `json:"classes"`
	Properties   []TableProperty    `json:"properties"`
	Enumerations []TableEnumeration `json:"enumerations"`
}

// TableClass is Class with JSON names.
type TableClass struct {
	Name     string   `json:"name"`
	Abstract bool     `json:"abstract"`
	Parents  []string `json:"parents"`
}

// TableEnumeration is Enumeration with JSON names.
type TableEnumeration struct {
	Name     string   `json:"name"`
	Literals []string `json:"literals"`
}

// TableProperty gives each property its kind ("object"/"datatype"), its range as a metaclass
// or enumeration name (else the datatype IRI) and its ecore upper bound (-1 unbounded).
type TableProperty struct {
	Name          string   `json:"name"`
	DefiningClass string   `json:"definingClass"`
	Kind          string   `json:"kind"`
	Range         string   `json:"range"`
	Lower         int      `json:"lower"`
	Upper         int      `json:"upper"`
	Ordered       bool     `json:"ordered"`
	Nonunique     bool     `json:"nonunique"`
	Derived       bool     `json:"derived"`
	Default       *string  `json:"default,omitempty"`
	Redefines     []string `json:"redefines,omitempty"`
	Subsets       []string `json:"subsets,omitempty"`
	Opposite      string   `json:"opposite,omitempty"`
}

// CurrentTable returns the committed table as one record.
func CurrentTable() Table {
	table := Table{
		Metamodel:    Version,
		PilotTag:     SourceTag,
		PilotCommit:  SourceCommit,
		Classes:      make([]TableClass, 0, len(classes)),
		Enumerations: make([]TableEnumeration, 0, len(enumerations)),
		Properties:   make([]TableProperty, 0, len(properties)),
	}
	for _, c := range classes {
		table.Classes = append(table.Classes, TableClass{Name: c.Name, Abstract: c.Abstract, Parents: append([]string{}, c.Parents...)})
	}
	for _, e := range enumerations {
		table.Enumerations = append(table.Enumerations, TableEnumeration{Name: e.Name, Literals: append([]string{}, e.Literals...)})
	}
	for _, p := range properties {
		table.Properties = append(table.Properties, tableProperty(p))
	}
	return table
}

func tableProperty(p Property) TableProperty {
	out := TableProperty{
		Name: p.Name, DefiningClass: p.DefiningClass, Kind: "datatype", Range: p.Range,
		Lower: p.Lower, Upper: p.UpperBound(), Ordered: p.Ordered, Nonunique: p.Nonunique, Derived: p.Derived,
		Redefines: append([]string(nil), p.Redefines...), Subsets: append([]string(nil), p.Subsets...),
		Opposite: p.Opposite,
	}
	if p.Kind == ObjectProperty {
		out.Kind = "object"
	}
	if local := LocalName(p.Range); local != "" {
		if _, ok := LookupClass(local); ok {
			out.Range = local
		} else if _, ok := LookupEnumeration(local); ok {
			out.Range = local
		}
	}
	if p.HasDefault {
		value := p.Default
		out.Default = &value
	}
	return out
}

// MarshalTable writes CurrentTable as indented JSON ending in a newline.
func MarshalTable() ([]byte, error) {
	out, err := json.MarshalIndent(CurrentTable(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
