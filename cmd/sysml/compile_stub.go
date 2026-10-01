//go:build sysml_prod || sysml_nocodegen

package main

import "errors"

func runCompile([]string) error {
	return errors.New("-compile is not available in this build (built without codegen)")
}
