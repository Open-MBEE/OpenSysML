// Command baseline-merge is the git merge driver of the count baselines (see .gitattributes):
// it merges BASE, OURS and THEIRS cell by cell into OURS, or exits 1 leaving OURS as it was.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/cellmerge"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) < 3 || len(args) > 4 {
		fmt.Fprintln(stderr, "usage: baseline-merge BASE OURS THEIRS [PATH]")
		return 2
	}
	name := args[1]
	if len(args) == 4 {
		name = args[3]
	}
	var versions [3][]byte
	for i, path := range args[:3] {
		content, err := os.ReadFile(path) // #nosec G304 -- the three versions git names
		if err != nil {
			fmt.Fprintf(stderr, "baseline-merge: %v\n", err)
			return 2
		}
		versions[i] = content
	}
	result, err := cellmerge.Merge(versions[0], versions[1], versions[2])
	var conflict *cellmerge.Conflict
	if errors.As(err, &conflict) {
		fmt.Fprintf(stderr, "baseline-merge: %s: %s; re-record it after the merge\n", name, conflict.Reason)
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "baseline-merge: %s: %v\n", name, err)
		return 2
	}
	if err := os.WriteFile(args[1], result.Content, 0o600); err != nil { // #nosec G703 -- the path git names for the result
		fmt.Fprintf(stderr, "baseline-merge: %v\n", err)
		return 2
	}
	for _, line := range result.Summed {
		fmt.Fprintf(stderr, "baseline-merge: %s: both sides moved a count, summed: %s\n", name, line)
	}
	fmt.Fprintf(stderr, "baseline-merge: %s merged cell by cell; it is the measurement only once its -check reproduces it\n", name)
	return 0
}
