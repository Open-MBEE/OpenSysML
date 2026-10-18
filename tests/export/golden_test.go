package export_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// A golden over goldenDigestBytes is committed as its sha256 and first goldenHeadLines
// lines (path + ".sha256"), under the fixture cap tests/hygiene enforces.
const (
	goldenDigestBytes = 200 * 1024
	goldenHeadLines   = 40
)

// checkGolden compares got with the golden at path, whole or digested; -update
// rewrites the form the output's size calls for and removes the other.
func checkGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	digestPath := path + ".sha256"
	if *update {
		write, remove, data := path, digestPath, got
		if len(got) > goldenDigestBytes {
			write, remove, data = digestPath, path, goldenDigest(got)
		}
		if err := os.WriteFile(write, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(remove); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err == nil {
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
		}
		return
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	digest, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatalf("%s: no golden, whole or digested (run with -update to create it)", path)
	}
	checkGoldenDigest(t, path, digest, got)
}

// goldenDigest is the digested form: one header line with the sha256, byte and
// line counts of the whole output, then its first goldenHeadLines lines.
func goldenDigest(got []byte) []byte {
	sum := sha256.Sum256(got)
	lines := strings.SplitAfter(string(got), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	head := lines
	if len(head) > goldenHeadLines {
		head = head[:goldenHeadLines]
	}
	header := fmt.Sprintf("sha256 %s bytes %d lines %d\n", hex.EncodeToString(sum[:]), len(got), len(lines))
	return []byte(header + strings.Join(head, ""))
}

// checkGoldenDigest fails when got does not hash to the digest, writing the whole
// output as path + ".actual" (gitignored) and naming the first recorded line that differs.
func checkGoldenDigest(t *testing.T, path string, digest, got []byte) {
	t.Helper()
	header, head, _ := strings.Cut(string(digest), "\n")
	var wantSum string
	var wantBytes, wantLines int
	if _, err := fmt.Sscanf(header, "sha256 %s bytes %d lines %d", &wantSum, &wantBytes, &wantLines); err != nil {
		t.Fatalf("%s.sha256: malformed header %q: %v", path, header, err)
	}
	sum := sha256.Sum256(got)
	if hex.EncodeToString(sum[:]) == wantSum {
		if !bytes.Equal(digest, goldenDigest(got)) {
			t.Errorf("%s.sha256: counts or head are stale for the output they digest (run with -update)", path)
		}
		return
	}
	actual := path + ".actual"
	if err := os.WriteFile(actual, got, 0o644); err != nil {
		t.Fatal(err)
	}
	gotLines := strings.SplitAfter(string(got), "\n")
	summary := fmt.Sprintf("the first %d lines match; the difference is after them", goldenHeadLines)
	for i, want := range strings.SplitAfter(head, "\n") {
		if want == "" {
			break
		}
		line := ""
		if i < len(gotLines) {
			line = gotLines[i]
		}
		if line != want {
			summary = fmt.Sprintf("line %d differs\n--- want ---\n%s--- got ---\n%s", i+1, want, line)
			break
		}
	}
	t.Errorf("%s differs: want %d bytes in %d lines, got %d bytes in %d lines; full output written to %s\n%s",
		path, wantBytes, wantLines, len(got), countLines(got), actual, summary)
}

// countLines counts a final line that lacks its newline too.
func countLines(data []byte) int {
	lines := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}
