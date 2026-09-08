package repl

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// doView reports what a view exposes, the way every other name-taking command
// reports: a name the session cannot find, or an element that is no view, is a
// line rather than a failure of the command.
func (s *Session) doView(name string) ([]string, bool, error) {
	lines, err := s.view(name)
	if err != nil {
		if errors.Is(err, errRuntimeInit) {
			return nil, false, err
		}
		return []string{"error: " + err.Error()}, false, nil
	}
	return lines, false, nil
}

// View reports what a view exposes, the views nested in it, and whether it
// conforms to the viewpoints it satisfies. A view exposing nothing says so; an
// element that is no view is semantics.ErrNotAView.
func (s *Session) View(name string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view(name)
}

func (s *Session) view(name string) ([]string, error) {
	sym, fqn, err := s.lookupSymbol(name)
	if err != nil {
		return nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
	}
	model := ctx.Model()
	exposed, err := model.ExposedElements(sym)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	out := []string{fmt.Sprintf("view %s", notationName(fqn))}
	if len(exposed) == 0 {
		out = append(out, "  exposes nothing")
	} else {
		out = append(out, "  exposes")
		for _, elem := range exposed {
			out = append(out, "    "+s.viewElementLine(elem))
		}
	}
	nested, err := model.NestedViews(sym)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	if len(nested) > 0 {
		out = append(out, "  nested views")
		for _, view := range nested {
			out = append(out, "    "+s.viewElementLine(view))
		}
	}
	evaluator := concernEvaluator{session: s, ctx: ctx, reported: newReportRuntime(s)}
	report, err := model.ViewConformance(sym, evaluator)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	return append(out, s.conformanceLines(report)...), nil
}

// doRender renders a view, reporting a name the session cannot find, an element
// that is no view, a rendering kind not produced, or a form the kind is not
// written in, as a line.
func (s *Session) doRender(name string, form view.Form) ([]string, bool, error) {
	lines, err := s.renderLines(name, form)
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	return lines, false, nil
}

// renderForms are the forms %render writes, as its second argument spells them.
func renderForms() []string {
	out := make([]string, 0, len(view.Forms()))
	for _, form := range view.Forms() {
		out = append(out, string(form))
	}
	return out
}

// renderLines renders a view in the kind its `render` member states and the form
// asked for, one line per line of the artifact.
func (s *Session) renderLines(name string, form view.Form) ([]string, error) {
	rendering, err := s.viewRendering(name)
	if err != nil {
		return nil, err
	}
	artifact, err := rendering.WriteWidth(form, s.renderWidth)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(artifact, "\n"), "\n"), nil
}

// SetRenderWidth sets the width a text rendering's table is written to fit. The
// frontend sets it from the terminal; view.WidthUnbounded, the default, writes
// every column as wide as its widest cell.
func (s *Session) SetRenderWidth(width int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renderWidth = width
}

// ViewRendering renders a view of the session's model. It reads the session's
// symbols and creates nothing in it: no object, no runtime, no change to a
// debugging session in progress.
func (s *Session) ViewRendering(name string) (*view.Rendering, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.viewRendering(name)
}

// viewRendering renders a view with the session already held.
func (s *Session) viewRendering(name string) (*view.Rendering, error) {
	if strings.HasPrefix(name, view.PseudoViewPrefix) {
		return s.renderPseudoView(name)
	}
	sym, fqn, err := s.lookupSymbol(name)
	if err != nil {
		return nil, err
	}
	renderer, err := s.viewRenderer()
	if err != nil {
		return nil, err
	}
	rendering, err := renderer.Render(sym)
	if err != nil {
		if errors.Is(err, semantics.ErrNotAView) {
			return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
		}
		return nil, err
	}
	return rendering, nil
}

func (s *Session) renderPseudoView(spec string) (*view.Rendering, error) {
	kind, target, ok := view.ParsePseudoView(spec)
	if !ok {
		return nil, fmt.Errorf("%s is no pseudo-view: write %s", spec, strings.Join(view.PseudoViewSpecs(), ", "))
	}
	renderer, err := s.viewRenderer()
	if err != nil {
		return nil, err
	}
	var exposed []*symbols.Symbol
	stated := "no view declared; rendering the loaded documents directly"
	if target != "" {
		sym, fqn, err := s.lookupSymbol(target)
		if err != nil {
			return nil, err
		}
		exposed = append(exposed, sym)
		stated = fmt.Sprintf("no view declared; rendering %s directly", notationName(fqn))
	} else {
		exposed = s.symbolsInLoadOrder(model.TopLevelDeclarations)
	}
	return renderer.RenderExposed(exposed, kind, stated)
}

// Views lists every view the session declares, in document then declaration
// order.
func (s *Session) Views() ([]model.ViewInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	renderer, err := s.viewRenderer()
	if err != nil {
		return nil, err
	}
	var out []model.ViewInfo
	for _, sym := range s.symbolsInLoadOrder(model.DeclaredViews) {
		info := model.ViewInfo{Name: s.viewElementFQN(sym), Supported: true}
		kind, _, err := renderer.KindOf(sym)
		switch {
		case err == nil:
			info.Kind = kind
		default:
			info.Supported = false
			info.Reason = err.Error()
			var unsupported *view.UnsupportedKindError
			if errors.As(err, &unsupported) {
				info.Kind = unsupported.Kind
			}
		}
		out = append(out, info)
	}
	return out, nil
}

func (s *Session) symbolsInLoadOrder(in func(*symbols.Scope) []*symbols.Symbol) []*symbols.Symbol {
	idx := s.browseIndex()
	var out []*symbols.Symbol
	for _, doc := range s.sessionDocs() {
		out = append(out, in(idx.DocumentRoot(doc.Name))...)
	}
	// The language documents are masked copies of one joined buffer, so their
	// spans share coordinates and sorting restores submission order.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset
	})
	return out
}

// viewRenderer returns a renderer over the session's symbols, in a semantic
// model of its own so that rendering leaves the session's runtime untouched.
func (s *Session) viewRenderer() (*view.Renderer, error) {
	idx := s.browseIndex()
	if idx == nil {
		return nil, fmt.Errorf("no declarations loaded")
	}
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	return view.NewRenderer(model, resolver, s.sessionSourceText()), nil
}

// sessionSourceText reads notation from the session's loaded documents, and
// behind them from the library files its index holds; it is nil when the
// session holds neither.
func (s *Session) sessionSourceText() view.SourceText {
	docs := s.sessionDocs()
	if len(docs) == 0 && s.libSource == nil {
		return nil
	}
	files := make(map[string]*source.SourceFile, len(docs))
	for _, doc := range docs {
		files[doc.Name] = source.New(doc.Name, doc.Content)
	}
	return source.TextOf(files, libs.Text(s.libSource))
}

// conformanceLines renders a conformance report in declaration order: each
// satisfy, each concern the viewpoint frames, each element checked.
func (s *Session) conformanceLines(report *semantics.ViewConformance) []string {
	if report == nil || len(report.Viewpoints) == 0 {
		return nil
	}
	out := []string{"  viewpoint conformance"}
	for _, vp := range report.Viewpoints {
		satisfy := fmt.Sprintf("satisfy %s", quoted(vp.Ref))
		if vp.SatisfiedIn != nil && vp.SatisfiedIn != report.View {
			satisfy += fmt.Sprintf(" (from %s)", s.viewElementName(vp.SatisfiedIn))
		}
		out = append(out, "    "+withReason(fmt.Sprintf("%s: %v", satisfy, vp.Verdict), vp.Reason))
		for _, concern := range vp.Concerns {
			out = append(out, "      "+withReason(fmt.Sprintf("concern %s: %v", quoted(concern.Name), concern.Verdict), concernReason(concern)))
			for _, check := range concern.Checks {
				if check.Holds {
					continue
				}
				out = append(out, fmt.Sprintf("        %s: %s", s.viewElementName(check.Element), checkReason(check, concern)))
			}
		}
		for _, party := range vp.Parties {
			if party.Reason != "" {
				out = append(out, "      ? "+party.Reason)
			}
		}
	}
	return out
}

// checkReason is why a check did not hold: its own error, else the reason the
// concern's verdict carries, since a check can answer false without an error.
func checkReason(check semantics.ConcernCheck, concern semantics.ConcernConformance) string {
	if check.Err != nil {
		return check.Err.Error()
	}
	if concern.Reason != "" {
		return concern.Reason
	}
	return "a required condition does not hold"
}

// concernReason is the reason a concern's verdict carries, suppressed for one
// whose per-element lines already say it.
func concernReason(concern semantics.ConcernConformance) string {
	if len(concern.Checks) > 0 {
		return ""
	}
	return concern.Reason
}

// withReason appends a verdict's reason in parentheses.
func withReason(line, reason string) string {
	if reason == "" {
		return line
	}
	return line + " (" + reason + ")"
}

// quoted renders a name the notation reads, so a name needing quotes keeps them.
func quoted(name string) string {
	if name == "" {
		return "<none>"
	}
	return notationName(name)
}

// concernEvaluator answers whether a framed concern holds of one exposed element
// through the runtime's requirement engine, as `satisfy <concern> by <element>`.
type concernEvaluator struct {
	session  *Session
	ctx      *runtime.Context
	reported *reportRuntime
}

// EvaluateConcern evaluates concern's conditions against an object of element.
func (e concernEvaluator) EvaluateConcern(concern, element *symbols.Symbol) (bool, error) {
	ctx, inst, err := e.viewSubject(element)
	if err != nil {
		return false, err
	}
	assertion := &runtime.SatisfyAssertion{
		Symbol:     concern,
		Subject:    element,
		SubjectRef: e.session.viewElementName(element),
	}
	if requirement := e.ctx.Model().FramedConcernTarget(concern); requirement != nil {
		// Named only when it resolved, so a reference naming nothing is reported
		// as unresolved rather than as this concern's own conditions.
		assertion.Requirement = requirement
		assertion.RequirementRef = requirement.Name
	}
	result, err := ctx.CheckSatisfactionOn(assertion, inst)
	// A concern stating no condition lacks a condition, not a requirement.
	if errors.Is(err, runtime.ErrNoRequirement) || errors.Is(err, runtime.ErrNoConditions) {
		return false, errNoConcernCondition
	}
	return result.Holds, err
}

// viewSubject returns the object a concern is evaluated against, with its runtime:
// the object the session holds for that element, else one the report materializes
// in a runtime of its own, so a report leaves the session holding nothing new.
func (e concernEvaluator) viewSubject(element *symbols.Symbol) (*runtime.Context, *runtime.Instance, error) {
	name := e.session.viewElementFQN(element)
	if inst, ok := e.session.instances[name]; ok {
		return e.ctx, inst, nil
	}
	ctx, err := e.reported.runtime()
	if err != nil {
		return nil, nil, err
	}
	if inst, ok := e.reported.objects[name]; ok {
		return ctx, inst, nil
	}
	inst, err := ctx.Instantiate(element)
	if err != nil {
		return nil, nil, err
	}
	e.reported.objects[name] = inst
	return ctx, inst, nil
}

// reportRuntime holds the objects one report materialized for itself, in a runtime
// the session does not keep, so they reach neither %instances nor a later check.
type reportRuntime struct {
	session *Session
	ctx     *runtime.Context
	objects map[string]*runtime.Instance
}

func newReportRuntime(s *Session) *reportRuntime {
	return &reportRuntime{session: s, objects: map[string]*runtime.Instance{}}
}

// runtime builds the report's own context over the session's symbols, once.
func (r *reportRuntime) runtime() (*runtime.Context, error) {
	if r.ctx != nil {
		return r.ctx, nil
	}
	idx := r.session.browseIndex()
	if idx == nil {
		return nil, fmt.Errorf("no document loaded")
	}
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	model.SetSourceText(r.session.sessionSourceText())
	ctx := runtime.NewContext(model, resolver, r.session.budgets.MaxSteps)
	if err := ctx.SetBudgets(r.session.budgets); err != nil {
		return nil, err
	}
	for _, doc := range r.session.sessionDocs() {
		ctx.RegisterSource(source.New(doc.Name, doc.Content))
	}
	// Recorded like the session's own evaluation, so a trace does not depend on
	// which objects the report had to materialize.
	ctx.SetTrace(r.session.trace)
	if err := ctx.SetSchedule(r.session.drivenSchedule()); err != nil {
		return nil, err
	}
	r.ctx = ctx
	return ctx, nil
}

// errNoConcernCondition is a framed concern that states nothing to evaluate.
var errNoConcernCondition = errors.New("states no condition to evaluate")

// IsViolation reports whether an evaluation error is the model answering false,
// which the REPL already distinguishes from a check it could not make.
func (e concernEvaluator) IsViolation(err error) bool {
	return err != nil && !unevaluable(err)
}

// viewElementLine names an element by qualified name and kind, as %search does.
func (s *Session) viewElementLine(sym *symbols.Symbol) string {
	return fmt.Sprintf("%s (%s)", s.viewElementName(sym), sym.Notation())
}

// viewElementName names an element as the notation writes it, for reporting.
func (s *Session) viewElementName(sym *symbols.Symbol) string {
	return notationName(s.viewElementFQN(sym))
}

// viewElementFQN is an element's qualified name as the index spells it, which is
// what the session keys an object of it under.
func (s *Session) viewElementFQN(sym *symbols.Symbol) string {
	if idx := s.browseIndex(); idx != nil {
		if fqn := idx.GetFQN(sym); fqn != "" {
			return fqn
		}
	}
	return sym.Name
}
