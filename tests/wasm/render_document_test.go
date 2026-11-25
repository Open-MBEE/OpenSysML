// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package wasm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
)

// telescopeFixture is the document renderer's own fixture, whose goldens the
// service's RenderDocument answers match; the engine answers the same text.
const telescopeFixture = "../../internal/doc/docrender/testdata/telescope_report.sysml"

// graphViewsFixture declares a document whose views carry layout positions, so
// a render without Graphviz must fall back to Mermaid and say so.
const graphViewsFixture = "../../internal/doc/docrender/testdata/graph_views.sysml"

func readFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	return string(data)
}

// parseEngineFixture loads a fixture file into eng and returns its model hash.
func parseEngineFixture(t *testing.T, eng *engine.Engine, path string) string {
	t.Helper()
	body := mustCall(t, eng, "ParseSources", renderParamsJSON(t, map[string]any{
		"documents": []map[string]string{{"name": filepath.Base(path), "content": readFixture(t, path)}},
	}))
	var parsed engine.JParseSourcesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decoding ParseSources: %v\n%s", err, body)
	}
	if parsed.ModelHash == "" || len(parsed.Diagnostics) != 0 {
		t.Fatalf("ParseSources(%s) = %+v, want a hash and no diagnostics", path, parsed)
	}
	return parsed.ModelHash
}

func engineRenderDocument(t *testing.T, eng *engine.Engine, hash, document, form string) engine.JRenderDocumentResponse {
	t.Helper()
	params := map[string]any{"modelHash": hash, "documentId": document}
	if form != "" {
		params["form"] = form
	}
	body := mustCall(t, eng, "RenderDocument", renderParamsJSON(t, params))
	var rendered engine.JRenderDocumentResponse
	if err := json.Unmarshal(body, &rendered); err != nil {
		t.Fatalf("decoding RenderDocument(%q): %v\n%s", form, err, body)
	}
	return rendered
}

// The engine renders a document to the text the renderer's goldens lock in,
// in each form, in the field of that form's name, with the file a set writes.
func TestEngineRenderDocumentMatchesTheRendererGoldens(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	hash := parseEngineFixture(t, eng, telescopeFixture)
	for _, tc := range []struct {
		form, golden, fileName string
		text                   func(engine.JRenderDocumentResponse) string
	}{
		{"", "telescope_report.golden.md", "Observatory-MassReport.md", func(r engine.JRenderDocumentResponse) string { return r.Markdown }},
		{"markdown", "telescope_report.golden.md", "Observatory-MassReport.md", func(r engine.JRenderDocumentResponse) string { return r.Markdown }},
		{"html", "telescope_report.golden.html", "Observatory-MassReport.html", func(r engine.JRenderDocumentResponse) string { return r.Html }},
		{"asciidoc", "telescope_report.golden.adoc", "Observatory-MassReport.adoc", func(r engine.JRenderDocumentResponse) string { return r.Asciidoc }},
	} {
		rendered := engineRenderDocument(t, eng, hash, "Observatory::MassReport", tc.form)
		want := readFixture(t, filepath.Join(filepath.Dir(telescopeFixture), tc.golden))
		if got := tc.text(rendered); got != want {
			t.Errorf("RenderDocument(%q) differs from %s:\n%s", tc.form, tc.golden, got)
		}
		if rendered.FileName != tc.fileName {
			t.Errorf("RenderDocument(%q) fileName = %q, want %q", tc.form, rendered.FileName, tc.fileName)
		}
		written := 0
		for _, text := range []string{rendered.Markdown, rendered.Html, rendered.Asciidoc} {
			if text != "" {
				written++
			}
		}
		if written != 1 {
			t.Errorf("RenderDocument(%q) wrote %d forms, want the one asked for", tc.form, written)
		}
	}
}

// Without Graphviz, which no engine build runs, a positioned diagram is written
// as Mermaid with the renderer's notice rather than as DOT nothing will draw.
func TestEngineRenderDocumentFallsBackToMermaid(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	hash := parseEngineFixture(t, eng, graphViewsFixture)
	notice := docrender.GraphvizFallbackNotice
	markdown := engineRenderDocument(t, eng, hash, "Pump::Handbook", "").Markdown
	if strings.Count(markdown, "```mermaid") != 2 || strings.Count(markdown, "*"+notice+"*") != 2 || strings.Contains(markdown, "```dot") {
		t.Errorf("Markdown does not fall back to Mermaid visibly:\n%s", markdown)
	}
	page := engineRenderDocument(t, eng, hash, "Pump::Handbook", "html").Html
	if strings.Count(page, `<pre class="mermaid">`) != 2 || strings.Count(page, `<p class="sysml-diagram-notice"><em>`+notice+"</em></p>") != 2 {
		t.Errorf("HTML does not fall back to Mermaid visibly:\n%s", page)
	}
}

// A request the engine refuses answers the status the service answers: the
// forms it renders are named, and a missing model or document is not found.
func TestEngineRenderDocumentRefusals(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	hash := parseEngineFixture(t, eng, telescopeFixture)
	for name, tc := range map[string]struct {
		params  string
		code    uint32
		message string
	}{
		"pdf":                 {fmt.Sprintf(`{"modelHash":%q,"documentId":"Observatory::MassReport","form":"pdf"}`, hash), jsonrpc.CodeInvalidArgument, `form "pdf" is not one a document is rendered in: markdown, html or asciidoc`},
		"an unknown form":     {fmt.Sprintf(`{"modelHash":%q,"documentId":"Observatory::MassReport","form":"docx"}`, hash), jsonrpc.CodeInvalidArgument, `form "docx" is not one a document is rendered in`},
		"an unknown document": {fmt.Sprintf(`{"modelHash":%q,"documentId":"Observatory::Missing"}`, hash), jsonrpc.CodeNotFound, "symbol not found: Observatory::Missing"},
		"a non-document":      {fmt.Sprintf(`{"modelHash":%q,"documentId":"Observatory::Subsystem"}`, hash), jsonrpc.CodeInvalidArgument, "Observatory::Subsystem is not a document: one is a part def specializing DocumentQueries::Document"},
		"no document":         {fmt.Sprintf(`{"modelHash":%q}`, hash), jsonrpc.CodeInvalidArgument, "a document to render must be named in documentId"},
		"a missing model":     {`{"modelHash":"missing","documentId":"Observatory::MassReport"}`, jsonrpc.CodeNotFound, "model not found: missing"},
		"no model":            {`{"documentId":"Observatory::MassReport"}`, jsonrpc.CodeNotFound, "model not found: "},
	} {
		_, err := eng.Call(context.Background(), "RenderDocument", []byte(tc.params))
		var callErr *jsonrpc.Error
		if !errors.As(err, &callErr) || callErr.Code != tc.code {
			t.Errorf("%s: error = %v, want code %d", name, err, tc.code)
			continue
		}
		if !strings.Contains(callErr.Message, tc.message) {
			t.Errorf("%s: message = %q, want it to contain %q", name, callErr.Message, tc.message)
		}
	}
}
