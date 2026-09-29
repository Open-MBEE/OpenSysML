package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// portLintKinds are the usage kinds whose two ends connect ports: a connection
// (`connect`), an interface and a flow.
var portLintKinds = map[ast.UsageKind]bool{
	ast.UsageConnection: true,
	ast.UsageConnector:  true,
	ast.UsageInterface:  true,
	ast.UsageFlow:       true,
}

// PortTypeMismatchPass warns when the two ends of a connection, interface or
// flow are ports whose definitions are unrelated — neither specializes the
// other, nor do they specialize a common definition — and neither of whose
// directed features all match the other's as a conjugate pair (SysML v2
// §7.12.2). The
// specification asks no such check of a connector, so this is a lint.
type PortTypeMismatchPass struct{}

// Level reports the constraint level: the ends' resolved port types are what it reads.
func (PortTypeMismatchPass) Level() PassLevel { return LevelConstraint }

// Run checks every connection, interface and flow the document declares.
func (PortTypeMismatchPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Index.IsLibraryDocument(name) {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	model, resolver := ctx.Model(), ctx.Resolver()
	var diags []diag.Diagnostic
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || !portLintKinds[u.Kind] {
			return
		}
		ends := w8dConnectorEndTargets(u)
		if len(ends) != 2 {
			return
		}
		var features [2]*symbols.Symbol
		for i, end := range ends {
			if ctx.DownstreamOfFailure(end) {
				return
			}
			// Ends name features of the connector's owner, as the resolver reads them.
			target, ok := resolver.ResolveTarget(sym.OwnerScope, end)
			if ok {
				target, ok = resolver.ResolveAliasTarget(target)
			}
			if !ok || target == nil {
				return
			}
			features[i] = target
		}
		portA, portB, mismatch := model.ConnectedPortsMismatch(sym, features[0], features[1])
		if !mismatch {
			return
		}
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Span:     ends[0].Span(),
			Message: fmt.Sprintf(
				"%s connects port %s : %s to port %s : %s, whose definitions are unrelated and whose directed features are not conjugate; type one end by the conjugate port (~%s) or by a common definition",
				portLintSubject(sym, u), features[0].Name, portA.Name, features[1].Name, portB.Name, portA.Name),
			Code:   CodePortTypeMismatch,
			Source: lintSource,
		})
	})
	return diags
}

// portLintSubject names the connector a port lint is about, as written.
func portLintSubject(sym *symbols.Symbol, u *ast.Usage) string {
	kind := "connection"
	switch u.Kind {
	case ast.UsageInterface:
		kind = "interface"
	case ast.UsageFlow:
		kind = "flow"
	}
	if sym.Name == "" {
		return "this " + kind
	}
	return kind + " " + sym.Name
}
