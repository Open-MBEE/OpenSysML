package repl

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtrace"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

func runViewKind(kind runtrace.Kind) view.Kind {
	if kind == runtrace.KindTimeline {
		return view.KindTimeline
	}
	return view.KindSequence
}

func (s *Session) metaRenderRun(args []string) ([]string, bool, error) {
	if len(args) < 1 || len(args) > 3 {
		return []string{renderRunUsage}, false, nil
	}
	kind, ok := runtrace.ParseKind(args[0])
	if !ok {
		return []string{fmt.Sprintf("unknown run rendering %q; want timeline or sequence; %s", args[0], renderRunUsage)}, false, nil
	}
	form := view.FormText
	formSet := false
	linkSet := false
	links := view.Links{}
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "link=") {
			if linkSet {
				return []string{renderRunUsage}, false, nil
			}
			links.Template = strings.TrimPrefix(arg, "link=")
			if err := view.ParseLinkTemplate(links.Template); err != nil {
				return []string{err.Error() + "; " + renderRunUsage}, false, nil
			}
			linkSet = true
			continue
		}
		if formSet {
			return []string{renderRunUsage}, false, nil
		}
		form = view.Form(arg)
		if !slices.Contains(view.Forms(), form) {
			return []string{fmt.Sprintf("unknown form %q; %s", arg, renderRunUsage)}, false, nil
		}
		formSet = true
	}
	if _, err := (&view.Rendering{Kind: runViewKind(kind), Run: true}).WriteWith(form, view.Options{Links: links}); err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	rendering, err := s.runTraceRendering(kind)
	if errors.Is(err, runtrace.ErrNoTrace) {
		return []string{"error: the session records no trace; %trace on before the run"}, false, nil
	}
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	options := view.Options{Width: s.renderWidth, Links: links}
	if links.Template != "" {
		renderer, err := s.viewRenderer()
		if err != nil {
			return []string{"error: " + err.Error()}, false, nil
		}
		options.Links.Sites = renderer.Sites(s.sessionLocator())
	}
	lines, err := artifactLines(rendering, form, options)
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	return lines, false, nil
}

// RunTraceRendering renders the trace the session recorded so far.
func (s *Session) RunTraceRendering(kind runtrace.Kind) (*view.Rendering, error) {
	defer s.enter()()
	return s.runTraceRendering(kind)
}

func (s *Session) runTraceRendering(kind runtrace.Kind) (*view.Rendering, error) {
	trace := s.trace
	var until float64
	if s.rtCtx != nil {
		if trace == nil {
			trace = s.rtCtx.Trace()
		}
		until = s.rtCtx.Clock().Now()
	}
	return runtrace.Render(kind, trace, runtrace.Options{Label: s.runTraceLabel, Until: until})
}
