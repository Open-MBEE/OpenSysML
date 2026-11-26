//go:build sysml_re || sysml_nofmi

package engines

import "github.com/Open-MBEE/OpenSysML/internal/exec/analysis"

func registerFMI(*analysis.Registry) error { return nil /* built without FMI: no tool:fmi */ }
