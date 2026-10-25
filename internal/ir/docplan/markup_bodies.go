package docplan

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// markupBodies lists the textual representations a content block carries, in
// declaration order: each is a markup body the block's prose is written in.
func (c *compiler) markupBodies(owner *symbols.Symbol) []*symbols.Symbol {
	var bodies []*symbols.Symbol
	for _, member := range c.effectiveMembers(owner) {
		if member.Kind == symbols.SymbolTextualRepresentation && !c.index.Library(member) {
			bodies = append(bodies, member)
		}
	}
	return bodies
}

// markupLanguage is the language a textual representation declares.
func markupLanguage(rep *symbols.Symbol) string {
	if decl, ok := rep.Decl.(*ast.TextualRepresentation); ok {
		return source.StringValue(decl.Language)
	}
	return ""
}

// compileParagraphBody compiles the one markup body a paragraph carries into
// its runs: the body must yield exactly one paragraph, whose runs they are.
func (c *compiler) compileParagraphBody(member *symbols.Symbol, bodies []*symbols.Symbol) ([]Run, error) {
	if len(bodies) > 1 {
		return nil, &Error{
			Kind:      ErrorMarkupBodyNotParagraph,
			Document:  c.document,
			Content:   symbols.FQNOf(member),
			Parameter: markupLanguage(bodies[0]),
			Actual:    fmt.Sprintf("%d bodies", len(bodies)),
			Origin:    bodies[1].Origin(),
		}
	}
	blocks, err := c.compileMarkupBlocks(member, bodies[0])
	if err != nil {
		return nil, err
	}
	if len(blocks) != 1 || blocks[0].kind != ContentParagraph {
		return nil, &Error{
			Kind:      ErrorMarkupBodyNotParagraph,
			Document:  c.document,
			Content:   symbols.FQNOf(member),
			Parameter: markupLanguage(bodies[0]),
			Actual:    describeBlocks(blocks),
			Origin:    bodies[0].Origin(),
		}
	}
	return blocks[0].runs, nil
}

// describeBlocks states what a body yielded, for a paragraph that wanted one paragraph.
func describeBlocks(blocks []Content) string {
	switch len(blocks) {
	case 0:
		return "no content"
	case 1:
		return "a " + string(blocks[0].kind)
	}
	kinds := make([]string, len(blocks))
	for i, block := range blocks {
		kinds[i] = string(block.kind)
	}
	return fmt.Sprintf("%d blocks (%s)", len(blocks), strings.Join(kinds, ", "))
}

// compileMarkupBlocks parses the body of a textual representation owned by a
// document, section or paragraph into content blocks, each run and block
// placed at its source position within the body, headings opening sections.
func (c *compiler) compileMarkupBlocks(owner, rep *symbols.Symbol) ([]Content, error) {
	parsed := c.parseMarkupBody(owner, rep)
	if parsed.err != nil {
		return nil, parsed.err
	}
	body, at := parsed.body, func(span markup.Span) symbols.Origin { return parsed.at(rep, span) }
	nodes := make([]markupNode, 0, len(parsed.blocks))
	for i := range parsed.blocks {
		block := &parsed.blocks[i]
		node := Content{origin: at(block.Span)}
		switch block.Kind {
		case markup.BlockHeading:
			nodes = append(nodes, markupNode{heading: block, content: node})
			continue
		case markup.BlockParagraph:
			node.kind = ContentParagraph
			runs, err := c.markupRuns(owner, rep, body, block.Runs, at)
			if err != nil {
				return nil, err
			}
			node.runs = runs
		case markup.BlockList:
			node.kind = ContentList
			node.style = ListBullet
			if block.Ordered {
				node.style = ListNumber
			}
			for _, item := range block.Items {
				runs, err := c.markupRuns(owner, rep, body, item.Runs, at)
				if err != nil {
					return nil, err
				}
				node.items = append(node.items, ListItem{runs: runs, origin: at(item.Span)})
			}
		case markup.BlockTable:
			node.kind = ContentTable
			node.caption = block.Caption
			node.columnLabels = append([]string(nil), block.ColumnLabels...)
			node.columnWidths = append([]int(nil), block.ColumnWidths...)
			for _, row := range block.Rows {
				node.rows = append(node.rows, TableRow{cells: append([]string(nil), row.Cells...), origin: at(row.Span)})
			}
		case markup.BlockFormula:
			node.kind = ContentFormula
			node.source = block.Source
			node.caption = block.Caption
		case markup.BlockImage:
			node.kind = ContentImage
			node.source = block.Location
			node.alt = block.Alt
			node.caption = block.Caption
		}
		nodes = append(nodes, markupNode{content: node})
	}
	return c.nestMarkupSections(owner, rep, parsed, nodes)
}

// markupRuns maps parsed inline runs onto planned runs, resolving references.
func (c *compiler) markupRuns(owner, rep *symbols.Symbol, body string, runs []markup.Run, at func(markup.Span) symbols.Origin) ([]Run, error) {
	planned := make([]Run, 0, len(runs))
	for _, run := range runs {
		origin := at(run.Span)
		switch run.Kind {
		case markup.RunLink:
			planned = append(planned, Run{kind: RunLink, text: run.Text, target: run.Destination, bound: true, origin: origin})
		case markup.RunMath:
			planned = append(planned, Run{kind: RunSpan, text: run.Text, style: StyleMath, bound: true, origin: origin})
		case markup.RunRef:
			ref, err := c.markupRef(owner, rep, body, run, origin)
			if err != nil {
				return nil, err
			}
			planned = append(planned, ref)
		default:
			planned = append(planned, Run{kind: RunSpan, text: run.Text, style: runStyle(run.Style), bound: true, origin: origin})
		}
	}
	return planned, nil
}

// runStyle is the planned style of a parsed text style.
func runStyle(style markup.Style) RunStyle {
	switch style {
	case markup.StyleEmphasis:
		return StyleEmphasis
	case markup.StyleStrong:
		return StyleStrong
	case markup.StyleCode:
		return StyleCode
	}
	return StylePlain
}

// markupRef resolves a `sysml:` reference the way a Ref's target is: the
// destination is one target expression, read in the scope the body is written in.
func (c *compiler) markupRef(owner, rep *symbols.Symbol, body string, run markup.Run, origin symbols.Origin) (Run, error) {
	line := 1 + strings.Count(body[:run.Span.Start], "\n")
	value, ok := parser.ParseOneExpression(rep.DocName, run.Destination)
	if !ok {
		if section, doc, found, _ := c.markupSectionRef(rep.OwnerScope, run.Destination, nil, origin); found {
			return Run{kind: RunRef, text: run.Text, refName: section.name, refNameDoc: doc, bound: true, origin: origin}, nil
		}
		return Run{}, &Error{
			Kind:     ErrorInvalidRefTarget,
			Document: c.document,
			Content:  symbols.FQNOf(owner),
			Actual:   fmt.Sprintf("%q", markup.RefScheme+run.Destination),
			Line:     line,
			Origin:   origin,
		}
	}
	target, root, err := c.resolveTargetValue(owner, rep.OwnerScope, value, ErrorInvalidRefTarget, ErrorUnknownRefTarget, origin)
	if err != nil {
		var planning *Error
		if errors.As(err, &planning) && (planning.Kind == ErrorUnknownRefTarget || planning.Kind == ErrorInvalidRefTarget) {
			section, doc, found, sectionErr := c.markupSectionRef(rep.OwnerScope, run.Destination, value, origin)
			if sectionErr != nil {
				err = sectionErr
			} else if found {
				return Run{kind: RunRef, text: run.Text, refName: section.name, refNameDoc: doc, bound: true, origin: origin}, nil
			}
		}
		if errors.As(err, &planning) {
			planning.Line = line
			if planning.Actual == "" {
				planning.Actual = fmt.Sprintf("%q", markup.RefScheme+run.Destination)
			}
		}
		return Run{}, err
	}
	return Run{kind: RunRef, text: run.Text, refSym: target, refRoot: root, bound: true, origin: origin}, nil
}
