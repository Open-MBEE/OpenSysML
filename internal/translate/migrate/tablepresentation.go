package migrate

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// excluded subtracts the rows a table excludes explicitly with Except; an
// excluded row that is not migrated is absent regardless and only noted.
func (m *migration) excluded(rows qx, refs []sysmlv1.ElementRef, l *lowered) qx {
	var names, unresolved, unwritten []string
	seen := map[string]bool{}
	for _, ref := range refs {
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
			names = append(names, m.plainName(ref.Element))
		}
	}
	for _, why := range summarizeMissing(unresolved, "resolve to no element", "resolves to no element") {
		l.note(strings.Replace(why, "the row ", "the excluded row ", 1) + ", and is absent regardless")
	}
	for _, why := range summarizeMissing(unwritten, "are not migrated", "is not migrated") {
		l.note(strings.Replace(why, "the row ", "the excluded row ", 1) + ", and is absent regardless")
	}
	if len(names) == 0 {
		return rows
	}
	l.apply(pluralRows(len(names), "excluded") + " subtracted")
	exclude := qcall("Named", qstrs("qualifiedName", names...))
	return qcall("Except", qarg1("source", rows), qarg1("exclude", exclude))
}

// pluralRows counts rows: "the excluded row is", "the 11 excluded rows are".
func pluralRows(n int, adjective string) string {
	if n == 1 {
		return "the " + adjective + " row is"
	}
	return "the " + strconv.Itoa(n) + " " + adjective + " rows are"
}

// arranged nests the rows as the table's display mode says: a compact tree
// nests each row under the nearest row containing it, a complete tree adds
// the scope's elements between them as levels, and either shows the scope
// itself as the root when the table says so. A list stays flat.
func (m *migration) arranged(rows qx, t *sysmlv1.Table, l *lowered) qx {
	switch t.DisplayMode {
	case "", sysmlv1.DisplayList:
		if t.ShowScopeAsRoot {
			l.note("the scope is shown as the root of a tree only; the list stays flat")
		}
		return rows
	case sysmlv1.DisplayCompactTree, sysmlv1.DisplayCompleteTree:
	default:
		l.note("the display mode " + strconv.Quote(t.DisplayMode) + " is not one the tool defines; the rows are listed flat")
		return rows
	}
	var ancestors []qx
	var scoped qx
	if len(l.roots) > 0 {
		scoped = qcall("Named", qstrs("qualifiedName", l.roots...))
	}
	applied := "the rows nest as a compact tree, each under the nearest row containing it"
	if t.DisplayMode == sysmlv1.DisplayCompleteTree {
		if scoped.isCall() {
			ancestors = append(ancestors, qcall("Descendants", qarg1("source", scoped)))
			applied = "the rows nest as a complete tree, under the scope's elements containing them"
		} else {
			l.note("a complete tree nests the rows under the scope's elements, and the table names no scope; the rows nest as a compact tree")
		}
	}
	if t.ShowScopeAsRoot {
		if scoped.isCall() {
			ancestors = append(ancestors, scoped)
			applied += ", with the scope as the root"
		} else {
			l.note("the scope is shown as the root, and the table names no scope")
		}
	}
	if len(t.Expanded) > 0 {
		l.note("which rows the tool had expanded is window state; every nested row is listed")
	}
	l.apply(applied)
	args := []qarg{qarg1("source", rows)}
	if len(ancestors) > 0 {
		args = append(args, qarg1("ancestors", union(ancestors)))
	}
	return qcall("Tree", args...)
}

// tagColumn is what a stereotype-tag column reads: a standard requirement tag
// as the query property the migration writes it to, a user stereotype's tag
// as a Column over the metadata def's feature it became.
func (m *migration) tagColumn(c sysmlv1.Column) columnSource {
	label := "«" + c.Stereotype + "»." + c.Tag
	if c.Profile != "" {
		label = "«" + c.Profile + "::" + c.Stereotype + "»." + c.Tag
	}
	tag := c.TagDefinition
	switch {
	case tag == nil:
		return columnSource{why: "the tag " + label + " belongs to no stereotype the document defines"}
	case requirementProperty(tag) != "":
		return columnSource{key: requirementProperty(tag)}
	case tag.Parent == nil || !m.userStereotype(tag.Parent):
		return columnSource{why: "the tag " + label + " belongs to a stereotype that is not written as a metadata def: " + m.stereotypeLibraryReason(tag.Parent)}
	case !m.written(tag.Parent) || !m.written(tag):
		return columnSource{why: "the tag " + label + " is not migrated"}
	}
	return columnSource{key: m.plainName(tag.Parent) + "::" + m.nameOf(tag), feature: tag, caption: m.nameOf(tag)}
}

// stereotypeLibraryReason says why a stereotype is library content rather
// than a user stereotype the migration writes as a metadata def.
func (m *migration) stereotypeLibraryReason(def *sysmlv1.Element) string {
	for cur := def; cur != nil; cur = cur.Parent {
		switch {
		case cur.Type == "Profile":
		case has(cur, "auxiliaryResource"):
			return "the " + kindOf(cur) + " " + qualifiedName(cur) + " is marked as an auxiliary resource"
		case has(cur, "ModelLibrary", "modelLibrary"):
			return "the " + kindOf(cur) + " " + qualifiedName(cur) + " is marked as a model library"
		}
	}
	if def == nil {
		return "it is defined outside the document"
	}
	return m.libraryReason(def)
}

// textFiltered applies the row filter the tool saved with the table as a
// WhereText over the projected columns it names, counted from 0 among the
// shown columns, or over every column when it names none.
func (m *migration) textFiltered(rows qx, f *sysmlv1.RowFilter, p *projection, l *lowered) qx {
	if f == nil {
		return rows
	}
	pattern := filterPattern(f)
	if _, err := regexp.Compile(pattern); err != nil {
		l.note("the saved row filter " + strconv.Quote(f.Text) + " is not a regular expression Go compiles, and is dropped: " + err.Error())
		return rows
	}
	var columns, unread []string
	for _, i := range f.Columns {
		if name, ok := p.names[i]; ok {
			if !slices.Contains(columns, name) {
				columns = append(columns, name)
			}
		} else {
			unread = append(unread, strconv.Itoa(i))
		}
	}
	applied := "the saved row filter " + strconv.Quote(f.Text) + " keeps the rows"
	switch {
	case len(columns) > 0:
		applied += " whose column " + strings.Join(columns, " or ") + " matches"
	case len(f.Columns) > 0:
		l.note("the saved row filter names the column " + strings.Join(unread, ", ") + ", which the query does not read; every column is searched")
		applied += " one of whose columns matches"
	default:
		applied += " one of whose columns matches"
	}
	if len(columns) > 0 && len(unread) > 0 {
		l.note("the saved row filter also names the column " + strings.Join(unread, ", ") + ", which the query does not read")
	}
	l.apply(applied)
	args := []qarg{qarg1("source", rows)}
	if len(columns) > 0 {
		args = append(args, qstrs("columns", columns...))
	}
	args = append(args, qarg1("operator", qstr("matches")), qarg1("value", qstr(pattern)))
	return qcall("WhereText", args...)
}

// filterPattern is the regular expression a saved row filter matches: its
// text as written for a regular-expression filter, with * and ? as wildcards
// for a wildcard one, literally otherwise; anchored and case-folded as the
// filter's options say. A filter without anchors matches anywhere in a cell.
func filterPattern(f *sysmlv1.RowFilter) string {
	var b strings.Builder
	if !f.CaseSensitive {
		b.WriteString("(?i)")
	}
	if f.FromStart {
		b.WriteString("^")
	}
	switch {
	case f.Regexp:
		b.WriteString(f.Text)
	case f.Wildcard:
		for _, r := range f.Text {
			switch r {
			case '*':
				b.WriteString(".*")
			case '?':
				b.WriteString(".")
			default:
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
		}
	default:
		b.WriteString(regexp.QuoteMeta(f.Text))
	}
	if f.FromEnd {
		b.WriteString("$")
	}
	return b.String()
}
