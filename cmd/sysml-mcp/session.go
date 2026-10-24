package main

import (
	"fmt"
	"io"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	engineset "github.com/Open-MBEE/OpenSysML/internal/exec/engines"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/buildinfo"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/sessiontools"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// sessionFactory makes REPL sessions under the run bounds the environment sets,
// as the sysml command starts one; the bounds are read once per session so a
// misconfigured variable is reported on the call that opens it.
func sessionFactory(info buildinfo.Info, cache *libs.Cache, stderr io.Writer) sessiontools.Factory {
	return func() (*repl.Session, error) {
		budgets, err := runtime.BudgetsFromEnv()
		if err != nil {
			return nil, err
		}
		jobs, err := analysis.JobsFromEnv()
		if err != nil {
			return nil, err
		}
		sess := repl.NewSessionWithSourceConverter(convert.ModelSource)
		sess.SetToolVersion(mcpCommand + " " + info.Version)
		if err := sess.SetBudgets(budgets); err != nil {
			return nil, err
		}
		if err := sess.SetEngines(engineset.Default()); err != nil {
			return nil, err
		}
		if err := sess.SetJobs(jobs); err != nil {
			return nil, err
		}
		sess.SetRecordCache(cache)
		if stderr != nil && cache == nil {
			fmt.Fprintf(stderr, "%srecord cache unavailable, the session holds every file loaded\n", commandPrefix)
		}
		return sess, nil
	}
}
