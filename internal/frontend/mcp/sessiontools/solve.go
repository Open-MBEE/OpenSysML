package sessiontools

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// SolveInput names the element a solver is asked about.
type SolveInput struct {
	SessionName
	Name string `json:"name" jsonschema:"qualified name of the constraint, requirement, satisfaction, part or analysis case"`
}

// ConfigureInput names a variation point to configure.
type ConfigureInput struct {
	SessionName
	Name       string   `json:"name" jsonschema:"qualified name of the constraint, requirement, satisfaction, part or definition whose variations are configured"`
	Selections []string `json:"selections,omitempty" jsonschema:"variants to fix, written variation=variant; the selection is checked and a conflict named. Not with all"`
	All        bool     `json:"all,omitempty" jsonschema:"enumerate every consistent selection instead of one. Not with selections"`
	Limit      int      `json:"limit,omitempty" jsonschema:"with all: at most this many selections"`
}

// solverTool registers one tool asking a solver about an element.
func solverTool(s *mcp.Server, m *Manager, name, title, description string, ask func(*repl.Session, string) []repl.SolveReport) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: toolkit.ReadOnly(title),
	}, handler(m, func(_ context.Context, sess *repl.Session, in SolveInput) (SolveResult, error) {
		if strings.TrimSpace(in.Name) == "" {
			return SolveResult{}, errors.New("name is required")
		}
		return solveResult(ask(sess, in.Name)), nil
	}))
}

// registerSolve adds the SMT solver tools. They are experimental, as the
// prompt's commands are: SysML v2 defines no solving.
func registerSolve(s *mcp.Server, m *Manager) {
	solverTool(s, m, "check", "Check satisfiability",
		"Ask an SMT solver whether a constraint, requirement or satisfaction can be satisfied at all (%check, experimental), with a satisfying assignment when it can. Needs z3 or cvc5 installed; the status is unavailable otherwise.",
		(*repl.Session).CheckSolve)
	solverTool(s, m, "explain", "Explain a conflict",
		"Ask an SMT solver which conditions of an unsatisfiable element conflict (%explain, experimental): a minimal set of conditions, each with the element declaring it and its location.",
		(*repl.Session).ExplainSolve)
	solverTool(s, m, "solve", "Solve for values",
		"Ask an SMT solver for values satisfying an element's conditions (%solve, experimental), keeping the values the model already fixes; reports the conflict when the fixed values admit none.",
		(*repl.Session).SolveValues)
	solverTool(s, m, "optimize", "Optimize objectives",
		"Ask an SMT solver for the best values an analysis case's objectives admit (%optimize, experimental), each objective improved as its trade-study definition says, within the case's conditions.",
		(*repl.Session).OptimizeSolve)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "configure",
		Description: "Ask an SMT solver which variants an element's conditions permit (%configure, experimental): a consistent selection, the check of the selections given, or every consistent selection up to a limit. A part or definition is configured over every assertion about its objects. Read-only: no variant is bound.",
		Annotations: toolkit.ReadOnly("Configure variants"),
	}, handler(m, func(_ context.Context, sess *repl.Session, in ConfigureInput) (SolveResult, error) {
		if strings.TrimSpace(in.Name) == "" {
			return SolveResult{}, errors.New("name is required")
		}
		if in.All && len(in.Selections) > 0 {
			return SolveResult{}, errors.New("selections and all are exclusive: %configure checks the selection given, or enumerates every consistent one")
		}
		args := append([]string{}, in.Selections...)
		if in.All {
			args = append(args, "all")
			if in.Limit > 0 {
				args = append(args, strconv.Itoa(in.Limit))
			}
		} else if in.Limit > 0 {
			return SolveResult{}, errors.New("limit applies with all")
		}
		return solveResult(sess.ConfigureVariants(in.Name, args)), nil
	}))
}
