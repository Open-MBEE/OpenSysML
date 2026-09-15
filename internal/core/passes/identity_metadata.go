package passes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// IdentityMetadataPass validates the IdentityMetadata annotations of a
// document: id shape, the enclosing ProjectRef an ElementId resolves against,
// and effective-id uniqueness over the whole generated id space of each
// project scope (element ids, `_om` membership ids, `_p` expression-node ids).
type IdentityMetadataPass struct{}

// duplicateIDCode is the diagnostic code for an effective id two elements share.
const duplicateIDCode = "identity-duplicate-id"

func (IdentityMetadataPass) Level() PassLevel { return LevelConstraint }

func (IdentityMetadataPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	// A project scope may span workspace documents, so uniqueness is judged
	// over the union of their gathers; each document only reports its own elements.
	union := ctx.Gathers().identitiesOf(ctx)
	c := &identityChecker{res: ctx.Resolver(), space: union.identityIndex, docRoot: rootScope}
	if c.table = union.tableOf(name); c.table == nil {
		c.table = identity.Build(ctx.Model(), ctx.Resolver(), rootScope)
		c.space = union.including(c.table)
	}
	c.check()
	return c.diags
}

type identityChecker struct {
	res *resolve.Resolver
	// table is the document's own identities; space the id space they are judged in.
	table   *identity.Table
	space   *identityIndex
	docRoot *symbols.Scope
	diags   []Diagnostic
}

// inDocument reports whether the info's symbol is declared in the document
// under validation, so each document reports only its own elements.
func (c *identityChecker) inDocument(info *identity.Info) bool {
	return c.inDoc(info.Symbol.OwnerScope)
}

// declInDocument reports whether the annotating node is declared in the
// document under validation: an `about`-form annotation may live in another
// document than the element it annotates, and its diagnostics belong there.
func (c *identityChecker) declInDocument(d identity.Declaration) bool {
	return c.inDoc(d.Scope)
}

func (c *identityChecker) inDoc(scope *symbols.Scope) bool {
	for sc := scope; sc != nil; sc = sc.Parent() {
		if sc == c.docRoot {
			return true
		}
	}
	return false
}

func (c *identityChecker) check() {
	for _, sym := range c.table.Symbols() {
		info, ok := c.table.Info(sym)
		if !ok {
			continue
		}
		if info.Annotated {
			c.checkShape(info)
			c.checkConflicts(info)
			if info.Scope == nil {
				if d, ok := c.firstInDocument(info); ok {
					c.errorf(d.Span, "identity-unscoped-id",
						"ElementId on %s has no enclosing ProjectRef to resolve against", info.FQN)
				}
			}
		}
		if info.Scope != nil && info.Scope.Symbol == sym {
			c.checkScopeConflicts(info)
		}
		c.checkIDSpace(info)
	}
}

// scopeKey groups elements by the project their scope names — org plus
// projectId — with the absent scope as a group of its own.
func scopeKey(info *identity.Info) string {
	if info.Scope == nil {
		return ""
	}
	return "bound\x00" + info.Scope.Key()
}

// checkShape reports the first byte of each declared id outside [a-zA-Z0-9_-],
// including the empty id, which has no legal byte at all.
func (c *identityChecker) checkShape(info *identity.Info) {
	for _, d := range info.Declarations {
		if !d.Declared || !c.declInDocument(d) {
			continue
		}
		if d.ID == "" {
			c.errorf(d.Span, "identity-id-shape",
				"element id of %s is empty; an id needs at least one byte of [a-zA-Z0-9_-]", info.FQN)
			continue
		}
		for i := 0; i < len(d.ID); i++ {
			b := d.ID[i]
			if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '-' {
				continue
			}
			c.errorf(d.Span, "identity-id-shape",
				"element id %q of %s: byte 0x%02x (%q) at offset %d is outside [a-zA-Z0-9_-]",
				d.ID, info.FQN, b, string(rune(b)), i)
			break
		}
	}
}

// checkConflicts errors on every ElementId annotation of an element when two
// of them bind distinct constant ids, one diagnostic per annotating node.
func (c *identityChecker) checkConflicts(info *identity.Info) {
	ids := make(map[string]bool)
	for _, d := range info.Declarations {
		if d.Declared {
			ids[d.ID] = true
		}
	}
	if len(ids) < 2 {
		return
	}
	distinct := make([]string, 0, len(ids))
	for id := range ids {
		distinct = append(distinct, fmt.Sprintf("%q", id))
	}
	sort.Strings(distinct)
	for _, d := range info.Declarations {
		if !d.Declared || !c.declInDocument(d) {
			continue
		}
		c.errorf(d.Span, "identity-conflicting-ids",
			"conflicting element ids of %s: %s", info.FQN, strings.Join(distinct, " and "))
	}
}

// checkScopeConflicts errors on every ProjectRef annotation of a namespace
// when two of them name distinct projects (org plus projectId; branch selects
// a version, never another identity), one diagnostic per annotating node.
func (c *identityChecker) checkScopeConflicts(info *identity.Info) {
	decls := info.Scope.Declarations
	projects := make(map[string]string)
	for _, d := range decls {
		projects[d.Key()] = projectName(d)
	}
	if len(projects) < 2 {
		return
	}
	distinct := make([]string, 0, len(projects))
	for _, name := range projects {
		distinct = append(distinct, name)
	}
	sort.Strings(distinct)
	for _, d := range decls {
		if !c.inDoc(d.Scope) {
			continue
		}
		c.errorf(d.Span, "identity-conflicting-projects",
			"conflicting project references of %s: %s", info.FQN, strings.Join(distinct, " and "))
	}
}

// projectName spells the project one ProjectRef declaration binds to.
func projectName(d identity.ScopeDeclaration) string {
	if d.Org == "" {
		return fmt.Sprintf("project %q", d.ProjectID)
	}
	return fmt.Sprintf("project %q of org %q", d.ProjectID, d.Org)
}

// checkIDSpace validates an element's place in the generated id space of its
// project scope: an effective id another element shares, a declared id that
// lands on another element's membership (`…_om`) or expression-node (`…_p…`)
// id, and another element's declared id landing on this one's.
func (c *identityChecker) checkIDSpace(info *identity.Info) {
	key := keyOf(info)
	// Distinct qualified names never derive one id, so a group of derived
	// ids is one name seen twice, not an identity conflict.
	if group := c.space.group(c.res, key); len(group) >= 2 && anyAnnotated(group) {
		names := make([]string, 0, len(group))
		for _, o := range group {
			names = append(names, o.FQN)
		}
		sort.Strings(names)
		if span, ok := c.reportSite(info); ok {
			c.errorf(span, duplicateIDCode,
				"duplicate element id %q in one project scope: %s",
				info.EffectiveID, strings.Join(names, " and "))
		}
	}
	for _, d := range info.Declarations {
		if !d.Declared || d.ID == "" || !c.declInDocument(d) {
			continue
		}
		for _, t := range derivedTargets(d.ID) {
			for _, owner := range c.space.group(c.res, identityKey{key.scope, t.base}) {
				if owner.Symbol == info.Symbol {
					continue
				}
				c.errorf(d.Span, duplicateIDCode,
					"element id %q of %s collides with %s of %s",
					d.ID, info.FQN, t.space, owner.FQN)
			}
		}
	}
	for _, hit := range c.space.hitsOn(c.res, key) {
		if hit.info.Symbol == info.Symbol {
			continue
		}
		if span, ok := c.reportSite(info); ok {
			c.errorf(span, duplicateIDCode,
				"%s of %s collides with element id %q of %s",
				hit.space, info.FQN, hit.decl.ID, hit.info.FQN)
		}
	}
}

// expressionPositions reports whether rest is a chain of `_p`-separated
// encoded positions, as rdf.ExpressionNodeID composes under an owner id; only
// then can an id land in an owner's expression-node id space.
func expressionPositions(rest string) bool {
	if rest == "" {
		return false
	}
	for _, part := range strings.Split(rest, "_p") {
		if _, ok := rdf.DecodeElementID(part); !ok {
			return false
		}
	}
	return true
}

// anyAnnotated reports whether any element of the group declares its id.
func anyAnnotated(group []*identity.Info) bool {
	for _, info := range group {
		if info.Annotated {
			return true
		}
	}
	return false
}

// firstInDocument is the element's first ElementId annotation declared in the
// document under validation.
func (c *identityChecker) firstInDocument(info *identity.Info) (identity.Declaration, bool) {
	for _, d := range info.Declarations {
		if c.declInDocument(d) {
			return d, true
		}
	}
	return identity.Declaration{}, false
}

// reportSite locates an element's diagnostic in the document under validation:
// its first in-document ElementId annotation, or its declaration when the
// element itself is in-document; not-ok when neither is.
func (c *identityChecker) reportSite(info *identity.Info) (source.Span, bool) {
	if d, ok := c.firstInDocument(info); ok {
		return d.Span, true
	}
	if c.inDocument(info) {
		return info.Symbol.DeclSpan, true
	}
	return source.Span{}, false
}

func (c *identityChecker) errorf(span source.Span, code, format string, args ...any) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  fmt.Sprintf(format, args...),
		Code:     code,
		Source:   "constraint",
	})
}
