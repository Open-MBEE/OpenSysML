package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// The targets -list names.
const (
	listDocuments   = "documents"
	listViews       = "views"
	listDiagrams    = "diagrams"
	listPseudoViews = "pseudo-views"
	listAll         = "all"
)

// The forms -list-form names.
const (
	listFormText = "text"
	listFormTSV  = "tsv"
	listFormJSON = "json"
)

func listTargets() []string {
	return []string{listDocuments, listViews, listDiagrams, listPseudoViews, listAll}
}

func listForms() []string { return []string{listFormText, listFormTSV, listFormJSON} }

// listMisuse is the refusal of a -list invocation another mode or option
// contradicts, or "" when it stands; checks are refused separately.
func listMisuse() string {
	switch {
	case renderView != "" || renderAllDir != "" || renderDoc != "" || renderDocsDir != "" || len(renderRuns) > 0:
		return "-list names the documents and views; -render, -render-all, -render-document, -render-documents and -render-run render them; ask for one per run"
	case convertFormat != "" || migrateFormat != "" || graphsSubject != "" || compileCalc != "" || syncDiffWith != "" || syncApplyTo != "":
		return "-list, -convert, -migrate, -graphs, -compile, -sync-diff and -sync-apply each answer a run of their own; ask for one per run"
	case queryText != "" || len(evalExprs) > 0 || fromFormat != "":
		return "-list cannot be combined with -query, -eval or -from"
	case outputPath != "":
		return "-list writes its listing to stdout and cannot be combined with -output"
	case len(dataImports) > 0 || importMap.given() || importFormat.given() || importDryRun || flagGiven("import-as"):
		return "-list names the documents and views and imports nothing; -import and its -import-map, -import-format, -import-as and -import-dry-run belong to a run of their own"
	}
	return ""
}

// listKindFilter reads -list-kind into the kinds it names, refusing an unknown
// kind and a filter on what has no kind to filter by.
func listKindFilter() ([]view.Kind, error) {
	if !flagGiven("list-kind") {
		return nil, nil
	}
	if listWhat == listDocuments || listWhat == listPseudoViews {
		return nil, fmt.Errorf("-list-kind filters the views of -list views, diagrams or all; -list %s has none to filter", listWhat)
	}
	var kinds []view.Kind
	for _, name := range strings.Split(listKinds, ",") {
		kind := view.Kind(strings.TrimSpace(name))
		if !slices.Contains(view.Kinds(), kind) {
			return nil, fmt.Errorf("unknown view kind %q; -list-kind takes a comma-separated list of %s", name, repl.ViewKindList())
		}
		kinds = append(kinds, kind)
	}
	return kinds, nil
}

// runList writes the documents, views or pseudo-views -list names to stdout in
// the -list-form asked for.
func runList(files []string) error {
	if !slices.Contains(listTargets(), listWhat) {
		return fmt.Errorf("unknown listing %q; -list takes %s", listWhat, strings.Join(listTargets(), ", "))
	}
	form := listForm
	if form == "" {
		form = listFormText
	}
	if !slices.Contains(listForms(), form) {
		return fmt.Errorf("unknown listing form %q; -list-form takes %s", listForm, strings.Join(listForms(), ", "))
	}
	kinds, err := listKindFilter()
	if err != nil {
		return err
	}
	var items []repl.ListItem
	if len(files) == 0 {
		if listWhat != listPseudoViews {
			return errors.New("no model to list; name the files to list, as `sysml model.sysml -list documents`")
		}
	} else {
		sess, err := loadArtifactModel(files, "nothing was listed")
		if err != nil {
			return err
		}
		if listWhat == listDocuments || listWhat == listAll {
			items = append(items, sess.ListDocuments()...)
		}
		if listWhat == listViews || listWhat == listDiagrams || listWhat == listAll {
			items = append(items, sess.ListViews(listWhat == listDiagrams, kinds)...)
		}
	}
	if listWhat == listPseudoViews {
		items = repl.PseudoViewItems()
	}
	return writeListing(os.Stdout, items, form)
}

// writeListing writes items in form; nothing at all when there are none.
func writeListing(w io.Writer, items []repl.ListItem, form string) error {
	switch form {
	case listFormTSV:
		return repl.WriteListTSV(w, items)
	case listFormJSON:
		return repl.WriteListJSON(w, items)
	}
	for _, line := range repl.ListText(items) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
