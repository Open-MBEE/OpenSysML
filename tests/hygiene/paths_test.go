package hygiene

import (
	"os/exec"
	"strings"
	"testing"
)

// Windows forbids a set of characters and device names in filesystem paths; a
// tracked file carrying one breaks `git checkout` on windows-latest, which the
// release workflow cannot survive.
var reservedDeviceNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// TestTrackedPathsAreWindowsPortable lists every tracked path a Windows
// checkout cannot hold: a character from `<>:"|?*`, a control character, a
// component ending in a space or a dot, or a component named for a reserved
// device.
func TestTrackedPathsAreWindowsPortable(t *testing.T) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}

	var bad []string
	for _, path := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if reason := windowsPathProblem(path); reason != "" {
			bad = append(bad, path+" ("+reason+")")
		}
	}
	if len(bad) > 0 {
		t.Errorf("%d tracked path(s) a Windows checkout cannot hold:\n%s", len(bad), strings.Join(bad, "\n"))
	}
}

func windowsPathProblem(path string) string {
	for _, r := range path {
		if strings.ContainsRune(`<>:"|?*`, r) {
			return "character " + string(r)
		}
		if r < 0x20 {
			return "control character"
		}
	}
	for _, component := range strings.Split(path, "/") {
		if strings.HasSuffix(component, " ") || strings.HasSuffix(component, ".") {
			return "component ends in a space or dot"
		}
		base := component
		if i := strings.Index(base, "."); i >= 0 {
			base = base[:i]
		}
		if reservedDeviceNames[strings.ToUpper(base)] {
			return "reserved device name"
		}
	}
	return ""
}
