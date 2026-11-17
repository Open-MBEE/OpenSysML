package view

// Request is what a caller of a render service states by name: the form to
// write and the options the CLI's -render-* flags take, each "" for its default.
type Request struct {
	Form         string
	Palette      string
	Style        string
	Lines        string
	Direction    string
	Link         string
	Unplaced     string
	Ports        string
	Compartments string
	Show         []string
}

// Written reports whether the request names a form or an option only a
// written form reads, which the data of a rendering has no use for.
func (q Request) Written() bool {
	return q.Form != "" || q.Palette != "" || q.Style != "" || q.Lines != "" ||
		q.Direction != "" || q.Link != "" || q.Unplaced != ""
}

// Options is the form the request names, "" when it names none, and the
// options it states; a name that names nothing is that option's error.
func (q Request) Options() (Form, Options, error) {
	var options Options
	var ok bool
	if options.Ports, ok = ParsePorts(q.Ports); !ok {
		return "", Options{}, &UnknownPortsError{Name: q.Ports}
	}
	var err error
	if options.Show, err = ShownOf(q.Show); err != nil {
		return "", Options{}, err
	}
	if options.Compartments, ok = ParseCompartments(q.Compartments); !ok {
		return "", Options{}, &UnknownCompartmentsError{Name: q.Compartments}
	}
	var form Form
	if q.Form != "" {
		if form, err = ParseViewForm(q.Form); err != nil {
			return "", Options{}, err
		}
	}
	if q.Palette != "" {
		if options.Palette, ok = ParsePalette(q.Palette); !ok {
			return "", Options{}, &UnknownPaletteError{Name: q.Palette}
		}
	}
	if q.Style != "" {
		if options.Style, ok = ParseDrawingStyle(q.Style); !ok {
			return "", Options{}, &UnknownDrawingStyleError{Name: q.Style}
		}
	}
	if options.Lines, ok = ParseLines(q.Lines); !ok {
		return "", Options{}, &UnknownLinesError{Name: q.Lines}
	}
	if q.Direction != "" {
		if options.Direction, ok = ParseDirection(q.Direction); !ok {
			return "", Options{}, &UnknownDirectionError{Name: q.Direction}
		}
	}
	if q.Link != "" {
		if err := ParseLinkTemplate(q.Link); err != nil {
			return "", Options{}, err
		}
		options.Links.Template = q.Link
	}
	if q.Unplaced != "" {
		if options.Unplaced, ok = ParseUnplaced(q.Unplaced); !ok {
			return "", Options{}, &UnknownUnplacedError{Name: q.Unplaced}
		}
	}
	return form, options, nil
}

// NeedsSites reports whether writing form with options reads source sites:
// a link template is filled from them, and draw.io records them unasked.
func (o Options) NeedsSites(form Form) bool {
	return o.Links.Template != "" || form == FormDrawio
}
