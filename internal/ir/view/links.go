package view

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Site is where a rendered element was declared, the values a link template fills.
type Site struct {
	File          string
	Line, Col     int
	QualifiedName string
	ID            string
}

// Locator places a located origin in its source file; ok is false when no
// on-disk source location is available.
type Locator func(Origin) (file string, pos source.Pos, ok bool)

// Sites answers the Site of an origin; false links nothing.
type Sites func(Origin) (Site, bool)

// Links is a link template and its source sites; the zero value writes none.
type Links struct {
	Template string
	Sites    Sites
}

// Enabled reports whether links have both a template and source locations.
func (l Links) Enabled() bool { return l.Template != "" && l.Sites != nil }

// URL fills the template for a located origin.
func (l Links) URL(o Origin) (string, bool) {
	if !o.Located() || !l.Enabled() {
		return "", false
	}
	if ParseLinkTemplate(l.Template) != nil {
		return "", false
	}
	site, ok := l.Sites(o)
	if !ok {
		return "", false
	}
	return fillLinkTemplate(l.Template, site), true
}

// ParseLinkTemplate rejects unknown placeholders and an unclosed opening brace.
func ParseLinkTemplate(template string) error {
	for i := 0; i < len(template); {
		if template[i] != '{' {
			i++
			continue
		}
		end := strings.IndexByte(template[i+1:], '}')
		if end < 0 {
			return fmt.Errorf("unclosed { in link template")
		}
		name := template[i+1 : i+1+end]
		switch name {
		case "file", "line", "col", "qname", "id":
		default:
			return fmt.Errorf("unknown link template placeholder {%s}", name)
		}
		i += end + 2
	}
	return nil
}

// Sites resolves origins to files and memoizes each located result.
func (r *Renderer) Sites(locate Locator) Sites {
	type result struct {
		site Site
		ok   bool
	}
	cache := map[Origin]result{}
	return func(origin Origin) (Site, bool) {
		if !origin.Located() || locate == nil {
			return Site{}, false
		}
		if found, ok := cache[origin]; ok {
			return found.site, found.ok
		}
		file, pos, ok := locate(origin)
		if !ok || file == "" {
			cache[origin] = result{}
			return Site{}, false
		}
		site := Site{File: filepath.ToSlash(file), Line: pos.Line, Col: pos.Col}
		if r != nil && r.resolver != nil && r.model != nil {
			if index := r.resolver.Index(); index != nil {
				if scope := index.DocumentRoot(origin.Doc); scope != nil {
					if sym := scope.DeclaredAt(origin.Span); sym != nil {
						site.QualifiedName = source.QualifiedNameOf(symbols.NameChain(sym))
						if info, ok := identity.Of(r.model, r.resolver, sym); ok && info != nil {
							site.ID = info.EffectiveID
						}
					}
				}
			}
		}
		cache[origin] = result{site: site, ok: true}
		return site, true
	}
}

// FileLocator locates origins in source files and maps their byte offsets to positions.
func FileLocator(model *semantics.Model, lines func(doc string) *source.LineIndex) Locator {
	return func(origin Origin) (string, source.Pos, bool) {
		if !origin.Located() || model == nil || lines == nil {
			return "", source.Pos{}, false
		}
		file := model.SourceFileOf(origin)
		lineIndex := lines(origin.Doc)
		if file == "" || lineIndex == nil {
			return "", source.Pos{}, false
		}
		return filepath.ToSlash(file), lineIndex.PosAt(origin.Span.Offset), true
	}
}

func fillLinkTemplate(template string, site Site) string {
	var out strings.Builder
	for i := 0; i < len(template); {
		if template[i] != '{' {
			start := i
			for i < len(template) && template[i] != '{' {
				i++
			}
			out.WriteString(encodeLinkLiteral(template[start:i]))
			continue
		}
		end := strings.IndexByte(template[i+1:], '}')
		if end < 0 {
			return ""
		}
		name := template[i+1 : i+1+end]
		value := ""
		switch name {
		case "file":
			value = site.File
		case "line":
			value = strconv.Itoa(site.Line)
		case "col":
			value = strconv.Itoa(site.Col)
		case "qname":
			value = site.QualifiedName
		case "id":
			value = site.ID
		}
		out.WriteString(encodeLinkValue(value))
		i += end + 2
	}
	return out.String()
}

func encodeLinkValue(value string) string {
	return percentEncode(value, func(b byte) bool {
		return isLinkUnreserved(b) || b == '/' || b == ':'
	})
}

func encodeLinkLiteral(value string) string {
	return percentEncode(value, func(b byte) bool {
		return b >= 0x21 && b < 0x7f && !strings.ContainsRune(`"<>\\^`+"`"+`{}|[]`, rune(b))
	})
}

func percentEncode(value string, allowed func(byte) bool) string {
	var out strings.Builder
	const hex = "0123456789ABCDEF"
	for i := 0; i < len(value); i++ {
		b := value[i]
		if allowed(b) {
			out.WriteByte(b)
		} else {
			out.WriteByte('%')
			out.WriteByte(hex[b>>4])
			out.WriteByte(hex[b&0xf])
		}
	}
	return out.String()
}

func isLinkUnreserved(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' ||
		b >= '0' && b <= '9' || strings.ContainsRune("-._~", rune(b))
}
