package docplan

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// markupSection is a section a heading of a markup body opens. It is no model
// element: the planner labels it by a name unique within the document.
type markupSection struct {
	name  string
	title string
	path  []string
}

// markupOutline is what one document's markup bodies contribute: the name
// each body's headings take, in order, the sections by name, and the name
// each explicit id was given, so a reference spelt as the id finds it.
type markupOutline struct {
	names    map[*symbols.Symbol][]string
	sections map[string]markupSection
	ids      map[string]string
}

// section is the section named by name, else the one whose explicit id name
// spells; an allocated name is never shadowed by a later heading's id.
func (o *markupOutline) section(name string) (markupSection, bool) {
	if section, ok := o.sections[name]; ok {
		return section, true
	}
	allocated, ok := o.ids[name]
	if !ok {
		return markupSection{}, false
	}
	section, ok := o.sections[allocated]
	return section, ok
}

// sectionAt is the section whose named path is path: its last segment a name
// or explicit id, the ones before it the names of the sections above it.
func (o *markupOutline) sectionAt(path []string) (markupSection, bool) {
	section, ok := o.section(path[len(path)-1])
	if !ok || len(section.path) != len(path) || !slices.Equal(section.path[:len(path)-1], path[:len(path)-1]) {
		return markupSection{}, false
	}
	return section, true
}

// parsedBody is a textual representation's body parsed once: its language,
// text, offsets into the source and blocks, or the planning error instead;
// unchecked is the warning a lenient plan left the body unparsed under.
type parsedBody struct {
	language  markup.Parser
	body      string
	offsets   []int
	blocks    []markup.Block
	err       *Error
	unchecked *Error
}

// at is the source origin of a span of the body.
func (p *parsedBody) at(rep *symbols.Symbol, span markup.Span) symbols.Origin {
	decl := rep.Decl.(*ast.TextualRepresentation)
	start, stop := p.offsets[span.Start], p.offsets[span.Stop]
	return symbols.OriginAt(rep.DocName, source.Span{Offset: decl.BodySpan.Offset + start, Len: stop - start})
}

// line is the 1-based body line a span starts on.
func (p *parsedBody) line(span markup.Span) int {
	return 1 + strings.Count(p.body[:span.Start], "\n")
}

// parseMarkupBody parses the body of a textual representation owned by a
// document, section or paragraph, once per representation.
func (c *compiler) parseMarkupBody(owner, rep *symbols.Symbol) *parsedBody {
	if parsed, ok := c.parsed[rep]; ok {
		return parsed
	}
	parsed := c.parseBody(owner, rep)
	c.parsed[rep] = parsed
	return parsed
}

func (c *compiler) parseBody(owner, rep *symbols.Symbol) *parsedBody {
	fail := func(kind ErrorKind, err error) *parsedBody {
		return &parsedBody{err: &Error{Kind: kind, Document: c.document, Content: symbols.FQNOf(owner), Origin: rep.Origin(), Err: err}}
	}
	decl, ok := rep.Decl.(*ast.TextualRepresentation)
	if !ok {
		return fail(ErrorInvalidContext, errors.New("textual representation symbol without its declaration"))
	}
	declared := source.StringValue(decl.Language)
	language, ok := markup.For(declared)
	if !ok {
		return &parsedBody{err: &Error{
			Kind:     ErrorUnsupportedMarkupLanguage,
			Document: c.document,
			Content:  symbols.FQNOf(owner),
			Actual:   declared,
			Expected: markup.Languages(),
			Origin:   rep.Origin(),
		}}
	}
	lookup := c.model.SourceText()
	if lookup == nil {
		return fail(ErrorInvalidContext, errors.New("the semantic model has no source text to read the body from"))
	}
	body, offsets := source.CommentBodyOffsets(lookup(rep.DocName, decl.BodySpan))
	parsed := &parsedBody{language: language, body: body, offsets: offsets}
	key := uncheckedKey{c.scanned, language.Language()}
	if prior := c.uncheckedBy[key]; prior != nil {
		prior.Bodies++
		parsed.unchecked = prior
		return parsed
	}
	blocks, err := language.Parse(body)
	if err == nil {
		parsed.blocks = blocks
		return parsed
	}
	var tool *markup.ToolError
	if errors.As(err, &tool) {
		kind := ErrorMarkupToolFailed
		if tool.Missing {
			kind = ErrorMarkupToolMissing
		}
		planning := &Error{Kind: kind, Document: symbols.FQNOf(c.scanned), Content: symbols.FQNOf(owner), Parameter: language.Language(), Origin: rep.Origin(), Err: tool}
		if c.lenient && tool.Missing {
			// Another document's bodies are its own plan's to warn of.
			planning.Bodies = 1
			if c.scanned == c.entry {
				c.unchecked = append(c.unchecked, planning)
			}
			c.uncheckedBy[key] = planning
			parsed.unchecked = planning
			return parsed
		}
		parsed.err = planning
		return parsed
	}
	var unsupported *markup.UnsupportedConstructError
	if errors.As(err, &unsupported) {
		parsed.err = &Error{
			Kind:      ErrorUnsupportedMarkupConstruct,
			Document:  c.document,
			Content:   symbols.FQNOf(owner),
			Parameter: language.Language(),
			Actual:    unsupported.Construct,
			Line:      unsupported.Line,
			Origin:    parsed.at(rep, unsupported.Span),
		}
		return parsed
	}
	return fail(ErrorInvalidContext, err)
}

// outlineOf is the markup outline of a document definition, scanned once.
func (c *compiler) outlineOf(doc *symbols.Symbol) *markupOutline {
	if outline, ok := c.outlines[doc]; ok {
		return outline
	}
	outline := &markupOutline{names: make(map[*symbols.Symbol][]string), sections: make(map[string]markupSection), ids: make(map[string]string)}
	c.outlines[doc] = outline
	used := make(map[string]bool)
	c.collectContentNames(doc, used)
	scanned := c.scanned
	c.scanned = doc
	c.scanBodies(doc, nil, outline, used)
	c.scanned = scanned
	return outline
}

// collectContentNames records the effective name of every content block of a
// document, at every depth, so no derived section name shadows one.
func (c *compiler) collectContentNames(owner *symbols.Symbol, used map[string]bool) {
	for _, member := range c.effectiveMembers(owner) {
		if member.Kind != symbols.SymbolPartUsage || !c.isContent(member) {
			continue
		}
		if name := c.effectiveName(member); name != "" {
			used[name] = true
		}
		c.collectContentNames(member, used)
	}
}

// scanBodies names the headings of every document and section body under
// owner in declaration order: the explicit id, else a slug of the title (see
// sectionName), each made unique within the document, with the named path
// the section takes; a repeated explicit id keeps naming its first heading.
func (c *compiler) scanBodies(owner *symbols.Symbol, path []string, outline *markupOutline, used map[string]bool) {
	for _, member := range c.effectiveMembers(owner) {
		if member.Kind == symbols.SymbolPartUsage && c.isContent(member) {
			if c.model.Conforms(member, c.bases.section) {
				c.scanBodies(member, append(path[:len(path):len(path)], c.effectiveName(member)), outline, used)
			}
			continue
		}
		if member.Kind != symbols.SymbolTextualRepresentation || c.index.Library(member) {
			continue
		}
		parsed := c.parseMarkupBody(owner, member)
		if parsed.err != nil {
			continue
		}
		type openHeading struct {
			level int
			name  string
		}
		var open []openHeading
		for _, block := range parsed.blocks {
			if block.Kind != markup.BlockHeading {
				continue
			}
			name := uniqueSectionName(sectionName(block.ID, block.Title), used)
			used[name] = true
			if _, taken := outline.ids[block.ID]; block.ID != "" && !taken {
				outline.ids[block.ID] = name
			}
			outline.names[member] = append(outline.names[member], name)
			for len(open) > 0 && open[len(open)-1].level >= block.Level {
				open = open[:len(open)-1]
			}
			full := append([]string(nil), path...)
			for _, section := range open {
				full = append(full, section.name)
			}
			full = append(full, name)
			open = append(open, openHeading{block.Level, name})
			outline.sections[name] = markupSection{name: name, title: block.Title, path: full}
		}
	}
}

// sectionName is a section's name before it is made unique: an explicit id
// that is a basic name as written, else the slug of the id, else of the title.
func sectionName(id, title string) string {
	if source.IsIdentifier(id) {
		return id
	}
	if id != "" {
		return sectionSlug(id)
	}
	return sectionSlug(title)
}

// sectionSlug derives a section name from text: lower-cased, every run of
// characters that are neither letters nor digits one underscore, and never
// starting with a digit, so the name is a SysML name a `sysml:` destination
// can spell.
func sectionSlug(text string) string {
	var b strings.Builder
	pending := false
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if pending && b.Len() > 0 {
				b.WriteByte('_')
			}
			pending = false
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		pending = true
	}
	slug := b.String()
	if slug == "" {
		return "section"
	}
	if first := slug[0]; first >= '0' && first <= '9' {
		return "_" + slug
	}
	return slug
}

// uniqueSectionName suffixes a name taken earlier in the document with the
// first free ordinal from 2: `overview`, `overview_2`, `overview_3`.
func uniqueSectionName(name string, used map[string]bool) string {
	if !used[name] {
		return name
	}
	for ordinal := 2; ; ordinal++ {
		candidate := name + "_" + strconv.Itoa(ordinal)
		if !used[candidate] {
			return candidate
		}
	}
}

// markupNode is one parsed block mapped onto content, a heading still to be
// nested into the section it opens.
type markupNode struct {
	heading *markup.Block
	content Content
}

// nestMarkupSections lays a body's blocks out as the document's content: each
// heading opens a section at the depth its level maps to, below the owner;
// the blocks after it are its children until a heading of its level or above.
func (c *compiler) nestMarkupSections(owner, rep *symbols.Symbol, parsed *parsedBody, nodes []markupNode) ([]Content, error) {
	base := len(c.stack) + 1
	names := c.outlineOf(c.entry).names[rep]
	type openSection struct {
		depth int
		node  *Content
	}
	var (
		content []Content
		open    []openSection
		first   = 0
		named   = 0
	)
	place := func(node Content) {
		if len(open) == 0 {
			content = append(content, node)
			return
		}
		top := open[len(open)-1].node
		top.children = append(top.children, node)
	}
	closeTo := func(depth int) {
		for len(open) > 0 && open[len(open)-1].depth >= depth {
			closing := open[len(open)-1]
			open = open[:len(open)-1]
			place(*closing.node)
		}
	}
	for _, node := range nodes {
		if node.heading == nil {
			place(node.content)
			continue
		}
		heading := node.heading
		if first == 0 {
			first = heading.Level
		}
		depth := base + heading.Level - first
		previous := base - 1
		if len(open) > 0 {
			previous = open[len(open)-1].depth
		}
		if heading.Level < first || depth > previous+1 {
			expected := fmt.Sprintf("a level between %d and %d", first, first+previous+1-base)
			return nil, &Error{
				Kind:      ErrorMarkupHeadingLevel,
				Document:  c.document,
				Content:   symbols.FQNOf(owner),
				Parameter: parsed.language.Language(),
				Actual:    fmt.Sprintf("level %d heading %q", heading.Level, heading.Title),
				Expected:  expected,
				Line:      parsed.line(heading.Span),
				Origin:    parsed.at(rep, heading.Span),
			}
		}
		closeTo(depth)
		section := &Content{kind: ContentSection, title: heading.Title, origin: node.content.origin}
		if named < len(names) {
			section.name = names[named]
		}
		named++
		open = append(open, openSection{depth: depth, node: section})
	}
	closeTo(base)
	return content, nil
}

// markupSectionRef resolves a reference destination no model element answers
// to as a markup-born section: a bare name or the named path of a nested
// heading in this document, or either qualified by a document definition or
// document usage, in that document.
func (c *compiler) markupSectionRef(scope *symbols.Scope, destination string, value ast.Node, origin symbols.Origin) (markupSection, *symbols.Symbol, bool, error) {
	name, ok := refQualifiedName(value)
	if !ok {
		qualifier, path := splitSectionPath(destination)
		if len(path) == 1 {
			section, found := c.outlineOf(c.entry).section(destination)
			return section, nil, found, nil
		}
		doc := c.entry
		if qualifier != nil {
			resolved, err := c.documentNamed(scope, qualifier, origin)
			if err != nil || resolved == nil {
				return markupSection{}, nil, false, err
			}
			doc = resolved
		}
		section, found := c.outlineOf(doc).sectionAt(path)
		if doc == c.entry {
			doc = nil
		}
		return section, doc, found, nil
	}
	parts := name.Parts
	if len(parts) == 1 && !name.Global {
		section, found := c.outlineOf(c.entry).section(parts[0].Text)
		return section, nil, found, nil
	}
	prefix := &ast.QualifiedName{Global: name.Global, Parts: parts[:len(parts)-1]}
	doc, err := c.documentNamed(scope, prefix, origin)
	if err != nil || doc == nil {
		return markupSection{}, nil, false, err
	}
	section, found := c.outlineOf(doc).section(parts[len(parts)-1].Text)
	if doc == c.entry {
		doc = nil
	}
	return section, doc, found, nil
}

// documentNamed is the document a qualified name stands for, or nil when the
// name resolves to nothing or to no document.
func (c *compiler) documentNamed(scope *symbols.Scope, name *ast.QualifiedName, origin symbols.Origin) (*symbols.Symbol, error) {
	resolved, ok := c.resolver.ResolveQualified(scope, name)
	if !ok || resolved == nil {
		return nil, nil
	}
	if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
		resolved = canonical
	}
	return c.documentOf(resolved, origin)
}

// splitSectionPath reads a dotted destination (`appendix.derivation`,
// `Report::appendix.derivation`) as a section path, with the document
// qualifier written ahead of its first segment when there is one.
func splitSectionPath(destination string) (*ast.QualifiedName, []string) {
	path := strings.Split(destination, ".")
	if len(path) < 2 {
		return nil, path
	}
	global := strings.HasPrefix(path[0], "::")
	segments := strings.Split(strings.TrimPrefix(path[0], "::"), "::")
	if len(segments) < 2 {
		return nil, path
	}
	qualifier := &ast.QualifiedName{Global: global}
	for _, text := range segments[:len(segments)-1] {
		qualifier.Parts = append(qualifier.Parts, ast.NameSegment{Text: text})
	}
	path[0] = segments[len(segments)-1]
	return qualifier, path
}

// documentOf is the document definition a symbol stands for: the definition
// itself, or the one definition typing a document usage; a usage typed by
// more than one document definition is an ambiguous target.
func (c *compiler) documentOf(sym *symbols.Symbol, origin symbols.Origin) (*symbols.Symbol, error) {
	if sym.Kind == symbols.SymbolPartDef {
		if c.model.Conforms(sym, c.bases.document) {
			return sym, nil
		}
		return nil, nil
	}
	if _, usage := sym.Decl.(*ast.Usage); !usage {
		return nil, nil
	}
	defs := c.documentTypesOf(sym)
	switch len(defs) {
	case 0:
		return nil, nil
	case 1:
		return defs[0], nil
	default:
		return nil, c.ambiguousDocument(sym, defs, origin)
	}
}

// refQualifiedName is the qualified name a target expression spells, if any.
func refQualifiedName(value ast.Node) (*ast.QualifiedName, bool) {
	if reference, ok := value.(*ast.FeatureReference); ok {
		value = reference.Name
	}
	name, ok := value.(*ast.QualifiedName)
	return name, ok && name != nil
}

// markupSectionTarget is the reference target of a run that names a
// markup-born section, in this document or another.
func (c *compiler) markupSectionTarget(run *Run) (refTarget, bool) {
	doc := c.entry
	if run.refNameDoc != nil {
		doc = run.refNameDoc
	}
	section, ok := c.outlineOf(doc).sections[run.refName]
	if !ok {
		return refTarget{}, false
	}
	target := refTarget{path: section.path, label: section.title}
	if run.refNameDoc != nil {
		target.document = symbols.FQNOf(run.refNameDoc)
	}
	return target, true
}
