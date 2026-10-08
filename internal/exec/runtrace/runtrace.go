package runtrace

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// Kind is a rendering of a recorded execution run.
type Kind string

const (
	// KindTimeline renders state occupancy over a run's clock time.
	KindTimeline Kind = "timeline"
	// KindSequence renders the messages recorded during a run.
	KindSequence Kind = "sequence"
	// DefaultLimit bounds the number of spans or messages in a run rendering.
	DefaultLimit = 200
)

// Kinds are the run renderings, in the order they are offered.
func Kinds() []Kind { return []Kind{KindTimeline, KindSequence} }

// ParseKind reports whether s names a run rendering.
func ParseKind(s string) (Kind, bool) {
	kind := Kind(s)
	switch kind {
	case KindTimeline, KindSequence:
		return kind, true
	}
	return "", false
}

// Options tune the labels, end instant and size of a run rendering.
type Options struct {
	Label func(*runtime.Instance) string
	Until float64
	Limit int
}

// ErrNoTrace reports a session with no recorded trace.
var ErrNoTrace = errors.New("the session records no trace")

// Render builds one run rendering from its trace.
func Render(kind Kind, trace *runtime.TraceRecorder, options Options) (*view.Rendering, error) {
	if trace == nil {
		return nil, ErrNoTrace
	}
	switch kind {
	case KindTimeline:
		return Timeline(trace, options), nil
	case KindSequence:
		return Sequence(trace, options), nil
	default:
		return nil, fmt.Errorf("unknown run rendering %q; ask for %s", kind, kindNames())
	}
}

// Timeline builds a state-occupancy timeline from the trace.
func Timeline(trace *runtime.TraceRecorder, options Options) *view.Rendering {
	return timeline(trace, options)
}

// Sequence builds a message sequence from the trace.
func Sequence(trace *runtime.TraceRecorder, options Options) *view.Rendering {
	return sequence(trace, options)
}

func kindNames() string {
	kinds := Kinds()
	names := make([]string, len(kinds))
	for i, kind := range kinds {
		names[i] = string(kind)
	}
	return strings.Join(names, ", ")
}

func runRendering(kind view.Kind, options Options) *view.Rendering {
	return &view.Rendering{
		Kind:     kind,
		Run:      true,
		RunUntil: options.Until,
		Stated:   fmt.Sprintf("the trace of a run to t = %s", runInstant(options.Until)),
	}
}
