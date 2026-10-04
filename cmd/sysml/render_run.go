package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtrace"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

type runRenderTarget struct {
	kind runtrace.Kind
	path string
	form view.Form
}

func runRenderModeMisuse() string {
	switch {
	case len(renderRuns) == 0:
		return "-render-run needs a value of the form <timeline|sequence>=<path>"
	case modelChecks.jsonOut && renderRunWritesStdout():
		return "-render-run cannot write to stdout with -json; name a file for the rendering"
	case flagGiven("compare-results"):
		return "-render-run cannot be combined with -compare-results"
	case len(modelChecks.actions) == 0 && len(modelChecks.states) == 0 && !modelChecks.advance.given:
		return "-render-run needs -action, -state or -advance to record a behavior run"
	case schedule.text == "explore":
		return "-render-run cannot render -schedule explore; render one declared or replayed run"
	case engine.text == "check" || engine.text == "smt" || engine.text == "all":
		return "-render-run cannot render -engine check, smt or all; run one behavior schedule"
	case modelChecks.runs.given || flagGiven("runs"):
		return "-render-run cannot be combined with -runs"
	case len(modelChecks.sweeps) > 0 || modelChecks.samples.given || flagGiven("sweep") || flagGiven("samples"):
		return "-render-run cannot be combined with -sweep or -samples"
	case len(modelChecks.records) > 0 || flagGiven("record-run"):
		return "-render-run cannot be combined with -record-run"
	case flagGiven("render") || renderView != "" || flagGiven("render-all") || renderAllDir != "":
		return "-render-run cannot be combined with -render or -render-all"
	case flagGiven("render-document") || renderDoc != "" || flagGiven("render-documents") || renderDocsDir != "":
		return "-render-run cannot be combined with -render-document or -render-documents"
	case flagGiven("convert") || convertFormat != "" || flagGiven("migrate") || migrateFormat != "":
		return "-render-run cannot be combined with -convert or -migrate"
	case flagGiven("query") || queryText != "":
		return "-render-run cannot be combined with -query"
	case outputPath != "":
		return "-render-run names each output path; do not combine it with -output"
	case renderPalette != "" || renderStyle != "" || renderPorts != "" || renderUnplaced != "":
		return "-render-palette, -render-style, -render-ports and -render-unplaced apply to model renderings, not -render-run"
	}
	return ""
}

func renderRunWritesStdout() bool {
	for _, value := range renderRuns {
		_, path, ok := strings.Cut(value, "=")
		if ok && path == "-" {
			return true
		}
	}
	return false
}

func runRenderTargetsFromFlags() ([]runRenderTarget, error) {
	targets := make([]runRenderTarget, 0, len(renderRuns))
	for _, value := range renderRuns {
		kindText, path, ok := strings.Cut(value, "=")
		if !ok || kindText == "" || path == "" {
			return nil, fmt.Errorf("-render-run takes <timeline|sequence>=<path>, not %q", value)
		}
		kind, ok := runtrace.ParseKind(kindText)
		if !ok {
			return nil, fmt.Errorf("unknown -render-run kind %q; want %s", kindText, strings.Join(runRenderKindNames(), ", "))
		}
		form := view.Form(renderForm)
		if renderForm == "" {
			if path == "-" {
				form = view.FormText
			} else {
				var found bool
				form, found = runRenderFormFromPath(path)
				if !found {
					return nil, fmt.Errorf("-render-run path %q has no recognized form extension; use -render-form text, mermaid or plantuml", path)
				}
			}
		} else if !slices.Contains(view.Forms(), form) {
			return nil, fmt.Errorf("unknown rendering form %q; -render-form takes %s", renderForm, formList())
		}
		probe := &view.Rendering{Kind: runRenderViewKind(kind), Run: true}
		if _, err := probe.WriteWith(form, view.Options{}); err != nil {
			return nil, fmt.Errorf("-render-run %s: %w", kind, err)
		}
		targets = append(targets, runRenderTarget{kind: kind, path: path, form: form})
	}
	return targets, nil
}

func runRenderKindNames() []string {
	kinds := runtrace.Kinds()
	names := make([]string, len(kinds))
	for i, kind := range kinds {
		names[i] = string(kind)
	}
	return names
}

func runRenderViewKind(kind runtrace.Kind) view.Kind {
	if kind == runtrace.KindTimeline {
		return view.KindTimeline
	}
	return view.KindSequence
}

func runRenderFormFromPath(path string) (view.Form, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mmd", ".mermaid":
		return view.FormMermaid, true
	case ".puml", ".plantuml":
		return view.FormPlantUML, true
	case ".txt":
		return view.FormText, true
	case ".dot", ".gv":
		return view.FormDot, true
	default:
		return "", false
	}
}

func finishRunCheck(rep *reporter, sess *repl.Session) int {
	status := rep.finish()
	if len(renderRuns) == 0 {
		return status
	}
	targets, err := runRenderTargetsFromFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 2
	}
	for _, target := range targets {
		rendering, err := sess.RunTraceRendering(target.kind)
		if err != nil {
			fmt.Fprintln(os.Stderr, errPrefix, err)
			return 2
		}
		artifact, err := rendering.WriteWith(target.form, view.Options{Width: artifactWidth(target.path, terminalWidth())})
		if err != nil {
			fmt.Fprintln(os.Stderr, errPrefix, err)
			return 2
		}
		name := "run " + string(target.kind)
		reportRenderNoticesFrom(rendering, name)
		if target.path == "-" {
			if err := writeArtifact(artifact, target.form); err != nil {
				fmt.Fprintln(os.Stderr, errPrefix, err)
				return 2
			}
		} else if err := writeArtifactFile(target.path, artifact, target.form); err != nil {
			fmt.Fprintln(os.Stderr, errPrefix, err)
			return 2
		}
	}
	return status
}
