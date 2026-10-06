package view

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// SelectionErrorKind classifies why a requested view could not be rendered.
type SelectionErrorKind uint8

const (
	SelectionInvalid SelectionErrorKind = iota
	SelectionNotFound
)

// SelectionError carries the status class and unchanged message of a failed
// view selection or rendering.
type SelectionError struct {
	Kind SelectionErrorKind
	Err  error
}

func (e *SelectionError) Error() string { return e.Err.Error() }

func (e *SelectionError) Unwrap() error { return e.Err }

// RenderNamed resolves a declared view or targeted pseudo-view and renders it.
// lookup resolves a qualified name in the caller's model index.
func RenderNamed(renderer *Renderer, name string, lookup func(string) []*symbols.Symbol) (*Rendering, error) {
	if strings.HasPrefix(name, PseudoViewPrefix) {
		kind, target, valid := ParsePseudoView(name)
		if !valid {
			return nil, selectionError(SelectionInvalid, fmt.Errorf("%s is no pseudo-view: write %s",
				name, strings.Join(PseudoViewSpecs(), ", ")))
		}
		if target == "" {
			return nil, selectionError(SelectionInvalid, fmt.Errorf(
				"%s is untargeted; name an element as #<kind>:<qualified name> (supported: %s)",
				name, strings.Join(PseudoViewSpecs(), ", ")))
		}
		matches := lookup(target)
		if len(matches) == 0 {
			return nil, selectionError(SelectionNotFound, fmt.Errorf("%s: %s names nothing in this model", name, target))
		}
		stated := fmt.Sprintf("no view declared; rendering %s directly", target)
		rendering, err := renderer.RenderExposed([]*symbols.Symbol{matches[0]}, kind, stated)
		if err != nil {
			return nil, selectionError(SelectionInvalid, err)
		}
		return rendering, nil
	}

	matches := lookup(name)
	if len(matches) == 0 {
		return nil, selectionError(SelectionNotFound, fmt.Errorf("no view named %s", name))
	}
	rendering, err := renderer.Render(matches[0])
	if err != nil {
		return nil, selectionError(SelectionInvalid, err)
	}
	return rendering, nil
}

func selectionError(kind SelectionErrorKind, err error) *SelectionError {
	return &SelectionError{Kind: kind, Err: err}
}
