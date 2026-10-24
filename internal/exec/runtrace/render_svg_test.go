package runtrace

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// TestRunSVGGoldensAreWellFormed parses every SVG golden as XML and checks
// each is one svg document in the SVG namespace with nothing active in it.
func TestRunSVGGoldensAreWellFormed(t *testing.T) {
	goldens, err := filepath.Glob(filepath.Join("testdata", "*.svg*.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if len(goldens) < 10 {
		t.Fatalf("found %d SVG goldens, want the static and animated set", len(goldens))
	}
	for _, path := range goldens {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			XMLName xml.Name
			ViewBox string `xml:"viewBox,attr"`
		}
		if err := xml.Unmarshal(data, &doc); err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if doc.XMLName.Space != "http://www.w3.org/2000/svg" || doc.XMLName.Local != "svg" {
			t.Errorf("%s: root is %v, want svg", path, doc.XMLName)
		}
		if doc.ViewBox == "" {
			t.Errorf("%s: no viewBox", path)
		}
		decoder := xml.NewDecoder(bytes.NewReader(data))
		for {
			token, err := decoder.Token()
			if err != nil {
				break
			}
			start, ok := token.(xml.StartElement)
			if !ok {
				continue
			}
			switch start.Name.Local {
			case "script", "style", "foreignObject", "image", "use":
				t.Errorf("%s: has a <%s>", path, start.Name.Local)
			}
			for _, attr := range start.Attr {
				if strings.HasPrefix(attr.Name.Local, "on") {
					t.Errorf("%s: <%s> has an %s handler", path, start.Name.Local, attr.Name.Local)
				}
				if attr.Name.Local == "href" && !strings.Contains(path, "-links.") {
					t.Errorf("%s: <%s> has an href %q in an unlinked rendering", path, start.Name.Local, attr.Value)
				}
			}
		}
		if strings.Contains(path, "-links.") {
			if !strings.Contains(string(data), `<a href="https://example.test/src/`) {
				t.Errorf("%s: a linked rendering has no <a href>", path)
			}
			err := imagefile.CheckStaticSVG(data)
			if err == nil || !strings.Contains(err.Error(), "external href") {
				t.Errorf("%s: CheckStaticSVG() = %v, want the external href refused", path, err)
			}
			continue
		}
		if err := imagefile.CheckStaticSVG(data); err != nil {
			t.Errorf("%s: CheckStaticSVG() = %v", path, err)
		}
	}
}

// TestRunSVGIsDeterministic renders each kind twice and compares the bytes.
func TestRunSVGIsDeterministic(t *testing.T) {
	trace, options := exampleRun(t)
	for _, kind := range []Kind{KindTimeline, KindSequence} {
		run, err := Render(kind, trace, options)
		if err != nil {
			t.Fatal(err)
		}
		for _, form := range []view.Form{view.FormSVG, view.FormSVGAnimated} {
			first, err := run.WriteWith(form, view.Options{})
			if err != nil {
				t.Fatal(err)
			}
			again, err := Render(kind, trace, options)
			if err != nil {
				t.Fatal(err)
			}
			second, err := again.WriteWith(form, view.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if first != second {
				t.Errorf("%s %s: two renderings differ", kind, form)
			}
		}
	}
}

// TestRunSVGAnimationKeepsTheFinalPicture checks the animated form draws what
// the static form draws — every shape, with the base attributes of the final
// state — and only adds SMIL animation elements, all looping over one duration.
func TestRunSVGAnimationKeepsTheFinalPicture(t *testing.T) {
	trace, options := exampleRun(t)
	for _, kind := range []Kind{KindTimeline, KindSequence} {
		run, err := Render(kind, trace, options)
		if err != nil {
			t.Fatal(err)
		}
		static, err := run.WriteWith(view.FormSVG, view.Options{})
		if err != nil {
			t.Fatal(err)
		}
		animated, err := run.WriteWith(view.FormSVGAnimated, view.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(animated, "<animate ") {
			t.Fatalf("%s: the animated form has no <animate>", kind)
		}
		if strings.Contains(static, "<animate ") {
			t.Errorf("%s: the static form animates", kind)
		}
		stripped := stripSVGAnimation(animated)
		stripped = strings.Replace(stripped, " (animated)", "", -1)
		if stripped != static {
			t.Errorf("%s: the animated form without its animation differs from the static form:\n%s", kind, firstDifference(static, stripped))
		}
		durations := map[string]bool{}
		for _, rest := range strings.Split(animated, "<animate ")[1:] {
			element := rest[:strings.Index(rest, "/>")]
			if !strings.Contains(element, `repeatCount="indefinite"`) {
				t.Errorf("%s: an animation does not loop: %s", kind, element)
			}
			if i := strings.Index(element, `dur="`); i >= 0 {
				durations[element[i+5:i+5+strings.Index(element[i+5:], `"`)]] = true
			} else {
				t.Errorf("%s: an animation has no dur: %s", kind, element)
			}
		}
		if len(durations) != 1 {
			t.Errorf("%s: animations run over %d durations, want one loop", kind, len(durations))
		}
	}
}

// stripSVGAnimation removes the <animate> elements and the circles that only
// exist to carry the travelling markers, leaving the static drawing.
func stripSVGAnimation(svg string) string {
	for {
		start := strings.Index(svg, "<animate ")
		if start < 0 {
			break
		}
		end := strings.Index(svg[start:], "/>") + 2
		svg = svg[:start] + svg[start+end:]
	}
	for {
		start := strings.Index(svg, "<circle ")
		if start < 0 {
			break
		}
		end := strings.Index(svg[start:], "</circle>") + len("</circle>")
		svg = svg[:start] + svg[start+end:]
	}
	return svg
}

// TestRunSVGEmptyRunDraws checks an empty run draws its reason, not a blank.
func TestRunSVGEmptyRunDraws(t *testing.T) {
	for _, kind := range []Kind{KindTimeline, KindSequence} {
		run, err := Render(kind, runtime.NewTraceRecorder(), Options{})
		if err != nil {
			t.Fatal(err)
		}
		svg, err := run.WriteWith(view.FormSVGAnimated, view.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(svg, "the rendering is empty") {
			t.Errorf("%s: empty rendering says nothing:\n%s", kind, svg)
		}
	}
}
