package migrate

import (
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// valueExpr writes a UML value specification as a v2 expression. ok is false
// when it has no v2 form; note explains an approximation or the refusal.
func (m *migration) valueExpr(v, scope *xmi.Element) (expr string, ok bool, note string) {
	switch v.Type {
	case "LiteralInteger", "LiteralUnlimitedNatural":
		val := v.Attrs["value"]
		if val == "" {
			val = "0"
		}
		if val == "*" {
			return "", false, "an unlimited natural value has no v2 expression outside a multiplicity"
		}
		return val, true, ""
	case "LiteralReal":
		val := v.Attrs["value"]
		if val == "" {
			val = "0.0"
		}
		f, err := strconv.ParseFloat(val, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return "", false, "real literal " + strconv.Quote(val) + " is not a finite number"
		}
		if !strings.ContainsAny(val, ".eE") {
			val += ".0"
		}
		return val, true, ""
	case "LiteralBoolean":
		switch v.Attrs["value"] {
		case "true", "1":
			return "true", true, ""
		case "false", "0", "":
			return "false", true, ""
		}
		return "", false, "boolean literal " + strconv.Quote(v.Attrs["value"]) + " is not a boolean"
	case "LiteralString":
		return lexer.StringText(v.Attrs["value"]), true, ""
	case "LiteralNull":
		return "null", true, ""
	case "InstanceValue":
		inst := m.model.Ref(v, "instance")
		if inst == nil {
			return "", false, "instance value refers to nothing in the document"
		}
		if inst.Type == "EnumerationLiteral" && inst.Parent != nil {
			return m.ref(inst.Parent, scope) + "::" + writeName(inst.Name), true, ""
		}
		if cat, _ := m.classify(inst); cat == catIndividualDef {
			return m.ref(inst, scope), true, ""
		}
		return "", false, "instance value of a " + inst.Type + " has no v2 expression"
	case "OpaqueExpression":
		body, lang := opaqueBody(v)
		if body == "" {
			return "", false, "opaque expression has no body"
		}
		if !parsesAsExpression(body) {
			return "", false, "opaque expression is not v2 expression syntax" + langNote(lang)
		}
		return body, true, "opaque expression copied verbatim" + langNote(lang)
	case "Expression", "TimeExpression", "Duration", "Interval", "StringExpression":
		return "", false, "a UML " + v.Type + " tree has no v2 form"
	}
	return "", false, "no v2 form for a UML " + v.Type
}

func langNote(lang string) string {
	if lang == "" {
		return ""
	}
	return " (language " + lang + ")"
}

// opaqueBody returns the first body of an opaque expression and its language.
func opaqueBody(v *xmi.Element) (body, lang string) {
	if b := v.Owned("body"); len(b) > 0 {
		body = strings.TrimSpace(b[0].Text)
	} else {
		body = strings.TrimSpace(v.Attrs["body"])
	}
	if l := v.Owned("language"); len(l) > 0 {
		lang = strings.TrimSpace(l[0].Text)
	} else {
		lang = strings.TrimSpace(v.Attrs["language"])
	}
	return body, lang
}

// parsesAsExpression reports whether text is one v2 expression: it is parsed
// as the value of an attribute and must leave no diagnostic.
func parsesAsExpression(text string) bool {
	if strings.ContainsAny(text, ";{}") {
		return false
	}
	src := source.New("probe.sysml", []byte("attribute probe = "+text+";"))
	p := parser.New(src)
	p.ParseFile()
	return len(p.Diagnostics) == 0
}

var (
	htmlTag    = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlBreak  = regexp.MustCompile(`(?i)</p>|<br\s*/?>`)
	blankLines = regexp.MustCompile(`\n{3,}`)
)

// commentText prepares a v1 comment body for a v2 comment: some tools store
// documentation as HTML, whose tags are dropped and entities decoded.
func commentText(body string) string {
	text := body
	if strings.Contains(strings.ToLower(text), "<html") || strings.Contains(text, "<p>") || strings.Contains(text, "<br") {
		text = htmlBreak.ReplaceAllString(text, "\n")
		text = htmlTag.ReplaceAllString(text, "")
		text = html.UnescapeString(text)
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = blankLines.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// commentLines writes a v2 comment body over one or more lines, closing any
// comment terminator the text itself contains.
func commentLines(text string) []string {
	text = strings.ReplaceAll(text, "*/", "* /")
	lines := strings.Split(text, "\n")
	if len(lines) == 1 {
		return []string{"/* " + lines[0] + " */"}
	}
	out := make([]string, 0, len(lines)+1)
	for i, l := range lines {
		l = strings.TrimRight(l, " \t")
		switch {
		case i == 0:
			out = append(out, "/* "+l)
		default:
			out = append(out, " * "+l)
		}
	}
	out = append(out, " */")
	return out
}
