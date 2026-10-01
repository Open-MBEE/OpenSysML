//go:build sysml_prod || sysml_nofmi

package engines

import "github.com/Open-MBEE/OpenSysML/internal/exec/analysis"

func registerFMI(*analysis.Registry) { /* built without FMI: no tool:fmi */ }
