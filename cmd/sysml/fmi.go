//go:build !sysml_prod && !sysml_nofmi

package main

// FMU import and tool:fmi are tagged alike in internal/translate/convert and internal/exec/engines.
func init() { fmiFeature.link(nil) }
