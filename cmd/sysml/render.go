package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chzyer/readline"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/filename"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
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
	options, err := renderOptions(artifactWidth(outputPath, terminalWidth()))
	if err != nil {
		return err
	}
	artifact, err := rendering.WriteWith(form, options)
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
	options, err := renderOptions(view.WidthUnbounded)
	if err != nil {
		return err
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

	filenames, err := renderFilenames(views, form)
	if err != nil {
		return err
	}
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
		artifact, err := rendering.WriteWith(writtenForm, options)
		if err != nil {
			if errors.Is(err, view.ErrWrongForm) {
				reportRenderSkip(info.Name, err.Error())
				continue
			}
			return err
		}
		path := filepath.Join(renderAllDir, filenames[info.Name])
		if err := writeArtifactFile(path, artifact, writtenForm); err != nil {
			return err
		}
	}
	return nil
}

// renderFilenames is the file -render-all writes each view it writes to, by view name;
// files meeting letter case aside are tagged until no two meet, or refused if two still do.
func renderFilenames(views []model.ViewInfo, form view.Form) (map[string]string, error) {
	forms := make(map[string]view.Form, len(views))
	var names []string
	for _, info := range views {
		written := form
		if written == "" {
			written = info.Kind.MachineForm()
		}
		if info.Supported && info.Kind.SupportsForm(written) {
			forms[info.Name] = written
			names = append(names, info.Name)
		}
	}
	filenames, err := filename.Plan(names, func(name string, tagged bool) string {
		return renderFilename(name, forms[name], tagged)
	})
	var collision *filename.CollisionError
	if errors.As(err, &collision) {
		return nil, fmt.Errorf("views %s and %s have the same rendering path %s", collision.Names[0], collision.Names[1], collision.File)
	}
	return filenames, err
}

// renderOptions is what -render and -render-all write with: the text width,
// the palette -render-palette names and the placement -render-unplaced names,
// each of which must be one there is.
func renderOptions(width int) (view.Options, error) {
	options := view.Options{Width: width}
	if renderPalette != "" {
		palette, ok := view.ParsePalette(renderPalette)
		if !ok {
			return view.Options{}, fmt.Errorf("-render-palette: %w", &view.UnknownPaletteError{Name: renderPalette})
		}
		options.Palette = palette
	}
	unplaced, err := unplacedOption()
	if err != nil {
		return view.Options{}, err
	}
	options.Unplaced = unplaced
	return options, nil
}

// unplacedOption is the placement -render-unplaced names for the nodes a
// positioned drawing leaves unplaced, which must be one there is; none
// named is the default, leaving them undrawn.
func unplacedOption() (view.Unplaced, error) {
	if renderUnplaced == "" {
		return "", nil
	}
	unplaced, ok := view.ParseUnplaced(renderUnplaced)
	if !ok {
		return "", fmt.Errorf("-render-unplaced: %w", &view.UnknownUnplacedError{Name: renderUnplaced})
	}
	return unplaced, nil
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
	// The objects -instantiate names are created first, so a document's queries
	// run over what the session holds under those names.
	for _, name := range modelChecks.instantiate {
		created, err := sess.InstantiateReport(name)
		if err != nil {
			return nil, err
		}
		writeLines(os.Stderr, created.Lines)
		if len(created.FeatureValueErrors) > 0 {
			writeLines(os.Stderr, created.FeatureValueErrors)
			return nil, fmt.Errorf("%s did not materialize cleanly; nothing was rendered", name)
		}
		if created.Bounded {
			fmt.Fprintf(os.Stderr, "%s: materialization is bounded; not every feature value was materialized\n", name)
		}
	}
	// The runs -record-run names are made and written into the model before a
	// document is rendered, so its queries see the records.
	for _, invocation := range modelChecks.records {
		verdict := modelChecks.record(sess, invocation)
		writeLines(os.Stderr, verdict.Lines)
		if verdict.Status != repl.VerdictHolds {
			return nil, fmt.Errorf("%s: the run was not recorded; nothing was rendered", invocation)
		}
	}
	return sess, nil
}

// renderFilename is the file -render-all writes a view to: its qualified name with `::` as `.`, every
// unsafe byte as `%XX` (the first too under a Windows device-name stem), cut to fit, `~` and a hash when tagged, the extension.
func renderFilename(name string, form view.Form, tagged bool) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case c == ':' && i+1 < len(name) && name[i+1] == ':':
			b.WriteByte('.')
			i++
		case c < 0x20 || c == 0x7f || strings.IndexByte(unsafeFilenameBytes, c) >= 0:
			fmt.Fprintf(&b, "%%%02X", c)
		default:
			b.WriteByte(c)
		}
	}
	encoded := b.String()
	if filename.DeviceStem(encoded) {
		encoded = fmt.Sprintf("%%%02X", encoded[0]) + encoded[1:]
	}
	return filename.Fit(encoded, renderExtension(form), '%', tagged)
}

// unsafeFilenameBytes are the printable bytes a rendering filename encodes: path separators,
// the drive colon, the encoding's own `%`, the `.` standing for `::`, and what Windows reserves.
const unsafeFilenameBytes = "/\\:%.<>\"|?*"

func renderExtension(form view.Form) string {
	switch form {
	case view.FormMermaid:
		return ".mmd"
	case view.FormMarkdown:
		return ".md"
	case view.FormDot:
		return ".dot"
	case view.FormPlantUML:
		return ".puml"
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
func formList() string { return view.FormNames(view.Forms()) }

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
