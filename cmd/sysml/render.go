package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/chzyer/readline"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
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
		artifact, err := rendering.WriteWith(writtenForm, options)
		if err != nil {
			if errors.Is(err, view.ErrWrongForm) {
				reportRenderSkip(info.Name, err.Error())
				continue
			}
			return err
		}
		path := filepath.Join(renderAllDir, renderFilename(info.Name, writtenForm))
		// A filesystem that ignores letter case hands two such paths one file, so the key ignores it too.
		key := caseFolded(path)
		if previous, exists := destinations[key]; exists {
			return fmt.Errorf("views %s and %s have the same rendering path %s", previous, info.Name, path)
		}
		destinations[key] = info.Name
		if err := writeArtifactFile(path, artifact, writtenForm); err != nil {
			return err
		}
	}
	return nil
}

// caseFolded is text under simple Unicode case folding: two texts fold alike
// exactly when strings.EqualFold holds of them.
func caseFolded(text string) string {
	var b strings.Builder
	for _, r := range text {
		least := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			least = min(least, f)
		}
		b.WriteRune(least)
	}
	return b.String()
}

// renderOptions is what -render and -render-all write with: the text width,
// and the palette -render-palette names, which must be one there is.
func renderOptions(width int) (view.Options, error) {
	options := view.Options{Width: width}
	if renderPalette != "" {
		palette, ok := view.ParsePalette(renderPalette)
		if !ok {
			return view.Options{}, fmt.Errorf("-render-palette: %w", &view.UnknownPaletteError{Name: renderPalette})
		}
		options.Palette = palette
	}
	return options, nil
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
	return sess, nil
}

// renderFilename is the file -render-all writes a view to: its qualified name with `::` as `.`,
// every unsafe byte as `%XX` (the first too under a Windows device-name stem), cut to fit, the extension.
func renderFilename(name string, form view.Form) string {
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
	filename := b.String()
	if stem, _, _ := strings.Cut(filename, "."); windowsDeviceNames[strings.ToUpper(strings.TrimRight(stem, " "))] {
		filename = fmt.Sprintf("%%%02X", filename[0]) + filename[1:]
	}
	ext := renderExtension(form)
	if len(filename)+len(ext) > maxFilenameBytes {
		sum := sha256.Sum256([]byte(filename))
		tag := "~" + hex.EncodeToString(sum[:filenameTagBytes])
		filename = cutFilename(filename, maxFilenameBytes-len(ext)-len(tag)) + tag
	}
	return filename + ext
}

// maxFilenameBytes is the longest name every common filesystem takes for one path component;
// filenameTagBytes of the encoded name's hash keep a cut name apart from its neighbours.
const (
	maxFilenameBytes = 255
	filenameTagBytes = 8
)

// cutFilename is the longest prefix of an encoded filename within n bytes that
// splits neither a UTF-8 sequence nor a `%XX` escape.
func cutFilename(filename string, n int) string {
	for n > 0 && n < len(filename) && !utf8.RuneStart(filename[n]) {
		n--
	}
	if i := strings.LastIndexByte(filename[:n], '%'); i >= 0 && i > n-3 {
		n = i
	}
	return filename[:n]
}

// unsafeFilenameBytes are the printable bytes a rendering filename encodes: path separators,
// the drive colon, the encoding's own `%`, the `.` standing for `::`, and what Windows reserves.
const unsafeFilenameBytes = "/\\:%.<>\"|?*"

// windowsDeviceNames are the stems Windows reads as devices whatever the extension,
// trailing spaces and letter case aside: the serial and printer ports include the
// superscript digits Windows counts among them.
var windowsDeviceNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM0": true, "COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"COM¹": true, "COM²": true, "COM³": true,
	"LPT0": true, "LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	"LPT¹": true, "LPT²": true, "LPT³": true,
}

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
