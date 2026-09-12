package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
	"github.com/Open-MBEE/OpenSysML/internal/docpdf"
	"github.com/Open-MBEE/OpenSysML/internal/fsutil"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// runRenderDocument renders the document -render-document names of the model
// named on the command line, writing the artifact to -o or to stdout and
// every notice to stderr.
func runRenderDocument(files []string) error {
	form, err := documentForm()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no model to render; name the files the document is declared in, as `sysml model.sysml -render-document MyReport`")
	}
	sess, err := loadRenderingModel(files)
	if err != nil {
		return err
	}
	if form == docFormHTML {
		opts, err := htmlOptions()
		if err != nil {
			return err
		}
		rendered, err := sess.RenderDocumentHTML(renderDoc, opts)
		if err != nil {
			return err
		}
		return writeArtifact(rendered, formHTML)
	}
	markdown, err := sess.RenderDocumentMarkdown(renderDoc)
	if err != nil {
		return err
	}
	if form == docFormPDF {
		pdf, err := docpdf.Render(markdown, pdfEngine, docpdf.Options{
			TitlePage:      pdfTitlePage,
			TOC:            pdfTOC,
			NumberSections: pdfNumbering,
		})
		if err != nil {
			return err
		}
		return writePDFArtifact(pdf)
	}
	return writeArtifact(markdown, view.FormMarkdown)
}

// runRenderDocuments renders every document definition of the model named on
// the command line as linked files in the directory -render-documents names,
// so cross-document references resolve on disk.
func runRenderDocuments(files []string) error {
	form, err := documentSetForm()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no model to render; name the files the documents are declared in, as `sysml model.sysml -render-documents rendered`")
	}
	sess, err := loadRenderingModel(files)
	if err != nil {
		return err
	}
	// A set links its stylesheets as files beside the pages, so a reader
	// downloads each once and edits it in one place.
	var sheets []repl.RenderedDocument
	var documents []repl.RenderedDocument
	if form == docFormHTML {
		links, assets, err := setStylesheets()
		if err != nil {
			return err
		}
		sheets = assets
		opts := documentOptions()
		opts.Stylesheets = links
		// The set links its sheets rather than inlining them in each page.
		opts.NoDefaultStylesheet = true
		documents, err = sess.RenderDocumentSetHTML(opts)
		if err != nil {
			return err
		}
	} else {
		documents, err = sess.RenderDocumentSetMarkdown()
		if err != nil {
			return err
		}
	}
	if len(documents) == 0 {
		return errors.New("the model declares no documents; nothing was rendered")
	}
	documents = append(documents, sheets...)
	if err := os.MkdirAll(renderDocsDir, 0o750); err != nil {
		return fmt.Errorf("create rendering directory %s: %w", renderDocsDir, err)
	}
	return commitDocumentSet(documents, form)
}

// documentOptions carries the flags shaping the document itself, leaving its
// stylesheets to the caller.
func documentOptions() docrender.HTMLOptions {
	return docrender.HTMLOptions{
		Fragment:            htmlFragment,
		NoDefaultStylesheet: htmlNoCSS,
		Theme:               htmlTheme,
		TitlePage:           pdfTitlePage,
		TOC:                 pdfTOC,
		NumberSections:      pdfNumbering,
		MermaidScript:       mermaidScriptURL(),
	}
}

// The -html-mermaid value naming the pinned CDN release.
const mermaidCDN = "cdn"

// mermaidScriptURL resolves -html-mermaid: cdn names the pinned release,
// anything else is the URL to load as it stands.
func mermaidScriptURL() string {
	if htmlMermaid == mermaidCDN {
		return docrender.MermaidScriptURL
	}
	return htmlMermaid
}

// setStylesheets resolves the stylesheets of an HTML set: the default sheet
// and each -html-css file are written beside the pages and linked, in
// command-line order, and a -html-css URL is linked as it stands.
func setStylesheets() ([]docrender.Stylesheet, []repl.RenderedDocument, error) {
	var links []docrender.Stylesheet
	var assets []repl.RenderedDocument
	taken := make(map[string]bool, len(htmlCSS)+1)
	add := func(name, content string) {
		name = setStylesheetName(name, taken)
		links = append(links, docrender.LinkedStylesheet(name))
		assets = append(assets, repl.RenderedDocument{Name: name, FileName: name, Content: content})
	}
	if !htmlNoCSS {
		base, err := docrender.ThemeStylesheet(htmlTheme)
		if err != nil {
			return nil, nil, err
		}
		add(docrender.StylesheetFileName, base)
	}
	for _, css := range htmlCSS {
		if isWebURL(css) {
			links = append(links, docrender.LinkedStylesheet(css))
			continue
		}
		// #nosec G304 -- the stylesheet is the file the run asked to style with.
		content, err := os.ReadFile(css)
		if err != nil {
			return nil, nil, fmt.Errorf("read stylesheet %s: %w", css, err)
		}
		add(filepath.Base(css), string(content))
	}
	return links, assets, nil
}

// setStylesheetName derives a stylesheet's asset name: escaped so the link a
// page carries names the file, and kept apart from a name already taken.
func setStylesheetName(name string, taken map[string]bool) string {
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = docrender.StylesheetFileName
	}
	name = shortenStylesheetName(escapeStylesheetName(name))
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	unique := name
	for n := 2; taken[unique]; n++ {
		unique = fmt.Sprintf("%s-%d%s", stem, n, ext)
	}
	taken[unique] = true
	return unique
}

// escapeStylesheetName encodes every byte outside [A-Za-z0-9._-] as "." and two
// hex digits, as anchors are, so a browser requests the file the set wrote.
func escapeStylesheetName(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		ch := name[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '_', ch == '-', ch == '.':
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, ".%02X", ch)
		}
	}
	return b.String()
}

// maxStylesheetName bounds an asset name well inside the 255-byte component
// limit filesystems commonly impose, leaving room for a uniquifying suffix.
const maxStylesheetName = 240

// shortenStylesheetName keeps an escaped name within the file name limit,
// ending an over-long one in a digest of the whole so two stay distinct.
func shortenStylesheetName(name string) string {
	if len(name) <= maxStylesheetName {
		return name
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))[:16]
	ext := filepath.Ext(name)
	if len(ext) > maxStylesheetName/2 {
		ext = ""
	}
	stem := strings.TrimSuffix(name, ext)[:maxStylesheetName-len(ext)-len(digest)-1]
	// A truncation must not end mid-escape, which would name another byte.
	switch {
	case strings.HasSuffix(stem, "."):
		stem = stem[:len(stem)-1]
	case len(stem) >= 2 && stem[len(stem)-2] == '.':
		stem = stem[:len(stem)-2]
	}
	return stem + "-" + digest + ext
}

// htmlOptions resolves the HTML flags, reading each -html-css file and
// linking each -html-css URL.
func htmlOptions() (docrender.HTMLOptions, error) {
	opts := documentOptions()
	for _, css := range htmlCSS {
		if isWebURL(css) {
			opts.Stylesheets = append(opts.Stylesheets, docrender.LinkedStylesheet(css))
			continue
		}
		// #nosec G304 -- the stylesheet is the file the run asked to style with.
		content, err := os.ReadFile(css)
		if err != nil {
			return opts, fmt.Errorf("read stylesheet %s: %w", css, err)
		}
		opts.Stylesheets = append(opts.Stylesheets, docrender.InlineStylesheet(string(content)))
	}
	return opts, nil
}

// isWebURL reports whether a value is an http(s) or protocol-relative URL a
// page can load a resource from, rather than a local file.
func isWebURL(css string) bool {
	lower := strings.ToLower(css)
	for _, scheme := range []string{"http://", "https://", "//"} {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	return false
}

// runDefaultStylesheet writes the default document stylesheet, with the
// -html-theme overrides when one is named, so a reader styling the HTML
// starts from the sheet it overrides.
func runDefaultStylesheet() error {
	css, err := docrender.ThemeStylesheet(htmlTheme)
	if err != nil {
		return err
	}
	return writeArtifact(css, formCSS)
}

// commitDocumentSet writes a rendered document set all-or-nothing: every
// document is staged beside its destination under a fresh temporary name,
// existing destinations are preserved through hard-linked backups, and the
// staged set is renamed over the destinations; any failure restores the
// directory's previous contents, and an interruption leaves every
// destination in place, old or new, never missing. A symlink is written
// through to its target, keeping the link. A pipe or device is written as it
// stands, last, after the regular set commits; bytes a pipe or device already
// received cannot be recalled, so the all-or-nothing guarantee covers the
// regular files of the set only.
func commitDocumentSet(documents []repl.RenderedDocument, form string) error {
	targets := make([]string, len(documents))
	direct := make([]bool, len(documents))
	replaced := make([]bool, len(documents))
	perms := make([]os.FileMode, len(documents))
	// Two destinations may be symlinks resolving to one file; such a set
	// would silently keep only the later document.
	resolved := make(map[string]string, len(documents))
	folding := make(map[string]bool)
	infos := make([]os.FileInfo, len(documents))
	for i, document := range documents {
		path := filepath.Join(renderDocsDir, document.FileName)
		target, info, err := export.Destination(path)
		if err != nil {
			return fmt.Errorf("cannot write %s: %w", path, err)
		}
		if info != nil && info.IsDir() {
			return fmt.Errorf("cannot write %s: it is a directory", path)
		}
		key := target
		// Dangling links can reach one absent file through aliased
		// directories; canonicalize the directory so they still collide.
		if dir, dirErr := filepath.EvalSymlinks(filepath.Dir(target)); dirErr == nil {
			key = filepath.Join(dir, filepath.Base(target))
		}
		dir := filepath.Dir(key)
		folds, known := folding[dir]
		if !known {
			folds = foldsCase(dir)
			folding[dir] = folds
		}
		if folds {
			key = strings.ToLower(key)
		}
		if other, ok := resolved[key]; ok {
			return fmt.Errorf("cannot write %s: %s and %s both resolve to %s; repoint one of the links so each document has its own file", path, other, document.FileName, target)
		}
		resolved[key] = document.FileName
		for j := range i {
			if info != nil && infos[j] != nil && os.SameFile(info, infos[j]) {
				return fmt.Errorf("cannot write %s: %s and %s both resolve to one file; repoint one of the links so each document has its own file", path, documents[j].FileName, document.FileName)
			}
		}
		infos[i] = info
		targets[i] = target
		replaced[i] = info != nil
		perms[i] = 0o644
		if info != nil {
			perms[i] = info.Mode().Perm()
			direct[i] = !info.Mode().IsRegular()
		}
	}
	staged := make([]string, len(documents))
	backups := make([]string, len(documents))
	moved := make([]bool, len(documents))
	committed := make([]bool, len(documents))
	rollback := func() {
		for i := range documents {
			if staged[i] != "" {
				_ = os.Remove(staged[i])
			}
			switch {
			case committed[i] && backups[i] != "":
				_ = fsutil.Replace(backups[i], targets[i])
			case committed[i]:
				_ = os.Remove(targets[i])
			case backups[i] != "":
				restoreBackup(targets[i], backups[i], moved[i])
			}
		}
	}
	for i, document := range documents {
		if direct[i] {
			continue
		}
		name, err := stageDocument(document, perms[i], filepath.Dir(targets[i]))
		staged[i] = name
		if err != nil {
			rollback()
			return err
		}
	}
	for i := range documents {
		if direct[i] || !replaced[i] {
			continue
		}
		name, movedAside, err := backUp(targets[i])
		if err != nil {
			rollback()
			return fmt.Errorf("write %s: %w", targets[i], err)
		}
		backups[i] = name
		moved[i] = movedAside
	}
	for i := range documents {
		if direct[i] {
			continue
		}
		if err := fsutil.Replace(staged[i], targets[i]); err != nil {
			rollback()
			return fmt.Errorf("write %s: %w", targets[i], err)
		}
		staged[i] = ""
		committed[i] = true
	}
	// Flush the renames before the backups go, so a crash keeps one of the
	// two sets whole.
	dirs := make(map[string]bool)
	for i := range documents {
		if committed[i] {
			dirs[filepath.Dir(targets[i])] = true
		}
	}
	for dir := range dirs {
		syncDir(dir)
	}
	for i, document := range documents {
		if !direct[i] {
			continue
		}
		path := filepath.Join(renderDocsDir, document.FileName)
		if _, err := export.WriteFile(path, documentBytes(document)); err != nil {
			rollback()
			return err
		}
	}
	for i, document := range documents {
		if backups[i] != "" {
			_ = os.Remove(backups[i])
		}
		path := filepath.Join(renderDocsDir, document.FileName)
		what := ""
		if replaced[i] {
			what = ", replaced the existing file"
		}
		fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", path, setForm(document, form), len(documentBytes(document)), what)
	}
	return nil
}

// documentBytes is what a rendered document writes: its content with one
// trailing newline.
func documentBytes(document repl.RenderedDocument) []byte {
	return []byte(strings.TrimRight(document.Content, "\n") + "\n")
}

// setForm names the form a file of a rendered set was written in; the
// stylesheets of an HTML set are CSS, not documents.
func setForm(document repl.RenderedDocument, form string) view.Form {
	switch {
	case strings.HasSuffix(document.FileName, ".css"):
		return formCSS
	case form == docFormHTML:
		return formHTML
	}
	return view.FormMarkdown
}

// stageDocument writes a document to a fresh temporary file beside its
// destination, so staging never touches an existing file and the commit
// rename never crosses a filesystem.
func stageDocument(document repl.RenderedDocument, perm os.FileMode, dir string) (string, error) {
	file, err := os.CreateTemp(dir, ".sysml-staged-*")
	if err != nil {
		return "", fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	name := file.Name()
	if _, err := file.Write(documentBytes(document)); err != nil {
		_ = file.Close()
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	// Without this a crash after the commit renames can leave an empty file
	// where a document was.
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	if err := file.Close(); err != nil {
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	// #nosec G302 -- a rendered document is not a secret, and an existing
	// destination keeps its permissions.
	if err := os.Chmod(name, perm); err != nil {
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	return name, nil
}

// syncDir flushes a directory's entries after renames. Best-effort: the set
// is already written, so a filesystem that refuses is not a failed render.
func syncDir(dir string) {
	// #nosec G304 -- dir holds a destination the caller asked to write.
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = f.Sync()
	_ = f.Close()
}

// foldsCase reports whether a directory treats names differing only by
// letter case as one entry, probed with a fresh file, so aliases of an
// absent file are detected before a document lands on them.
func foldsCase(dir string) bool {
	file, err := os.CreateTemp(dir, ".sysml-case-*a")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	defer func() { _ = os.Remove(name) }()
	lower, err := os.Stat(name)
	if err != nil {
		return false
	}
	upper, err := os.Stat(name[:len(name)-1] + "A")
	if err != nil {
		return false
	}
	return os.SameFile(lower, upper)
}

// restoreBackup moves a backup back over a missing or set-aside destination,
// and sheds a hard-linked backup when the destination is still in place.
func restoreBackup(target, backup string, movedAside bool) {
	if movedAside {
		_ = os.Rename(backup, target)
		return
	}
	if _, err := os.Lstat(target); err != nil {
		_ = os.Rename(backup, target)
		return
	}
	_ = os.Remove(backup)
}

// backUp preserves an existing destination so a failed commit can restore
// it: a hard link keeps the destination itself in place through the commit,
// falling back to a set-aside rename where the filesystem has no hard links.
func backUp(target string) (name string, movedAside bool, err error) {
	file, err := os.CreateTemp(filepath.Dir(target), ".sysml-replaced-*")
	if err != nil {
		return "", false, err
	}
	name = file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return "", false, err
	}
	if err := os.Remove(name); err != nil {
		return "", false, err
	}
	if os.Link(target, name) == nil {
		return name, false, nil
	}
	if err := os.Rename(target, name); err != nil {
		return "", false, err
	}
	return name, true, nil
}

// The forms -doc-form takes.
const (
	docFormMarkdown = "markdown"
	docFormHTML     = "html"
	docFormPDF      = "pdf"
)

// The forms a written artifact is reported in that no rendering form names.
const (
	formHTML view.Form = "html"
	formCSS  view.Form = "css"
)

// htmlFlagsGiven reports whether the run asked for anything only HTML output
// shapes.
func htmlFlagsGiven() bool {
	return htmlPageFlagsGiven() || htmlTheme != ""
}

// htmlPageFlagsGiven reports whether the run asked for anything only a
// rendered HTML page carries; -html-theme is left out, as it also shapes the
// sheet -html-default-css writes.
func htmlPageFlagsGiven() bool {
	return len(htmlCSS) > 0 || htmlNoCSS || htmlFragment || htmlMermaid != ""
}

// checkTheme rejects an -html-theme value that names no bundled theme.
func checkTheme() error {
	_, err := docrender.ThemeStylesheet(htmlTheme)
	return err
}

// checkMermaidScript rejects an -html-mermaid value that is neither cdn nor a
// URL a page can load a script from.
func checkMermaidScript() error {
	if htmlMermaid == "" || htmlMermaid == mermaidCDN || isWebURL(htmlMermaid) {
		return nil
	}
	return fmt.Errorf("-html-mermaid takes %s or the URL of a Mermaid script; %q is neither", mermaidCDN, htmlMermaid)
}

// documentSetForm resolves -doc-form for -render-documents, which writes a
// linked set of files rather than one artifact.
func documentSetForm() (string, error) {
	switch form := docFormOrDefault(); form {
	case docFormMarkdown:
		if htmlFlagsGiven() {
			return "", errors.New("the -html- options shape HTML output; ask for it with -doc-form html")
		}
		if pdfEngine != "" {
			return "", errors.New("-pdf-engine shapes PDF output, which -render-documents does not write; render one document at a time with -render-document")
		}
		if pdfTitlePage || pdfTOC || pdfNumbering {
			return "", errors.New("the title page, contents and numbering options shape HTML output; ask for it with -doc-form html")
		}
		return form, nil
	case docFormHTML:
		if pdfEngine != "" {
			return "", errors.New("-pdf-engine shapes PDF output; -doc-form html needs no external converter")
		}
		if err := checkMermaidScript(); err != nil {
			return "", err
		}
		if err := checkThemeUse(); err != nil {
			return "", err
		}
		return form, nil
	case docFormPDF:
		return "", errors.New("-render-documents writes a linked set of files, which PDF cannot be; render one document at a time with -render-document -doc-form pdf")
	default:
		return "", unknownDocumentForm(form)
	}
}

// checkThemeUse rejects an -html-theme that names no bundled theme or would
// layer over a default sheet -html-no-default-css leaves out.
func checkThemeUse() error {
	if htmlNoCSS && htmlTheme != "" {
		return errors.New("-html-theme layers a theme over the default stylesheet, which -html-no-default-css leaves out; ask for one or the other")
	}
	return checkTheme()
}

func docFormOrDefault() string {
	if docForm == "" {
		return docFormMarkdown
	}
	return docForm
}

func unknownDocumentForm(form string) error {
	return fmt.Errorf("unknown document form %q; -doc-form takes %s, %s or %s", form, docFormMarkdown, docFormHTML, docFormPDF)
}

// documentForm resolves -doc-form and checks the flag combination: the PDF
// options apply to PDF output, and a PDF is written to a file, not stdout.
func documentForm() (string, error) {
	switch form := docFormOrDefault(); form {
	case docFormMarkdown:
		if htmlFlagsGiven() {
			return "", errors.New("the -html- options shape HTML output; ask for it with -doc-form html")
		}
		if pdfEngine != "" || pdfTitlePage || pdfTOC || pdfNumbering {
			return "", errors.New("-pdf-engine and the title page, contents and numbering options shape HTML and PDF output; ask for one with -doc-form html or -doc-form pdf")
		}
		return form, nil
	case docFormHTML:
		if pdfEngine != "" {
			return "", errors.New("-pdf-engine shapes PDF output; -doc-form html needs no external converter")
		}
		if htmlFragment && htmlNoCSS {
			return "", errors.New("-html-fragment already writes no stylesheet; -html-no-default-css leaves the default sheet out of a whole page")
		}
		if htmlFragment && len(htmlCSS) > 0 {
			return "", errors.New("-html-fragment writes the document element alone, with no place for a stylesheet; style the page you embed it in")
		}
		if htmlFragment && htmlMermaid != "" {
			return "", errors.New("-html-fragment writes the document element alone, with no place for a script; load Mermaid in the page you embed it in")
		}
		if err := checkMermaidScript(); err != nil {
			return "", err
		}
		if htmlFragment && htmlTheme != "" {
			return "", errors.New("-html-fragment writes the document element alone, with no place for a stylesheet; -html-theme styles a whole page")
		}
		if err := checkThemeUse(); err != nil {
			return "", err
		}
		return form, nil
	case docFormPDF:
		if htmlFlagsGiven() {
			return "", errors.New("the -html- options shape HTML output; ask for it with -doc-form html")
		}
		if outputPath == "" {
			return "", errors.New("-doc-form pdf writes a binary artifact; name the file to write with -o")
		}
		return form, nil
	default:
		return "", unknownDocumentForm(form)
	}
}

// writePDFArtifact writes the PDF bytes to -o, byte-exact.
func writePDFArtifact(pdf []byte) error {
	replaced, err := export.WriteFile(outputPath, pdf)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (pdf, %d bytes%s)\n", outputPath, len(pdf), what)
	return nil
}
