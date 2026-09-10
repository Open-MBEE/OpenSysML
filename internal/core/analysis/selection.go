package analysis

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Mode is how a plan chooses the engines it puts a question to.
type Mode int

const (
	// SelectAuto puts the question to the strongest covering engine, and to the
	// next on a not-covered result; the plan names each.
	SelectAuto Mode = iota
	// SelectNamed puts the question to exactly one engine; its refusal or
	// not-covered result is the result.
	SelectNamed
	// SelectAll puts the question to every covering engine, in name order, and
	// composes what they answered.
	SelectAll
)

// Selection is the engine choice a surface makes: `auto`, `all`, or one engine by name.
type Selection struct {
	Mode Mode
	// Engine is the one engine a SelectNamed selection puts the question to.
	Engine string
}

// Auto is the selection every surface makes when none is stated.
func Auto() Selection { return Selection{Mode: SelectAuto} }

// All is the selection that runs every covering engine.
func All() Selection { return Selection{Mode: SelectAll} }

// Only is the selection of the one engine named.
func Only(engine string) Selection { return Selection{Mode: SelectNamed, Engine: engine} }

// String spells the selection as a flag takes it: `auto`, `all`, or the engine's name.
func (s Selection) String() string {
	switch s.Mode {
	case SelectAll:
		return "all"
	case SelectNamed:
		return s.Engine
	}
	return "auto"
}

// ParseSelection reads a selection as a flag spells it; empty is `auto`, which
// is what an unset field means on the wire.
func ParseSelection(text string) Selection {
	switch strings.TrimSpace(text) {
	case "", "auto":
		return Auto()
	case "all":
		return All()
	}
	return Only(strings.TrimSpace(text))
}

// Explores is the exploration a selection and a schedule together ask of a
// behavior: the schedule's own when it explores, else the default one when the
// selection names the explore engine, `-engine explore` being `-schedule explore`.
func Explores(selection Selection, schedule runtime.SchedulePolicy) (runtime.SchedulePolicy, bool) {
	if _, ok := schedule.Exploration(); ok {
		return schedule, true
	}
	if selection == Only(ExploreEngineName) {
		return runtime.DefaultExploreSchedulePolicy, true
	}
	return schedule, false
}

// Select parses a selection and checks that a named engine is registered, so a
// misspelling is refused where it is written rather than run as a refusal.
func (r *Registry) Select(text string) (Selection, error) {
	selection := ParseSelection(text)
	if selection.Mode == SelectNamed {
		if _, ok := r.engines[selection.Engine]; !ok {
			return selection, &UnknownEngineError{Name: selection.Engine, Known: r.Names()}
		}
	}
	return selection, nil
}

// Names is every registered engine's name, in name order.
func (r *Registry) Names() []string {
	engines := r.Engines()
	names := make([]string, len(engines))
	for i, e := range engines {
		names[i] = e.Name()
	}
	return names
}

// ErrUnknownEngine is the typed error for a selection naming no registered engine.
var ErrUnknownEngine = errors.New("no such engine")

// UnknownEngineError reports a selection naming an engine the registry does not hold.
type UnknownEngineError struct {
	Name string
	// Known is every registered name, so the message can list the choices.
	Known []string
}

// Error names the engine and lists the registered ones.
func (e *UnknownEngineError) Error() string {
	return fmt.Sprintf("analysis: no engine named %q; the engines are %s, or auto, or all", e.Name, strings.Join(e.Known, ", "))
}

// Is matches ErrUnknownEngine.
func (e *UnknownEngineError) Is(target error) bool { return target == ErrUnknownEngine }
