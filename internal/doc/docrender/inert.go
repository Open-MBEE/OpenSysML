package docrender

import (
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// activeElements are the elements that run script or load a document of their own.
var activeElements = map[string]bool{
	"script": true, "iframe": true, "frame": true, "object": true, "embed": true,
	"applet": true, "base": true, "meta": true, "link": true, "form": true,
	"animate": true, "set": true, "animatetransform": true, "animatemotion": true,
}

// locationAttributes are the attributes a browser follows or loads.
var locationAttributes = map[string]bool{
	"href": true, "xlink:href": true, "src": true, "action": true, "formaction": true, "srcdoc": true,
}

// activeMarkup names the first construct of markup written verbatim into a page
// that would run there: an active element, an event handler or a script URL.
func activeMarkup(markup string) (string, bool) {
	z := html.NewTokenizer(strings.NewReader(markup))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return "", false
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			// The tokenizer lowercases names; a prefix (svg:script) is still the element.
			name := token.Data[strings.LastIndexByte(token.Data, ':')+1:]
			if activeElements[name] {
				return "<" + token.Data + ">", true
			}
			for _, a := range token.Attr {
				switch {
				case strings.HasPrefix(a.Key, "on"):
					return a.Key + " on <" + token.Data + ">", true
				case locationAttributes[a.Key] && scriptURL(a.Val):
					return a.Key + " on <" + token.Data + ">", true
				}
			}
		}
	}
}

// scriptURL reports whether a location runs script or carries a document: a
// javascript: or vbscript: URL, or a data: URL of anything but an image.
func scriptURL(location string) bool {
	location = strings.ToLower(strings.Map(func(r rune) rune {
		// Browsers drop tabs and line ends from a URL and trim controls and spaces.
		if r <= ' ' {
			return -1
		}
		return r
	}, location))
	switch {
	case strings.HasPrefix(location, "javascript:"), strings.HasPrefix(location, "vbscript:"):
		return true
	case strings.HasPrefix(location, "data:"):
		return !strings.HasPrefix(location, "data:image/")
	}
	return false
}

// checkInert refuses typeset formulas and drawn diagrams holding active markup:
// both are written into the page as they stand.
func checkInert(math map[Formula]string, svg []string) error {
	formulas := make([]Formula, 0, len(math))
	for formula := range math {
		formulas = append(formulas, formula)
	}
	sort.Slice(formulas, func(i, j int) bool {
		if formulas[i].Source != formulas[j].Source {
			return formulas[i].Source < formulas[j].Source
		}
		return !formulas[i].Display && formulas[j].Display
	})
	for _, formula := range formulas {
		if found, active := activeMarkup(math[formula]); active {
			return &Error{Kind: ErrorUnsafeMath, Content: formula.TeX(), Actual: found}
		}
	}
	for i, drawing := range svg {
		if found, active := activeMarkup(drawing); active {
			return &Error{Kind: ErrorUnsafeDiagram, Count: i + 1, Actual: found}
		}
	}
	return nil
}
