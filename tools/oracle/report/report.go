// Package report holds what every referee's report shares: the verdict
// buckets, and the output-directory files (JSON, text, extra formats).
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Files is one run's output set: every file shares the directory and the stem,
// so build/pilot-diff holds pilot-diff.json, pilot-diff.txt and so on.
type Files struct {
	dir     string
	stem    string
	written []string
}

// Open creates dir and returns the set whose files are named stem.<ext>.
func Open(dir, stem string) (*Files, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Files{dir: dir, stem: stem}, nil
}

// Path names the file with the given extension, whether or not it exists yet.
func (f *Files) Path(ext string) string {
	return filepath.Join(f.dir, f.stem+"."+ext)
}

// JSON writes v, indented and newline-terminated, as stem.json and returns the
// bytes written so the caller can compare them against a baseline.
func (f *Files) JSON(v any) ([]byte, error) {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	encoded = append(encoded, '\n')
	if err := f.Write("json", encoded); err != nil {
		return nil, err
	}
	return encoded, nil
}

// Text writes the human-readable rendering as stem.txt.
func (f *Files) Text(text string) error {
	return f.Write("txt", []byte(text))
}

// Write stores content as stem.<ext> and records it for Announce.
func (f *Files) Write(ext string, content []byte) error {
	path := f.Path(ext)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return err
	}
	f.written = append(f.written, path)
	return nil
}

// Written lists the files written so far, in order.
func (f *Files) Written() []string {
	return append([]string(nil), f.written...)
}

// Announce reports the files written and the run's headline on w.
func (f *Files) Announce(w io.Writer, headline string) {
	if len(f.written) > 0 {
		fmt.Fprintf(w, "wrote %s\n", list(f.written))
	}
	fmt.Fprintf(w, "%s\n", headline)
}

// list joins names as "a, b and c".
func list(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
