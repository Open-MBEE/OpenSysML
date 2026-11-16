// Package cellmerge merges three versions of a line-oriented count baseline cell by cell, where
// git's line merge conflicts as soon as two branches move counts on neighbouring lines.
package cellmerge

import (
	"fmt"
	"strconv"
	"strings"
)

// line is one line of a baseline: its text with every count replaced by a placeholder, the
// counts themselves, and the key that addresses it in the other versions.
type line struct {
	key    string
	parts  []string
	counts []int
}

// Conflict is a movement the cells cannot merge; the caller falls back to re-recording.
type Conflict struct{ Reason string }

func (c *Conflict) Error() string { return c.Reason }

// Result is a merged baseline and what the merge did to it.
type Result struct {
	Content []byte
	// Summed names the lines a count of which both sides moved differently, merged as
	// ours + theirs - base; only a check that reproduces the result makes it the measurement.
	Summed []string
}

// Merge merges ours and theirs over base: a count one side moved takes that side's value, one both
// moved takes the sum of the movements. Lines are addressed by text and enclosing lines, not position.
func Merge(base, ours, theirs []byte) (*Result, error) {
	b, err := parse(base)
	if err != nil {
		return nil, err
	}
	o, err := parse(ours)
	if err != nil {
		return nil, err
	}
	t, err := parse(theirs)
	if err != nil {
		return nil, err
	}
	inBase, inOurs, inTheirs := index(b), index(o), index(t)
	if err := replacedByBoth(b, o, t, inOurs, inTheirs); err != nil {
		return nil, err
	}
	result := &Result{}
	var merged []line
	placed := make(map[string]bool)
	emit := func(l line) {
		merged = append(merged, l)
		placed[l.key] = true
	}
	// Ours gives the order; a line theirs added follows the line it follows in theirs.
	after := make(map[string][]line)
	previous := ""
	for _, l := range t {
		if _, ok := inOurs[l.key]; !ok {
			if _, ok := inBase[l.key]; !ok {
				after[previous] = append(after[previous], l)
				continue
			}
		}
		previous = l.key
	}
	var follow func(key string)
	follow = func(key string) {
		for _, l := range after[key] {
			emit(l)
			follow(l.key)
		}
	}
	follow("")
	for _, l := range o {
		theirLine, theirsHas := inTheirs[l.key]
		baseLine, baseHas := inBase[l.key]
		switch {
		case theirsHas && baseHas:
			cells, summed := mergeCounts(baseLine.counts, l.counts, theirLine.counts)
			l.counts = cells
			if summed {
				result.Summed = append(result.Summed, l.text())
			}
			emit(l)
		case theirsHas:
			if !equal(l.counts, theirLine.counts) {
				return nil, &Conflict{Reason: fmt.Sprintf("both sides added %q with different counts", l.text())}
			}
			emit(l)
		case baseHas:
			// Theirs removed the line: the removal stands unless ours moved a count on it.
			if !equal(l.counts, baseLine.counts) {
				return nil, &Conflict{Reason: fmt.Sprintf("one side removed %q and the other moved its counts", baseLine.text())}
			}
			continue
		default:
			emit(l)
		}
		follow(l.key)
	}
	for _, l := range t {
		if placed[l.key] {
			continue
		}
		if _, ok := inOurs[l.key]; ok {
			continue
		}
		baseLine, ok := inBase[l.key]
		if !ok {
			// Added by theirs after a line ours removed: it has no neighbour left to follow.
			emit(l)
			continue
		}
		// Ours removed a line theirs still has: the removal stands unless theirs moved it.
		if !equal(l.counts, baseLine.counts) {
			return nil, &Conflict{Reason: fmt.Sprintf("one side removed %q and the other moved its counts", baseLine.text())}
		}
	}
	var out strings.Builder
	for _, l := range merged {
		out.WriteString(l.text())
		out.WriteByte('\n')
	}
	content := out.String()
	if !strings.HasSuffix(string(ours), "\n") {
		content = strings.TrimSuffix(content, "\n")
	}
	result.Content = []byte(content)
	return result, nil
}

// replacedByBoth refuses a place where both sides dropped a line of base and each wrote its own
// in its stead, as two recordings do to a date or a digest: keeping both lines is no version of the file.
func replacedByBoth(base, ours, theirs []line, inOurs, inTheirs map[string]line) error {
	common := func(key string) bool {
		_, o := inOurs[key]
		_, t := inTheirs[key]
		return o && t
	}
	dropped := make(map[string]string)
	anchor := ""
	for _, l := range base {
		_, o := inOurs[l.key]
		_, t := inTheirs[l.key]
		switch {
		case o && t:
			anchor = l.key
		case !o && !t:
			dropped[anchor] = l.text()
		}
	}
	written := func(side []line) map[string][]string {
		out := make(map[string][]string)
		anchor := ""
		for _, l := range side {
			if common(l.key) {
				anchor = l.key
				continue
			}
			if _, both := dropped[anchor]; both {
				out[anchor] = append(out[anchor], l.key)
			}
		}
		return out
	}
	theirsWrote := written(theirs)
	for anchor, mine := range written(ours) {
		if yours := theirsWrote[anchor]; len(yours) > 0 && strings.Join(mine, "\n") != strings.Join(yours, "\n") {
			return &Conflict{Reason: fmt.Sprintf("both sides replaced %q, each with its own text", dropped[anchor])}
		}
	}
	return nil
}

// mergeCounts merges one line's counts and reports whether any took the sum of two movements.
func mergeCounts(base, ours, theirs []int) ([]int, bool) {
	merged, summed := make([]int, len(ours)), false
	for i := range ours {
		switch {
		case ours[i] == base[i]:
			merged[i] = theirs[i]
		case theirs[i] == base[i], ours[i] == theirs[i]:
			merged[i] = ours[i]
		default:
			merged[i], summed = ours[i]+theirs[i]-base[i], true
		}
	}
	return merged, summed
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func index(lines []line) map[string]line {
	byKey := make(map[string]line, len(lines))
	for _, l := range lines {
		byKey[l.key] = l
	}
	return byKey
}

// text spells the line with its counts in place.
func (l line) text() string {
	var b strings.Builder
	for i, part := range l.parts {
		b.WriteString(part)
		if i < len(l.counts) {
			b.WriteString(strconv.Itoa(l.counts[i]))
		}
	}
	return b.String()
}

// parse splits content into lines keyed by their template under their less indented ancestors;
// the n-th line of one key under one parent is a distinct line, as the elements of a list are.
func parse(content []byte) ([]line, error) {
	text := strings.TrimSuffix(string(content), "\n")
	if strings.Contains(text, "\r") {
		return nil, &Conflict{Reason: "the file has carriage returns, which no recorder writes"}
	}
	type ancestor struct {
		indent int
		key    string
	}
	var stack []ancestor
	seen := make(map[string]int)
	var lines []line
	for _, raw := range strings.Split(text, "\n") {
		l := split(raw)
		template := strings.Join(l.parts, "\x00")
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if strings.TrimSpace(raw) != "" {
			for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
				stack = stack[:len(stack)-1]
			}
		}
		parent := ""
		if len(stack) > 0 {
			parent = stack[len(stack)-1].key
		}
		key := parent + "\x01" + template
		seen[key]++
		if n := seen[key]; n > 1 {
			key += "\x02" + strconv.Itoa(n)
		}
		l.key = key
		lines = append(lines, l)
		if strings.TrimSpace(raw) != "" && !closes(raw) {
			stack = append(stack, ancestor{indent: indent, key: key})
		}
	}
	return lines, nil
}

// closes reports a line that only closes a JSON object or list, which heads nothing.
func closes(raw string) bool {
	switch strings.TrimSpace(raw) {
	case "}", "},", "]", "],":
		return true
	}
	return false
}

// split separates a line's counts from its text: a count is a run of digits between the line's
// start or a blank and its end, a blank or a comma, so a digit inside a name or a digest is text.
func split(raw string) line {
	var l line
	start := 0
	for i := 0; i < len(raw); {
		if raw[i] < '0' || raw[i] > '9' {
			i++
			continue
		}
		end := i
		for end < len(raw) && raw[end] >= '0' && raw[end] <= '9' {
			end++
		}
		before := i == 0 || raw[i-1] == ' ' || raw[i-1] == '\t'
		after := end == len(raw) || raw[end] == ' ' || raw[end] == '\t' || raw[end] == ','
		if count, err := strconv.Atoi(raw[i:end]); before && after && err == nil && (end-i == 1 || raw[i] != '0') {
			l.parts = append(l.parts, raw[start:i])
			l.counts = append(l.counts, count)
			start = end
		}
		i = end
	}
	l.parts = append(l.parts, raw[start:])
	return l
}
