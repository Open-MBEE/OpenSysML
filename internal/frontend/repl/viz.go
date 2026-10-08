package repl

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

const vizUsage = "usage: %viz [--view=<VIEW>|--view <VIEW>] [--style=<STYLE>...] [text|mermaid|markdown|dot|plantuml|d2|csv|tsv] <name> [<name>...]"

// vizDefaultView is the view that chooses the rendering kind from what the names
// resolve to; it is the view when --view is absent.
const vizDefaultView = "DEFAULT"

// vizViews are the views %viz draws, as the pilot kernel spells them: the
// pilot's own list, and CASE for the case rendering the pilot chooses but does
// not name.
func vizViews() []string {
	return []string{vizDefaultView, "TREE", "INTERCONNECTION", "STATE", "ACTION", "SEQUENCE", "MIXED", "CASE"}
}

// vizViewKind is the rendering kind a view name asks for, in any letter case;
// the default view is the empty kind, chosen later from the elements.
func vizViewKind(name string) (view.Kind, bool) {
	upper := strings.ToUpper(name)
	if !slices.Contains(vizViews(), upper) {
		return "", false
	}
	if upper == vizDefaultView {
		return "", true
	}
	return view.PseudoViewKind(strings.ToLower(upper))
}

// vizStyles are the words --style accepts, as they are offered: the pilot's
// directions and styles as the pilot spells them, and this renderer's drawing
// styles, palettes and port displays.
func vizStyles() []string {
	out := []string{string(view.DirectionTopBottom), string(view.DirectionLeftRight), string(view.DirectionRightLeft), string(view.DirectionBottomTop)}
	for _, style := range view.DrawingStyles() {
		out = append(out, string(style))
	}
	for _, palette := range view.Palettes() {
		out = append(out, string(palette))
	}
	for _, ports := range view.PortsChoices() {
		out = append(out, string(ports))
	}
	out = append(out, vizPilotDefaultStyle, vizPlantUMLCodeStyle)
	for _, style := range view.PilotStyles() {
		out = append(out, string(style))
	}
	return out
}

const (
	// vizPilotDefaultStyle is the pilot's name for its standard black-and-white
	// look, which is the pilot drawing style here.
	vizPilotDefaultStyle = "DEFAULT"
	// vizPlantUMLCodeStyle is the pilot's style asking for the PlantUML source
	// rather than a picture, which is the plantuml form here.
	vizPlantUMLCodeStyle = "PUMLCODE"
)

// vizRequest is a parsed %viz line: the kind asked for ("" chooses from the
// elements), the form asked for ("" leaves it to the caller), the drawing
// options, the notes on styles accepted but not drawn, and the names to draw.
type vizRequest struct {
	kind      view.Kind
	form      view.Form
	formStyle string // the style word that chose the form, when one did
	opts      view.Options
	notes     []string
	names     []string
}

// parseVizArgs reads %viz's arguments; usage is what to print instead when they
// do not read. Options and the form are read wherever they stand; an unquoted
// word spelling a form is the form, so an element so named is written with its
// quotes.
func parseVizArgs(args []string) (vizRequest, []string) {
	var req vizRequest
	viewSet := false
	value := func(i *int, option string) (string, bool) {
		if *i+1 >= len(args) {
			return "", false
		}
		*i++
		return args[*i], true
	}
	for i := 0; i < len(args); i++ {
		word := args[i]
		switch {
		case word == "--help" || word == "-h":
			return req, []string{vizUsage}
		case word == "--view" || strings.HasPrefix(word, "--view="):
			name, ok := strings.CutPrefix(word, "--view=")
			if !ok {
				if name, ok = value(&i, word); !ok {
					return req, vizUsageOf("--view takes a view name")
				}
			}
			if viewSet {
				return req, vizUsageOf("--view is given twice")
			}
			kind, ok := vizViewKind(name)
			if !ok {
				return req, vizUsageOf(fmt.Sprintf("unknown view %q; the views are %s", name, strings.Join(vizViews(), ", ")))
			}
			req.kind, viewSet = kind, true
		case word == "--style" || strings.HasPrefix(word, "--style="):
			name, ok := strings.CutPrefix(word, "--style=")
			if !ok {
				if name, ok = value(&i, word); !ok {
					return req, vizUsageOf("--style takes a style name")
				}
			}
			if usage := req.applyStyle(name); usage != nil {
				return req, usage
			}
		case strings.HasPrefix(word, "-") && len(word) > 1:
			return req, vizUsageOf(fmt.Sprintf("unknown option %q; the options are --view and --style", word))
		case slices.Contains(view.Forms(), view.Form(word)):
			if req.form != "" && req.form != view.Form(word) {
				if req.formStyle != "" {
					return req, vizUsageOf(fmt.Sprintf("style %s asks for the %s form, but the %s form was asked for", req.formStyle, req.form, word))
				}
				return req, vizUsageOf(fmt.Sprintf("%s and %s both name the form", req.form, word))
			}
			req.form = view.Form(word)
		default:
			req.names = append(req.names, word)
		}
	}
	if len(req.names) == 0 {
		return req, vizUsageOf("name at least one element to draw")
	}
	return req, nil
}

// applyStyle reads one --style word: a direction, a drawing style, a palette or
// a port display is applied; a pilot style this renderer does not draw is noted;
// any other word is refused with the list.
func (req *vizRequest) applyStyle(name string) []string {
	if direction, ok := view.ParseDirection(strings.ToUpper(name)); ok {
		if req.opts.Direction != "" {
			return vizUsageOf(fmt.Sprintf("style %s and style %s both set the direction", req.opts.Direction, name))
		}
		req.opts.Direction = direction
		return nil
	}
	lower := strings.ToLower(name)
	if strings.EqualFold(name, vizPilotDefaultStyle) {
		lower = string(view.StylePilot)
	}
	if style, ok := view.ParseDrawingStyle(lower); ok && lower != "" {
		if req.opts.Style != "" {
			return vizUsageOf(fmt.Sprintf("style %s and style %s both set the drawing style", req.opts.Style, name))
		}
		req.opts.Style = style
		return nil
	}
	if palette, ok := view.ParsePalette(lower); ok && palette != "" {
		if req.opts.Palette != "" {
			return vizUsageOf(fmt.Sprintf("style %s and style %s both set the palette", req.opts.Palette, name))
		}
		req.opts.Palette = palette
		return nil
	}
	if ports, ok := view.ParsePorts(lower); ok && ports != "" {
		if req.opts.Ports != "" {
			return vizUsageOf(fmt.Sprintf("style %s and style %s both set the port display", req.opts.Ports, name))
		}
		req.opts.Ports = ports
		return nil
	}
	if strings.EqualFold(name, vizPlantUMLCodeStyle) {
		if req.form != "" && req.form != view.FormPlantUML {
			return vizUsageOf(fmt.Sprintf("style %s asks for the plantuml form, but the %s form was asked for", name, req.form))
		}
		req.form, req.formStyle = view.FormPlantUML, name
		return nil
	}
	if style, ok := view.ParsePilotStyle(name); ok {
		if !slices.Contains(req.notes, style.Notice()) {
			req.notes = append(req.notes, style.Notice())
		}
		return nil
	}
	return vizUsageOf(fmt.Sprintf("unknown style %q; the styles are %s", name, strings.Join(vizStyles(), ", ")))
}

func vizUsageOf(problem string) []string {
	return []string{errPrefix + problem, vizUsage}
}

// metaViz runs %viz at the prompt, where a rendering with no form asked for is
// written as %render writes one: as text.
func (s *Session) metaViz(args []string) ([]string, bool, error) {
	req, usage := parseVizArgs(args)
	if usage != nil {
		return usage, false, nil
	}
	rendered, err := s.viz(req)
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	return rendered[0].Lines, false, nil
}

// Viz runs %viz with its arguments and answers what it drew: the artifact in
// the form asked for, or, when none was, in each of the fallback forms the
// chosen rendering kind is written in — the first alone when it is written in
// none of them — or as text when no fallback is given. A usage problem is a *UsageError holding what the prompt
// prints; a name that does not resolve is an error naming it.
func (s *Session) Viz(args []string, fallback ...view.Form) ([]Rendered, error) {
	defer s.enter()()
	req, usage := parseVizArgs(args)
	if usage != nil {
		return nil, &UsageError{Lines: usage}
	}
	return s.viz(req, fallback...)
}

func (s *Session) viz(req vizRequest, fallback ...view.Form) ([]Rendered, error) {
	if len(fallback) == 0 {
		fallback = []view.Form{view.FormText}
	}
	rendering, err := s.renderExposed(req.kind, req.names, "")
	if err != nil {
		return nil, err
	}
	rendering.Notices = append(rendering.Notices, req.notes...)
	forms := []view.Form{req.form}
	if req.form == "" {
		forms = forms[:0]
		for _, form := range fallback {
			if rendering.Kind.SupportsForm(form) {
				forms = append(forms, form)
			}
		}
		if len(forms) == 0 {
			forms = fallback[:1]
		}
	}
	out := make([]Rendered, 0, len(forms))
	for _, form := range forms {
		lines, err := s.artifact(rendering, form, req.opts)
		if err != nil {
			return nil, err
		}
		out = append(out, Rendered{Form: form, Lines: lines})
	}
	return out, nil
}
