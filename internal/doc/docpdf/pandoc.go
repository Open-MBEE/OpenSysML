package docpdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// This file hands a Markdown-reading converter the drawn diagrams, the typeset
// formulas and the captions as a pandoc Lua filter, so pandoc reads the
// Markdown backend's text as written and marks the artwork up on its own
// syntax tree.

// artworkFilterName is the Lua filter Render writes beside the Markdown.
const artworkFilterName = "artwork.lua"

// Notices written ahead of a diagram kept as source because the optional tool
// that draws its form was not found, naming the variable to set; the print
// stylesheet says the same over the HTML backend's page.
const (
	dotNotice      = "This diagram is written in Graphviz DOT, which the PDF backend did not draw: Graphviz was not found. Install it and point " + DotEnv + " at its dot to have the diagram drawn; its source follows."
	plantumlNotice = "This diagram is written in PlantUML, which the PDF backend did not draw: the PlantUML jar or java was not found. Point " + PlantUMLJarEnv + " at the jar, and " + JavaEnv + " at a java that is not on PATH, to have the diagram drawn; its source follows."
)

// writeArtworkFilter writes the filter for a document with diagrams (drawn,
// or kept as source under a notice), typeset formulas or captions, returning
// its name; a document with none needs no filter, and "" is returned.
func writeArtworkFilter(dir string, images []string, math formulas, captions []string) (string, error) {
	if len(images) == 0 && len(math.html) == 0 && len(captions) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("-- Marks the captions, swaps the diagram fences for the images drawn from\n")
	b.WriteString("-- them and the formulas for their typeset HTML, in document order.\n")
	b.WriteString("local forms = {")
	for i, form := range view.DiagramForms() {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(string(form) + " = true")
	}
	b.WriteString("}\n")
	b.WriteString("local images = {")
	for i, image := range images {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(luaString(image))
	}
	b.WriteString("}\n")
	b.WriteString("local math = {\n")
	for _, m := range math.keys() {
		b.WriteString("  [" + luaString(mathKey(m.Display, m.TeX())) + "] = " + luaString(math.html[m]) + ",\n")
	}
	b.WriteString("}\n")
	b.WriteString("local captions = {")
	for i, caption := range captions {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(luaString(caption))
	}
	b.WriteString("}\n")
	b.WriteString("local notices = {dot = " + luaString(dotNotice) + ", plantuml = " + luaString(plantumlNotice) + "}\n")
	b.WriteString("local fallbackNotice = " + luaString(docrender.GraphvizFallbackNotice) + "\n")
	b.WriteString(artworkFilterBody)
	if err := os.WriteFile(filepath.Join(dir, artworkFilterName), []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return artworkFilterName, nil
}

// mathKey is how the filter looks a formula up: pandoc's math kind and the
// LaTeX between the delimiters, trimmed as the filter trims it.
func mathKey(display bool, tex string) string {
	if display {
		return "display:" + tex
	}
	return "inline:" + tex
}

// artworkFilterBody is the filter proper, after the tables the document fills
// in. A caption is the emphasized paragraph the Markdown backend writes ahead
// of a table, a diagram or a formula block, matched by text in order, so an
// emphasized paragraph elsewhere stays one. A formula pandoc reads that was
// not typeset is a mismatch between the Markdown backend and Formulas, and
// fails the conversion rather than setting the LaTeX as text.
const artworkFilterBody = `local drawn = 0
local captioned = 0

local function trim(text)
  return (text:gsub("^%s+", ""):gsub("%s+$", ""))
end

local function words(text)
  return trim(text:gsub("%s+", " "))
end

local function isDisplayMath(block)
  return block.t == "Para" and #block.content == 1 and block.content[1].t == "Math"
    and block.content[1].mathtype == "DisplayMath"
end

-- The diagram form a fenced code block is written in, nil for other code.
local function formOf(block)
  if block == nil or block.t ~= "CodeBlock" then
    return nil
  end
  for _, class in ipairs(block.classes) do
    if forms[class] then
      return class
    end
  end
  return nil
end

local function isGroupKey(block)
  return block ~= nil and block.t == "Para" and #block.content == 1 and block.content[1].t == "Strong"
end

-- The HTML comment a table-kind diagram's rendering opens with, or one
-- stating a diagram's fallback form; the Markdown backend writes no other,
-- since prose escapes "<".
local function isRenderingComment(block)
  return block.t == "RawBlock" and block.format == "html" and block.text:sub(1, 4) == "<!--"
end

-- An image block is a Para holding the image alone, as the Markdown
-- backend writes it.
local function isImage(block)
  return block ~= nil and block.t == "Para" and #block.content == 1 and block.content[1].t == "Image"
end

local function isEmphasized(block)
  return block ~= nil and block.t == "Para" and #block.content == 1 and block.content[1].t == "Emph"
end

-- The notice the Markdown backend writes between a positioned diagram's
-- caption and the Mermaid source it fell back to without Graphviz.
local function isFallbackNotice(block)
  return isEmphasized(block) and words(pandoc.utils.stringify(block)) == words(fallbackNotice)
end

-- A caption heads a table (or a grouped table's first group key), a diagram
-- fence (or the fallback notice ahead of one), a table-kind diagram's
-- rendering comment, a formula block or an image.
local function isCaptioned(blocks, i)
  local block = blocks[i]
  if block == nil then
    return false
  end
  return block.t == "Table" or isDisplayMath(block) or isRenderingComment(block) or isImage(block)
    or formOf(block) ~= nil
    or (isFallbackNotice(block) and formOf(blocks[i + 1]) ~= nil)
    or (isGroupKey(block) and blocks[i + 1] ~= nil and blocks[i + 1].t == "Table")
end

local function isCaption(blocks, i)
  local block = blocks[i]
  if not isEmphasized(block) then
    return false
  end
  if not isCaptioned(blocks, i + 1) then
    return false
  end
  local expected = captions[captioned + 1]
  return expected ~= nil and words(pandoc.utils.stringify(block)) == words(expected)
end

local function markCaptions(blocks)
  for i = 1, #blocks do
    if isCaption(blocks, i) then
      captioned = captioned + 1
      blocks[i] = pandoc.Para({pandoc.Span(blocks[i].content, {class = "caption"})})
    end
  end
  return blocks
end

local function typeset(el)
  local kind = el.mathtype == "DisplayMath" and "display:" or "inline:"
  local html = math[kind .. trim(el.text)]
  if html == nil then
    error("formula not typeset ahead of conversion: " .. el.text)
  end
  return html
end

return {
  { Blocks = markCaptions },
  {
    Para = function(el)
      if #el.content == 1 and el.content[1].t == "Math" and el.content[1].mathtype == "DisplayMath" then
        return pandoc.RawBlock("html", '<div class="formula">' .. typeset(el.content[1]) .. '</div>')
      end
      return nil
    end,
    CodeBlock = function(el)
      local form = formOf(el)
      if form == nil then
        return nil
      end
      drawn = drawn + 1
      local image = images[drawn]
      if image ~= nil and image ~= "" then
        return pandoc.Para({pandoc.Image({}, image)})
      end
      local notice = notices[form]
      if notice == nil then
        return nil
      end
      return {pandoc.Para({pandoc.Emph({pandoc.Str(notice)})}), el}
    end,
  },
  {
    Math = function(el)
      return pandoc.RawInline("html", '<span class="math">' .. typeset(el) .. '</span>')
    end,
  },
}
`

// luaString writes text as a Lua string literal: the quote, the backslash and
// every control byte escaped, so any LaTeX or HTML round-trips byte for byte.
func luaString(text string) string {
	var b strings.Builder
	b.Grow(len(text) + 2)
	b.WriteByte('"')
	for i := 0; i < len(text); i++ {
		switch ch := text[i]; {
		case ch == '"' || ch == '\\':
			b.WriteByte('\\')
			b.WriteByte(ch)
		case ch == '\n':
			b.WriteString(`\n`)
		case ch == '\r':
			b.WriteString(`\r`)
		case ch < 0x20 || ch == 0x7f:
			fmt.Fprintf(&b, `\%03d`, ch)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteByte('"')
	return b.String()
}
