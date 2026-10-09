package notebook

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Selection picks the code cells of a notebook a load reads: by position among
// the code cells, 1-based, as `1,3-5`, or by a tag they carry, as `tag:model`.
// The zero Selection picks every cell.
type Selection struct {
	ranges []cellRange
	tag    string
}

type cellRange struct{ from, to int }

// ParseSelection reads a selection as it is written after `--cells`.
func ParseSelection(text string) (Selection, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Selection{}, errors.New("a selection names cells by position (1,3-5) or by tag (tag:<tag>)")
	}
	if tag, ok := strings.CutPrefix(text, "tag:"); ok {
		if tag == "" {
			return Selection{}, errors.New("tag: names the tag the cells carry")
		}
		return Selection{tag: tag}, nil
	}
	var sel Selection
	for _, part := range strings.Split(text, ",") {
		r, err := parseRange(strings.TrimSpace(part))
		if err != nil {
			return Selection{}, err
		}
		sel.ranges = append(sel.ranges, r)
	}
	return sel, nil
}

// parseRange reads one position or one from-to span of positions.
func parseRange(part string) (cellRange, error) {
	from, to, spanned := strings.Cut(part, "-")
	lo, err := parsePosition(from)
	if err != nil {
		return cellRange{}, err
	}
	if !spanned {
		return cellRange{from: lo, to: lo}, nil
	}
	hi, err := parsePosition(to)
	if err != nil {
		return cellRange{}, err
	}
	if hi < lo {
		return cellRange{}, fmt.Errorf("cells %d-%d: the range runs backwards", lo, hi)
	}
	return cellRange{from: lo, to: hi}, nil
}

func parsePosition(text string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%q is not a cell position; cells are counted from 1 among the notebook's code cells", text)
	}
	return n, nil
}

// IsZero reports whether the selection picks every cell.
func (s Selection) IsZero() bool { return len(s.ranges) == 0 && s.tag == "" }

// String is the selection as it was written.
func (s Selection) String() string {
	if s.tag != "" {
		return "tag:" + s.tag
	}
	parts := make([]string, 0, len(s.ranges))
	for _, r := range s.ranges {
		if r.from == r.to {
			parts = append(parts, strconv.Itoa(r.from))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r.from, r.to))
		}
	}
	return strings.Join(parts, ",")
}

// Apply picks the notebook's cells the selection names, in notebook order. A
// position past the notebook's code cells is an error naming how many there
// are; a tag no cell carries is an error naming the tag.
func (s Selection) Apply(nb *Notebook) ([]Cell, error) {
	if s.IsZero() {
		return nb.Cells, nil
	}
	if s.tag != "" {
		var out []Cell
		for _, c := range nb.Cells {
			if c.Tagged(s.tag) {
				out = append(out, c)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("%s: no code cell is tagged %s", nb.Name, s.tag)
		}
		return out, nil
	}
	picked := make([]bool, len(nb.Cells)+1)
	for _, r := range s.ranges {
		if r.to > len(nb.Cells) {
			return nil, fmt.Errorf("%s: cells %s selected, but the notebook has %s", nb.Name, s, countCells(len(nb.Cells)))
		}
		for i := r.from; i <= r.to; i++ {
			picked[i] = true
		}
	}
	var out []Cell
	for _, c := range nb.Cells {
		if picked[c.Index] {
			out = append(out, c)
		}
	}
	return out, nil
}

// countCells spells a code-cell count.
func countCells(n int) string {
	if n == 1 {
		return "1 code cell"
	}
	return fmt.Sprintf("%d code cells", n)
}
