// Package exporter is the contract of the pilot JSON exporter's launcher, which
// scripts/download-pilot-json-exporter.sh writes and the api-json differential and derived census run.
package exporter

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/subprocess"
)

// SetupStatus is the launcher's exit status when it cannot start the bridge;
// the bridge itself exits 1 for the files it failed on.
const SetupStatus = 78

// ProvisionCommand builds the launcher on the pinned jar.
const ProvisionCommand = "./scripts/download-pilot-json-exporter.sh"

// Status is the exit status, 0 or 1, of a launcher run over dir, or an error when the bridge
// did not run (SetupStatus, or 1 with an "Error:" line on stderr) or exited otherwise.
func Status(dir string, runErr error, stderr string) (int, error) {
	status := 0
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) {
			return 0, fmt.Errorf("pilot export of %s: %w\n%s", dir, runErr, strings.TrimSpace(stderr))
		}
		status = exit.ExitCode()
	}
	cause := launcherError(stderr)
	switch {
	case status == SetupStatus:
		if cause == "" {
			cause = fmt.Sprintf("the launcher's setup exit status %d", SetupStatus)
		}
		return 0, NotRun(dir, cause)
	case status == 1 && cause != "":
		return 0, NotRun(dir, cause)
	case status != 0 && status != 1:
		return 0, fmt.Errorf("pilot export of %s: %w\n%s", dir, runErr, strings.TrimSpace(stderr))
	}
	return status, nil
}

// Wrote refuses a run that exited 1 without writing a document for any of its files.
func Wrote(dir string, status, written, files int) error {
	if status == 1 && written == 0 && files > 0 {
		return NotRun(dir, fmt.Sprintf("exit status 1 with no document written for its %d files", files))
	}
	return nil
}

// NotRun is the error of a run in which the bridge exported nothing.
func NotRun(dir, cause string) error {
	return fmt.Errorf("pilot JSON exporter did not run on %s: %s; run %s to provision it", dir, cause, ProvisionCommand)
}

// launcherError is the first "Error:" line on stderr, which the launcher and the JVM
// print when the bridge never started; the bridge's own lines name a file first.
func launcherError(stderr string) string {
	for _, line := range strings.Split(stderr, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "Error:") {
			return line
		}
	}
	return ""
}

// Root is how many of a corpus root's files the pilot was given and failed on.
type Root struct {
	Name          string
	Files, Failed int
}

// RefuseUnexported refuses roots on every file of which the pilot failed, the record of an
// exporter that never ran there rather than of the pilot's verdicts.
func RefuseUnexported(roots []Root) error {
	var whole []string
	for _, root := range roots {
		if root.Files > 0 && root.Failed == root.Files {
			whole = append(whole, fmt.Sprintf("%s (%d files)", root.Name, root.Files))
		}
	}
	if len(whole) == 0 {
		return nil
	}
	slices.Sort(whole)
	return fmt.Errorf("the pilot failed on every file of %s, so its exporter did not run there; run %s and retry, never record or accept such a baseline",
		strings.Join(whole, ", "), ProvisionCommand)
}

var featureRelease = regexp.MustCompile(`^[0-9]+$`)

// jvmTimeout and maxJVMAnswer bound the launcher's answer to --jvm; an error shows maxJVMShown of it.
const (
	jvmTimeout   = time.Minute
	maxJVMAnswer = 64 << 10
	maxJVMShown  = 200
)

// JVM asks the launcher which Java feature release it runs the export under.
func JVM(launcher string) (string, error) {
	out, _, err := subprocess.Output(jvmTimeout, maxJVMAnswer, launcher, "--jvm")
	jvm := strings.TrimSpace(string(out))
	if err != nil || !featureRelease.MatchString(jvm) {
		if err == nil {
			err = errors.New("exit status 0")
		}
		shown := jvm[:min(len(jvm), maxJVMShown)]
		return "", fmt.Errorf("%s --jvm names no Java feature release (%q, %w); re-provision it with %s --force", launcher, shown, err, ProvisionCommand)
	}
	return jvm, nil
}
