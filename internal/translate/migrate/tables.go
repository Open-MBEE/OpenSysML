package migrate

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// tableDoc is one table, matrix or relation map definition planned as a
// Document holding a Table, beside its diagram's view in the view's host.
type tableDoc struct {
	t *sysmlv1.Table
	v *view
	// doc and query are the names reserved in the host for the Document
	// definition and the row query; title is the diagram's name, trimmed.
	doc, query, title string
	// l is the definition lowered to its query, set by lowerTable.
	l *lowered
}

// written reports whether the Document is written, for the view to expose.
func (td *tableDoc) written() bool {
	return td.l != nil && td.l.refused == ""
}

// lowered is a table definition lowered to a query: the row expression, the
// notes that make it approximate, and why it was refused when it was.
type lowered struct {
	rows    qx
	notes   []string
	refused string
}

func (l *lowered) note(s string) {
	if s != "" {
		l.notes = append(l.notes, s)
	}
}

func (l *lowered) refuse(why string) {
	if l.refused == "" {
		l.refused = why
	}
}

// documentSuffix and rowsSuffix name the Document and query after the diagram.
const (
	documentSuffix = " Table"
	rowsSuffix     = " Rows"
)

// planTables pairs every table definition with its diagram's view and reserves
// its names, once views are planned so the names account for each other.
func (m *migration) planTables() {
	for _, t := range m.model.Tables {
		if t.Diagram == nil {
			continue
		}
		v := m.viewOf[t.Diagram]
		if v == nil || v.host == nil {
			continue
		}
		name := strings.TrimSpace(t.Diagram.Name)
		if name == "" {
			name = "diagram"
		}
		td := &tableDoc{t: t, v: v, title: name}
		td.doc = m.viewName(v.host, name+documentSuffix)
		td.query = m.viewName(v.host, name+rowsSuffix)
		m.tableOf[t] = td
		v.tables = append(v.tables, td)
	}
}

// tableEntry is the report row of a table definition, keyed by the stereotype
// application that defines it and named as its diagram is.
func (m *migration) tableEntry(t *sysmlv1.Table, v Verdict, target, note string) *Entry {
	e := m.diagramEntry(t.Diagram, v, target, note)
	e.ID = t.Application.ID
	e.Kind = "«" + string(t.Kind) + "» Diagram"
	return e
}

// unplacedTables reports the table definitions no view was planned for: those
// naming no diagram, or a diagram whose view has no host.
func (m *migration) unplacedTables() {
	for _, t := range m.model.Tables {
		switch {
		case t.Diagram == nil:
			e := &Entry{ID: t.Application.ID, Kind: "«" + string(t.Kind) + "»", Name: "<Diagram>", Verdict: Unmapped,
				Note: "base_Diagram " + t.DiagramID + " names no diagram of the document"}
			m.w.lines(commentLines("not migrated: " + e.Kind + " " + t.DiagramID + " — " + e.Note))
			m.report.Entries = append(m.report.Entries, *e)
		case m.tableOf[t] == nil:
			v := m.viewOf[t.Diagram]
			e := m.tableEntry(t, Unmapped, "", "its diagram is not written as a view: "+v.entry.Note)
			m.report.Entries = append(m.report.Entries, *e)
		}
	}
}

// writeTable writes a table definition as a query and a Document holding one
// Table over it, or as a comment when the definition has no query form.
func (m *migration) writeTable(td *tableDoc) {
	t, host, l := td.t, td.v.host, td.l
	prefix := m.queryPrefix(host)
	kind := string(t.Kind)
	if l.refused != "" {
		note := joinNotes(l.refused, strings.Join(l.notes, "; "))
		m.w.lines(commentLines("not migrated: «" + kind + "» '" + t.Diagram.Name + "' — " + note))
		m.report.Entries = append(m.report.Entries, *m.tableEntry(t, Unmapped, "", note))
		return
	}
	m.writeQueryDef(td.query, prefix, l.rows)
	m.w.block("part def "+writeName(td.doc)+" :> "+prefix+"Document", func() {
		m.w.line("attribute redefines title = " + stringLiteral(td.title) + ";")
		m.w.block("part rows : "+prefix+"Table", func() {
			m.w.line("attribute redefines caption = " + stringLiteral(td.title) + ";")
			m.w.line("calc rows : " + writeName(td.query) + ";")
		})
	})
	note := "the «" + kind + "» is written as a Document holding a Table over the query " + writeName(td.query)
	note = joinNotes(note, strings.Join(l.notes, "; "))
	verdict := Mapped
	if len(l.notes) > 0 {
		verdict = Approximated
	}
	target := m.qualified(append(m.segments(host), td.doc))
	m.report.Entries = append(m.report.Entries, *m.tableEntry(t, verdict, "part def "+target, note))
}

// lowerTable lowers a table definition of any kind to its row query, ahead of
// writing, so the view knows whether a Document follows it.
func (m *migration) lowerTable(td *tableDoc) {
	t, host := td.t, td.v.host
	l := &lowered{}
	td.l = l
	for _, bad := range t.Malformed {
		l.refuse(bad)
	}
	switch t.Kind {
	case sysmlv1.InstanceTable, sysmlv1.DiagramTable:
		m.lowerElementTable(t, host, l)
	case sysmlv1.DependencyMatrix:
		m.lowerMatrix(t, host, l)
	case sysmlv1.RelationMap:
		m.lowerRelationMap(t, host, l)
	}
}

// lowerElementTable lowers an instance or generic table: the scope's
// descendants and the explicit rows, filtered by row type, sorted, projected.
func (m *migration) lowerElementTable(t *sysmlv1.Table, host *sysmlv1.Element, l *lowered) {
	src := m.scopeQuery(t.Scope, t.WholeModel, t.Rows, l)
	if l.refused != "" {
		return
	}
	rows := m.typedRows(src, t.RowTypes, t.IncludeSubtypes, t.Kind == sysmlv1.InstanceTable, l)
	if l.refused != "" {
		return
	}
	rows = m.sorted(rows, t, host, l)
	l.rows = m.projected(rows, t, host, l)
}

// scopeQuery is the elements a table draws rows from: every descendant of its
// scope, the whole model's descendants, and the rows it lists explicitly.
func (m *migration) scopeQuery(scope []sysmlv1.ElementRef, whole bool, rows []sysmlv1.ElementRef, l *lowered) qx {
	var roots []string
	if whole {
		roots = m.topLevelNames()
	}
	for _, ref := range scope {
		if name, why := m.namedRoot(ref, "scope"); why != "" {
			l.refuse(why)
		} else {
			roots = append(roots, name)
		}
	}
	var listed []string
	var unresolved, unwritten []string
	seen := map[string]bool{}
	for _, ref := range rows {
		if seen[ref.ID] {
			continue
		}
		seen[ref.ID] = true
		switch {
		case ref.Element == nil:
			unresolved = append(unresolved, ref.ID)
		case !m.written(ref.Element):
			unwritten = append(unwritten, kindOf(ref.Element)+" "+qualifiedName(ref.Element))
		default:
			listed = append(listed, m.plainName(ref.Element))
		}
	}
	missing := summarizeMissing(unresolved, "resolve to no element", "resolves to no element")
	missing = append(missing, summarizeMissing(unwritten, "are not migrated", "is not migrated")...)
	var src qx
	switch {
	case len(roots) > 0:
		named := qcall("Named", qstrs("qualifiedName", roots...))
		src = qcall("Descendants", qarg1("source", named))
		if whole {
			src = qcall("Union", qarg1("source", named), qarg1("other", src))
		}
	case len(listed) == 0 && len(missing) > 0:
		l.refuse("none of the rows listed is an element of the document: " + strings.Join(missing, "; "))
		return qx{}
	case len(listed) == 0:
		l.refuse("the table names no scope and no rows")
		return qx{}
	}
	for _, why := range missing {
		if strings.HasPrefix(why, "the row ") {
			l.note(why + ", and is not listed")
		} else {
			l.note(why + ", and are not listed")
		}
	}
	if len(listed) > 0 {
		extra := qcall("Named", qstrs("qualifiedName", listed...))
		if len(roots) == 0 {
			return extra
		}
		src = qcall("Union", qarg1("source", src), qarg1("other", extra))
	}
	return src
}

// summarizeMissing words why listed rows are left out: one row by name, more
// by count with the first two named.
func summarizeMissing(items []string, many, one string) []string {
	switch len(items) {
	case 0:
		return nil
	case 1:
		return []string{"the row " + items[0] + " " + one}
	case 2:
		return []string{"2 rows (" + items[0] + ", " + items[1] + ") " + many}
	}
	return []string{fmt.Sprintf("%d rows (%s, %s and %d more) %s", len(items), items[0], items[1], len(items)-2, many)}
}

// topLevelNames lists the written top-level elements of the user model, the
// members of the global namespace a whole-model scope starts from.
func (m *migration) topLevelNames() []string {
	var names []string
	add := func(c *sysmlv1.Element) {
		if cat, _ := m.classify(c); m.written(c) && cat.keyword() != "" {
			names = append(names, m.plainName(c))
		}
	}
	for _, r := range m.model.Roots {
		switch {
		case m.isLibrary(r):
		case r.Type != "Model":
			add(r)
		default:
			for _, c := range r.Children {
				add(c)
			}
		}
	}
	return names
}

// namedRoot is the qualified name Named resolves ref by, or why it has none.
func (m *migration) namedRoot(ref sysmlv1.ElementRef, role string) (name, why string) {
	switch {
	case ref.Element == nil:
		return "", "the " + role + " " + ref.ID + " resolves to no element"
	case !m.written(ref.Element):
		return "", "the " + role + " " + kindOf(ref.Element) + " " + qualifiedName(ref.Element) + " is not migrated"
	}
	return m.plainName(ref.Element), ""
}

// plainName is e's migrated qualified name as a query string names it: the
// segments joined by ::, unquoted.
func (m *migration) plainName(e *sysmlv1.Element) string {
	return strings.Join(m.segments(e), "::")
}

// typedRows filters src by the row types; individuals only for an instance table.
func (m *migration) typedRows(src qx, types []sysmlv1.ElementRef, subtypes, individuals bool, l *lowered) qx {
	if len(types) == 0 {
		if individuals {
			l.refuse("the instance table names no classifier")
		}
		return src
	}
	filters := make([]typeFilter, len(types))
	for i, ref := range types {
		filters[i] = m.typeFilter(ref)
	}
	rows := src
	if !typesAdmitAll(filters, l) {
		var qs []qx
		seen := map[string]bool{}
		for _, f := range filters {
			switch {
			case f.refused != "" && len(filters) == 1:
				l.refuse(f.refused)
				return src
			case f.refused != "":
				l.note("elements of type " + f.label + " are not listed: " + f.refused)
				continue
			case len(f.classifiers) > 0:
				l.note(f.note)
				for _, c := range f.classifiers {
					qs = append(qs, whereType(src, m.plainName(c)))
				}
				continue
			case f.metadata != "":
				qs = append(qs, qcall("WhereMetadata", qarg1("source", src), qarg1("'metadata'", qstr(f.metadata))))
				continue
			}
			l.note(f.note)
			for _, typ := range f.types {
				if !seen[typ] {
					seen[typ] = true
					qs = append(qs, whereType(src, typ))
				}
			}
		}
		if len(qs) == 0 {
			l.refuse("none of the element types has a v2 form rows could be filtered by")
			return src
		}
		rows = union(qs)
	}
	if !subtypes {
		l.note("rows of subtypes of the row types are listed too: a type filter admits conforming elements")
	}
	if individuals {
		rows = qcall("WhereFeature", qarg1("source", rows), qarg1("'feature'", qstr("isIndividual")),
			qarg1("operator", qstr("=")), qarg1("value", qstr("true")))
	}
	return rows
}

// typesAdmitAll reports whether one of the filters admits every element, which
// makes the others moot.
func typesAdmitAll(filters []typeFilter, l *lowered) bool {
	for _, f := range filters {
		if f.all {
			l.note(f.note)
			return true
		}
	}
	return false
}

func whereType(src qx, typ string) qx {
	return qcall("WhereType", qarg1("source", src), qarg1("type", qstr(typ)))
}

// union joins queries with Union, left to right.
func union(qs []qx) qx {
	out := qs[0]
	for _, q := range qs[1:] {
		out = qcall("Union", qarg1("source", out), qarg1("other", q))
	}
	return out
}

// queryProperties maps the UML properties a column or sort reads to the query
// properties the row's migrated element has.
var queryProperties = map[string]string{
	"name":          "name",
	"documentation": "documentation",
	"qualifiedName": "qualifiedName",
	"owner":         "owner",
	"ID":            "@id",
	"Id":            "@id",
	"id":            "@id",
	"type":          "type",
	"isAbstract":    "isAbstract",
}

// columnKey is what a column reads of a row as a query property or feature
// name, or why it reads nothing a query can.
func (m *migration) columnKey(c sysmlv1.Column, host *sysmlv1.Element) (key string, feature *sysmlv1.Element, why string) {
	switch c.Kind {
	case sysmlv1.ColumnProperty:
		if p, ok := queryProperties[c.Property]; ok {
			return p, nil, ""
		}
		return "", nil, "no query property stands for the UML property " + c.Property
	case sysmlv1.ColumnFeature:
		f := c.Feature.Element
		switch {
		case f == nil:
			return "", nil, "the column " + c.ID + " names no property of the document"
		case !m.written(f):
			return "", nil, "the column's " + kindOf(f) + " " + qualifiedName(f) + " is not migrated"
		case f.Parent == nil || f.Type != "Property" || !m.isDefinition(f.Parent):
			return "", nil, "the column's " + kindOf(f) + " " + qualifiedName(f) + " is not a property of a classifier"
		}
		return m.nameOf(f), f, ""
	case sysmlv1.ColumnPropertyPair:
		return "", nil, "the column " + c.ID + " reads a property of a property, which no Column expression reads"
	}
	return "", nil, "the column " + c.ID + " is of a form the migrator does not read"
}

// sorted orders rows by the table's sort keys, least significant first so the
// stable sorts compose.
func (m *migration) sorted(rows qx, t *sysmlv1.Table, host *sysmlv1.Element, l *lowered) qx {
	for i := len(t.Sorts) - 1; i >= 0; i-- {
		s := t.Sorts[i]
		col, ok := columnByID(t, s.Column)
		if !ok {
			switch {
			case s.Column == "-1" || s.Column == "" || strings.HasPrefix(s.Column, "_"):
				continue
			case s.Column == "ID" || strings.HasSuffix(s.Column, ":hierarchyId"):
				l.note("the sort by " + s.Column + " orders rows by a tool id, which is dropped")
				continue
			}
			l.note("the sort by " + s.Column + " names no column and is dropped")
			continue
		}
		if col.Kind == sysmlv1.ColumnTool {
			continue
		}
		key, _, why := m.columnKey(col, host)
		if why != "" {
			l.note("the sort by " + s.Column + " is dropped: " + why)
			continue
		}
		dir := "ascending"
		if s.Descending {
			dir = "descending"
		}
		rows = qcall("OrderBy", qarg1("source", rows), qarg1("property", qstr(key)),
			qarg1("direction", qstr(dir)), qarg1("missing", qstr("last")), qarg1("multiple", qstr("first")))
	}
	return rows
}

// isDefinition reports whether e migrates to a definition a feature can belong to.
func (m *migration) isDefinition(e *sysmlv1.Element) bool {
	cat, _ := m.classify(e)
	return cat.keyword() != "" && cat != catPackage
}

func columnByID(t *sysmlv1.Table, id string) (sysmlv1.Column, bool) {
	for _, c := range t.Columns {
		if c.ID == id {
			return c, true
		}
	}
	return sysmlv1.Column{}, false
}

// projected selects the table's shown columns: query properties as
// properties, features as Column expressions reading them.
func (m *migration) projected(rows qx, t *sysmlv1.Table, host *sysmlv1.Element, l *lowered) qx {
	var props []string
	var cols []qx
	shown := 0
	names := columnNames{}
	for _, c := range t.Columns {
		if c.Hidden || c.Kind == sysmlv1.ColumnTool {
			continue
		}
		shown++
		key, f, why := m.columnKey(c, host)
		if why != "" {
			l.note("the column " + c.ID + " is omitted: " + why)
			continue
		}
		if f == nil {
			if names[key] {
				l.note("the column " + c.ID + " repeats the column " + key + " and is omitted")
				continue
			}
			names[key] = true
			props = append(props, key)
			continue
		}
		if unique := names.claim(key); unique != key {
			l.note("the column " + key + " is written as " + unique + ": column names are unique")
			key = unique
		}
		cols = append(cols, qcall("Column", qarg1("name", qstr(key)),
			qarg1("expression", qlit(m.ref(f, host)+" ?? \"\""))))
	}
	if shown > 0 && len(props)+len(cols) == 0 {
		l.refuse("none of the table's columns reads what a query can")
		return rows
	}
	if shown == 0 {
		l.note("the table shows no column beyond the row number; rows are projected by name")
		props = []string{"name"}
	}
	args := []qarg{qarg1("source", rows)}
	if len(props) > 0 {
		args = append(args, qstrs("properties", props...))
	}
	if len(cols) > 0 {
		args = append(args, qlist("columns", cols...))
	}
	return qcall("Project", args...)
}

// relationKinds maps the SysML relationship stereotypes and UML metaclasses a
// criterion may walk to the relationship kinds RelatedElements knows.
var relationKinds = map[string]string{
	"Refine":         "refinement",
	"Satisfy":        "satisfaction",
	"Verify":         "verification",
	"DeriveReqt":     "derivation",
	"Allocate":       "allocation",
	"Generalization": "specialization",
}

// criterionKind is the relationship kind a criterion walks, or why none does.
func criterionKind(c sysmlv1.Criterion) (kind, why string) {
	label := c.Name
	if label == "" {
		label = string(c.Kind) + " criterion"
	}
	switch {
	case c.Malformed != "":
		return "", "the criterion " + label + " is malformed: " + c.Malformed
	case c.Kind != sysmlv1.CriterionRelation:
		return "", "the criterion " + label + " is a " + c.Expression + ", which no relationship walk expresses"
	case c.Metaclass != "":
		if k, ok := relationKinds[c.Metaclass]; ok {
			return k, ""
		}
		return "", "the criterion " + label + " walks UML " + c.Metaclass + " relationships, which RelatedElements has no kind for"
	case c.Stereotype.ID == "":
		return "", "the criterion " + label + " names no relationship"
	case c.Stereotype.Name == "":
		return "", "the criterion " + label + " walks the relationship stereotype " + c.Stereotype.ID + ", which the archive does not describe"
	case !isStandardNamespace(c.Stereotype.Namespace):
		return "", "the criterion " + label + " walks «" + c.Stereotype.Name + "» of a user profile, which RelatedElements has no kind for"
	}
	if k, ok := relationKinds[c.Stereotype.Name]; ok {
		return k, ""
	}
	return "", "the criterion " + label + " walks «" + c.Stereotype.Name + "», which RelatedElements has no kind for"
}

// walkDirections are the directions a criterion walks, as RelatedElements
// spells them; "" for a direction the tool did not write.
func walkDirections(direction string) []string {
	switch direction {
	case "DIRECT", "Row to column":
		return []string{"outgoing"}
	case "REVERSED", "Column to row":
		return []string{"incoming"}
	case "BOTH", "Both":
		return []string{"outgoing", "incoming"}
	}
	return nil
}

// lowerMatrix lowers a dependency matrix: the typed rows projected by name,
// with one related column per criterion and direction listing the column
// elements each row is related to.
func (m *migration) lowerMatrix(t *sysmlv1.Table, host *sysmlv1.Element, l *lowered) {
	rowSrc := m.scopeQuery(t.Scope, t.WholeModel, nil, l)
	if l.refused != "" {
		return
	}
	rows := m.typedRows(rowSrc, t.RowTypes, t.IncludeSubtypes, false, l)
	if len(t.RowTypes) == 0 {
		l.refuse("the matrix names no row element type")
	}
	colSrc := m.scopeQuery(t.ColumnScope, t.WholeModel, nil, l)
	if l.refused != "" {
		return
	}
	cols := m.typedRows(colSrc, t.ColumnTypes, t.IncludeColumnSubtypes, false, l)
	if len(t.ColumnTypes) == 0 {
		l.refuse("the matrix names no column element type")
	}
	if len(t.Criteria) == 0 {
		l.refuse("the matrix names no dependency criterion")
	}
	if l.refused != "" {
		return
	}
	dirs := walkDirections(t.Direction)
	if dirs == nil {
		dirs = []string{"outgoing"}
		l.note("the matrix names no direction; rows are read as the clients of the relationships")
	}
	if len(dirs) > 1 {
		l.note("the matrix reads relationships in both directions, which become two columns per criterion")
	}
	var related []qx
	names := columnNames{"name": true}
	for _, c := range t.Criteria {
		kind, why := criterionKind(c)
		if why != "" {
			l.refuse(why)
			return
		}
		for _, dir := range dirs {
			name := c.Name
			if name == "" {
				name = kind
			}
			if len(dirs) > 1 {
				name += " (" + dir + ")"
			}
			if unique := names.claim(name); unique != name {
				l.note("the column " + name + " is written as " + unique + ": column names are unique")
				name = unique
			}
			related = append(related, qcall("RelatedColumn", qarg1("name", qstr(name)),
				qarg1("relationshipKind", qstr(kind)), qarg1("direction", qstr(dir)), qint1("maxDepth", 1),
				qarg1("aggregate", qstr("list")), qarg1("targets", cols)))
		}
	}
	if t.ShowElements == "With relations" {
		var kept []qx
		for _, c := range t.Criteria {
			kind, _ := criterionKind(c)
			for _, dir := range dirs {
				kept = append(kept, qcall("WhereRelated", qarg1("source", rows), qarg1("relationshipKind", qstr(kind)),
					qarg1("direction", qstr(dir)), qint1("maxDepth", 1), qarg1("exists", qlit("true"))))
			}
		}
		rows = union(kept)
		l.note("rows with a relationship of the criterion's kind to any element are listed, not only to a column element")
	}
	l.rows = qcall("Project", qarg1("source", rows), qstrs("properties", "name"), qlist("columns", related...))
}

func qint1(name string, n int) qarg { return qarg1(name, qint(n)) }

// columnNames are the column names a projection has claimed.
type columnNames map[string]bool

// claim returns name, or name with the first free numeric suffix once taken.
func (c columnNames) claim(name string) string {
	unique := name
	for i := 2; c[unique]; i++ {
		unique = fmt.Sprintf("%s %d", name, i)
	}
	c[unique] = true
	return unique
}

// lowerRelationMap lowers a relation map: the elements reached from the
// context by the criteria within the depth, filtered by type, listed by
// qualified name and type.
func (m *migration) lowerRelationMap(t *sysmlv1.Table, host *sysmlv1.Element, l *lowered) {
	var roots []string
	for _, ref := range t.Scope {
		if name, why := m.namedRoot(ref, "context element"); why != "" {
			l.refuse(why)
		} else {
			roots = append(roots, name)
		}
	}
	if len(roots) == 0 {
		l.refuse("the relation map names no context element")
	}
	if len(t.Criteria) == 0 {
		l.refuse("the relation map names no relation criterion")
	}
	if l.refused != "" {
		return
	}
	ctx := qcall("Named", qstrs("qualifiedName", roots...))
	var walks []qx
	for _, c := range t.Criteria {
		kind, why := criterionKind(c)
		if why != "" {
			l.refuse(why)
			return
		}
		dirs := walkDirections(c.Direction)
		if dirs == nil {
			dirs = []string{"outgoing"}
			l.note("the criterion " + c.Name + " names no direction; the context is read as the client of the relationships")
		}
		for _, dir := range dirs {
			args := []qarg{qarg1("source", ctx), qarg1("relationshipKind", qstr(kind)), qarg1("direction", qstr(dir))}
			if t.Depth > 0 {
				args = append(args, qint1("maxDepth", t.Depth))
			}
			walks = append(walks, qcall("RelatedElements", args...))
		}
	}
	reached := union(walks)
	if len(t.Criteria) > 1 {
		l.note("each criterion is walked from the context on its own; a path mixing criteria is not followed")
	}
	rows := m.typedRows(reached, t.RowTypes, t.IncludeSubtypes, false, l)
	if l.refused != "" {
		return
	}
	l.rows = qcall("Project", qarg1("source", rows), qstrs("properties", "qualifiedName", "@type"))
}
