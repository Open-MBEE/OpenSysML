package docir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// Evaluate evaluates a compiled document plan into an immutable document,
// executing every referenced query through the query execution engine.
// text supplies notation labels to diagram renderings and may be nil.
func Evaluate(
	plan *docplan.Plan,
	context queryexec.Context,
	options queryexec.Options,
	text view.SourceText,
) (*Document, error) {
	return evaluate(plan, context, options, text, nil)
}

// EvaluateLinked evaluates one plan like Evaluate while also emitting the
// stable anchors the sibling documents' plans reference into it, so a
// document rendered on its own carries the anchors incoming cross-document
// links expect once the files share a directory.
func EvaluateLinked(
	plan *docplan.Plan,
	siblings []*docplan.Plan,
	context queryexec.Context,
	options queryexec.Options,
	text view.SourceText,
) (*Document, error) {
	external := make(map[string]map[string]bool)
	for _, sibling := range siblings {
		if sibling.Compiled() {
			collectCrossAnchors(sibling.Content(), external)
		}
	}
	return evaluate(plan, context, options, text, external[plan.Name()])
}

// EvaluateSet evaluates a set of compiled document plans together, so a
// content block one document references from another carries its anchor in
// the rendered target document.
func EvaluateSet(
	plans []*docplan.Plan,
	context queryexec.Context,
	options queryexec.Options,
	text view.SourceText,
) ([]*Document, error) {
	external := make(map[string]map[string]bool)
	for _, plan := range plans {
		if !plan.Compiled() {
			return nil, &Error{Kind: ErrorInvalidPlan}
		}
		collectCrossAnchors(plan.Content(), external)
	}
	documents := make([]*Document, 0, len(plans))
	for _, plan := range plans {
		document, err := evaluate(plan, context, options, text, external[plan.Name()])
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, nil
}

func evaluate(
	plan *docplan.Plan,
	context queryexec.Context,
	options queryexec.Options,
	text view.SourceText,
	external map[string]bool,
) (*Document, error) {
	if !plan.Compiled() {
		return nil, &Error{Kind: ErrorInvalidPlan}
	}
	if context.Index == nil || context.Resolver == nil || context.Model == nil {
		return nil, &Error{Kind: ErrorInvalidContext, Document: plan.Name()}
	}
	referenced := referencedAnchors(plan.Content())
	for anchor := range external {
		referenced[anchor] = true
	}
	e := &evaluator{
		document:   plan.Name(),
		context:    context,
		options:    options,
		text:       text,
		referenced: referenced,
	}
	content, err := e.evaluateContent(plan.Content())
	if err != nil {
		return nil, err
	}
	return &Document{
		name:    plan.Name(),
		title:   plan.Title(),
		content: content,
		origin:  plan.Origin(),
	}, nil
}

// collectCrossAnchors records, per target document, the anchors that other
// documents' reference runs require it to emit.
func collectCrossAnchors(planned []docplan.Content, external map[string]map[string]bool) {
	for _, node := range planned {
		for _, run := range node.Runs() {
			if run.Kind() != docplan.RunRef || run.RefDocument() == "" || len(run.RefPath()) == 0 {
				continue
			}
			if external[run.RefDocument()] == nil {
				external[run.RefDocument()] = make(map[string]bool)
			}
			external[run.RefDocument()][AnchorFor(run.RefPath())] = true
		}
		collectCrossAnchors(node.Children(), external)
	}
}

type evaluator struct {
	document   string
	context    queryexec.Context
	options    queryexec.Options
	text       view.SourceText
	referenced map[string]bool
	path       []string
}

// referencedAnchors collects the anchors of every content node a reference
// run targets.
func referencedAnchors(planned []docplan.Content) map[string]bool {
	anchors := make(map[string]bool)
	var walk func(nodes []docplan.Content)
	walk = func(nodes []docplan.Content) {
		for _, node := range nodes {
			for _, run := range node.Runs() {
				if run.Kind() == docplan.RunRef && run.RefDocument() == "" {
					anchors[AnchorFor(run.RefPath())] = true
				}
			}
			walk(node.Children())
		}
	}
	walk(planned)
	return anchors
}

// AnchorFor derives a content node's stable anchor from its named path:
// segments joined by "-", with every byte outside [A-Za-z0-9_] encoded as
// "." and two uppercase hex digits, so distinct paths never collide.
func AnchorFor(path []string) string {
	segments := make([]string, len(path))
	for i, segment := range path {
		var b strings.Builder
		for j := 0; j < len(segment); j++ {
			ch := segment[j]
			switch {
			case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9', ch == '_':
				b.WriteByte(ch)
			default:
				fmt.Fprintf(&b, ".%02X", ch)
			}
		}
		segments[i] = b.String()
	}
	return strings.Join(segments, "-")
}

func (e *evaluator) evaluateContent(planned []docplan.Content) ([]Content, error) {
	content := make([]Content, 0, len(planned))
	for _, node := range planned {
		e.path = append(e.path, node.Name())
		evaluated, err := e.evaluateNode(node)
		if err != nil {
			e.path = e.path[:len(e.path)-1]
			return nil, err
		}
		if anchor := AnchorFor(e.path); e.referenced[anchor] {
			evaluated.anchor = anchor
		}
		e.path = e.path[:len(e.path)-1]
		content = append(content, evaluated)
	}
	return content, nil
}

func (e *evaluator) evaluateNode(node docplan.Content) (Content, error) {
	switch node.Kind() {
	case docplan.ContentSection:
		children, err := e.evaluateContent(node.Children())
		if err != nil {
			return Content{}, err
		}
		return Content{
			kind:     ContentSection,
			name:     node.Name(),
			title:    node.Title(),
			children: children,
			origin:   node.Origin(),
		}, nil
	case docplan.ContentParagraph:
		return e.evaluateParagraph(node)
	case docplan.ContentTable:
		return e.evaluateTable(node)
	case docplan.ContentList:
		return e.evaluateList(node)
	case docplan.ContentDefinitions:
		return e.evaluateDefinitions(node)
	case docplan.ContentDiagram:
		return e.evaluateDiagram(node)
	default:
		return Content{}, &Error{
			Kind:     ErrorInvalidPlan,
			Document: e.document,
			Content:  node.Name(),
			Origin:   node.Origin(),
		}
	}
}

func (e *evaluator) evaluateParagraph(node docplan.Content) (Content, error) {
	content := Content{
		kind:   ContentParagraph,
		name:   node.Name(),
		origin: node.Origin(),
	}
	if planned := node.Runs(); len(planned) > 0 {
		content.runs = make([]TextRun, len(planned))
		for i, run := range planned {
			content.runs[i] = evaluatedRun(run)
		}
		return content, nil
	}
	if node.Query() == nil {
		content.runs = []TextRun{{text: node.Text(), origin: node.Origin()}}
		return content, nil
	}
	result, err := e.executeQuery(node)
	if err != nil {
		return Content{}, err
	}
	content.query = node.Query().Entry()
	content.queryOrigin = result.Origin()
	if templates := node.ColumnRuns(); len(templates) > 0 {
		at, err := e.templateIndexes(node, result.Columns(), templates)
		if err != nil {
			return Content{}, err
		}
		for number, row := range result.Rows() {
			runs, err := e.templateRuns(node, at, templates, row, number+1)
			if err != nil {
				return Content{}, err
			}
			content.runs = append(content.runs, runs...)
		}
		return content, nil
	}
	for _, row := range result.Rows() {
		content.runs = append(content.runs, e.rowRuns(row)...)
	}
	return content, nil
}

// templateIndexes resolves every column a node's column runs name against
// the result's projected columns.
func (e *evaluator) templateIndexes(
	node docplan.Content,
	columns []queryexec.Column,
	templates []docplan.ColumnRun,
) (map[string]int, error) {
	at := make(map[string]int, len(columns))
	for i, column := range columns {
		at[column.Name()] = i
	}
	for _, template := range templates {
		for _, name := range []string{template.Column(), template.StyleColumn(), template.TargetColumn()} {
			if name == "" {
				continue
			}
			if _, ok := at[name]; !ok {
				return nil, &Error{
					Kind:     ErrorUnknownRunColumn,
					Document: e.document,
					Content:  node.Name(),
					Query:    node.Query().Entry(),
					Column:   name,
					Origin:   template.Origin(),
				}
			}
		}
	}
	return at, nil
}

// templateRuns renders one query row through the node's column runs, in
// declaration order; number is the row's 1-based position.
func (e *evaluator) templateRuns(
	node docplan.Content,
	at map[string]int,
	templates []docplan.ColumnRun,
	row queryexec.Row,
	number int,
) ([]TextRun, error) {
	cells := row.Cells()
	cellOf := func(name string) queryexec.Cell {
		if i, ok := at[name]; ok && i < len(cells) {
			return cells[i]
		}
		return queryexec.Cell{}
	}
	var runs []TextRun
	for _, template := range templates {
		switch template.Kind() {
		case docplan.TemplateLink:
			target, err := e.rowTarget(node, template, cellOf(template.TargetColumn()), number)
			if err != nil {
				return nil, err
			}
			for _, value := range cellOf(template.Column()).Values() {
				runs = append(runs, TextRun{kind: RunLink, text: e.valueText(value), target: target, origin: value.Origin()})
			}
		default:
			kind := styledKind(template.Style())
			if template.StyleColumn() != "" {
				styled, err := e.rowStyle(node, template, cellOf(template.StyleColumn()), number)
				if err != nil {
					return nil, err
				}
				kind = styled
			}
			for _, value := range cellOf(template.Column()).Values() {
				runs = append(runs, TextRun{kind: kind, text: e.valueText(value), origin: value.Origin()})
			}
		}
	}
	return runs, nil
}

// rowStyle reads one row's style from a span column run's style column.
func (e *evaluator) rowStyle(node docplan.Content, template docplan.ColumnRun, cell queryexec.Cell, number int) (RunKind, error) {
	invalid := func(actual string) error {
		return &Error{
			Kind:     ErrorInvalidRunStyle,
			Document: e.document,
			Content:  node.Name(),
			Query:    node.Query().Entry(),
			Column:   template.StyleColumn(),
			Row:      number,
			Actual:   actual,
			Origin:   template.Origin(),
		}
	}
	values := cell.Values()
	if len(values) != 1 {
		return "", invalid(fmt.Sprintf("%d values", len(values)))
	}
	style, ok := values[0].String()
	if !ok {
		return "", invalid(strconv.Quote(e.valueText(values[0])))
	}
	switch docplan.RunStyle(style) {
	case docplan.StylePlain, docplan.StyleEmphasis, docplan.StyleStrong, docplan.StyleCode:
		return styledKind(docplan.RunStyle(style)), nil
	default:
		return "", invalid(strconv.Quote(style))
	}
}

// rowTarget reads one row's link destination from a link column run's
// target column.
func (e *evaluator) rowTarget(node docplan.Content, template docplan.ColumnRun, cell queryexec.Cell, number int) (string, error) {
	invalid := func(actual string) error {
		return &Error{
			Kind:     ErrorInvalidRunTarget,
			Document: e.document,
			Content:  node.Name(),
			Query:    node.Query().Entry(),
			Column:   template.TargetColumn(),
			Row:      number,
			Actual:   actual,
			Origin:   template.Origin(),
		}
	}
	values := cell.Values()
	if len(values) != 1 {
		return "", invalid(fmt.Sprintf("%d values", len(values)))
	}
	target, ok := values[0].String()
	if !ok {
		return "", invalid(strconv.Quote(e.valueText(values[0])))
	}
	if target == "" {
		return "", invalid("an empty value")
	}
	return target, nil
}

func (e *evaluator) evaluateTable(node docplan.Content) (Content, error) {
	result, err := e.executeQuery(node)
	if err != nil {
		return Content{}, err
	}
	content := Content{
		kind:        ContentTable,
		name:        node.Name(),
		caption:     node.Caption(),
		groupBy:     node.GroupBy(),
		columns:     result.Columns(),
		rows:        result.Rows(),
		query:       node.Query().Entry(),
		queryOrigin: result.Origin(),
		origin:      node.Origin(),
	}
	if content.groupBy != "" {
		groups, err := e.groupRows(node, content.columns, content.rows)
		if err != nil {
			return Content{}, err
		}
		content.groups = groups
	}
	return content, nil
}

// groupRows partitions a grouped table's rows by the group column's cell
// text, in order of first appearance.
func (e *evaluator) groupRows(node docplan.Content, columns []queryexec.Column, rows []queryexec.Row) ([]TableGroup, error) {
	index := -1
	for i, column := range columns {
		if column.Name() == node.GroupBy() {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, &Error{
			Kind:     ErrorUnknownGroup,
			Document: e.document,
			Content:  node.Name(),
			Query:    node.Query().Entry(),
			Actual:   node.GroupBy(),
			Origin:   node.Origin(),
		}
	}
	var groups []TableGroup
	at := make(map[string]int)
	for _, row := range rows {
		key := ""
		if cells := row.Cells(); index < len(cells) {
			key = e.cellText(cells[index])
		}
		position, ok := at[key]
		if !ok {
			position = len(groups)
			at[key] = position
			groups = append(groups, TableGroup{key: key})
		}
		groups[position].rows = append(groups[position].rows, row)
	}
	return groups, nil
}

// cellText renders one projected cell as deterministic plain text: its
// values joined by ", ".
func (e *evaluator) cellText(cell queryexec.Cell) string {
	values := cell.Values()
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = e.valueText(value)
	}
	return strings.Join(parts, ", ")
}

// evaluatedRun converts one planned inline run into an IR text run.
func evaluatedRun(run docplan.Run) TextRun {
	switch run.Kind() {
	case docplan.RunLink:
		return TextRun{kind: RunLink, text: run.Text(), target: run.Target(), origin: run.Origin()}
	case docplan.RunRef:
		var anchor string
		if path := run.RefPath(); len(path) > 0 {
			anchor = AnchorFor(path)
		}
		return TextRun{kind: RunRef, text: run.Text(), target: anchor, document: run.RefDocument(), origin: run.Origin()}
	default:
		return TextRun{kind: styledKind(run.Style()), text: run.Text(), origin: run.Origin()}
	}
}

func styledKind(style docplan.RunStyle) RunKind {
	switch style {
	case docplan.StyleEmphasis:
		return RunEmphasis
	case docplan.StyleStrong:
		return RunStrong
	case docplan.StyleCode:
		return RunCode
	default:
		return RunPlain
	}
}

func (e *evaluator) evaluateList(node docplan.Content) (Content, error) {
	result, err := e.executeQuery(node)
	if err != nil {
		return Content{}, err
	}
	rows := result.Rows()
	items := make([]ListItem, 0, len(rows))
	if templates := node.ColumnRuns(); len(templates) > 0 {
		at, err := e.templateIndexes(node, result.Columns(), templates)
		if err != nil {
			return Content{}, err
		}
		for number, row := range rows {
			runs, err := e.templateRuns(node, at, templates, row, number+1)
			if err != nil {
				return Content{}, err
			}
			items = append(items, ListItem{runs: runs, element: row.Element(), origin: row.Origin()})
		}
		return Content{
			kind:        ContentList,
			name:        node.Name(),
			style:       ListStyle(node.Style()),
			items:       items,
			query:       node.Query().Entry(),
			queryOrigin: result.Origin(),
			origin:      node.Origin(),
		}, nil
	}
	for _, row := range rows {
		items = append(items, ListItem{runs: e.rowRuns(row), element: row.Element(), origin: row.Origin()})
	}
	return Content{
		kind:        ContentList,
		name:        node.Name(),
		style:       ListStyle(node.Style()),
		items:       items,
		query:       node.Query().Entry(),
		queryOrigin: result.Origin(),
		origin:      node.Origin(),
	}, nil
}

// evaluateDefinitions runs a definitions block's query once and turns each
// row into one entry: its term column names it, its description column
// describes it, and every run keeps the value it came from.
func (e *evaluator) evaluateDefinitions(node docplan.Content) (Content, error) {
	result, err := e.executeQuery(node)
	if err != nil {
		return Content{}, err
	}
	at := make(map[string]int, len(result.Columns()))
	for i, column := range result.Columns() {
		at[column.Name()] = i
	}
	indexOf := func(name string) (int, error) {
		if i, ok := at[name]; ok {
			return i, nil
		}
		return 0, &Error{
			Kind:     ErrorUnknownDefinitionColumn,
			Document: e.document,
			Content:  node.Name(),
			Query:    node.Query().Entry(),
			Column:   name,
			Origin:   node.Origin(),
		}
	}
	termAt, err := indexOf(node.Term())
	if err != nil {
		return Content{}, err
	}
	descriptionAt, err := indexOf(node.Description())
	if err != nil {
		return Content{}, err
	}
	rows := result.Rows()
	entries := make([]Definition, 0, len(rows))
	for _, row := range rows {
		cells := row.Cells()
		entries = append(entries, Definition{
			term:        e.cellRuns(cells, termAt),
			description: e.cellRuns(cells, descriptionAt),
			element:     row.Element(),
			origin:      row.Origin(),
		})
	}
	return Content{
		kind:        ContentDefinitions,
		name:        node.Name(),
		definitions: entries,
		query:       node.Query().Entry(),
		queryOrigin: result.Origin(),
		origin:      node.Origin(),
	}, nil
}

// cellRuns renders one cell of a row as plain runs, one per value.
func (e *evaluator) cellRuns(cells []queryexec.Cell, index int) []TextRun {
	if index >= len(cells) {
		return nil
	}
	var runs []TextRun
	for _, value := range cells[index].Values() {
		runs = append(runs, TextRun{text: e.valueText(value), origin: value.Origin()})
	}
	return runs
}

// evaluateDiagram renders the planned view reference of a diagram through the
// view engine, storing the backend-neutral rendering in the node.
func (e *evaluator) evaluateDiagram(node docplan.Content) (Content, error) {
	reference := node.Diagram()
	if reference == nil {
		return Content{}, &Error{
			Kind:     ErrorInvalidPlan,
			Document: e.document,
			Content:  node.Name(),
			Origin:   node.Origin(),
		}
	}
	renderer := view.NewRenderer(e.context.Model, e.context.Resolver, e.text)
	var rendering *view.Rendering
	var err error
	if declared, ok := reference.View(); ok {
		rendering, err = renderer.Render(declared)
	} else if target, ok := reference.Target(); ok {
		rendering, err = renderer.RenderExposed([]*symbols.Symbol{target}, reference.Kind(), reference.Stated())
	} else {
		return Content{}, &Error{
			Kind:     ErrorInvalidPlan,
			Document: e.document,
			Content:  node.Name(),
			Origin:   node.Origin(),
		}
	}
	if err != nil {
		return Content{}, &Error{
			Kind:     ErrorViewRendering,
			Document: e.document,
			Content:  node.Name(),
			Origin:   reference.Origin(),
			Err:      err,
		}
	}
	return Content{
		kind:      ContentDiagram,
		name:      node.Name(),
		caption:   node.Caption(),
		rendering: rendering,
		direction: reference.Direction(),
		form:      reference.Form(),
		origin:    node.Origin(),
	}, nil
}

// executeQuery runs the planned query of a query-backed node.
func (e *evaluator) executeQuery(node docplan.Content) (*queryexec.RowSet, error) {
	reference := node.Query()
	bindings := make(queryexec.Bindings, len(reference.Bindings()))
	for _, binding := range reference.Bindings() {
		values := binding.Values()
		bound := make([]queryexec.Value, 0, len(values))
		for _, value := range values {
			bound = append(bound, executionValue(value))
		}
		bindings[binding.Parameter()] = bound
	}
	result, err := queryexec.Execute(reference.Program(), e.context, bindings, e.options)
	if err != nil {
		return nil, &Error{
			Kind:     ErrorQueryExecution,
			Document: e.document,
			Content:  node.Name(),
			Query:    reference.Entry(),
			Origin:   reference.Origin(),
			Err:      err,
		}
	}
	return result, nil
}

// executionValue converts one planned binding value into an execution value.
func executionValue(value docplan.BindingValue) queryexec.Value {
	if element, ok := value.Element(); ok {
		return queryexec.ElementValue(element)
	}
	if text, ok := value.String(); ok {
		return queryexec.StringValue(text)
	}
	if integer, ok := value.Integer(); ok {
		return queryexec.IntegerValue(integer)
	}
	if realVal, ok := value.Real(); ok {
		return queryexec.RealValue(realVal)
	}
	if boolean, ok := value.Boolean(); ok {
		return queryexec.BooleanValue(boolean)
	}
	return queryexec.Value{}
}

// rowRuns renders one query row as text runs: one run per projected cell
// value, or the element's name when the row has no projected cells.
func (e *evaluator) rowRuns(row queryexec.Row) []TextRun {
	cells := row.Cells()
	if len(cells) == 0 {
		return []TextRun{{text: e.valueText(row.Element()), origin: row.Origin()}}
	}
	var runs []TextRun
	for _, cell := range cells {
		for _, value := range cell.Values() {
			runs = append(runs, TextRun{text: e.valueText(value), origin: value.Origin()})
		}
	}
	return runs
}

// valueText renders one typed query value as deterministic plain text.
func (e *evaluator) valueText(value queryexec.Value) string {
	if element, ok := value.Element(); ok {
		if name := e.context.Model.EffectiveNameOf(element); name != "" {
			return name
		}
		return symbols.FQNOf(element)
	}
	if text, ok := value.String(); ok {
		return text
	}
	if integer, ok := value.Integer(); ok {
		return strconv.FormatInt(integer, 10)
	}
	if realVal, ok := value.Real(); ok {
		return strconv.FormatFloat(realVal, 'g', -1, 64)
	}
	if boolean, ok := value.Boolean(); ok {
		return strconv.FormatBool(boolean)
	}
	if value.Kind() == queryexec.ValueInfinity {
		return "*"
	}
	if quantity, ok := value.Quantity(); ok {
		magnitude, _ := value.Magnitude()
		return quantity.TextWithMagnitude(e.valueText(magnitude))
	}
	return ""
}
