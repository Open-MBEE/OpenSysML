package repl

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const metadataUsage = "usage: %metadata <element-or-object>"

// doMetadata lists the metadata annotating what target denotes: an object the
// session holds by the elements it is of, any other name as the element itself.
func (s *Session) doMetadata(target string) ([]string, bool, error) {
	inst, label, objErr := s.resolveObject(target)
	if objErr == nil {
		ctx, err := s.getOrCreateRuntime()
		if err != nil {
			return nil, false, fmt.Errorf("runtime init: %w", err)
		}
		return s.objectMetadata(ctx, inst, label), false, nil
	}
	// A name the session holds no object under may still be an element; one
	// that is neither is answered by why it is no object when written as a path.
	failure := objErr
	var sym *symbols.Symbol
	var fqn string
	if !strings.HasPrefix(target, "#") {
		var symErr error
		if sym, fqn, symErr = s.lookupSymbol(target); symErr == nil && sym != nil {
			failure = nil
		} else if !strings.ContainsAny(target, ".[") {
			failure = symErr
		}
	}
	if failure != nil || sym == nil {
		out := []string{errPrefix + failure.Error()}
		if note := s.lost.lostNote(); note != "" {
			out = append(out, note)
		}
		return out, false, nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}
	w := &annotationListing{s: s, ctx: ctx}
	lines := []string{fmt.Sprintf("Element: %s (%s)", s.declaredName(fqn), sym.Kind)}
	lines = append(lines, w.of(sym, "  ")...)
	s.noteMaterializationFailure(w.errs...)
	return lines, false, nil
}

// objectMetadata lists the annotations of the elements an object is of, each
// group headed by the element annotated: an annotation is never the object's.
func (s *Session) objectMetadata(ctx *runtime.Context, inst *runtime.Instance, label string) []string {
	lines := []string{fmt.Sprintf("Object: %s (ID: %d)", label, inst.ID)}
	w := &annotationListing{s: s, ctx: ctx}
	seen := map[ast.Node]bool{}
	var direct []*symbols.Symbol
	for _, typ := range append(heldAs(ctx, inst), ctx.TypesOfObject(inst)...) {
		if typ == nil || typ.Decl == nil || seen[typ.Decl] {
			continue
		}
		seen[typ.Decl] = true
		direct = append(direct, typ)
		lines = append(lines, fmt.Sprintf("Annotations of %s %s, which the object is an instance of:",
			elementRole(typ), s.objectLabelOf(typ)))
		lines = append(lines, w.of(typ, "  ")...)
	}
	// The object's own type comes last, and a general type is named under it
	// rather than under the usage that is typed by it.
	for i := len(direct) - 1; i >= 0; i-- {
		typ := direct[i]
		for _, sup := range ctx.Semantics().AllSupertypes(typ) {
			if sup == nil || sup.Decl == nil || seen[sup.Decl] {
				continue
			}
			seen[sup.Decl] = true
			// A general type is listed for what annotates it; most have nothing.
			if len(ctx.Semantics().ElementMetadataOf(sup)) == 0 {
				continue
			}
			lines = append(lines, fmt.Sprintf("Annotations of %s %s, which %s specializes:",
				elementRole(sup), s.objectLabelOf(sup), s.objectLabelOf(typ)))
			lines = append(lines, w.of(sup, "  ")...)
		}
	}
	s.noteMaterializationFailure(w.errs...)
	return lines
}

// heldAs answers the usage a nested object is the value of in the object
// holding it, which its own type, the usage's definition, does not name.
func heldAs(ctx *runtime.Context, inst *runtime.Instance) []*symbols.Symbol {
	owner, name := inst.Owner()
	if owner == nil {
		return nil
	}
	for _, of := range ctx.FeaturesOfObject(owner) {
		if of.Name == name && of.Feature.Symbol != nil {
			return []*symbols.Symbol{of.Feature.Symbol}
		}
	}
	return nil
}

// elementRole names a declaration as the definition or the usage it is.
func elementRole(sym *symbols.Symbol) string {
	if _, isDef := sym.Decl.(*ast.Definition); isDef {
		return "definition"
	}
	return "usage"
}

// annotationListing renders the annotations of elements; errs are the values
// it could not evaluate, each rendered in its place as an error.
type annotationListing struct {
	s    *Session
	ctx  *runtime.Context
	errs []error
	// inherited is base's answer once it is asked for.
	inherited map[*symbols.Symbol]bool
}

// of lists what annotates one element: each annotation by its metadata type and
// where it is stated, with the values its features hold beneath it.
func (w *annotationListing) of(sym *symbols.Symbol, indent string) []string {
	annotations := w.ctx.AnnotationObjectsOf(sym)
	if len(annotations) == 0 {
		return []string{indent + "(no annotations)"}
	}
	var lines []string
	for _, a := range annotations {
		head := indent + "@" + w.s.objectLabelOf(a.Annotation.Type)
		if a.Annotation.About {
			head += " (stated with `about` in " + a.Annotation.Doc + ")"
		}
		lines = append(lines, head)
		if a.Err != nil {
			w.errs = append(w.errs, a.Err)
			lines = append(lines, fmt.Sprintf("%s  <error: %v>", indent, a.Err))
			continue
		}
		lines = append(lines, w.values(a, indent+"  ")...)
	}
	return lines
}

// values lists the features of an annotation's object, marking each the body
// did not bind as the metadata type's default or as unset.
func (w *annotationListing) values(a runtime.AnnotationObject, indent string) []string {
	bound := map[string]bool{}
	for _, b := range a.Annotation.Bindings {
		bound[b.Feature] = true
	}
	walk := &featureValueWalk{
		ctx:      w.ctx,
		onPath:   map[*symbols.Symbol]bool{a.Object.Type: true},
		listing:  map[int64]bool{a.Object.ID: true},
		maxDepth: maxFeatureValueDepth,
		budget:   maxFeatureValueLines,
		hint:     "%eval reads one value of an annotation",
	}
	var lines []string
	for _, of := range w.ctx.FeaturesOfObject(a.Object) {
		if of.Feature.Symbol == nil || w.base()[of.Feature.Symbol] {
			continue
		}
		fv, err := a.Object.GetFeatureValue(w.ctx, of.Name)
		if err != nil {
			w.errs = append(w.errs, err)
			lines = append(lines, fmt.Sprintf("%s%s: <error: %v>", indent, of.Name, err))
			continue
		}
		text := formatFeatureValue(w.ctx, fv)
		// A feature left empty has no default to mark.
		if !bound[of.Name] && text != runtime.UnsetText && text != "[]" {
			text += " (default)"
		}
		lines = append(lines, fmt.Sprintf("%s%s = %s", indent, of.Name, text))
		lines = walk.expandNested(lines, fv, indent, 0, 0)
	}
	w.errs = append(w.errs, walk.errs...)
	if len(lines) == 0 {
		return []string{indent + "(no features)"}
	}
	return lines
}

// base answers the features every metadata object has as an item, which say
// nothing an annotation states.
func (w *annotationListing) base() map[*symbols.Symbol]bool {
	if w.inherited == nil {
		w.inherited = map[*symbols.Symbol]bool{}
		if item, _, err := w.s.lookupSymbol("Metadata::MetadataItem"); err == nil && item != nil {
			for _, f := range w.ctx.FeaturesOf(item) {
				w.inherited[f.Symbol] = true
			}
		}
	}
	return w.inherited
}
