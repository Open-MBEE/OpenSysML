package repl

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
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
	if len(args) < 1 || len(args) > 2 {
		return []string{renderRunUsage}, false, nil
	}
	kind, ok := runtrace.ParseKind(args[0])
	if !ok {
		return []string{fmt.Sprintf("unknown run rendering %q; want timeline or sequence; %s", args[0], renderRunUsage)}, false, nil
	}
	form := view.FormText
	if len(args) == 2 {
		form = view.Form(args[1])
		if !slices.Contains(view.Forms(), form) {
			return []string{fmt.Sprintf("unknown form %q; %s", args[1], renderRunUsage)}, false, nil
		}
	}
	if _, err := (&view.Rendering{Kind: runViewKind(kind), Run: true}).WriteWith(form, view.Options{}); err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	rendering, err := s.runTraceRendering(kind)
	if errors.Is(err, runtrace.ErrNoTrace) {
		return []string{"error: the session records no trace; %trace on before the run"}, false, nil
	}
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	lines, err := artifactLines(rendering, form, view.Options{Width: s.renderWidth})
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
	var trace *runtime.TraceRecorder
	var until float64
	if s.rtCtx != nil {
		trace = s.rtCtx.Trace()
		until = s.rtCtx.Clock().Now()
	}
	return runtrace.Render(kind, trace, runtrace.Options{Label: s.runTraceLabel, Until: until})
}
