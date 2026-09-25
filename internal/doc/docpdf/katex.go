package docpdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
)

// This file typesets a document's formulas ahead of conversion: KaTeX renders
// each LaTeX source to HTML that any paged-media engine lays out with the
// KaTeX fonts, so no converter needs a TeX or MathML engine of its own.

// formulas is the typeset HTML of a document's formulas, keyed as
// docrender.Formulas lists them and kept in that order, and the stylesheet
// they need as a path within the working directory; a document without
// formulas has neither.
type formulas struct {
	list []docrender.Formula
	html map[docrender.Formula]string
	css  string
}

// keys lists the typeset formulas in document order.
func (f formulas) keys() []docrender.Formula { return f.list }

// Where the KaTeX stylesheet and its fonts are copied to within the working
// directory; the stylesheet names the fonts relative to itself.
const (
	katexDir     = "katex"
	katexCSSName = "katex.min.css"
	katexFontDir = "fonts"
)

// renderFormulas typesets each formula to HTML with the katex command-line
// tool and copies its stylesheet and fonts into dir. A document without
// formulas needs no KaTeX at all.
func renderFormulas(dir string, list []docrender.Formula) (formulas, error) {
	if len(list) == 0 {
		return formulas{}, nil
	}
	katex, err := katexTool.locate("")
	if err != nil {
		return formulas{}, err
	}
	css, err := katexStylesheet(katex)
	if err != nil {
		return formulas{}, err
	}
	if err := copyStylesheet(css, filepath.Join(dir, katexDir)); err != nil {
		return formulas{}, err
	}
	typeset := formulas{list: list, html: make(map[docrender.Formula]string, len(list)), css: filepath.ToSlash(filepath.Join(katexDir, katexCSSName))}
	for i, m := range list {
		input := fmt.Sprintf("formula-%d.tex", i+1)
		output := fmt.Sprintf("formula-%d.html", i+1)
		if err := os.WriteFile(filepath.Join(dir, input), []byte(m.TeX()+"\n"), 0o600); err != nil {
			return formulas{}, err
		}
		// HTML alone: the MathML KaTeX would add for assistive technology
		// carries the LaTeX source, which paged-media engines set as text.
		args := []string{"--format", "html", "--input", input, "--output", output}
		if m.Display {
			args = append(args, "--display-mode")
		}
		if err := runToolWith(dir, katex, katexDetail, args...); err != nil {
			return formulas{}, err
		}
		rendered, err := os.ReadFile(filepath.Join(dir, output)) // #nosec G304 -- the working directory is this package's own
		if err != nil || strings.TrimSpace(string(rendered)) == "" {
			return formulas{}, &Error{Kind: ErrorToolFailed, Tool: katexTool.name, Detail: "wrote no HTML for " + input}
		}
		typeset.html[m] = strings.TrimSpace(string(rendered))
	}
	return typeset, nil
}

// katexDetail picks the parse error out of katex's stderr, which otherwise
// ends in a stack trace, falling back to the last lines.
func katexDetail(stderr string) string {
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "KaTeX parse error") {
			return strings.TrimPrefix(strings.TrimSpace(line), "ParseError: ")
		}
	}
	return tail(stderr)
}

// katexStylesheet finds katex.min.css: where KatexCSSEnv points, or in the
// dist directory of the package the katex executable belongs to.
func katexStylesheet(katex string) (string, error) {
	if override := strings.TrimSpace(os.Getenv(KatexCSSEnv)); override != "" {
		if info, err := os.Stat(override); err != nil || info.IsDir() {
			return "", &Error{Kind: ErrorToolMissing, Tool: override, EnvVar: KatexCSSEnv}
		}
		return override, nil
	}
	resolved, err := filepath.EvalSymlinks(katex)
	if err != nil {
		resolved = katex
	}
	css := filepath.Join(filepath.Dir(resolved), "dist", katexCSSName)
	if info, err := os.Stat(css); err != nil || info.IsDir() {
		return "", &Error{Kind: ErrorToolMissing, Tool: katexCSSName, EnvVar: KatexCSSEnv}
	}
	return css, nil
}

// copyStylesheet copies the KaTeX stylesheet and the fonts directory beside
// it into dest.
func copyStylesheet(css, dest string) error {
	if err := os.MkdirAll(filepath.Join(dest, katexFontDir), 0o700); err != nil {
		return err
	}
	if err := copyFile(css, filepath.Join(dest, katexCSSName)); err != nil {
		return err
	}
	fontDir := filepath.Join(filepath.Dir(css), katexFontDir)
	fonts, err := os.ReadDir(fontDir)
	if err != nil {
		return &Error{Kind: ErrorToolMissing, Tool: fontDir, EnvVar: KatexCSSEnv}
	}
	for _, font := range fonts {
		if font.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(fontDir, font.Name()), filepath.Join(dest, katexFontDir, font.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies one regular file, readable by its owner alone.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src) // #nosec G304 -- the source is the operator's own KaTeX installation
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
