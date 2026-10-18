package repl

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/nsmerge"
)

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

// memberName is the declared name of a member, "" for one without.
func memberName(m ast.Node) string { return nsmerge.MemberName(m) }
