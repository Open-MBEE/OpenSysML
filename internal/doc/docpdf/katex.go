package docpdf

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// This file typesets a document's formulas ahead of conversion: KaTeX renders
// each LaTeX source to HTML that any paged-media engine lays out with the
// KaTeX fonts, so no converter needs a TeX or MathML engine of its own.

// formula is one LaTeX source to typeset, inline or on a display line.
type formula struct {
	Source  string
	Display bool
}

// formulas is the typeset HTML of a document's formulas and the stylesheet it
// needs, as a path within the working directory; a document without formulas
// has neither.
type formulas struct {
	html map[formula]string
	css  string
}

// typeset returns the HTML for one formula; one never typeset shows its
// source, escaped.
func (f formulas) typeset(m formula) string {
	if typeset, ok := f.html[m]; ok {
		return typeset
	}
	return html.EscapeString(m.Source)
}

// collectFormulas lists the distinct formulas the blocks show, in document
// order: display blocks, and the inline math of every prose line.
func collectFormulas(blocks []block) []formula {
	var list []formula
	seen := map[formula]bool{}
	add := func(m formula) {
		if !seen[m] {
			seen[m] = true
			list = append(list, m)
		}
	}
	inline := func(text string) {
		for _, seg := range splitMath(text) {
			if seg.math {
				add(formula{Source: seg.text})
			}
		}
	}
	for _, blk := range blocks {
		switch blk.Kind {
		case blockHeading, blockParagraph, blockCaption:
			inline(blk.Text)
		case blockList:
			for _, item := range blk.Items {
				inline(item)
			}
		case blockTable:
			for _, cell := range blk.Header {
				inline(cell)
			}
			for _, row := range blk.Rows {
				for _, cell := range row {
					inline(cell)
				}
			}
		case blockFormula:
			add(formula{Source: blk.Source, Display: true})
		}
	}
	return list
}

// Where the KaTeX stylesheet and its fonts are copied to within the working
// directory; the stylesheet names the fonts relative to itself.
const (
	katexDir     = "katex"
	katexCSSName = "katex.min.css"
	katexFontDir = "fonts"
)

// renderFormulas typesets each formula in blocks to HTML with the katex
// command-line tool and copies its stylesheet and fonts into dir. A document
// without formulas needs no KaTeX at all.
func renderFormulas(dir string, blocks []block) (formulas, error) {
	list := collectFormulas(blocks)
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
	typeset := formulas{html: make(map[formula]string, len(list)), css: filepath.ToSlash(filepath.Join(katexDir, katexCSSName))}
	for i, m := range list {
		input := fmt.Sprintf("formula-%d.tex", i+1)
		output := fmt.Sprintf("formula-%d.html", i+1)
		if err := os.WriteFile(filepath.Join(dir, input), []byte(m.Source+"\n"), 0o600); err != nil {
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

// markdownWithFormulas rewrites the document's Markdown with each formula
// replaced by its typeset HTML in a raw-attribute span or block, which a
// converter reading the Markdown passes through without interpreting.
// Fenced code blocks are kept as written, dollars and all.
func markdownWithFormulas(markdown string, typeset formulas) string {
	lines := strings.Split(markdown, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "```") {
			end := fenceEndAt(lines, i+1, "```")
			if end < 0 {
				end = len(lines) - 1
			}
			out = append(out, lines[i:end+1]...)
			i = end
			continue
		}
		if lines[i] == mathFence {
			end := fenceEndAt(lines, i+1, mathFence)
			if end < 0 {
				end = len(lines)
			}
			source := strings.Join(lines[i+1:end], "\n")
			out = append(out, "```{=html}", `<div class="formula">`+typeset.typeset(formula{Source: source, Display: true})+"</div>", "```")
			i = end
			continue
		}
		out = append(out, lineWithFormulas(lines[i], typeset))
	}
	return strings.Join(out, "\n")
}

// lineWithFormulas replaces each inline formula of one line with its typeset
// HTML in a raw-attribute code span, keeping the prose around it as written.
func lineWithFormulas(line string, typeset formulas) string {
	segs := splitMath(line)
	if len(segs) == 1 && !segs[0].math {
		return line
	}
	var b strings.Builder
	for _, seg := range segs {
		if !seg.math {
			b.WriteString(seg.text)
			continue
		}
		rendered := `<span class="math">` + typeset.typeset(formula{Source: seg.text}) + "</span>"
		fence := strings.Repeat("`", longestBacktickRun(rendered)+1)
		b.WriteString(fence + rendered + fence + "{=html}")
	}
	return b.String()
}

// longestBacktickRun is the length of the longest run of backticks in text.
func longestBacktickRun(text string) int {
	longest := 0
	for i := 0; i < len(text); {
		run := backtickRun(text, i)
		if run > longest {
			longest = run
		}
		i += run + 1
	}
	return longest
}
