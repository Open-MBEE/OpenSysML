//go:build !sysml_prod && !sysml_nofmi

package engines

import (
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/fmi"
)

// registerFMI adds tool:fmi, which a build made without FMI leaves out.
func registerFMI(r *analysis.Registry) {
	if err := r.Register(fmi.New(nil)); err != nil {
		panic(err)
	}
}
