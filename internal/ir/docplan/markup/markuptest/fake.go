// Package markuptest stands in for the AsciiDoc converter in tests: a
// recorded converter answers each body it has a recording for, so the
// mapping is tested without node.
package markuptest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// RecordingsDir is the repository directory of recorded converter answers:
// each `<name>.adoc` body beside the `<name>.json` the pinned Asciidoctor.js
// converter writes for it, relative to this package's parent.
const RecordingsDir = "testdata"

// RecordedConverter writes an executable into dir that answers a body on
// stdin with the recording made for it under recordings (`*.adoc` bodies
// beside `*.json` answers), and fails naming the body when none matches.
// Trailing whitespace on each line and trailing line breaks do not take part in the match. The executable is a
// shell script, so it reports an error on hosts without `sh`.
func RecordedConverter(dir, recordings string) (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("the recorded converter is a shell script, which %s does not run", runtime.GOOS)
	}
	recordings, err := filepath.Abs(recordings)
	if err != nil {
		return "", err
	}
	script := "#!/bin/sh\n" +
		"body=$(sed -e 's/[[:space:]]*$//')\n" +
		"for adoc in " + shellQuote(recordings) + "/*.adoc; do\n" +
		"\tif [ \"$body\" = \"$(sed -e 's/[[:space:]]*$//' \"$adoc\")\" ]; then\n" +
		"\t\tcat \"${adoc%.adoc}.json\"\n" +
		"\t\texit 0\n" +
		"\tfi\n" +
		"done\n" +
		"printf 'no recorded converter answer for this body:\\n%s\\n' \"$body\" >&2\n" +
		"exit 1\n"
	path := filepath.Join(dir, "asciidoctor-json")
	err = os.WriteFile(path, []byte(script), 0o700) // #nosec G306 -- an executable test double
	if err != nil {
		return "", err
	}
	return path, nil
}

func shellQuote(s string) string {
	quoted := "'"
	for _, r := range s {
		if r == '\'' {
			quoted += `'\''`
			continue
		}
		quoted += string(r)
	}
	return quoted + "'"
}
