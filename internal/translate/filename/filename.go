// Package filename fits the file names a run derives from qualified names
// into what every common filesystem takes, and keeps a set of them apart:
// a name too long for one path component, or whose stem a filesystem reads
// as a device, is cut and tagged with a hash of the whole, and names that
// meet letter case aside are tagged until no two meet.
package filename

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Max is the longest name every common filesystem takes for one path component;
// TagBytes of the encoded name's hash keep a cut name apart from its neighbours.
const (
	Max      = 255
	TagBytes = 8
)

// Fit is name+ext, cut to Max bytes and tagged with `~` and a hash of the whole
// name when tagged, when it is too long, or when its stem is a device name.
// The cut splits neither a UTF-8 sequence nor a trailing `%XX` or `.XX` escape.
func Fit(name, ext string, tagged bool) string {
	if tagged || len(name)+len(ext) > Max || DeviceStem(name) {
		sum := sha256.Sum256([]byte(name))
		tag := "~" + hex.EncodeToString(sum[:TagBytes])
		name = cut(name, Max-len(ext)-len(tag)) + tag
	}
	return name + ext
}

// cut is the longest prefix of name within n bytes that splits neither a
// UTF-8 sequence nor a three-byte escape opened by `%` or `.`.
func cut(name string, n int) string {
	n = min(n, len(name))
	for n > 0 && n < len(name) && !utf8.RuneStart(name[n]) {
		n--
	}
	if i := strings.LastIndexAny(name[:n], "%."); i >= 0 && i > n-3 {
		n = i
	}
	return name[:n]
}

// DeviceStem reports whether the stem of name, what precedes its first `.`,
// is one Windows reads as a device whatever the extension, trailing spaces
// and letter case aside.
func DeviceStem(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	return windowsDeviceNames[strings.ToUpper(strings.TrimRight(stem, " "))]
}

// windowsDeviceNames are the stems Windows reads as devices whatever the extension,
// trailing spaces and letter case aside: the serial and printer ports include the
// superscript digits Windows counts among them.
var windowsDeviceNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM0": true, "COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"COM¹": true, "COM²": true, "COM³": true,
	"LPT0": true, "LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	"LPT¹": true, "LPT²": true, "LPT³": true,
}

// CollisionError reports two names whose files meet even when tagged, which
// only names that are the same, or differ in bytes the encoding drops, can.
type CollisionError struct {
	Names [2]string
	File  string
}

func (e *CollisionError) Error() string {
	return fmt.Sprintf("%s and %s have the same file name %s", e.Names[0], e.Names[1], e.File)
}

// Plan is the file each name is written to, by name: file gives a name's file,
// tagged or not, and the files that meet letter case aside are tagged until no
// two meet. Two names whose tagged files still meet are a CollisionError.
func Plan(names []string, file func(name string, tagged bool) string) (map[string]string, error) {
	tagged := make(map[string]bool, len(names))
	for {
		meeting := map[string][]string{}
		var keys []string
		for _, name := range names {
			key := CaseFolded(file(name, tagged[name]))
			if _, seen := meeting[key]; !seen {
				keys = append(keys, key)
			}
			meeting[key] = append(meeting[key], name)
		}
		progressed := false
		for _, key := range keys {
			group := meeting[key]
			if len(group) < 2 {
				continue
			}
			settled := true
			for _, name := range group {
				if !tagged[name] {
					tagged[name], settled, progressed = true, false, true
				}
			}
			if settled {
				return nil, &CollisionError{Names: [2]string{group[0], group[1]}, File: file(group[0], true)}
			}
		}
		if !progressed {
			break
		}
	}
	files := make(map[string]string, len(names))
	for _, name := range names {
		files[name] = file(name, tagged[name])
	}
	return files, nil
}

// CaseFolded is text under simple Unicode case folding, the key two files of a
// Plan meet under: two texts fold alike exactly when strings.EqualFold holds of them.
func CaseFolded(text string) string {
	var b strings.Builder
	for _, r := range text {
		least := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			least = min(least, f)
		}
		b.WriteRune(least)
	}
	return b.String()
}
