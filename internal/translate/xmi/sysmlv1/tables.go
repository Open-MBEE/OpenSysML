package sysmlv1

import (
	"fmt"
	"strconv"
	"strings"
)

// TableKind is what kind of tabular diagram a Table defines.
type TableKind string

// The MagicDraw diagram kinds that carry a semantic definition.
const (
	InstanceTable    TableKind = "InstanceTable"
	DiagramTable     TableKind = "DiagramTable"
	DependencyMatrix TableKind = "DependencyMatrix"
	RelationMap      TableKind = "RelationMap"
)

// Table is the semantic definition of a MagicDraw table, dependency matrix or
// relation map: what it lists, how it filters, sorts and lays out columns,
// read from the tool's exact profile applications on the diagram. The
// presentation tags of those applications are not read.
type Table struct {
	// Kind is the table's kind.
	Kind TableKind
	// Diagram is the diagram the definition applies to; nil when the
	// base_Diagram id names no diagram of the read documents.
	Diagram *Diagram
	// DiagramID is the base_Diagram id as written.
	DiagramID string
	// Application is the profile application that defines the table; Filter
	// is the matrix's MatrixFilter application, nil otherwise.
	Application, Filter *Stereotype
	// Scope lists the elements whose subtrees are searched for rows: a
	// table's scope, a matrix's rowScope, a relation map's contextElement.
	Scope []ElementRef
	// WholeModel is the takeWholeModelAsScope flag.
	WholeModel bool
	// RowTypes are the classifiers, stereotypes or metaclasses rows must be
	// of: a table's classifiers or rowElementType, a matrix's
	// rowElementType, a relation map's elementTypes.
	RowTypes []ElementRef
	// IncludeSubtypes is whether rows of subtypes of RowTypes are listed too;
	// MagicDraw defaults it to true.
	IncludeSubtypes bool
	// Rows are the elements listed as rows regardless of scope: a table's
	// rowElements and additionalElements.
	Rows []ElementRef
	// Excluded are the elements a table's excludedElements removes from the
	// rows, each with the rows nested under it.
	Excluded []ElementRef
	// DisplayMode is a table's displayMode tag: "List", "Compact tree" or
	// "Complete tree"; "" when the tool wrote none, which displays a list.
	DisplayMode string
	// ShowScopeAsRoot is a table's showScopeAsRoot tag: whether a tree
	// display mode lists the scope element as the root row.
	ShowScopeAsRoot bool
	// Expanded are the rows a table's expandedRows records as expanded in
	// the tool's window, in serialized order.
	Expanded []ElementRef
	// RowFilter is the row filter the tool saved with the table's diagram;
	// nil when none is saved.
	RowFilter *RowFilter
	// Columns are the table's columns in serialized order, hidden ones included.
	Columns []Column
	// Sorts are the table's sort keys in priority order.
	Sorts []Sort
	// ColumnScope and ColumnTypes are a matrix's columnScope and
	// columnElementType; IncludeColumnSubtypes its includeSubtypesOfColumnTypes.
	ColumnScope, ColumnTypes []ElementRef
	IncludeColumnSubtypes    bool
	// Direction is a matrix's direction tag: "Row to column", "Column to row"
	// or "Both".
	Direction string
	// ShowElements is a matrix's showElements tag: "All" or "With relations".
	ShowElements string
	// Criteria are a matrix's dependencyCriteria or a relation map's
	// relationCriterion, in serialized order.
	Criteria []Criterion
	// Depth is a relation map's depth; 0 when absent.
	Depth int
	// Malformed lists what in the serialization could not be read, each a
	// short phrase naming the tag and the value.
	Malformed []string
}

// ColumnKind classifies a column id.
type ColumnKind string

// The column ids MagicDraw writes.
const (
	// ColumnTool is a tool column with no model content: row numbers, the
	// margin, an empty spacer.
	ColumnTool ColumnKind = "tool"
	// ColumnProperty is a QPROP:Element:<property> column: a UML property of
	// the row element, named by Property.
	ColumnProperty ColumnKind = "property"
	// ColumnFeature is an IColumn:<id> column: a feature of the row
	// classifier, resolved in Feature.
	ColumnFeature ColumnKind = "feature"
	// ColumnPropertyPair is the PROPERTY_COLUMN / VALUE_COLUMN pair of a
	// generic table's property view.
	ColumnPropertyPair ColumnKind = "propertyPair"
	// ColumnStereotypeTag is a QPROP:stereotypeTags:<<Profile::Stereotype>>.tag
	// column: a tag of a stereotype applied to the row element.
	ColumnStereotypeTag ColumnKind = "stereotypeTag"
	// ColumnUnknown is an id in a form the reader does not know.
	ColumnUnknown ColumnKind = "unknown"
)

// Column is one column of a table.
type Column struct {
	// ID is the column id as written.
	ID string
	// Kind classifies the id.
	Kind ColumnKind
	// Property is the QPROP property name: "name", "documentation", "owner"...
	Property string
	// Feature is the IColumn feature; its Element is nil when dangling.
	Feature ElementRef
	// Profile and Stereotype name a stereotype-tag column's stereotype as the
	// id qualifies it, Profile "" when the id names the stereotype alone;
	// Tag is the tag's name and TagDefinition the stereotype's property of
	// that name, nil when the read documents define no such stereotype.
	Profile, Stereotype, Tag string
	TagDefinition            *Element
	// Hidden is whether hideColumns lists the column.
	Hidden bool
	// Width is the column's columnWidth in pixels; 0 when the tool wrote
	// none or -1, which sizes the column automatically.
	Width int
}

// RowFilter is the filter a tool saved with a table: the text its rows are
// searched for and how it is matched, as MagicDraw's diagram properties
// SAVE_FILTER_VALUE, OPTION_FILTER_SEARCHING_TEXT, OPTION_FILTER_COLUMN_INDEXES
// and the OPTION_FILTER_* flags record it.
type RowFilter struct {
	// Text is the text searched for.
	Text string
	// Columns are the indexes of the columns searched, counted from 0 among
	// the columns shown in their order, as the property's value lists them
	// joined by ^; nil when the value is empty and every column is searched.
	// The property's choices are the indexes on offer, not a selection.
	Columns []int
	// Wildcard reads Text as a wildcard pattern (* and ?); Regexp as a
	// regular expression; CaseSensitive matches case; FromStart and FromEnd
	// anchor the match at a cell's start and end.
	Wildcard, Regexp, CaseSensitive, FromStart, FromEnd bool
}

// Sort is one sort key: a column id and a direction.
type Sort struct {
	// Column is the column id sorted by, in the form Column.ID uses.
	Column string
	// Descending is the direction.
	Descending bool
}

// CriterionKind is the form a matrix or relation map criterion takes.
type CriterionKind string

// The criterion forms MagicDraw's structured expressions take.
const (
	// CriterionRelation walks relationships of one stereotype or metaclass.
	CriterionRelation CriterionKind = "relation"
	// CriterionMetachain navigates a chain of UML or stereotype properties.
	CriterionMetachain CriterionKind = "metachain"
	// CriterionProperty reads one UML property.
	CriterionProperty CriterionKind = "property"
	// CriterionScript evaluates an inline script, such as OCL.
	CriterionScript CriterionKind = "script"
	// CriterionOther is any other expression form.
	CriterionOther CriterionKind = "other"
)

// Criterion is one dependency or relation criterion of a matrix or relation
// map, decoded from the structured expression the tool serialized.
type Criterion struct {
	// Name is the display name the tool recorded; "" when none.
	Name string
	// Kind is the expression form.
	Kind CriterionKind
	// Stereotype names the relationship stereotype a relation criterion
	// walks; zero when it walks a metaclass instead.
	Stereotype StereotypeRef
	// Metaclass is the relationship metaclass a relation criterion walks;
	// "" when it walks a stereotype.
	Metaclass string
	// Direction is the walk direction as written: "DIRECT", "REVERSED",
	// "BOTH", or "" when the tool wrote none.
	Direction string
	// IncludeSubtypes is the criterion's includeSubtypes flag.
	IncludeSubtypes bool
	// Expression is the expression's xsi:type for forms other than a
	// relation, and Detail its body: the chain steps, the property, the script.
	Expression, Detail string
	// Malformed is why the criterion could not be decoded; "" when it could.
	Malformed string
}

// readTables gives every table definition its typed form once every document
// is read, since a definition may precede its diagram or the elements it names.
func (m *Model) readTables() {
	filters := map[string]*Stereotype{}
	for _, s := range m.Applications(DependencyMatrixNS) {
		if s.Name == "MatrixFilter" {
			filters[s.BaseID] = s
		}
	}
	for _, s := range m.Stereotypes {
		switch {
		case s.Namespace == MagicDrawProfileNS && s.Name == string(InstanceTable):
			m.Tables = append(m.Tables, m.instanceTable(s))
		case s.Namespace == MagicDrawProfileNS && s.Name == string(DiagramTable):
			m.Tables = append(m.Tables, m.diagramTable(s))
		case s.Namespace == MagicDrawProfileNS && s.Name == string(RelationMap):
			m.Tables = append(m.Tables, m.relationMap(s))
		case s.Namespace == DependencyMatrixNS && s.Name == string(DependencyMatrix):
			m.Tables = append(m.Tables, m.matrix(s, filters[s.BaseID]))
		}
	}
}

// newTable reads what every kind shares: the diagram and the scope.
func (m *Model) newTable(kind TableKind, s *Stereotype) *Table {
	t := &Table{Kind: kind, Application: s, DiagramID: s.BaseID}
	t.Diagram = m.Diagram(t.DiagramID)
	if t.Diagram == nil {
		t.malformed("base_Diagram", t.DiagramID, "names no diagram")
	}
	t.WholeModel = s.Bool("takeWholeModelAsScope")
	return t
}

func (t *Table) malformed(tag, value, why string) {
	if value == "" {
		t.Malformed = append(t.Malformed, fmt.Sprintf("%s: %s", tag, why))
		return
	}
	t.Malformed = append(t.Malformed, fmt.Sprintf("%s %q: %s", tag, value, why))
}

// flag reads a boolean tag MagicDraw defaults to true when absent.
func flag(s *Stereotype, name string) bool {
	return s.Tag(name) != "false"
}

func (m *Model) instanceTable(s *Stereotype) *Table {
	t := m.newTable(InstanceTable, s)
	t.RowTypes = m.TagRefs(s, "classifiers")
	t.readRows(m, s)
	return t
}

func (m *Model) diagramTable(s *Stereotype) *Table {
	t := m.newTable(DiagramTable, s)
	t.RowTypes = m.TagRefs(s, "rowElementType")
	t.readRows(m, s)
	return t
}

// The display modes MagicDraw writes to a table's displayMode tag.
const (
	DisplayList         = "List"
	DisplayCompactTree  = "Compact tree"
	DisplayCompleteTree = "Complete tree"
)

// readRows reads what an instance and a generic table share: the scope and
// the rows listed, excluded and expanded, the display mode, the columns and
// the saved row filter.
func (t *Table) readRows(m *Model, s *Stereotype) {
	t.Scope = m.TagRefs(s, "scope")
	t.IncludeSubtypes = flag(s, "includeSubtypesOfRowTypes")
	t.Rows = append(m.TagRefs(s, "rowElements"), m.TagRefs(s, "additionalElements")...)
	t.Excluded = m.TagRefs(s, "excludedElements")
	t.ShowScopeAsRoot = s.Bool("showScopeAsRoot")
	switch mode := s.Tag("displayMode"); mode {
	case "", DisplayList, DisplayCompactTree, DisplayCompleteTree:
		t.DisplayMode = mode
	default:
		t.malformed("displayMode", mode, "not List, Compact tree or Complete tree")
	}
	for _, v := range s.Tags["expandedRows"] {
		t.readExpanded(m, v)
	}
	t.readColumns(m, s)
	if t.Diagram != nil {
		t.RowFilter = rowFilter(t.Diagram)
	}
}

// expandedNone is the expandedRows value MagicDraw writes for a table none of
// whose rows is expanded.
const expandedNone = "NoExpanded"

// readExpanded reads one expandedRows value: entries of the form
// <level>,<id> joined by colons, one per expanded row, or expandedNone.
func (t *Table) readExpanded(m *Model, v string) {
	if v == expandedNone {
		return
	}
	for _, entry := range strings.Split(v, ":") {
		if entry == "" {
			continue
		}
		level, id, ok := strings.Cut(entry, ",")
		if _, err := strconv.Atoi(level); !ok || err != nil || id == "" {
			t.malformed("expandedRows", entry, "not in the form <level>,<id>")
			continue
		}
		t.Expanded = append(t.Expanded, m.elementRef(id))
	}
}

// rowFilter reads the filter saved with a table's diagram; nil when the tool
// saved none, or saved one with no text.
func rowFilter(d *Diagram) *RowFilter {
	text, ok := d.Property("OPTION_FILTER_SEARCHING_TEXT")
	if !ok || text.Value == "" || !d.Flag("SAVE_FILTER_VALUE") {
		return nil
	}
	f := &RowFilter{
		Text:          text.Value,
		Wildcard:      d.Flag("OPTION_FILTER_WILDCARD"),
		Regexp:        d.Flag("OPTION_FILTER_REGEXP"),
		CaseSensitive: d.Flag("OPTION_FILTER_CASE_SENSITIVE"),
		FromStart:     d.Flag("OPTION_FILTER_FROM_START"),
		FromEnd:       d.Flag("OPTION_FILTER_FROM_END"),
	}
	if columns, ok := d.Property("OPTION_FILTER_COLUMN_INDEXES"); ok {
		for _, index := range strings.Split(columns.Value, "^") {
			if i, err := strconv.Atoi(index); err == nil && i >= 0 {
				f.Columns = append(f.Columns, i)
			}
		}
	}
	return f
}

func (m *Model) matrix(s, filter *Stereotype) *Table {
	t := m.newTable(DependencyMatrix, s)
	t.Filter = filter
	t.Direction = s.Tag("direction")
	t.ShowElements = s.Tag("showElements")
	for _, raw := range s.Tags["dependencyCriteria"] {
		t.Criteria = append(t.Criteria, m.criterion(raw))
	}
	if filter == nil {
		t.malformed("MatrixFilter", "", "no filter application names the diagram")
		return t
	}
	t.Scope = m.TagRefs(filter, "rowScope")
	t.RowTypes = m.TagRefs(filter, "rowElementType")
	t.IncludeSubtypes = flag(filter, "includeSubtypesOfRowTypes")
	t.ColumnScope = m.TagRefs(filter, "columnScope")
	t.ColumnTypes = m.TagRefs(filter, "columnElementType")
	t.IncludeColumnSubtypes = flag(filter, "includeSubtypesOfColumnTypes")
	for _, tag := range []string{"rowQuery", "columnQuery"} {
		if filter.Tag(tag) != "" {
			t.malformed(tag, "", "a structured query selects the elements")
		}
	}
	return t
}

func (m *Model) relationMap(s *Stereotype) *Table {
	t := m.newTable(RelationMap, s)
	t.Scope = m.TagRefs(s, "contextElement")
	t.RowTypes = m.TagRefs(s, "elementTypes")
	t.IncludeSubtypes = flag(s, "includeSubtypes")
	if v := s.Tag("depth"); v != "" {
		d, err := strconv.Atoi(v)
		if err != nil || d < 0 {
			t.malformed("depth", v, "not a non-negative integer")
		} else {
			t.Depth = d
		}
	}
	for _, raw := range s.Tags["relationCriterion"] {
		t.Criteria = append(t.Criteria, m.criterion(raw))
	}
	return t
}

// readColumns reads columnIds, hideColumns, columnWidth and sort. The widths
// are listed in the columns' order.
func (t *Table) readColumns(m *Model, s *Stereotype) {
	hidden := map[string]bool{}
	for _, id := range s.Tags["hideColumns"] {
		hidden[id] = true
	}
	widths := s.Tags["columnWidth"]
	for i, id := range s.Tags["columnIds"] {
		c := m.column(id)
		c.Hidden = hidden[id]
		if i < len(widths) {
			w, err := strconv.Atoi(widths[i])
			switch {
			case err != nil || w < -1:
				t.malformed("columnWidth", widths[i], "not a width in pixels or -1")
			case w > 0:
				c.Width = w
			}
		}
		t.Columns = append(t.Columns, c)
	}
	for _, v := range s.Tags["sort"] {
		column, direction, ok := strings.Cut(v, "^")
		switch {
		case !ok:
			t.malformed("sort", v, "not in the form <column>^Asc|Desc")
		case column == "-1", column == "_EMPTY_", column == "":
			// Unsorted, as the tool writes it.
		case direction == "Asc", direction == "Desc":
			t.Sorts = append(t.Sorts, Sort{Column: column, Descending: direction == "Desc"})
		default:
			t.malformed("sort", v, "not in the form <column>^Asc|Desc")
		}
	}
}

// column classifies one column id.
func (m *Model) column(id string) Column {
	c := Column{ID: id}
	switch {
	case id == "_NUMBER_", id == "MARGIN_COLUMN", id == "_EMPTY_":
		c.Kind = ColumnTool
	case id == "PROPERTY_COLUMN", id == "VALUE_COLUMN":
		c.Kind = ColumnPropertyPair
	case strings.HasPrefix(id, "QPROP:Element:"):
		c.Kind, c.Property = ColumnProperty, strings.TrimPrefix(id, "QPROP:Element:")
	case strings.HasPrefix(id, "IColumn:"):
		c.Kind, c.Feature = ColumnFeature, m.elementRef(strings.TrimPrefix(id, "IColumn:"))
	case strings.HasPrefix(id, stereotypeTagPrefix):
		m.stereotypeTagColumn(&c, strings.TrimPrefix(id, stereotypeTagPrefix))
	default:
		c.Kind = ColumnUnknown
	}
	return c
}

const stereotypeTagPrefix = "QPROP:stereotypeTags:"

// stereotypeTagColumn reads the <<Profile::Stereotype>>.tag of a stereotype
// tag column, the stereotype qualified by its profile path or named alone,
// and resolves the tag to the stereotype's property when a read document
// defines the stereotype under a profile of that name.
func (m *Model) stereotypeTagColumn(c *Column, spec string) {
	c.Kind = ColumnUnknown
	if !strings.HasPrefix(spec, "<<") {
		return
	}
	qualified, tag, ok := strings.Cut(spec[2:], ">>.")
	if !ok || qualified == "" || tag == "" {
		return
	}
	c.Kind, c.Tag = ColumnStereotypeTag, tag
	c.Stereotype = qualified
	if i := strings.LastIndex(qualified, "::"); i >= 0 {
		c.Profile, c.Stereotype = qualified[:i], qualified[i+2:]
	}
	c.TagDefinition = m.stereotypeTag(c.Profile, c.Stereotype, tag)
}

// stereotypeTag finds the property named tag of the stereotype named
// stereotype under the profile whose qualified name is profile ("" for any),
// in the first read document that defines one; nil when none does.
func (m *Model) stereotypeTag(profile, stereotype, tag string) *Element {
	for _, d := range m.stereotypeDefinitions() {
		if d.Name != stereotype || !underProfile(d, profile) {
			continue
		}
		for _, p := range d.Owned("ownedAttribute") {
			if p.Name == tag {
				return p
			}
		}
	}
	return nil
}

// underProfile reports whether e sits under a Profile whose qualified name
// (the names of the profile and the packages above it, joined by ::) is
// profile, or under any profile when profile is "".
func underProfile(e *Element, profile string) bool {
	for p := e.Parent; p != nil; p = p.Parent {
		if p.Type != "Profile" {
			continue
		}
		if profile == "" {
			return true
		}
		var names []string
		for q := p; q != nil; q = q.Parent {
			names = append([]string{q.Name}, names...)
		}
		for i := range names {
			if strings.Join(names[i:], "::") == profile {
				return true
			}
		}
	}
	return false
}
