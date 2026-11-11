package repl

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/doc/querytranslate"
)

// asViewOption is what %run-query and %query take before the query to print
// its answer as a QVTS::QueryResult view in place of its rows.
const asViewOption = "-as-view"

const queryUsage = "usage: %query [-as-view] <oslc-query>"

// sessionSource is the source a result view states for a session that loaded no file.
const sessionSource = "session"

// queryAnswer is what one query run reports: its rows as lines and as data,
// and, when the session writes them, the run as a QVTS::QueryResult view.
type queryAnswer struct {
	lines  []string
	values []NamedValue
	table  *VerdictTable
	view   string
}

// SetResultViews makes each document query the session runs also write its
// answer as a QVTS::QueryResult view, reported in Verdict.ResultView.
func (s *Session) SetResultViews(on bool) {
	defer s.enter()()
	s.resultViews = on
}

// writingResultViews turns result views on until the returned func is called.
func (s *Session) writingResultViews() func() {
	previous := s.resultViews
	s.resultViews = true
	return func() { s.resultViews = previous }
}

// cutAsView reads a leading -as-view off a query invocation.
func cutAsView(invocation string) (string, bool) {
	invocation = strings.TrimSpace(invocation)
	rest, ok := strings.CutPrefix(invocation, asViewOption)
	if !ok || (rest != "" && rest[0] != ' ' && rest[0] != '\t') {
		return invocation, false
	}
	return strings.TrimSpace(rest), true
}

func viewLines(text string) []string {
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

// resultView writes one run as a view named name; empty when the session writes
// none or the run has no outcome.
func (s *Session) resultView(name, query string, context queryexec.Context, result *queryexec.RowSet, err error) string {
	if !s.resultViews {
		return ""
	}
	text, _ := s.resultViewSpec(name, query).Write(context, result, err)
	return text
}

// refusedQueryView is the view of query text the translator refused with a QVTS
// record, an unknown property being a path error; empty for any other refusal.
func (s *Session) refusedQueryView(query string, context queryexec.Context, err error) string {
	outcome := querytranslate.OutcomeOf(err)
	if !s.resultViews || outcome == nil {
		return ""
	}
	return s.resultViewSpec(queryexec.DefaultResultViewName, query).WriteOutcome(context, *outcome)
}

// resultViewSpec states a run over the files the session loaded, identified by
// the digest of its documents.
func (s *Session) resultViewSpec(name, query string) queryexec.ResultView {
	var files []string
	for _, sn := range s.snippets {
		if sn.origin != "" {
			files = append(files, sourcePath(sn.origin))
		}
	}
	origin := sessionSource
	if len(files) > 0 {
		origin = strings.Join(files, " ")
	}
	digest := sha256.New()
	for _, doc := range s.sessionDocs() {
		digest.Write([]byte(doc.Content))
		digest.Write([]byte{0})
	}
	return queryexec.ResultView{
		Name:       name,
		Query:      query,
		Source:     origin,
		Provenance: "sha256:" + hex.EncodeToString(digest.Sum(nil)),
	}
}

// sourcePath names a loaded file as a view states it: relative to the working
// directory when it is under it, with forward slashes, so a view moves between machines.
func sourcePath(path string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, path); err == nil && filepath.IsLocal(rel) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}
