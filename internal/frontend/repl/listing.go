package repl

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// The categories a listing item is of.
const (
	ListDocument   = "document"
	ListView       = "view"
	ListPseudoView = "pseudo-view"
)

// ListItem is one document, view or pseudo-view a listing names. Name is the
// qualified name as the notation writes it, which -render-document and -render
// read back as the same element; File and Line locate the declaration.
type ListItem struct {
	Category  string    `json:"category"`
	Name      string    `json:"name"`
	Kind      view.Kind `json:"kind,omitempty"`
	Supported *bool     `json:"supported,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	File      string    `json:"file,omitempty"`
	Line      int       `json:"line,omitempty"`
}

// DocumentItems lists document definitions as items, in the order given.
func DocumentItems(defs []model.DocumentDefinition) []ListItem {
	out := make([]ListItem, 0, len(defs))
	for _, def := range defs {
		out = append(out, ListItem{Category: ListDocument, Name: def.Notation, File: def.Doc, Line: def.Line})
	}
	return out
}

// ViewItems lists views as items, in the order given, an unsupported view
// carrying the reason it is not produced.
func ViewItems(views []model.ViewInfo) []ListItem {
	out := make([]ListItem, 0, len(views))
	for _, info := range views {
		item := ListItem{
			Category:  ListView,
			Name:      info.Notation,
			Kind:      info.Kind,
			Supported: boolRef(info.Supported),
			File:      info.Doc,
			Line:      info.Line,
		}
		if !info.Supported {
			item.Reason = info.Reason
		}
		out = append(out, item)
	}
	return out
}

// PseudoViewItems lists the pseudo-views a document is rendered through when
// it declares no view of its own.
func PseudoViewItems() []ListItem {
	specs := view.PseudoViewSpecs()
	out := make([]ListItem, 0, len(specs))
	for _, spec := range specs {
		kind := view.Kind(strings.TrimPrefix(spec, view.PseudoViewPrefix))
		out = append(out, ListItem{Category: ListPseudoView, Name: spec, Kind: kind, Supported: boolRef(true)})
	}
	return out
}

func boolRef(b bool) *bool { return &b }

// FilterViews keeps the views of the kinds named, every kind when none is, and
// only the graph-shaped ones when diagrams is set.
func FilterViews(views []model.ViewInfo, diagrams bool, kinds []view.Kind) []model.ViewInfo {
	want := make(map[view.Kind]bool, len(kinds))
	for _, kind := range kinds {
		want[kind] = true
	}
	out := []model.ViewInfo{}
	for _, info := range views {
		if diagrams && !info.Kind.GraphShaped() {
			continue
		}
		if len(want) > 0 && !want[info.Kind] {
			continue
		}
		out = append(out, info)
	}
	return out
}

// ListText writes items one per line, the columns aligned: `document  <name>`,
// `view  <kind>  <name>`, an unsupported view followed by the reason.
func ListText(items []ListItem) []string {
	categoryWidth, kindWidth := 0, 0
	for _, item := range items {
		categoryWidth = max(categoryWidth, len(item.Category))
		kindWidth = max(kindWidth, len(item.Kind))
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		var b strings.Builder
		b.WriteString(pad(item.Category, categoryWidth))
		b.WriteString("  ")
		if item.Category != ListDocument {
			b.WriteString(pad(string(item.Kind), kindWidth))
			b.WriteString("  ")
		}
		b.WriteString(escapeLineBreaks(item.Name))
		if item.Supported != nil && !*item.Supported {
			b.WriteString("  (unsupported: " + escapeLineBreaks(item.Reason) + ")")
		}
		out = append(out, b.String())
	}
	return out
}

func pad(s string, width int) string {
	return s + strings.Repeat(" ", width-len(s))
}

// listTSVHeader is the header record of the tab-separated listing.
const listTSVHeader = "category\tkind\tsupported\tfile\tline\tname\treason"

// WriteListTSV writes items as tab-separated values under a header record;
// nothing at all when there are none.
func WriteListTSV(w io.Writer, items []ListItem) error {
	if len(items) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(listTSVHeader + "\n")
	for _, item := range items {
		supported, line := "", ""
		if item.Supported != nil {
			supported = strconv.FormatBool(*item.Supported)
		}
		if item.Line > 0 {
			line = strconv.Itoa(item.Line)
		}
		fields := []string{item.Category, string(item.Kind), supported, item.File, line, item.Name, item.Reason}
		for i, field := range fields {
			fields[i] = tsvField(field)
		}
		b.WriteString(strings.Join(fields, "\t") + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// WriteListJSON writes items as a JSON array; nothing at all when there are none.
func WriteListJSON(w io.Writer, items []ListItem) error {
	if len(items) == 0 {
		return nil
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("write listing: %w", err)
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// escapeLineBreaks keeps a field on its line, writing a line break as the notation's
// escape for it.
func escapeLineBreaks(s string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(s)
}

// tsvField keeps a field within its column and its record.
func tsvField(s string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(s)
}

// ListDocuments lists the document definitions the session's model declares.
func (s *Session) ListDocuments() []ListItem {
	defer s.enter()()
	return s.listDocuments()
}

func (s *Session) listDocuments() []ListItem {
	return DocumentItems(s.ws.DocumentDefinitions())
}

// ListViews lists the views the session's model declares, those of the kinds
// named when any are, and only the graph-shaped ones when diagrams is set.
func (s *Session) ListViews(diagrams bool, kinds []view.Kind) []ListItem {
	defer s.enter()()
	return s.listViews(diagrams, kinds)
}

func (s *Session) listViews(diagrams bool, kinds []view.Kind) []ListItem {
	return ViewItems(FilterViews(s.ws.AllViews(), diagrams, kinds))
}

// viewsDiagrams is the %views argument keeping the graph-shaped views.
const viewsDiagrams = "diagrams"

const viewsUsage = "usage: %views [diagrams] [<kind>...]"

// doDocuments answers %documents.
func (s *Session) doDocuments() ([]string, bool, error) {
	lines := ListText(s.listDocuments())
	if len(lines) == 0 {
		return []string{"(no documents)"}, false, nil
	}
	return lines, false, nil
}

// doViews answers %views: every view, or those of the kinds the arguments name.
func (s *Session) doViews(args []string) ([]string, bool, error) {
	diagrams := false
	var kinds []view.Kind
	for _, arg := range args {
		if arg == viewsDiagrams {
			diagrams = true
			continue
		}
		kind := view.Kind(arg)
		if !slices.Contains(view.Kinds(), kind) {
			return []string{fmt.Sprintf("error: unknown view kind %q; %%views takes diagrams or %s", arg, ViewKindList()), viewsUsage}, false, nil
		}
		kinds = append(kinds, kind)
	}
	lines := ListText(s.listViews(diagrams, kinds))
	if len(lines) == 0 {
		return []string{"(no views)"}, false, nil
	}
	return lines, false, nil
}

// viewsArguments are the words %views takes.
func viewsArguments() []string {
	out := []string{viewsDiagrams}
	for _, kind := range view.Kinds() {
		out = append(out, string(kind))
	}
	return out
}

// ViewKindList names the view kinds, as a refusal lists them.
func ViewKindList() string {
	names := make([]string, 0, len(view.Kinds()))
	for _, kind := range view.Kinds() {
		names = append(names, string(kind))
	}
	return strings.Join(names, ", ")
}
