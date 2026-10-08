//go:build sysml_prod || sysml_nov1

package main

import "github.com/Open-MBEE/OpenSysML/internal/translate/convert"

// runMigrateExit answers -migrate in a build made without the migration; the
// flag itself is refused before this is reached (omittedUse).
func runMigrateExit([]string) int { return fail(convert.ErrMigrationNotLinked) }
