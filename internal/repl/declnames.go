package repl

import "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

// declaredNames returns the replaceable top-level names introduced by a parsed
// submission, in source order. A named declaration is replaceable: re-typing it
// supersedes the earlier snippet and invalidates anything debugging it.
// Imports/comments/etc. declare nothing replaceable.
func declaredNames(root *ast.RootNamespace) []string {
	if root == nil {
		return nil
	}
	var out []string
	for _, m := range root.Members {
		if name := memberName(m); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// topLevelMembers indexes a submission's top-level declarations by name, so a
// report about a declaration this one supersedes can be compared against its
// replacement. The first of a repeated name wins, matching how a re-declaration
// is looked up elsewhere.
func topLevelMembers(root *ast.RootNamespace) map[string]ast.Node {
	out := make(map[string]ast.Node)
	if root == nil {
		return out
	}
	for _, m := range root.Members {
		name := memberName(m)
		if name == "" {
			continue
		}
		if _, seen := out[name]; !seen {
			out[name] = m
		}
	}
	return out
}

// memberName returns the replaceable name a namespace member declares, or "" for
// a member that declares none (an import, a comment, an error node).
func memberName(m ast.Node) string {
	node := m
	if mem, ok := m.(*ast.Membership); ok {
		node = mem.Member
	}
	switch d := node.(type) {
	case *ast.Package:
		return d.Ident.Name
	case *ast.Namespace:
		return d.Ident.Name
	case *ast.Alias:
		return d.Ident.Name
	case *ast.RelationshipMember:
		return d.Ident.Name
	case *ast.Definition:
		return d.Ident.Name
	case *ast.Usage:
		// A usage that declares no name of its own answers to its reference's or
		// redefinition's, which is the name re-typing it supersedes
		// (ast.EffectiveName).
		name, _ := ast.EffectiveName(d)
		return name
	}
	return ""
}
