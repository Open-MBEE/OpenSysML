package docplan

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

const (
	documentBaseFQN    = "DocumentQueries::Document"
	sectionBaseFQN     = "DocumentQueries::Section"
	paragraphBaseFQN   = "DocumentQueries::Paragraph"
	tableBaseFQN       = "DocumentQueries::Table"
	listBaseFQN        = "DocumentQueries::List"
	definitionsBaseFQN = "DocumentQueries::Definitions"
	diagramBaseFQN     = "DocumentQueries::Diagram"
	runBaseFQN         = "DocumentQueries::Run"
	spanBaseFQN        = "DocumentQueries::Span"
	linkBaseFQN        = "DocumentQueries::Link"
	refBaseFQN         = "DocumentQueries::Ref"

	columnRunBaseFQN  = "DocumentQueries::ColumnRun"
	spanColumnBaseFQN = "DocumentQueries::SpanColumn"
	linkColumnBaseFQN = "DocumentQueries::LinkColumn"
)

// diagramSourceName is the reference member a diagram names its view or
// target element through.
const diagramSourceName = "source"

type compiler struct {
	index    *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
	document string
	entry    *symbols.Symbol
	bases    bases
	stack    []string
	targets  map[*symbols.Symbol]refTarget
}

type bases struct {
	document    *symbols.Symbol
	section     *symbols.Symbol
	paragraph   *symbols.Symbol
	table       *symbols.Symbol
	list        *symbols.Symbol
	definitions *symbols.Symbol
	diagram     *symbols.Symbol
	run         *symbols.Symbol
	span        *symbols.Symbol
	link        *symbols.Symbol
	ref         *symbols.Symbol

	columnRun  *symbols.Symbol
	spanColumn *symbols.Symbol
	linkColumn *symbols.Symbol
}

// refTarget is one referenceable target: the document that owns it ("" for
// the planned document), the named path from that document's root (empty for
// a document root), and the label a reference without text falls back to.
type refTarget struct {
	document string
	path     []string
	label    string
}

// IsDocumentDefinition reports whether sym specializes DocumentQueries::Document.
func IsDocumentDefinition(index *symbols.Index, model *semantics.Model, sym *symbols.Symbol) bool {
	base := libraryBase(index, documentBaseFQN)
	return base != nil && sym != nil && sym != base &&
		sym.Kind == symbols.SymbolPartDef && model != nil && model.Conforms(sym, base)
}

// QueryTarget resolves the query definition a calc usage is typed by, following
// redefinition lineage; nil when it is not typed by a query definition.
func QueryTarget(index *symbols.Index, model *semantics.Model, resolver *resolve.Resolver, usage *symbols.Symbol) *symbols.Symbol {
	if index == nil || model == nil || resolver == nil || usage == nil || usage.Kind != symbols.SymbolCalcUsage {
		return nil
	}
	c := &compiler{index: index, model: model, resolver: resolver}
	target := c.typingTarget(usage)
	if target == nil || !queryplan.IsQueryDefinition(index, model, target) {
		return nil
	}
	return target
}

// Compile compiles a document definition into an immutable document plan.
func Compile(index *symbols.Index, model *semantics.Model, resolver *resolve.Resolver, entry *symbols.Symbol) (*Plan, error) {
	if index == nil || model == nil || resolver == nil {
		return nil, &Error{Kind: ErrorInvalidContext}
	}
	all := bases{
		document:    libraryBase(index, documentBaseFQN),
		section:     libraryBase(index, sectionBaseFQN),
		paragraph:   libraryBase(index, paragraphBaseFQN),
		table:       libraryBase(index, tableBaseFQN),
		list:        libraryBase(index, listBaseFQN),
		definitions: libraryBase(index, definitionsBaseFQN),
		diagram:     libraryBase(index, diagramBaseFQN),
		run:         libraryBase(index, runBaseFQN),
		span:        libraryBase(index, spanBaseFQN),
		link:        libraryBase(index, linkBaseFQN),
		ref:         libraryBase(index, refBaseFQN),

		columnRun:  libraryBase(index, columnRunBaseFQN),
		spanColumn: libraryBase(index, spanColumnBaseFQN),
		linkColumn: libraryBase(index, linkColumnBaseFQN),
	}
	if all.document == nil || all.section == nil || all.paragraph == nil ||
		all.table == nil || all.list == nil || all.definitions == nil || all.diagram == nil ||
		all.run == nil || all.span == nil || all.link == nil || all.ref == nil ||
		all.columnRun == nil || all.spanColumn == nil || all.linkColumn == nil {
		return nil, &Error{Kind: ErrorLibraryUnavailable}
	}
	name := symbols.FQNOf(entry)
	if entry == nil || entry == all.document || entry.Kind != symbols.SymbolPartDef || !model.Conforms(entry, all.document) {
		return nil, &Error{Kind: ErrorNotDocumentDefinition, Document: name, Origin: provenance.Symbol(entry)}
	}
	c := &compiler{
		index:    index,
		model:    model,
		resolver: resolver,
		document: name,
		entry:    entry,
		bases:    all,
		targets:  make(map[*symbols.Symbol]refTarget),
	}
	title, err := c.requiredText(entry, "title", ErrorMissingTitle)
	if err != nil {
		return nil, err
	}
	content, err := c.compileMembers(entry)
	if err != nil {
		return nil, err
	}
	if err := c.resolveRefs(content); err != nil {
		return nil, err
	}
	return &Plan{
		compiled: true,
		name:     name,
		title:    title,
		content:  content,
		origin:   provenance.Symbol(entry),
	}, nil
}

func libraryBase(index *symbols.Index, fqn string) *symbols.Symbol {
	if index == nil {
		return nil
	}
	matches := symbols.PreferDeclared(index.LookupQualified(fqn))
	if len(matches) != 1 {
		return nil
	}
	return matches[0]
}

// compileMembers compiles the ordered structural members of a document
// definition or a section usage.
func (c *compiler) compileMembers(owner *symbols.Symbol) ([]Content, error) {
	content := make([]Content, 0)
	for _, member := range c.effectiveMembers(owner) {
		if member.Kind == symbols.SymbolPartUsage && c.isContent(member) {
			c.stack = append(c.stack, c.effectiveName(member))
			node, err := c.compileContent(member)
			if err != nil {
				c.stack = c.stack[:len(c.stack)-1]
				return nil, err
			}
			c.registerTarget(member, refTarget{
				path:  append([]string(nil), c.stack...),
				label: contentLabel(node),
			})
			c.stack = c.stack[:len(c.stack)-1]
			content = append(content, node)
			continue
		}
		if !c.index.Library(member) {
			if err := c.rejectStructural(owner, member); err != nil {
				return nil, err
			}
		}
	}
	return content, nil
}

// registerTarget records a reference target under the content symbol and every
// feature it redefines, transitively, so an inherited reference that resolves
// to a base document's content binds to its effective replacement.
func (c *compiler) registerTarget(member *symbols.Symbol, target refTarget) {
	pending := []*symbols.Symbol{member}
	for i := 0; i < len(pending); i++ {
		sym := pending[i]
		if _, seen := c.targets[sym]; seen {
			continue
		}
		c.targets[sym] = target
		pending = append(pending, c.model.RedefinedFeatures(sym)...)
	}
}

// rejectStructural rejects declarations that cannot appear in a structural
// scope, letting attributes, annotations, and other non-content members
// through.
func (c *compiler) rejectStructural(owner, member *symbols.Symbol) error {
	if member.Kind == symbols.SymbolCalcUsage || member.Kind == symbols.SymbolPartUsage {
		return &Error{
			Kind:     ErrorInvalidContent,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	return nil
}

// isContent reports whether a part usage conforms to a document content kind.
func (c *compiler) isContent(member *symbols.Symbol) bool {
	return c.model.Conforms(member, c.bases.document) ||
		c.model.Conforms(member, c.bases.section) ||
		c.model.Conforms(member, c.bases.paragraph) ||
		c.model.Conforms(member, c.bases.table) ||
		c.model.Conforms(member, c.bases.list) ||
		c.model.Conforms(member, c.bases.definitions) ||
		c.model.Conforms(member, c.bases.diagram)
}

func (c *compiler) compileContent(member *symbols.Symbol) (Content, error) {
	switch {
	case c.model.Conforms(member, c.bases.document):
		return Content{}, &Error{
			Kind:     ErrorNestedDocument,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	case c.model.Conforms(member, c.bases.section):
		return c.compileSection(member)
	case c.model.Conforms(member, c.bases.paragraph):
		return c.compileParagraph(member)
	case c.model.Conforms(member, c.bases.table):
		return c.compileTable(member)
	case c.model.Conforms(member, c.bases.list):
		return c.compileList(member)
	case c.model.Conforms(member, c.bases.definitions):
		return c.compileDefinitions(member)
	case c.model.Conforms(member, c.bases.diagram):
		return c.compileDiagram(member)
	default:
		return Content{}, &Error{
			Kind:     ErrorInvalidContent,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
}

func (c *compiler) compileSection(member *symbols.Symbol) (Content, error) {
	title, err := c.requiredText(member, "title", ErrorMissingTitle)
	if err != nil {
		return Content{}, err
	}
	children, err := c.compileMembers(member)
	if err != nil {
		return Content{}, err
	}
	return Content{
		kind:     ContentSection,
		name:     c.effectiveName(member),
		title:    title,
		children: children,
		origin:   provenance.Symbol(member),
	}, nil
}

func (c *compiler) compileParagraph(member *symbols.Symbol) (Content, error) {
	text, stated, err := c.optionalText(member, "text")
	if err != nil {
		return Content{}, err
	}
	query, err := c.compileQueryRef(member)
	if err != nil {
		return Content{}, err
	}
	runs, err := c.compileRuns(member)
	if err != nil {
		return Content{}, err
	}
	columnRuns, err := c.compileColumnRuns(member, query)
	if err != nil {
		return Content{}, err
	}
	if len(columnRuns) > 0 && query == nil {
		return Content{}, &Error{
			Kind:     ErrorColumnRunWithoutQuery,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	if len(columnRuns) > 0 && (stated || len(runs) > 0) {
		return Content{}, &Error{
			Kind:     ErrorConflictingColumnRuns,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	if len(runs) > 0 && (stated || query != nil) {
		return Content{}, &Error{
			Kind:     ErrorConflictingRuns,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	if stated && query != nil {
		return Content{}, &Error{
			Kind:     ErrorConflictingText,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	if !stated && query == nil && len(runs) == 0 {
		return Content{}, &Error{
			Kind:     ErrorMissingText,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	if err := c.rejectNestedContent(member); err != nil {
		return Content{}, err
	}
	return Content{
		kind:       ContentParagraph,
		name:       c.effectiveName(member),
		text:       text,
		runs:       runs,
		columnRuns: columnRuns,
		query:      query,
		origin:     provenance.Symbol(member),
	}, nil
}

// compileColumnRuns compiles the ordered column runs declared in a
// query-backed paragraph or list.
func (c *compiler) compileColumnRuns(member *symbols.Symbol, query *QueryRef) ([]ColumnRun, error) {
	var runs []ColumnRun
	for _, candidate := range c.effectiveMembers(member) {
		if candidate.Kind != symbols.SymbolPartUsage || !c.isColumnRun(candidate) {
			continue
		}
		run, err := c.compileColumnRun(candidate, query)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// isColumnRun reports whether a part usage conforms to a column run kind.
func (c *compiler) isColumnRun(member *symbols.Symbol) bool {
	return c.model.Conforms(member, c.bases.columnRun)
}

// compileColumnRun compiles one column run: a styled span or link template
// applied to each result row.
func (c *compiler) compileColumnRun(member *symbols.Symbol, query *QueryRef) (ColumnRun, error) {
	if err := c.rejectNestedContent(member); err != nil {
		return ColumnRun{}, err
	}
	if err := c.rejectQuery(member); err != nil {
		return ColumnRun{}, err
	}
	span := c.model.Conforms(member, c.bases.spanColumn)
	link := c.model.Conforms(member, c.bases.linkColumn)
	if span == link {
		kind := ErrorInvalidContent
		if span {
			kind = ErrorAmbiguousRun
		}
		return ColumnRun{}, &Error{
			Kind:     kind,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	column, err := c.requiredColumn(member, "column")
	if err != nil {
		return ColumnRun{}, err
	}
	run := ColumnRun{kind: TemplateSpan, column: column, style: StylePlain, origin: provenance.Symbol(member)}
	if link {
		targetColumn, err := c.requiredColumn(member, "targetColumn")
		if err != nil {
			return ColumnRun{}, err
		}
		run.kind, run.style, run.targetColumn = TemplateLink, "", targetColumn
	} else {
		style, styleStated, err := c.optionalText(member, "style")
		if err != nil {
			return ColumnRun{}, err
		}
		styleColumn, styleColumnStated, err := c.optionalText(member, "styleColumn")
		if err != nil {
			return ColumnRun{}, err
		}
		if styleStated && styleColumnStated {
			return ColumnRun{}, &Error{
				Kind:     ErrorConflictingRunStyle,
				Document: c.document,
				Content:  c.contentName(member),
				Origin:   provenance.Symbol(member),
			}
		}
		if styleStated {
			run.style = RunStyle(style)
			switch run.style {
			case StylePlain, StyleEmphasis, StyleStrong, StyleCode:
			default:
				return ColumnRun{}, &Error{
					Kind:     ErrorInvalidRunStyle,
					Document: c.document,
					Content:  c.contentName(member),
					Actual:   style,
					Origin:   provenance.Symbol(member),
				}
			}
		}
		if styleColumnStated {
			if styleColumn == "" {
				return ColumnRun{}, &Error{
					Kind:      ErrorMissingRunColumn,
					Document:  c.document,
					Content:   c.contentName(member),
					Parameter: "styleColumn",
					Origin:    provenance.Symbol(member),
				}
			}
			run.style, run.styleColumn = "", styleColumn
		}
	}
	if query != nil {
		if err := c.validateRunColumns(member, query, run); err != nil {
			return ColumnRun{}, err
		}
	}
	return run, nil
}

// requiredColumn reads a column run's required column-name attribute.
func (c *compiler) requiredColumn(member *symbols.Symbol, attribute string) (string, error) {
	name, stated, err := c.optionalText(member, attribute)
	if err != nil {
		return "", err
	}
	if !stated || name == "" {
		return "", &Error{
			Kind:      ErrorMissingRunColumn,
			Document:  c.document,
			Content:   c.contentName(member),
			Parameter: attribute,
			Origin:    provenance.Symbol(member),
		}
	}
	return name, nil
}

// validateRunColumns checks a column run's names against the columns its
// query statically projects.
func (c *compiler) validateRunColumns(member *symbols.Symbol, query *QueryRef, run ColumnRun) error {
	columns, known := staticColumns(query.program, query.entry)
	if !known {
		return nil
	}
	for _, name := range []string{run.column, run.styleColumn, run.targetColumn} {
		if name != "" && !containsColumn(columns, name) {
			return &Error{
				Kind:     ErrorUnknownRunColumn,
				Document: c.document,
				Content:  c.contentName(member),
				Query:    query.entry,
				Actual:   name,
				Origin:   provenance.Symbol(member),
			}
		}
	}
	return nil
}

// compileRuns compiles the ordered inline runs declared in a paragraph.
func (c *compiler) compileRuns(member *symbols.Symbol) ([]Run, error) {
	var runs []Run
	for _, candidate := range c.effectiveMembers(member) {
		if candidate.Kind != symbols.SymbolPartUsage || !c.isRun(candidate) {
			continue
		}
		run, err := c.compileRun(candidate)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// isRun reports whether a part usage conforms to an inline run kind.
func (c *compiler) isRun(member *symbols.Symbol) bool {
	return c.model.Conforms(member, c.bases.run)
}

// compileRun compiles one inline run: a styled span, a link, or a reference.
func (c *compiler) compileRun(member *symbols.Symbol) (Run, error) {
	if err := c.rejectNestedContent(member); err != nil {
		return Run{}, err
	}
	if err := c.rejectQuery(member); err != nil {
		return Run{}, err
	}
	span := c.model.Conforms(member, c.bases.span)
	link := c.model.Conforms(member, c.bases.link)
	ref := c.model.Conforms(member, c.bases.ref)
	if (span && link) || (span && ref) || (link && ref) {
		return Run{}, &Error{
			Kind:     ErrorAmbiguousRun,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	switch {
	case span:
		return c.compileSpanRun(member)
	case link:
		return c.compileLinkRun(member)
	case ref:
		return c.compileRefRun(member)
	default:
		return Run{}, &Error{
			Kind:     ErrorInvalidContent,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
}

func (c *compiler) compileSpanRun(member *symbols.Symbol) (Run, error) {
	text, err := c.requiredRunText(member)
	if err != nil {
		return Run{}, err
	}
	style, stated, err := c.optionalText(member, "style")
	if err != nil {
		return Run{}, err
	}
	runStyle := StylePlain
	if stated {
		runStyle = RunStyle(style)
		switch runStyle {
		case StylePlain, StyleEmphasis, StyleStrong, StyleCode:
		default:
			return Run{}, &Error{
				Kind:     ErrorInvalidRunStyle,
				Document: c.document,
				Content:  c.contentName(member),
				Actual:   style,
				Origin:   provenance.Symbol(member),
			}
		}
	}
	return Run{kind: RunSpan, text: text, style: runStyle, origin: provenance.Symbol(member)}, nil
}

func (c *compiler) compileLinkRun(member *symbols.Symbol) (Run, error) {
	text, err := c.requiredRunText(member)
	if err != nil {
		return Run{}, err
	}
	target, stated, err := c.optionalText(member, "target")
	if err != nil {
		return Run{}, err
	}
	if !stated || target == "" {
		return Run{}, &Error{
			Kind:     ErrorMissingLinkTarget,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	return Run{kind: RunLink, text: text, target: target, origin: provenance.Symbol(member)}, nil
}

func (c *compiler) compileRefRun(member *symbols.Symbol) (Run, error) {
	text, _, err := c.optionalText(member, "text")
	if err != nil {
		return Run{}, err
	}
	target, root, err := c.refRunTarget(member)
	if err != nil {
		return Run{}, err
	}
	return Run{kind: RunRef, text: text, refSym: target, refRoot: root, origin: provenance.Symbol(member)}, nil
}

// requiredRunText reads a run's text attribute, which must be a non-empty
// string.
func (c *compiler) requiredRunText(member *symbols.Symbol) (string, error) {
	text, stated, err := c.optionalText(member, "text")
	if err != nil {
		return "", err
	}
	if !stated || text == "" {
		return "", &Error{
			Kind:     ErrorMissingRunText,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	return text, nil
}

// refRunTarget resolves the element a reference run's target names, and the
// usage a dot-notation chain starts from, when the target is one.
func (c *compiler) refRunTarget(member *symbols.Symbol) (*symbols.Symbol, *symbols.Symbol, error) {
	for _, candidate := range c.effectiveMembers(member) {
		if c.effectiveName(candidate) != "target" {
			continue
		}
		resolved, root, stated, err := c.namedTarget(member, candidate, ErrorInvalidRefTarget, ErrorUnknownRefTarget, make(map[*symbols.Symbol]bool))
		if err != nil || stated {
			return resolved, root, err
		}
	}
	return nil, nil, &Error{
		Kind:     ErrorMissingRefTarget,
		Document: c.document,
		Content:  c.contentName(member),
		Origin:   provenance.Symbol(member),
	}
}

// chainRoot resolves the usage a feature chain's innermost operand names.
func (c *compiler) chainRoot(scope *symbols.Scope, chain *ast.FeatureChainExpr) *symbols.Symbol {
	operand := chain.Operand
	for {
		inner, ok := operand.(*ast.FeatureChainExpr)
		if !ok {
			break
		}
		operand = inner.Operand
	}
	if reference, ok := operand.(*ast.FeatureReference); ok {
		operand = reference.Name
	}
	name, ok := operand.(*ast.QualifiedName)
	if !ok {
		return nil
	}
	resolved, ok := c.resolver.ResolveQualified(scope, name)
	if !ok || resolved == nil {
		return nil
	}
	if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
		return canonical
	}
	return resolved
}

// qualifiedRoot returns the deepest document definition a resolved qualified
// target's written prefix names, so an inherited block links to the document
// the author wrote rather than the base declaring it.
func (c *compiler) qualifiedRoot(name *ast.QualifiedName) *symbols.Symbol {
	for i := len(name.Parts) - 2; i >= 0; i-- {
		sym, ok := c.resolver.PartSymbol(name, i)
		if !ok || sym == nil {
			continue
		}
		if canonical, ok := c.resolver.ResolveAliasTarget(sym); ok {
			sym = canonical
		}
		if sym.Kind == symbols.SymbolPartDef && c.model.Conforms(sym, c.bases.document) {
			return sym
		}
	}
	return nil
}

// rejectQuery rejects a query declared in content that renders no query rows:
// an inline run, or a diagram, which renders a view.
func (c *compiler) rejectQuery(member *symbols.Symbol) error {
	for _, candidate := range c.effectiveMembers(member) {
		if candidate.Kind == symbols.SymbolCalcUsage {
			return &Error{
				Kind:     ErrorInvalidContent,
				Document: c.document,
				Content:  c.contentName(candidate),
				Origin:   provenance.Symbol(candidate),
			}
		}
	}
	return nil
}

// contentLabel is what a reference without text renders: the target's title,
// caption, or declared name.
func contentLabel(node Content) string {
	if node.title != "" {
		return node.title
	}
	if node.caption != "" {
		return node.caption
	}
	return node.name
}

// resolveRefs resolves every reference run against the compiled content tree,
// filling its named path and fallback label.
func (c *compiler) resolveRefs(content []Content) error {
	for i := range content {
		for r := range content[i].runs {
			run := &content[i].runs[r]
			if run.kind != RunRef {
				continue
			}
			target, ok := c.targets[run.refSym]
			if !ok {
				cross, err := c.crossDocumentTarget(run)
				if err != nil {
					return err
				}
				target = cross
			}
			for _, segment := range target.path {
				if segment == "" {
					return &Error{
						Kind:     ErrorInvalidRefTarget,
						Document: c.document,
						Content:  c.contentName(run.refSym),
						Actual:   "an anonymous content block",
						Origin:   run.origin,
					}
				}
			}
			run.ref = append([]string(nil), target.path...)
			run.refDocument = target.document
			if run.text == "" {
				run.text = target.label
			}
		}
		if err := c.resolveRefs(content[i].children); err != nil {
			return err
		}
	}
	return nil
}

// crossDocumentTarget resolves a reference whose target lives outside the
// planned document: another document definition's root or one of its named
// content blocks.
func (c *compiler) crossDocumentTarget(run *Run) (refTarget, error) {
	sym := run.refSym
	if root, err := c.documentRootTarget(sym, run); root != nil || err != nil {
		if err != nil {
			return refTarget{}, err
		}
		label, stated, err := c.optionalText(root, "title")
		if err != nil {
			return refTarget{}, err
		}
		if !stated || label == "" {
			label = c.effectiveName(root)
		}
		return refTarget{document: symbols.FQNOf(root), label: label}, nil
	}
	destination, err := c.chainRootDocument(run)
	if err != nil {
		return refTarget{}, err
	}
	var path []string
	node := sym
	for node != nil && node.Kind == symbols.SymbolPartUsage && c.isContent(node) {
		path = append([]string{c.effectiveName(node)}, path...)
		owner := (*symbols.Symbol)(nil)
		if node.OwnerScope != nil {
			owner = node.OwnerScope.Owner()
		}
		if owner != nil && owner != c.entry && owner.Kind == symbols.SymbolPartDef && c.model.Conforms(owner, c.bases.document) {
			label, err := c.crossContentLabel(sym)
			if err != nil {
				return refTarget{}, err
			}
			// A chain through a typed usage targets the usage's document,
			// not the base definition declaring the inherited block.
			if destination == nil {
				destination = owner
			}
			if destination == c.entry {
				return refTarget{path: path, label: label}, nil
			}
			return refTarget{document: symbols.FQNOf(destination), path: path, label: label}, nil
		}
		node = owner
	}
	return refTarget{}, &Error{
		Kind:     ErrorInvalidRefTarget,
		Document: c.document,
		Content:  c.contentName(sym),
		Actual:   c.contentName(sym),
		Origin:   run.origin,
	}
}

// documentRootTarget returns the document definition a reference names as a
// whole: the definition itself, or the single document definition typing a
// referenced document usage. A usage typed by more than one document
// definition is an ambiguous target.
func (c *compiler) documentRootTarget(sym *symbols.Symbol, run *Run) (*symbols.Symbol, error) {
	if sym == nil || sym == c.entry {
		return nil, nil
	}
	if sym.Kind == symbols.SymbolPartDef {
		if c.model.Conforms(sym, c.bases.document) {
			return sym, nil
		}
		return nil, nil
	}
	if _, usage := sym.Decl.(*ast.Usage); !usage || c.contentOwner(sym) != nil {
		return nil, nil
	}
	defs := c.documentTypesOf(sym)
	switch len(defs) {
	case 0:
		return nil, nil
	case 1:
		if defs[0] == c.entry {
			return nil, nil
		}
		return defs[0], nil
	default:
		names := make([]string, len(defs))
		for i, def := range defs {
			names[i] = symbols.FQNOf(def)
		}
		return nil, &Error{
			Kind:     ErrorAmbiguousRefTarget,
			Document: c.document,
			Content:  c.contentName(sym),
			Actual:   strings.Join(names, " and "),
			Origin:   run.origin,
		}
	}
}

// chainRootDocument returns the document definition typing the usage a
// reference's dot-notation chain starts from, when there is exactly one; a
// root typed by more than one document is an ambiguous target.
func (c *compiler) chainRootDocument(run *Run) (*symbols.Symbol, error) {
	root := run.refRoot
	if root == nil {
		return nil, nil
	}
	if root.Kind == symbols.SymbolPartDef {
		if c.model.Conforms(root, c.bases.document) {
			return root, nil
		}
		return nil, nil
	}
	defs := c.documentTypesOf(root)
	switch len(defs) {
	case 0:
		return nil, nil
	case 1:
		return defs[0], nil
	default:
		names := make([]string, len(defs))
		for i, def := range defs {
			names[i] = symbols.FQNOf(def)
		}
		return nil, &Error{
			Kind:     ErrorAmbiguousRefTarget,
			Document: c.document,
			Content:  c.contentName(root),
			Actual:   strings.Join(names, " and "),
			Origin:   run.origin,
		}
	}
}

// contentOwner returns the document definition a content block belongs to,
// walking the block's owners; nil when sym is not owned by one.
func (c *compiler) contentOwner(sym *symbols.Symbol) *symbols.Symbol {
	node := sym
	for node != nil && node.OwnerScope != nil {
		owner := node.OwnerScope.Owner()
		if owner != nil && owner.Kind == symbols.SymbolPartDef && c.model.Conforms(owner, c.bases.document) {
			return owner
		}
		node = owner
	}
	return nil
}

// documentTypesOf returns the document definitions a usage has as types,
// stated on the usage itself or inherited through its redefinition lineage
// when the usage states none.
func (c *compiler) documentTypesOf(sym *symbols.Symbol) []*symbols.Symbol {
	return c.documentTypes(sym, make(map[*symbols.Symbol]bool))
}

func (c *compiler) documentTypes(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) []*symbols.Symbol {
	if sym == nil || seen[sym] {
		return nil
	}
	seen[sym] = true
	var defs []*symbols.Symbol
	stated := false
	for _, relationship := range semantics.RelationshipsOf(sym) {
		if relationship == nil || relationship.Kind != ast.RelTyping || relationship.Target == nil {
			continue
		}
		stated = true
		target := relationship.Target
		if reference, ok := target.(*ast.FeatureReference); ok {
			target = reference.Name
		}
		name, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		resolved, ok := c.resolver.ResolveQualified(sym.OwnerScope, name)
		if !ok || resolved == nil {
			continue
		}
		if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
			resolved = canonical
		}
		if resolved.Kind == symbols.SymbolPartDef && c.model.Conforms(resolved, c.bases.document) {
			defs = appendDocumentType(defs, resolved)
		}
	}
	if stated {
		return defs
	}
	for _, target := range c.model.RedefinedFeatures(sym) {
		for _, def := range c.documentTypes(target, seen) {
			defs = appendDocumentType(defs, def)
		}
	}
	return defs
}

// appendDocumentType adds a document definition to defs unless it holds it.
func appendDocumentType(defs []*symbols.Symbol, def *symbols.Symbol) []*symbols.Symbol {
	for _, held := range defs {
		if held == def {
			return defs
		}
	}
	return append(defs, def)
}

// crossContentLabel derives the default reference label of another document's
// content block: its title, caption, or effective name.
func (c *compiler) crossContentLabel(sym *symbols.Symbol) (string, error) {
	for _, attr := range []string{"title", "caption"} {
		label, stated, err := c.optionalText(sym, attr)
		if err != nil {
			return "", err
		}
		if stated && label != "" {
			return label, nil
		}
	}
	return c.effectiveName(sym), nil
}

func (c *compiler) compileTable(member *symbols.Symbol) (Content, error) {
	caption, _, err := c.optionalText(member, "caption")
	if err != nil {
		return Content{}, err
	}
	groupBy, groupStated, err := c.optionalText(member, "groupBy")
	if err != nil {
		return Content{}, err
	}
	query, err := c.requiredQueryRef(member)
	if err != nil {
		return Content{}, err
	}
	if groupStated {
		if columns, known := staticColumns(query.program, query.entry); groupBy == "" ||
			(known && !containsColumn(columns, groupBy)) {
			return Content{}, &Error{
				Kind:     ErrorUnknownGroupColumn,
				Document: c.document,
				Content:  c.contentName(member),
				Query:    query.entry,
				Actual:   groupBy,
				Origin:   provenance.Symbol(member),
			}
		}
	}
	if err := c.rejectNestedContent(member); err != nil {
		return Content{}, err
	}
	return Content{
		kind:    ContentTable,
		name:    c.effectiveName(member),
		caption: caption,
		groupBy: groupBy,
		query:   query,
		origin:  provenance.Symbol(member),
	}, nil
}

// staticColumns resolves the columns a compiled query statically projects,
// mirroring how execution carries cells: projection sets them, filters and
// ordering preserve them, traversals drop them. known is false when the
// projected properties are parameter-driven.
func staticColumns(program *queryplan.Program, entry string) ([]string, bool) {
	for _, definition := range program.Definitions() {
		if definition.Name() == entry {
			return expressionColumns(program, definition.Expression())
		}
	}
	return nil, true
}

func expressionColumns(program *queryplan.Program, expression queryplan.Expression) ([]string, bool) {
	switch expression.Operation() {
	case queryplan.OperationProject:
		var names []string
		known := false
		for _, argument := range expression.Arguments() {
			switch argument.Name {
			case "properties":
				properties, ok := literalStrings(argument.Value)
				if !ok {
					return nil, false
				}
				names = append(names, properties...)
				known = true
			case "columns":
				names = append(names, columnNames(argument.Value)...)
				known = true
			}
		}
		return names, known
	case queryplan.OperationInvoke:
		return staticColumns(program, expression.Target())
	case queryplan.OperationWhereType,
		queryplan.OperationWhereMetadata,
		queryplan.OperationWhereName,
		queryplan.OperationWhereFeature,
		queryplan.OperationOrderBy:
		for _, argument := range expression.Arguments() {
			if argument.Name == "source" {
				return expressionColumns(program, argument.Value)
			}
		}
		return nil, true
	default:
		return nil, true
	}
}

// literalStrings extracts the string literals of a planned argument value;
// ok is false when any element is not a string literal.
func literalStrings(value queryplan.Expression) ([]string, bool) {
	if value.Operation() == queryplan.OperationSequence {
		var out []string
		for _, element := range value.Arguments() {
			strings, ok := literalStrings(element.Value)
			if !ok {
				return nil, false
			}
			out = append(out, strings...)
		}
		return out, true
	}
	if value.Operation() == queryplan.OperationLiteral {
		if kind, raw := value.Literal(); kind == queryplan.LiteralString {
			text, err := strconv.Unquote(raw)
			if err != nil {
				return nil, false
			}
			return []string{text}, true
		}
	}
	return nil, false
}

// columnNames extracts the explicit names of a projection's planned
// computed columns.
func columnNames(value queryplan.Expression) []string {
	if value.Operation() == queryplan.OperationSequence {
		var out []string
		for _, element := range value.Arguments() {
			out = append(out, columnNames(element.Value)...)
		}
		return out
	}
	if value.Operation() == queryplan.OperationColumn {
		return []string{value.Target()}
	}
	return nil
}

func containsColumn(columns []string, name string) bool {
	for _, column := range columns {
		if column == name {
			return true
		}
	}
	return false
}

func (c *compiler) compileList(member *symbols.Symbol) (Content, error) {
	style, stated, err := c.optionalText(member, "style")
	if err != nil {
		return Content{}, err
	}
	listStyle := ListBullet
	if stated {
		listStyle = ListStyle(style)
		if listStyle != ListBullet && listStyle != ListNumber {
			return Content{}, &Error{
				Kind:     ErrorInvalidStyle,
				Document: c.document,
				Content:  c.contentName(member),
				Actual:   style,
				Origin:   provenance.Symbol(member),
			}
		}
	}
	query, err := c.requiredQueryRef(member)
	if err != nil {
		return Content{}, err
	}
	columnRuns, err := c.compileColumnRuns(member, query)
	if err != nil {
		return Content{}, err
	}
	if err := c.rejectNestedContent(member); err != nil {
		return Content{}, err
	}
	return Content{
		kind:       ContentList,
		name:       c.effectiveName(member),
		style:      listStyle,
		columnRuns: columnRuns,
		query:      query,
		origin:     provenance.Symbol(member),
	}, nil
}

// compileDefinitions compiles a definitions block: the query it renders one
// entry per row of, and the projected columns naming and describing entries.
func (c *compiler) compileDefinitions(member *symbols.Symbol) (Content, error) {
	term, err := c.definitionColumn(member, "term")
	if err != nil {
		return Content{}, err
	}
	description, err := c.definitionColumn(member, "description")
	if err != nil {
		return Content{}, err
	}
	query, err := c.requiredQueryRef(member)
	if err != nil {
		return Content{}, err
	}
	if columns, known := staticColumns(query.program, query.entry); known {
		for _, column := range []struct{ attribute, name string }{{"term", term}, {"description", description}} {
			if !containsColumn(columns, column.name) {
				return Content{}, &Error{
					Kind:      ErrorUnknownDefinitionColumn,
					Document:  c.document,
					Content:   c.contentName(member),
					Query:     query.entry,
					Parameter: column.attribute,
					Actual:    column.name,
					Origin:    provenance.Symbol(member),
				}
			}
		}
	}
	if err := c.rejectNestedContent(member); err != nil {
		return Content{}, err
	}
	return Content{
		kind:        ContentDefinitions,
		name:        c.effectiveName(member),
		term:        term,
		description: description,
		query:       query,
		origin:      provenance.Symbol(member),
	}, nil
}

// definitionColumn reads a definitions block's required column-name attribute.
func (c *compiler) definitionColumn(member *symbols.Symbol, attribute string) (string, error) {
	name, stated, err := c.optionalText(member, attribute)
	if err != nil {
		return "", err
	}
	if !stated || name == "" {
		return "", &Error{
			Kind:      ErrorMissingDefinitionColumn,
			Document:  c.document,
			Content:   c.contentName(member),
			Parameter: attribute,
			Origin:    provenance.Symbol(member),
		}
	}
	return name, nil
}

// compileDiagram compiles a diagram content block: the view or element its
// source names, the resolved rendering kind, and the stated presentation.
func (c *compiler) compileDiagram(member *symbols.Symbol) (Content, error) {
	caption, _, err := c.optionalText(member, "caption")
	if err != nil {
		return Content{}, err
	}
	kindText, kindStated, err := c.optionalText(member, "kind")
	if err != nil {
		return Content{}, err
	}
	directionText, directionStated, err := c.optionalText(member, "direction")
	if err != nil {
		return Content{}, err
	}
	source, err := c.diagramSource(member)
	if err != nil {
		return Content{}, err
	}
	reference := &DiagramRef{origin: provenance.Symbol(member)}
	if semantics.IsView(source) {
		if kindStated {
			return Content{}, &Error{
				Kind:     ErrorConflictingKind,
				Document: c.document,
				Content:  c.contentName(member),
				Actual:   kindText,
				Origin:   provenance.Symbol(member),
			}
		}
		renderer := view.NewRenderer(c.model, c.resolver, nil)
		kind, stated, err := renderer.KindOf(source)
		if err != nil {
			if errors.Is(err, view.ErrUnsupportedKind) {
				return Content{}, &Error{
					Kind:     ErrorUnsupportedKind,
					Document: c.document,
					Content:  c.contentName(member),
					Origin:   provenance.Symbol(member),
					Err:      err,
				}
			}
			return Content{}, &Error{
				Kind:     ErrorInvalidViewSource,
				Document: c.document,
				Content:  c.contentName(member),
				Origin:   provenance.Symbol(member),
				Err:      err,
			}
		}
		reference.view, reference.kind, reference.stated = source, kind, stated
	} else {
		if !kindStated {
			return Content{}, &Error{
				Kind:     ErrorMissingDiagramKind,
				Document: c.document,
				Content:  c.contentName(member),
				Origin:   provenance.Symbol(member),
			}
		}
		kind, ok := view.PseudoViewKind(kindText)
		if !ok {
			return Content{}, &Error{
				Kind:     ErrorUnsupportedKind,
				Document: c.document,
				Content:  c.contentName(member),
				Actual:   kindText,
				Origin:   provenance.Symbol(member),
			}
		}
		reference.target, reference.kind = source, kind
		reference.stated = fmt.Sprintf("the diagram states kind %q", kindText)
	}
	if directionStated {
		direction, ok := view.ParseDirection(directionText)
		if !ok {
			return Content{}, &Error{
				Kind:     ErrorInvalidDirection,
				Document: c.document,
				Content:  c.contentName(member),
				Actual:   directionText,
				Origin:   provenance.Symbol(member),
			}
		}
		if !reference.kind.SupportsDirection() {
			return Content{}, &Error{
				Kind:     ErrorUnsupportedDirection,
				Document: c.document,
				Content:  c.contentName(member),
				Expected: string(reference.kind),
				Actual:   directionText,
				Origin:   provenance.Symbol(member),
			}
		}
		reference.direction = direction
	}
	if err := c.rejectQuery(member); err != nil {
		return Content{}, err
	}
	if err := c.rejectNestedContent(member); err != nil {
		return Content{}, err
	}
	return Content{
		kind:    ContentDiagram,
		name:    c.effectiveName(member),
		caption: caption,
		diagram: reference,
		origin:  provenance.Symbol(member),
	}, nil
}

// diagramSource resolves the element the diagram's source reference names.
func (c *compiler) diagramSource(member *symbols.Symbol) (*symbols.Symbol, error) {
	for _, candidate := range c.effectiveMembers(member) {
		if c.effectiveName(candidate) != diagramSourceName {
			continue
		}
		resolved, stated, err := c.sourceTarget(member, candidate, make(map[*symbols.Symbol]bool))
		if err != nil || stated {
			return resolved, err
		}
	}
	return nil, &Error{
		Kind:     ErrorMissingViewSource,
		Document: c.document,
		Content:  c.contentName(member),
		Origin:   provenance.Symbol(member),
	}
}

// sourceTarget resolves a source declaration's named value, following
// redefinition lineage when the declaration itself is unvalued.
func (c *compiler) sourceTarget(
	member *symbols.Symbol,
	candidate *symbols.Symbol,
	seen map[*symbols.Symbol]bool,
) (*symbols.Symbol, bool, error) {
	resolved, _, stated, err := c.namedTarget(member, candidate, ErrorInvalidViewSource, ErrorUnknownViewSource, seen)
	return resolved, stated, err
}

// namedTarget resolves a reference declaration's named value, following
// redefinition lineage when the declaration itself is unvalued. The second
// symbol is the usage a dot-notation chain starts from, nil otherwise.
func (c *compiler) namedTarget(
	member *symbols.Symbol,
	candidate *symbols.Symbol,
	invalid ErrorKind,
	unknown ErrorKind,
	seen map[*symbols.Symbol]bool,
) (*symbols.Symbol, *symbols.Symbol, bool, error) {
	if candidate == nil || seen[candidate] {
		return nil, nil, false, nil
	}
	seen[candidate] = true
	if declaration, ok := candidate.Decl.(*ast.Usage); ok && declaration.Value != nil {
		target := declaration.Value
		if reference, ok := target.(*ast.FeatureReference); ok {
			target = reference.Name
		}
		if chain, ok := target.(*ast.FeatureChainExpr); ok {
			resolved, ok := c.resolver.ResolveTarget(candidate.OwnerScope, chain)
			if !ok || resolved == nil {
				return nil, nil, false, &Error{
					Kind:     unknown,
					Document: c.document,
					Content:  c.contentName(member),
					Actual:   chainText(chain),
					Origin:   provenance.Node(candidate.DocName, declaration.Value),
				}
			}
			return resolved, c.chainRoot(candidate.OwnerScope, chain), true, nil
		}
		name, ok := target.(*ast.QualifiedName)
		if !ok {
			return nil, nil, false, &Error{
				Kind:     invalid,
				Document: c.document,
				Content:  c.contentName(member),
				Origin:   provenance.Node(candidate.DocName, declaration.Value),
			}
		}
		resolved, ok := c.resolver.ResolveQualified(candidate.OwnerScope, name)
		if !ok || resolved == nil {
			return nil, nil, false, &Error{
				Kind:     unknown,
				Document: c.document,
				Content:  c.contentName(member),
				Actual:   qualifiedNameText(name),
				Origin:   provenance.Node(candidate.DocName, declaration.Value),
			}
		}
		if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
			resolved = canonical
		}
		return resolved, c.qualifiedRoot(name), true, nil
	}
	for _, target := range c.model.RedefinedFeatures(candidate) {
		resolved, root, stated, err := c.namedTarget(member, target, invalid, unknown, seen)
		if err != nil || stated {
			return resolved, root, stated, err
		}
	}
	return nil, nil, false, nil
}

// chainText renders a feature chain the way it was written, for diagnostics.
func chainText(chain *ast.FeatureChainExpr) string {
	operand := ""
	switch node := chain.Operand.(type) {
	case *ast.FeatureChainExpr:
		operand = chainText(node)
	case *ast.FeatureReference:
		if node.Name != nil {
			operand = qualifiedNameText(node.Name)
		}
	case *ast.QualifiedName:
		operand = qualifiedNameText(node)
	}
	if chain.Member == nil {
		return operand
	}
	return operand + "." + qualifiedNameText(chain.Member)
}

func qualifiedNameText(name *ast.QualifiedName) string {
	parts := make([]string, 0, len(name.Parts))
	for _, part := range name.Parts {
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "::")
}

// rejectNestedContent rejects content blocks nested inside a content block,
// inline runs nested anywhere but a paragraph, and column runs nested
// anywhere but a paragraph or list.
func (c *compiler) rejectNestedContent(owner *symbols.Symbol) error {
	allowRuns := c.model.Conforms(owner, c.bases.paragraph)
	allowColumnRuns := allowRuns || c.model.Conforms(owner, c.bases.list)
	for _, member := range c.effectiveMembers(owner) {
		if member.Kind != symbols.SymbolPartUsage {
			continue
		}
		if c.isContent(member) || (!allowRuns && c.isRun(member)) ||
			(!allowColumnRuns && c.isColumnRun(member)) {
			return &Error{
				Kind:     ErrorInvalidContent,
				Document: c.document,
				Content:  c.contentName(member),
				Origin:   provenance.Symbol(member),
			}
		}
	}
	return nil
}

// requiredQueryRef compiles the single query reference a table, list or
// definitions block needs.
func (c *compiler) requiredQueryRef(member *symbols.Symbol) (*QueryRef, error) {
	query, err := c.compileQueryRef(member)
	if err != nil {
		return nil, err
	}
	if query == nil {
		return nil, &Error{
			Kind:     ErrorMissingQuery,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	return query, nil
}

// compileQueryRef compiles the calc usage inside a content block into a
// planned query reference, or nil when the block declares none.
func (c *compiler) compileQueryRef(owner *symbols.Symbol) (*QueryRef, error) {
	var usage *symbols.Symbol
	for _, member := range c.effectiveMembers(owner) {
		if member.Kind != symbols.SymbolCalcUsage {
			continue
		}
		if usage != nil {
			return nil, &Error{
				Kind:     ErrorConflictingQuery,
				Document: c.document,
				Content:  c.contentName(owner),
				Origin:   provenance.Symbol(member),
			}
		}
		usage = member
	}
	if usage == nil {
		return nil, nil
	}
	target := c.typingTarget(usage)
	if target == nil || !queryplan.IsQueryDefinition(c.index, c.model, target) {
		return nil, &Error{
			Kind:     ErrorUnknownQuery,
			Document: c.document,
			Content:  c.contentName(owner),
			Query:    symbols.FQNOf(target),
			Origin:   provenance.Symbol(usage),
		}
	}
	entry := symbols.FQNOf(target)
	program, err := queryplan.Compile(c.index, c.model, c.resolver, target)
	if err != nil {
		return nil, &Error{
			Kind:     ErrorQueryPlanning,
			Document: c.document,
			Content:  c.contentName(owner),
			Query:    entry,
			Origin:   provenance.Symbol(usage),
			Err:      err,
		}
	}
	bindings, err := c.compileBindings(owner, usage, entry, program)
	if err != nil {
		return nil, err
	}
	return &QueryRef{
		entry:    entry,
		program:  program,
		bindings: bindings,
		origin:   provenance.Symbol(usage),
	}, nil
}

// typingTarget resolves the declared type of a usage, following redefinition
// lineage when the declaration omits an explicit type.
func (c *compiler) typingTarget(sym *symbols.Symbol) *symbols.Symbol {
	return c.typingTargetSeen(sym, make(map[*symbols.Symbol]bool))
}

func (c *compiler) typingTargetSeen(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) *symbols.Symbol {
	if sym == nil || seen[sym] {
		return nil
	}
	seen[sym] = true
	declared := false
	for _, relationship := range semantics.RelationshipsOf(sym) {
		if relationship == nil || relationship.Kind != ast.RelTyping || relationship.Target == nil {
			continue
		}
		declared = true
		target := relationship.Target
		if reference, ok := target.(*ast.FeatureReference); ok {
			target = reference.Name
		}
		name, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		if resolved, ok := c.resolver.ResolveQualified(sym.OwnerScope, name); ok {
			if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
				return canonical
			}
		}
	}
	if declared {
		return nil // an explicit type that fails to resolve must not inherit one
	}
	for _, target := range c.model.RedefinedFeatures(sym) {
		if resolved := c.typingTargetSeen(target, seen); resolved != nil {
			return resolved
		}
	}
	return nil
}

// compileBindings compiles the `in` members of a query usage against the
// compiled signature of the entry query.
func (c *compiler) compileBindings(
	content *symbols.Symbol,
	usage *symbols.Symbol,
	entry string,
	program *queryplan.Program,
) ([]Binding, error) {
	parameters := entryParameters(program, entry)
	known := make(map[string]queryplan.Parameter, len(parameters))
	for _, parameter := range parameters {
		known[parameter.Name] = parameter
	}
	bindings := make([]Binding, 0)
	bound := make(map[string]bool)
	for _, member := range localMembers(usage) {
		declaration, ok := member.Decl.(*ast.Usage)
		if !ok || declaration.Direction != ast.DirIn {
			continue
		}
		name := c.effectiveName(member)
		if _, ok := known[name]; !ok {
			return nil, &Error{
				Kind:      ErrorUnknownParameter,
				Document:  c.document,
				Content:   c.contentName(content),
				Query:     entry,
				Parameter: name,
				Origin:    provenance.Symbol(member),
			}
		}
		if bound[name] {
			return nil, &Error{
				Kind:      ErrorDuplicateBinding,
				Document:  c.document,
				Content:   c.contentName(content),
				Query:     entry,
				Parameter: name,
				Origin:    provenance.Symbol(member),
			}
		}
		bound[name] = true
		values, err := c.bindingValues(content, member, entry, name, declaration.Value)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, Binding{
			parameter: name,
			values:    values,
			origin:    provenance.Symbol(member),
		})
	}
	for _, parameter := range parameters {
		if bound[parameter.Name] {
			binding := bindingFor(bindings, parameter.Name)
			if err := c.validateBinding(content, entry, parameter, binding); err != nil {
				return nil, err
			}
			continue
		}
		// An unbound defaulted parameter is evaluated by the query executor.
		if parameter.HasDefault || (parameter.Multiplicity.Known && parameter.Multiplicity.Lower == 0) {
			continue
		}
		return nil, &Error{
			Kind:      ErrorMissingBinding,
			Document:  c.document,
			Content:   c.contentName(content),
			Query:     entry,
			Parameter: parameter.Name,
			Origin:    provenance.Symbol(usage),
		}
	}
	return bindings, nil
}

func entryParameters(program *queryplan.Program, entry string) []queryplan.Parameter {
	for _, definition := range program.Definitions() {
		if definition.Name() == entry {
			return definition.Parameters()
		}
	}
	return nil
}

func bindingFor(bindings []Binding, name string) Binding {
	for _, binding := range bindings {
		if binding.parameter == name {
			return binding
		}
	}
	return Binding{}
}

// bindingValues compiles one binding expression into planned values. A KerML
// sequence is flat, so `null` contributes nothing and a nested sequence its own
// values, in written order.
func (c *compiler) bindingValues(
	content *symbols.Symbol,
	member *symbols.Symbol,
	entry string,
	parameter string,
	node ast.Node,
) ([]BindingValue, error) {
	if node == nil {
		return nil, c.unsupportedBinding(content, member, entry, parameter)
	}
	switch expression := node.(type) {
	case *ast.NullExpr:
		return nil, nil
	case *ast.SequenceExpr:
		values := make([]BindingValue, 0, len(expression.Elements))
		for _, element := range expression.Elements {
			elementValues, err := c.bindingValues(content, member, entry, parameter, element)
			if err != nil {
				return nil, err
			}
			values = append(values, elementValues...)
		}
		return values, nil
	}
	value, err := c.bindingValue(content, member, entry, parameter, node)
	if err != nil {
		return nil, err
	}
	return []BindingValue{value}, nil
}

func (c *compiler) bindingValue(
	content *symbols.Symbol,
	member *symbols.Symbol,
	entry string,
	parameter string,
	node ast.Node,
) (BindingValue, error) {
	origin := provenance.Node(member.DocName, node)
	switch expression := node.(type) {
	case *ast.FeatureReference:
		return c.elementBinding(content, member, entry, parameter, expression.Name, origin)
	case *ast.QualifiedName:
		return c.elementBinding(content, member, entry, parameter, expression, origin)
	case *ast.LiteralString:
		text, err := strconv.Unquote(expression.Value)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingString, text: text, origin: origin}, nil
	case *ast.LiteralInteger:
		integer, err := strconv.ParseInt(expression.Value, 10, 64)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingInteger, integer: integer, origin: origin}, nil
	case *ast.LiteralReal:
		realVal, err := strconv.ParseFloat(expression.Value, 64)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingReal, real: realVal, origin: origin}, nil
	case *ast.LiteralBool:
		return BindingValue{kind: BindingBoolean, boolean: expression.Value, origin: origin}, nil
	case *ast.OperatorExpr:
		return c.signedBinding(content, member, entry, parameter, expression, origin)
	default:
		return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
	}
}

// signedBinding compiles a unary-signed numeric literal binding.
func (c *compiler) signedBinding(
	content *symbols.Symbol,
	member *symbols.Symbol,
	entry string,
	parameter string,
	expression *ast.OperatorExpr,
	origin provenance.Origin,
) (BindingValue, error) {
	if (expression.Operator != ast.OpNeg && expression.Operator != ast.OpPos) || len(expression.Operands) != 1 {
		return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
	}
	sign := ""
	if expression.Operator == ast.OpNeg {
		sign = "-"
	}
	switch operand := expression.Operands[0].(type) {
	case *ast.LiteralInteger:
		integer, err := strconv.ParseInt(sign+operand.Value, 10, 64)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingInteger, integer: integer, origin: origin}, nil
	case *ast.LiteralReal:
		realVal, err := strconv.ParseFloat(sign+operand.Value, 64)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingReal, real: realVal, origin: origin}, nil
	default:
		return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
	}
}

func (c *compiler) elementBinding(
	content *symbols.Symbol,
	member *symbols.Symbol,
	entry string,
	parameter string,
	name *ast.QualifiedName,
	origin provenance.Origin,
) (BindingValue, error) {
	resolved, ok := c.resolver.ResolveQualified(member.OwnerScope, name)
	if !ok || resolved == nil {
		return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
	}
	if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
		resolved = canonical
	}
	return BindingValue{kind: BindingElement, element: resolved, origin: origin}, nil
}

func (c *compiler) unsupportedBinding(content, member *symbols.Symbol, entry, parameter string) error {
	return &Error{
		Kind:      ErrorUnsupportedBinding,
		Document:  c.document,
		Content:   c.contentName(content),
		Query:     entry,
		Parameter: parameter,
		Origin:    provenance.Symbol(member),
	}
}

// validateBinding checks one bound parameter against its compiled signature.
func (c *compiler) validateBinding(
	content *symbols.Symbol,
	entry string,
	parameter queryplan.Parameter,
	binding Binding,
) error {
	if !withinMultiplicity(int64(len(binding.values)), parameter.Multiplicity) {
		return &Error{
			Kind:      ErrorBindingMultiplicity,
			Document:  c.document,
			Content:   c.contentName(content),
			Query:     entry,
			Parameter: parameter.Name,
			Expected:  multiplicityString(parameter.Multiplicity),
			Actual:    strconv.Itoa(len(binding.values)),
			Origin:    binding.origin,
		}
	}
	for _, value := range binding.values {
		if !c.valueConforms(value, parameter.Type) {
			actual := string(value.Kind())
			if element, ok := value.Element(); ok {
				actual = symbols.FQNOf(element)
			}
			return &Error{
				Kind:      ErrorBindingType,
				Document:  c.document,
				Content:   c.contentName(content),
				Query:     entry,
				Parameter: parameter.Name,
				Expected:  parameter.Type,
				Actual:    actual,
				Origin:    value.Origin(),
			}
		}
	}
	return nil
}

func withinMultiplicity(count int64, multiplicity queryplan.Multiplicity) bool {
	if !multiplicity.Known {
		return true
	}
	if count < multiplicity.Lower {
		return false
	}
	return multiplicity.UpperInfinite || count <= multiplicity.Upper
}

func multiplicityString(multiplicity queryplan.Multiplicity) string {
	if !multiplicity.Known {
		return "unknown"
	}
	upper := strconv.FormatInt(multiplicity.Upper, 10)
	if multiplicity.UpperInfinite {
		upper = "*"
	}
	return strconv.FormatInt(multiplicity.Lower, 10) + ".." + upper
}

// valueConforms mirrors execution-time binding conformance for planned values.
func (c *compiler) valueConforms(value BindingValue, expected string) bool {
	if element, ok := value.Element(); ok {
		for _, target := range c.index.LookupQualified(expected) {
			if symbols.SameElement(element, target) || c.model.Conforms(element, target) {
				return true
			}
		}
		return expected == "Element" || expected == "KerML::Root::Element"
	}
	actual, ok := scalarBindingType(value)
	if !ok {
		return false
	}
	for _, target := range c.index.LookupQualified(expected) {
		expectedType := c.model.PrimTypeOf(target)
		if expectedType != semantics.PrimUnknown && semantics.PrimConforms(actual, expectedType) {
			return true
		}
	}
	return false
}

func scalarBindingType(value BindingValue) (semantics.PrimType, bool) {
	switch value.Kind() {
	case BindingBoolean:
		return semantics.PrimBoolean, true
	case BindingString:
		return semantics.PrimString, true
	case BindingInteger:
		return semantics.PrimInteger, true
	case BindingReal:
		return semantics.PrimReal, true
	default:
		return semantics.PrimUnknown, false
	}
}

// requiredText reads a required string attribute of one structural member.
func (c *compiler) requiredText(member *symbols.Symbol, attribute string, missing ErrorKind) (string, error) {
	text, stated, err := c.optionalText(member, attribute)
	if err != nil {
		return "", err
	}
	if !stated {
		return "", &Error{
			Kind:     missing,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	return text, nil
}

// optionalText reads an optional string attribute of one structural member.
func (c *compiler) optionalText(member *symbols.Symbol, attribute string) (string, bool, error) {
	for _, candidate := range c.effectiveMembers(member) {
		if candidate.Kind != symbols.SymbolAttributeUsage || c.effectiveName(candidate) != attribute {
			continue
		}
		text, stated, err := c.attributeText(member, candidate, attribute, make(map[*symbols.Symbol]bool))
		if err != nil || stated {
			return text, stated, err
		}
	}
	return "", false, nil
}

// attributeText reads a declaration's string value, following redefinition
// lineage when the declaration itself is unvalued.
func (c *compiler) attributeText(
	member *symbols.Symbol,
	candidate *symbols.Symbol,
	attribute string,
	seen map[*symbols.Symbol]bool,
) (string, bool, error) {
	if candidate == nil || seen[candidate] {
		return "", false, nil
	}
	seen[candidate] = true
	if declaration, ok := candidate.Decl.(*ast.Usage); ok && declaration.Value != nil {
		literal, ok := declaration.Value.(*ast.LiteralString)
		if !ok {
			return "", false, c.invalidAttribute(member, candidate, attribute)
		}
		text, err := strconv.Unquote(literal.Value)
		if err != nil {
			return "", false, c.invalidAttribute(member, candidate, attribute)
		}
		return text, true, nil
	}
	for _, target := range c.model.RedefinedFeatures(candidate) {
		text, stated, err := c.attributeText(member, target, attribute, seen)
		if err != nil || stated {
			return text, stated, err
		}
	}
	return "", false, nil
}

func (c *compiler) invalidAttribute(member, candidate *symbols.Symbol, attribute string) error {
	return &Error{
		Kind:      ErrorInvalidAttribute,
		Document:  c.document,
		Content:   c.contentName(member),
		Parameter: attribute,
		Origin:    provenance.Symbol(candidate),
	}
}

func (c *compiler) contentName(member *symbols.Symbol) string {
	if fqn := symbols.FQNOf(member); fqn != "" {
		return fqn
	}
	return member.Name
}

// localMembers returns a scope's named and anonymous declarations in source order.
func localMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	members := sym.Scope.Members()
	if sym.Scope.HasAnonymousMembers() {
		members = append(members, sym.Scope.AnonymousMembers()...)
		sort.SliceStable(members, func(i, j int) bool {
			return members[i].DeclSpan.Offset < members[j].DeclSpan.Offset
		})
	}
	return members
}

// effectiveMembers returns local declarations in source order followed by
// unmasked inherited members.
func (c *compiler) effectiveMembers(sym *symbols.Symbol) []*symbols.Symbol {
	local := localMembers(sym)
	seen := make(map[*symbols.Symbol]bool, len(local))
	for _, member := range local {
		seen[member] = true
	}
	members := local
	for _, member := range c.model.MembersOf(sym) {
		if !seen[member] {
			members = append(members, member)
		}
	}
	return members
}

// effectiveName returns a member's declared or redefinition-inherited name.
func (c *compiler) effectiveName(sym *symbols.Symbol) string {
	if name := c.model.EffectiveNameOf(sym); name != "" {
		return name
	}
	return sym.Name
}
