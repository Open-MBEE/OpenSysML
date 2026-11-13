package view

import (
	"errors"
	"slices"
	"testing"
)

func TestRequestOptionsParsesWhatTheFlagsTake(t *testing.T) {
	form, options, err := Request{
		Form: "dot", Palette: "okabe-ito", Style: "cameo", Lines: "ortho", Direction: "LR",
		Link: "https://example.test/{file}#L{line}", Unplaced: "strip",
		Ports: "full", Compartments: "most", Show: []string{"keywords"},
	}.Options()
	if err != nil {
		t.Fatal(err)
	}
	if form != FormDot || options.Palette != PaletteOkabeIto || options.Style != StyleCameo ||
		options.Lines != LinesOrtho || options.Direction != DirectionLeftRight ||
		options.Links.Template != "https://example.test/{file}#L{line}" || options.Unplaced != UnplacedStrip ||
		options.Ports != PortsFull || options.Compartments != CompartmentsMost || !options.Show[ElementKeywords] {
		t.Errorf("form %q, options %+v", form, options)
	}
	if options.Width != WidthUnbounded {
		t.Errorf("width = %d, want unbounded", options.Width)
	}
}

func TestRequestOptionsOfAnEmptyRequestAreTheDefaults(t *testing.T) {
	form, options, err := Request{}.Options()
	if err != nil {
		t.Fatal(err)
	}
	if want := (Options{Lines: LinesSplines}); form != "" || options.Lines != want.Lines || options.Show != nil ||
		options.Links.Template != "" || options.Palette != "" || options.Style != "" || options.Direction != "" {
		t.Errorf("form %q, options %+v, want no form and the defaults", form, options)
	}
	if (Request{}).Written() || (Request{Ports: "full", Compartments: "most", Show: []string{"all"}}).Written() {
		t.Error("a request stating no form and no writer option reports a written one")
	}
}

func TestRequestWrittenIsAnyFormOrWriterOption(t *testing.T) {
	for name, request := range map[string]Request{
		"form": {Form: "dot"}, "palette": {Palette: "x"}, "style": {Style: "x"}, "lines": {Lines: "x"},
		"direction": {Direction: "x"}, "link": {Link: "x"}, "unplaced": {Unplaced: "x"},
	} {
		if !request.Written() {
			t.Errorf("a request stating a %s reports nothing written", name)
		}
	}
}

func TestRequestOptionsRefusesWhatNamesNothing(t *testing.T) {
	for _, test := range []struct {
		name    string
		request Request
		is      error
		message string
	}{
		{"form", Request{Form: "png"}, ErrUnknownForm, `unknown rendering form "png"; the forms are text, mermaid, markdown, dot, plantuml, d2, drawio, csv and tsv`},
		{"run-only form", Request{Form: "svg-animated"}, ErrUnknownForm, `unknown rendering form "svg-animated"; the forms are text, mermaid, markdown, dot, plantuml, d2, drawio, csv and tsv`},
		{"palette", Request{Palette: "neon"}, ErrUnknownPalette, (&UnknownPaletteError{Name: "neon"}).Error()},
		{"style", Request{Style: "neon"}, ErrUnknownDrawingStyle, (&UnknownDrawingStyleError{Name: "neon"}).Error()},
		{"lines", Request{Lines: "zigzag"}, ErrUnknownLines, (&UnknownLinesError{Name: "zigzag"}).Error()},
		{"direction", Request{Direction: "lr"}, ErrUnknownDirection, `unknown direction "lr"; the directions are TB, LR, RL, BT`},
		{"unplaced", Request{Unplaced: "pile"}, ErrUnknownUnplaced, (&UnknownUnplacedError{Name: "pile"}).Error()},
		{"ports", Request{Ports: "all"}, ErrUnknownPorts, (&UnknownPortsError{Name: "all"}).Error()},
		{"compartments", Request{Compartments: "none"}, ErrUnknownCompartments, (&UnknownCompartmentsError{Name: "none"}).Error()},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := test.request.Options()
			if !errors.Is(err, test.is) || err.Error() != test.message {
				t.Errorf("error = %v, want %q wrapping %v", err, test.message, test.is)
			}
		})
	}
	if _, _, err := (Request{Link: "https://example.test/{nothing}"}).Options(); err == nil {
		t.Error("a link template with an unknown placeholder was taken")
	}
	if _, _, err := (Request{Show: []string{"no-such-element"}}).Options(); err == nil {
		t.Error("an unknown shown element was taken")
	}
}

func TestParseViewFormTakesTheModelViewFormsOnly(t *testing.T) {
	for _, form := range ViewForms() {
		if got, err := ParseViewForm(string(form)); err != nil || got != form {
			t.Errorf("ParseViewForm(%q) = %q, %v", form, got, err)
		}
	}
	for _, form := range Forms() {
		if slices.Contains(ViewForms(), form) {
			continue
		}
		var unknown *UnknownFormError
		if _, err := ParseViewForm(string(form)); !errors.As(err, &unknown) || !slices.Equal(unknown.Forms, ViewForms()) {
			t.Errorf("ParseViewForm(%q) error = %v, want the model-view forms named", form, err)
		}
	}
}

func TestWriteViewWithWritesTheModelViewFormsOnly(t *testing.T) {
	table := &Rendering{Kind: KindTable, Columns: []string{"name"}, Rows: [][]string{{"wheel"}}}
	for _, form := range table.Kind.SupportedForms() {
		want, wantErr := table.WriteWith(form, Options{})
		if got, err := table.WriteViewWith(form, Options{}); got != want || err != nil || wantErr != nil {
			t.Errorf("WriteViewWith(%q) = %q, %v; WriteWith = %q, %v", form, got, err, want, wantErr)
		}
	}
	var wrong *WrongFormError
	if _, err := table.WriteViewWith(FormDot, Options{}); !errors.As(err, &wrong) {
		t.Errorf("WriteViewWith(dot) of a table: %v, want a wrong-form error", err)
	}
	run := &Rendering{Kind: KindTimeline, Run: true}
	for _, form := range []Form{FormSVG, FormSVGAnimated, "no-such-form"} {
		var unknown *UnknownFormError
		if _, err := run.WriteViewWith(form, Options{}); !errors.As(err, &unknown) || !slices.Equal(unknown.Forms, ViewForms()) {
			t.Errorf("WriteViewWith(%q) error = %v, want the model-view forms named", form, err)
		}
	}
	if _, err := run.WriteWith(FormSVG, Options{}); err != nil {
		t.Errorf("WriteWith(svg) of a run: %v", err)
	}
	var unknown *UnknownFormError
	if _, err := run.WriteWith("no-such-form", Options{}); !errors.As(err, &unknown) || !slices.Equal(unknown.Forms, Forms()) {
		t.Errorf("WriteWith of an unknown form: %v, want every form named", err)
	}
}

func TestOptionsNeedSitesForLinksAndDrawio(t *testing.T) {
	linked := Options{Links: Links{Template: "{file}"}}
	if !linked.NeedsSites(FormDot) || !(Options{}).NeedsSites(FormDrawio) || (Options{}).NeedsSites(FormDot) {
		t.Error("sites are read exactly by a link template and by draw.io")
	}
}

func TestArtifactEndsInOneNewline(t *testing.T) {
	for written, want := range map[string]string{"": "\n", "a": "a\n", "a\n": "a\n", "a\n\n\n": "a\n", "a\nb": "a\nb\n"} {
		if got := Artifact(written); got != want {
			t.Errorf("Artifact(%q) = %q, want %q", written, got, want)
		}
	}
}
