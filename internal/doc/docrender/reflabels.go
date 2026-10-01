package docrender

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
)

// The label a numbered section is referenced by opens with this word.
const sectionLabel = "Section"

// refLabels labels the content blocks a reference run may target by their
// numbers, keyed by anchor: "Section 2.1 - Title" when the sections are
// numbered, "Figure 3" or "Table 2" when the figures and tables are. A
// reference stating no text of its own takes the label over the target's
// title or caption, so prose reads as the numbered headings and captions do.
func refLabels(document *docir.Document, sections, figures bool) map[string]string {
	labels := map[string]string{}
	if !sections && !figures {
		return labels
	}
	captions := captionNumbering{on: figures}
	var walk func(nodes []docir.Content, prefix string)
	walk = func(nodes []docir.Content, prefix string) {
		count := 0
		for _, node := range nodes {
			switch node.Kind() {
			case docir.ContentSection:
				count++
				number := prefix + strconv.Itoa(count)
				if sections && node.Anchor() != "" {
					label := sectionLabel + " " + number
					if title := node.Title(); title != "" {
						label += " - " + title
					}
					labels[node.Anchor()] = label
				}
				walk(node.Children(), number+".")
			case docir.ContentTable, docir.ContentDiagram, docir.ContentImage:
				if label := captions.caption(node).label; label != "" && node.Anchor() != "" {
					labels[node.Anchor()] = label
				}
			}
		}
	}
	walk(document.Content(), "")
	return labels
}

// refText is the text a reference run renders as: its own when it states one,
// else the numbered label of its target in this document when there is one,
// else the target's title, caption or name.
func refText(run docir.TextRun, labels map[string]string) string {
	if run.TextDefaulted() && run.TargetDocument() == "" && run.TargetElement() == "" {
		if label, ok := labels[run.Target()]; ok {
			return label
		}
	}
	return run.Text()
}

// joinRuns joins rendered runs by single spaces, except where the text binds
// them: no space goes before a run opening with closing punctuation, nor
// after a run closing with an opening bracket or quote, so a reference reads
// as "(see Section 2)." rather than "( see Section 2 ) .".
func joinRuns(runs []docir.TextRun, rendered []string) string {
	var b strings.Builder
	for i, part := range rendered {
		if i > 0 && !docplan.BindsLeft(runs[i].Text()) && !docplan.BindsRight(runs[i-1].Text()) {
			b.WriteByte(' ')
		}
		b.WriteString(part)
	}
	return b.String()
}
