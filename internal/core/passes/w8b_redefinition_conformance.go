package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Reference messages of the feature-conformance rules (KerML 8.3.3.3).
const (
	msgDirectionConformance  = "Redefining feature must have a compatible direction"
	msgUniquenessConformance = "Subsetting/redefining feature cannot be nonunique if subsetted/redefined feature is unique"
	msgConstancyConformance  = "Subsetting/redefining feature must be constant if subsetted/redefined feature is constant"
)

// RedefinitionConformancePass reports the feature-conformance rules a subsetting
// or redefinition breaks: direction, uniqueness and constancy (KerML 8.3.3.3).
// The predicates live in semantics; this pass only locates and words them.
type RedefinitionConformancePass struct{}

func (RedefinitionConformancePass) Level() PassLevel { return LevelConstraint }

func (RedefinitionConformancePass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	rc := &redefinitionConformanceChecker{
		model:         ctx.Model(),
		seen:          make(map[*symbols.Symbol]bool),
		directionOnly: false,
	}
	rc.walk(rootScope)
	return rc.diags
}

// RedefinitionDirectionPass reports direction conformance as a typing property.
type RedefinitionDirectionPass struct{}

func (RedefinitionDirectionPass) Level() PassLevel { return LevelType }

func (RedefinitionDirectionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	rc := &redefinitionConformanceChecker{
		model:         ctx.Model(),
		seen:          make(map[*symbols.Symbol]bool),
		directionOnly: true,
	}
	rc.walk(rootScope)
	return rc.diags
}

type redefinitionConformanceChecker struct {
	model         *semantics.Model
	seen          map[*symbols.Symbol]bool
	diags         []diag.Diagnostic
	directionOnly bool
}

// walk visits every symbol of the subtree once, including anonymous members.
func (rc *redefinitionConformanceChecker) walk(scope *symbols.Scope) {
	if scope == nil {
		return
	}
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym == nil || rc.seen[sym] {
			return true
		}
		rc.seen[sym] = true
		rc.check(sym)
		rc.walk(sym.Scope)
		return true
	})
}

func (rc *redefinitionConformanceChecker) check(sym *symbols.Symbol) {
	for _, v := range rc.model.ConformanceViolations(sym) {
		if !rc.ownsViolation(v.Kind) {
			continue
		}
		msg, code := conformanceMessage(v.Kind)
		if msg == "" || v.Ref == nil {
			continue
		}
		rc.diags = append(rc.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     v.Ref.Span(),
			Message:  msg,
			Code:     code,
			Source:   "constraint",
		})
	}
}

func (rc *redefinitionConformanceChecker) ownsViolation(
	kind semantics.ConformanceViolationKind,
) bool {
	if rc.directionOnly {
		return kind == semantics.ViolationDirection
	}
	return kind != semantics.ViolationDirection
}

func conformanceMessage(kind semantics.ConformanceViolationKind) (msg, code string) {
	switch kind {
	case semantics.ViolationDirection:
		return msgDirectionConformance, "redefinition-direction-conformance"
	case semantics.ViolationUniqueness:
		return msgUniquenessConformance, "subsetting-uniqueness-conformance"
	case semantics.ViolationConstancy:
		return msgConstancyConformance, "subsetting-constancy-conformance"
	}
	return "", ""
}
