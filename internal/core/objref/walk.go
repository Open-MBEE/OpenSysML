package objref

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// PathError reports a segment of an object reference that names no object of
// the one before it: Object is that object as the surface reports it, Segment
// the offending segment as written.
type PathError struct {
	Object  string
	Segment string
	Detail  string
	// Err is what kept the segment's feature value from materializing, nil when
	// the segment reached a value that is no object or no feature at all.
	Err error
}

func (e *PathError) Error() string { return e.Detail }

func (e *PathError) Unwrap() error { return e.Err }

func pathError(object string, seg Segment, format string, args ...any) *PathError {
	return &PathError{Object: object, Segment: seg.Text, Detail: fmt.Sprintf(format, args...)}
}

// Walker walks object paths through the feature values of a runtime's objects.
type Walker struct {
	Runtime *runtime.Context
	// Index, when set, tells library-declared features apart in an unknown-feature
	// hint, which counts them rather than listing them.
	Index *symbols.Index
	// Format spells a value a path reached in place of an object; nil formats
	// as the runtime does.
	Format func(runtime.Value) string
}

// Walk follows segments through feature values from inst, labelled label, to
// the object they reach. Each segment must name a feature of the object before
// it that holds an object — one, or one picked by index from a multi-valued
// feature — and the first that does not is a PathError. The label spells walked
// features after `.`, which only ever reads as a feature.
func (w Walker) Walk(inst *runtime.Instance, label string, segments []Segment) (*runtime.Instance, string, error) {
	ctx := w.Runtime
	for _, seg := range segments {
		fv, err := inst.GetFeatureValue(ctx, seg.Name)
		if err != nil {
			if _, has := inst.FeatureValues[seg.Name]; !has {
				return nil, "", pathError(label, seg, "%s has no feature %q%s", label, seg.Name, w.featureListHint(inst))
			}
			perr := pathError(label, seg, "%s of %s could not be materialized: %v", lexer.NameText(seg.Name), label, err)
			perr.Err = err
			return nil, "", perr
		}
		var val runtime.Value
		next := label + "." + lexer.NameText(seg.Name)
		if fv.Values.Kind != runtime.ValInvalid {
			elements := CollectionElements(fv.Values)
			switch {
			case len(elements) == 0:
				return nil, "", pathError(label, seg, "%s of %s holds no objects", lexer.NameText(seg.Name), label)
			case seg.Index == 0:
				return nil, "", pathError(label, seg, "%s of %s holds %d %s: pick one by index, %s[1] to %s[%d]",
					lexer.NameText(seg.Name), label, len(elements), plural(len(elements), "object", "objects"), lexer.NameText(seg.Name), lexer.NameText(seg.Name), len(elements))
			case seg.Index > len(elements):
				return nil, "", pathError(label, seg, "%s of %s holds %d %s, so %s names none (indexes run from 1 to %d)",
					lexer.NameText(seg.Name), label, len(elements), plural(len(elements), "object", "objects"), seg.Text, len(elements))
			}
			val = elements[seg.Index-1]
			next = fmt.Sprintf("%s[%d]", next, seg.Index)
		} else {
			if seg.Index > 0 {
				return nil, "", pathError(label, seg, "%s of %s holds one value and takes no index: write %s, not %s",
					lexer.NameText(seg.Name), label, lexer.NameText(seg.Name), seg.Text)
			}
			val = fv.Value
		}
		id, isObject := val.Object()
		switch {
		case val.Kind == runtime.ValInvalid:
			return nil, "", pathError(label, seg, "%s of %s holds no object", lexer.NameText(seg.Name), label)
		case !isObject || ctx.HoldsNoValue(val):
			return nil, "", pathError(label, seg, "%s of %s holds a value (%s), not an object", lexer.NameText(seg.Name), label, w.format(val))
		}
		child, ok := ctx.Instance(id)
		if !ok {
			return nil, "", pathError(label, seg, "%s of %s holds object #%d, which is no longer held", lexer.NameText(seg.Name), label, id)
		}
		inst, label = child, next
	}
	return inst, label, nil
}

func (w Walker) format(val runtime.Value) string {
	if w.Format != nil {
		return w.Format(val)
	}
	if w.Runtime.HoldsNoValue(val) {
		return runtime.UnsetText
	}
	return runtime.FormatValue(val)
}

// CollectionElements is what a multi-valued feature holds, in order; its
// contents are either a sequence or a set.
func CollectionElements(val runtime.Value) []runtime.Value {
	switch val.Kind {
	case runtime.ValSequence:
		if val.Sequence() != nil {
			return val.Sequence().Elements()
		}
	case runtime.ValSet:
		if val.Set() != nil {
			return val.Set().Elements()
		}
	}
	return nil
}

// featureListLimit bounds the features an unknown-feature error lists.
const featureListLimit = 12

// featureListHint names the features an object has, so a misspelt one can be
// corrected: the model's own by name, the library's by count.
func (w Walker) featureListHint(inst *runtime.Instance) string {
	names, library := make([]string, 0, len(inst.FeatureValues)), 0
	for name, fv := range inst.FeatureValues {
		if w.Index != nil && fv.Feature != nil && w.Index.Library(fv.Feature.Symbol) {
			library++
			continue
		}
		names = append(names, name)
	}
	return featureHint(names, library)
}

// DeclaredFeatureHint is featureListHint for the features an object of a
// declaration will have, before one is materialized.
func (w Walker) DeclaredFeatureHint(features []runtime.EffectiveFeature) string {
	names, library := make([]string, 0, len(features)), 0
	for i := range features {
		if w.Index != nil && w.Index.Library(features[i].Symbol) {
			library++
			continue
		}
		names = append(names, features[i].Name)
	}
	return featureHint(names, library)
}

func featureHint(names []string, library int) string {
	if len(names)+library == 0 {
		return " (it has no features)"
	}
	sort.Strings(names)
	more := ""
	if len(names) > featureListLimit {
		more = fmt.Sprintf(", … (%d in all)", len(names))
		names = names[:featureListLimit]
	}
	switch {
	case len(names) == 0:
		return fmt.Sprintf(" (its %d features are all declared by the library)", library)
	case library > 0:
		more += fmt.Sprintf(", and %d more the library declares", library)
	}
	return fmt.Sprintf(" (its features are %s%s)", strings.Join(names, ", "), more)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
