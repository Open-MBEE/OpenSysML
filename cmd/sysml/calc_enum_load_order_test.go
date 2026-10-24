package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// enumModesModel declares an enumeration and calcs comparing a parameter against
// a literal written in this file.
const enumModesModel = `package Modes {
    private import ScalarValues::Boolean;
    private import SequenceFunctions::includes;
    enum def Maturity { heritage; modified; newDesign; }
    calc def IsModified { in m : Maturity; return : Boolean = m == Maturity::modified; }
    calc def HasModified { in ms : Maturity[*]; return : Boolean = includes(ms, Maturity::modified); }
}
`

const enumOtherModel = "package Other { part def Box; }\n"

// runOrdered runs the binary over files written under one directory, loaded in
// the order given, since the order is what is under test.
func runOrdered(t *testing.T, binary string, files []string, args ...string) runOutcome {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, len(files))
	for i, text := range files {
		paths[i] = filepath.Join(dir, string(rune('a'+i))+".sysml")
		if err := os.WriteFile(paths[i], []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(binary, append(args, paths...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := runOutcome{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		result.status = exit.ExitCode()
	default:
		t.Fatalf("%v: %v\n%s", args, err, result.output())
	}
	return result
}

// TestCalcEnumArgumentMatchesAcrossLoadOrder checks that a qualified enumeration
// literal given as a -calc argument is the literal the calc body names whichever
// file is loaded last, not only when the defining file is.
func TestCalcEnumArgumentMatchesAcrossLoadOrder(t *testing.T) {
	binary := buildCLI(t)
	for name, files := range map[string][]string{
		"defining file alone": {enumModesModel},
		"defining file last":  {enumOtherModel, enumModesModel},
		"defining file first": {enumModesModel, enumOtherModel},
	} {
		t.Run(name, func(t *testing.T) {
			got := runOrdered(t, binary, files, "-calc", "Modes::IsModified(Modes::Maturity::modified)")
			wantReport(t, got, 0, "= [true]")
			got = runOrdered(t, binary, files, "-calc", "Modes::HasModified((Modes::Maturity::heritage, Modes::Maturity::modified))")
			wantReport(t, got, 0, "= [true]")
			got = runOrdered(t, binary, files, "-calc", "Modes::IsModified(Modes::Maturity::heritage)")
			wantReport(t, got, 0, "= [false]")
		})
	}
}
