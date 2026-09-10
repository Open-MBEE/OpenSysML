package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chzyer/readline"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// runRender renders the view -render names of the model the files named on the
// command line make up, writing the artifact to -o or to stdout and every
// notice to stderr.
func runRender(files []string) error {
	form := view.Form(renderForm)
	if renderForm != "" && !slices.Contains(view.Forms(), form) {
		return fmt.Errorf("unknown rendering form %q; -render-form takes %s", renderForm, formList())
	}
	if len(files) == 0 {
		return errors.New("no model to render; name the files the view is declared in, as `sysml model.sysml -render MyView`")
	}

	sess, err := loadRenderingModel(files)
	if err != nil {
		return err
	}

	rendering, err := sess.ViewRendering(renderView)
	if err != nil {
		return err
	}
	if form == "" {
		form = defaultRenderForm(rendering.Kind, outputPath, atStdoutTerminal())
	}
	artifact, err := rendering.WriteWidth(form, artifactWidth(outputPath, terminalWidth()))
	if err != nil {
		return err
	}
	reportRenderNotices(rendering)
	return writeArtifact(artifact, form)
}

// runRenderAll renders every declared view into the directory -render-all names.
func runRenderAll(files []string) error {
	form := view.Form(renderForm)
	if renderForm != "" && !slices.Contains(view.Forms(), form) {
		return fmt.Errorf("unknown rendering form %q; -render-form takes %s", renderForm, formList())
	}
	if len(files) == 0 {
		return errors.New("no model to render; name at least one file before -render-all")
	}
	sess, err := loadRenderingModel(files)
	if err != nil {
		return err
	}
	views, err := sess.Views()
	if err != nil {
		return err
	}
	if len(views) == 0 {
		return errors.New("the model declares no views; nothing was rendered")
	}
	if err := os.MkdirAll(renderAllDir, 0o750); err != nil {
		return fmt.Errorf("create rendering directory %s: %w", renderAllDir, err)
	}

	destinations := map[string]string{}
	for _, info := range views {
		if !info.Supported {
			reportRenderSkip(info.Name, info.Reason)
			continue
		}
		rendering, err := sess.ViewRendering(info.Name)
		if err != nil {
			return err
		}
		writtenForm := form
		if writtenForm == "" {
			writtenForm = rendering.Kind.MachineForm()
		}
		reportRenderNoticesFrom(rendering, info.Name)
		artifact, err := rendering.WriteWidth(writtenForm, view.WidthUnbounded)
		if err != nil {
			if errors.Is(err, view.ErrWrongForm) {
				reportRenderSkip(info.Name, err.Error())
				continue
			}
			return err
		}
		filename, err := renderFilename(info.Name, writtenForm)
		if err != nil {
			return err
		}
		path := filepath.Join(renderAllDir, filename)
		if previous, exists := destinations[path]; exists {
			return fmt.Errorf("views %s and %s have the same rendering path %s", previous, info.Name, path)
		}
		destinations[path] = info.Name
		if err := writeArtifactFile(path, artifact, writtenForm); err != nil {
			return err
		}
	}
	return nil
}

// loadRenderingModel loads and reports a model whose stdout is reserved for
// rendering artifacts.
func loadRenderingModel(files []string) (*repl.Session, error) {
	sess := newSession()
	report, err := sess.LoadPathsReport(files)
	if err != nil {
		return nil, err
	}
	writeLines(os.Stderr, report.Loaded)
	writeLines(os.Stderr, report.Found)
	writeLines(os.Stderr, report.Declared)
	if report.Errors {
		return nil, fmt.Errorf("%s did not analyse cleanly; nothing was rendered", strings.Join(files, ", "))
	}
	return sess, nil
}

func renderFilename(name string, form view.Form) (string, error) {
	filename := strings.ReplaceAll(name, "::", ".") + renderExtension(form)
	if filepath.Base(filename) != filename || filename == "." || filename == ".." {
		return "", fmt.Errorf("view %s does not form a safe rendering filename", name)
	}
	return filename, nil
}

func renderExtension(form view.Form) string {
	switch form {
	case view.FormMermaid:
		return ".mmd"
	case view.FormMarkdown:
		return ".md"
	default:
		return ".txt"
	}
}

// defaultRenderForm is the form -render writes where -render-form named none:
// the text form on a terminal, read by a person, and the machine-readable form
// of the kind rendered into a file or a pipe, read by a tool.
func defaultRenderForm(kind view.Kind, output string, terminal bool) view.Form {
	if output == "" && terminal {
		return view.FormText
	}
	return kind.MachineForm()
}

// artifactWidth is the width -render writes the text form to fit: view.WidthUnbounded
// into a file, so a saved artifact does not depend on the window it was written from.
func artifactWidth(output string, width int) int {
	if output != "" {
		return view.WidthUnbounded
	}
	return width
}

// terminalWidth is stdout's width, and view.WidthUnbounded where stdout is no
// terminal.
func terminalWidth() int {
	width, _, err := readline.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return view.WidthUnbounded
	}
	return width
}

// atStdoutTerminal reports whether the artifact is written to a terminal.
func atStdoutTerminal() bool { return readline.IsTerminal(int(os.Stdout.Fd())) }

// formList names the forms -render-form takes, as its help and errors spell them.
func formList() string {
	names := make([]string, 0, len(view.Forms()))
	for _, form := range view.Forms() {
		names = append(names, string(form))
	}
	return strings.Join(names, ", ")
}

// reportRenderNotices reports on stderr what the rendering says about itself: an
// empty artifact, and every element it could not represent.
func reportRenderNotices(rendering *view.Rendering) {
	if rendering.Empty() {
		fmt.Fprintf(os.Stderr, "note: %s renders empty\n", rendering.View)
	}
	for _, notice := range rendering.Notices {
		fmt.Fprintf(os.Stderr, "note: %s\n", notice)
	}
}

func reportRenderNoticesFrom(rendering *view.Rendering, name string) {
	if rendering.Empty() {
		fmt.Fprintf(os.Stderr, "%s: note: renders empty\n", name)
	}
	for _, notice := range rendering.Notices {
		fmt.Fprintf(os.Stderr, "%s: note: %s\n", name, notice)
	}
}

func reportRenderSkip(name, reason string) {
	reason = strings.TrimPrefix(reason, name+": ")
	fmt.Fprintf(os.Stderr, "%s: skipped: %s\n", name, reason)
}

// writeArtifact writes the rendering to -o, or to stdout when no file was named.
func writeArtifact(artifact string, form view.Form) error {
	out := []byte(strings.TrimRight(artifact, "\n") + "\n")
	if outputPath == "" {
		_, err := os.Stdout.Write(out)
		return err
	}
	return writeArtifactFile(outputPath, artifact, form)
}

func writeArtifactFile(path, artifact string, form view.Form) error {
	out := []byte(strings.TrimRight(artifact, "\n") + "\n")
	replaced, err := export.WriteFile(path, out)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", path, form, len(out), what)
	return nil
}
