package gobuild

import (
	"fmt"
	"os/exec"
	"runtime/debug"
	"strings"
)

// ReadBuildInfo reads the build information the toolchain embedded in a built
// binary, as debug.ReadBuildInfo reports it from inside: `go version -m` prints
// the same records, indented under a line naming the binary and its toolchain.
func ReadBuildInfo(binary string) (*debug.BuildInfo, error) {
	out, err := exec.Command("go", "version", "-m", binary).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go version -m %s: %v\n%s", binary, err, out)
	}
	header, body, found := strings.Cut(string(out), "\n")
	if !found {
		return nil, fmt.Errorf("go version -m %s: no build information in %q", binary, out)
	}
	_, goVersion, found := strings.Cut(header, ": ")
	if !found {
		return nil, fmt.Errorf("go version -m %s: unexpected first line %q", binary, header)
	}
	var records strings.Builder
	fmt.Fprintf(&records, "go\t%s\n", goVersion)
	for _, line := range strings.Split(body, "\n") {
		if rest, indented := strings.CutPrefix(line, "\t"); indented {
			records.WriteString(rest)
			records.WriteByte('\n')
		}
	}
	info, err := debug.ParseBuildInfo(records.String())
	if err != nil {
		return nil, fmt.Errorf("go version -m %s: %v", binary, err)
	}
	return info, nil
}
