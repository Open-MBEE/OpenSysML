package repl

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	sedit "github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/dataimport"
)

const importUsage = "usage: %import <file> [as values] [map <file>] [format csv|tsv|json|jsonl] [dry-run]"

// ImportOptions are how ImportData reads its file: a mapping file, a format
// overriding the extension's, and whether the model is left as it was.
type ImportOptions struct {
	Map    string
	Format string
	DryRun bool
}

// ImportData sets the feature values a CSV, TSV, JSON or JSON Lines file
// assigns, one row per element, as one edit of the model's source: a value
// the model refuses leaves every other row unapplied too.
func (s *Session) ImportData(path string, opts ImportOptions) Verdict {
	defer s.enter()()
	return s.importData(path, opts)
}

func (s *Session) doImport(tail string) ([]string, bool, error) {
	args := parseArgs(strings.TrimSpace(tail))
	if len(args) == 0 {
		return []string{importUsage}, false, nil
	}
	path := nameText(args[0])
	var opts ImportOptions
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "dry-run":
			opts.DryRun = true
		case "map", "format":
			if i+1 >= len(args) {
				return []string{errPrefix + args[i] + " names a value", importUsage}, false, nil
			}
			if args[i] == "map" {
				opts.Map = nameText(args[i+1])
			} else {
				opts.Format = args[i+1]
			}
			i++
		case "as":
			if i+1 >= len(args) || args[i+1] != "values" {
				return []string{errPrefix + "only `as values` is supported", importUsage}, false, nil
			}
			i++
		default:
			return []string{errPrefix + "unexpected " + args[i], importUsage}, false, nil
		}
	}
	return s.importData(path, opts).Lines, false, nil
}

// importData reads the file, lowers its rows to edits and applies them as one
// edit of the model's source, as dataimport lowers them for the CLI too.
func (s *Session) importData(path string, opts ImportOptions) Verdict {
	fail := func(err error) Verdict {
		return Verdict{Subject: path, Status: VerdictFails, Lines: []string{errPrefix + err.Error(), "  the model was left unchanged"}}
	}
	mapPath := ""
	if opts.Map != "" {
		mapPath = expandHome(opts.Map)
		if err := s.guardFiles([]string{mapPath}, false); err != nil {
			return fail(err)
		}
	}
	if err := s.guardFiles([]string{expandHome(path)}, false); err != nil {
		return fail(err)
	}
	rows, err := dataimport.ReadFile(expandHome(path), path, mapPath, opts.Format)
	if err != nil {
		return fail(err)
	}
	idx := s.symbolIndex()
	if idx == nil {
		return fail(errors.New("no model is loaded to import into"))
	}
	resolver := resolve.New(idx)
	sem := semantics.NewModel(resolver)
	resolver.SetModel(sem)
	own := map[string]bool{docName: true}
	for _, sn := range s.snippets {
		if sn.origin != "" && !sn.open {
			own[sn.origin] = true
		}
	}
	ops, summary, err := dataimport.Lower(idx, resolver, sem, func(doc string) bool { return own[doc] }, rows)
	if err != nil {
		return fail(err)
	}
	if summary.Values == 0 {
		return Verdict{Subject: path, Status: VerdictHolds, Lines: []string{"✓ " + path + ": no values to import"}}
	}
	edited, err := s.importEdits(ops)
	if err != nil {
		return fail(err)
	}
	head := fmt.Sprintf("✓ imported %s into %s from %s", countOf(summary.Values, "value", "values"), countOf(summary.Elements, "element", "elements"), path)
	if opts.DryRun {
		head = fmt.Sprintf("dry run: would import %s into %s from %s; the model is unchanged", countOf(summary.Values, "value", "values"), countOf(summary.Elements, "element", "elements"), path)
	} else if err := s.commitImport(edited); err != nil {
		return fail(err)
	}
	lines := []string{head}
	for _, o := range ops {
		lines = append(lines, "  "+o.Description())
	}
	return Verdict{Subject: path, Status: VerdictHolds, Lines: lines}
}

// importEdits applies the operations to the snippets declaring their targets,
// each snippet's as one edit, and returns each edited snippet's new text.
func (s *Session) importEdits(ops []dataimport.Assignment) (map[int]string, error) {
	type group struct {
		doc     string
		snippet int
		start   int
		ops     []int
	}
	var groups []*group
	byKey := map[[2]int]*group{}
	docIDs := map[string]int{}
	for i, o := range ops {
		snippet, start := s.importSnippet(o.Doc, o.At)
		if snippet < 0 {
			return nil, fmt.Errorf("%s: %s is not in a session document", o.Where, o.Op.Target+o.Op.Owner)
		}
		id, ok := docIDs[o.Doc]
		if !ok {
			id = len(docIDs)
			docIDs[o.Doc] = id
		}
		key := [2]int{id, snippet}
		g := byKey[key]
		if g == nil {
			g = &group{doc: o.Doc, snippet: snippet, start: start}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.ops = append(g.ops, i)
	}
	out := map[int]string{}
	for _, g := range groups {
		list := make([]sedit.Operation, len(g.ops))
		for j, i := range g.ops {
			list[j] = ops[i].Op
		}
		res, _, ok, err := s.ws.ApplyEdit(g.doc, list, nil)
		if err == nil && !ok {
			err = fmt.Errorf("%s is not open", g.doc)
		}
		if err != nil {
			return nil, s.importFailure(g.doc, ops, g.ops, err)
		}
		doc := res.Documents[0]
		src := s.snippets[g.snippet].src
		text, ok := spliceSnippet(src, g.start, doc.Original, doc.Content)
		if !ok {
			return nil, fmt.Errorf("the import changed %s outside the declaration it edits", g.doc)
		}
		out[g.snippet] = text
	}
	return out, nil
}

// importFailure locates a refused edit at the input value it came from; an
// edit refused as a whole is retried an operation at a time to find it.
func (s *Session) importFailure(doc string, ops []dataimport.Assignment, group []int, err error) error {
	var e *sedit.Error
	if !errors.As(err, &e) {
		return err
	}
	if e.OperationIndex >= 0 && e.OperationIndex < len(group) {
		return fmt.Errorf("%s: %s", ops[group[e.OperationIndex]].Where, e.Message)
	}
	for _, i := range group {
		if _, _, _, one := s.ws.ApplyEdit(doc, []sedit.Operation{ops[i].Op}, nil); one != nil {
			var oneErr *sedit.Error
			if errors.As(one, &oneErr) {
				return fmt.Errorf("%s: %s", ops[i].Where, oneErr.Message)
			}
			return fmt.Errorf("%s: %w", ops[i].Where, one)
		}
	}
	return err
}

// importSnippet is the snippet holding offset in doc and where its text starts in
// that document, or -1.
func (s *Session) importSnippet(doc string, offset int) (int, int) {
	if doc != docName {
		for i := len(s.snippets) - 1; i >= 0; i-- {
			if sn := s.snippets[i]; sn.origin == doc && !sn.open {
				return i, 0
			}
		}
		return -1, 0
	}
	acc := 0
	for i, sn := range s.snippets {
		end := acc + len(sn.src)
		if offset >= acc && offset < end && sn.origin == "" && !sn.open {
			return i, acc
		}
		acc = end + 1
	}
	return -1, 0
}

// spliceSnippet carries the change between a document's original and edited
// text into the snippet starting at start in it; false when the change reaches
// outside the snippet.
func spliceSnippet(src string, start int, original, edited []byte) (string, bool) {
	p := 0
	for p < len(original) && p < len(edited) && original[p] == edited[p] {
		p++
	}
	q := 0
	for q < len(original)-p && q < len(edited)-p && original[len(original)-1-q] == edited[len(edited)-1-q] {
		q++
	}
	from, to := p-start, len(original)-q-start
	if from < 0 || to > len(src) || from > to {
		return "", false
	}
	return src[:from] + string(edited[p:len(edited)-q]) + src[to:], true
}

// commitImport replaces the edited snippets and rebuilds the model, restoring
// the buffer when the result reports an error it did not before.
func (s *Session) commitImport(edited map[int]string) error {
	before := slices.Clone(s.snippets)
	beforeErrors := s.errorCounts()
	s.version++
	for i, text := range edited {
		s.snippets[i].src = text
		if s.snippets[i].origin != "" {
			s.snippets[i].gen = s.version
		}
	}
	s.rebuildOver(nil)
	s.idxVersion = 0
	s.names = nil
	if problems := newProblems(beforeErrors, s.diagnostics()); len(problems) > 0 {
		s.rollbackSubmit(before)
		return fmt.Errorf("the imported values make the model invalid: %s", strings.Join(problems, "; "))
	}
	return nil
}
