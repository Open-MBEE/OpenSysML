package docplan

import "strings"

// Punctuation a run opens or closes with that binds it to its neighbour.
const (
	closingPunctuation = ".,;:!?)]}»”’"
	openingPunctuation = "([{«“‘"
)

// BindsLeft reports whether a run's text binds to the run before it, so no
// space separates them when the runs are joined: it opens with closing
// punctuation, as the ")." of "(see Section 2)." does.
func BindsLeft(text string) bool {
	return text != "" && strings.ContainsRune(closingPunctuation, firstRune(text))
}

// BindsRight reports whether a run's text binds to the run after it: it
// closes with an opening bracket or quote.
func BindsRight(text string) bool {
	return text != "" && strings.ContainsRune(openingPunctuation, lastRune(text))
}

func firstRune(text string) rune {
	for _, r := range text {
		return r
	}
	return 0
}

func lastRune(text string) rune {
	var last rune
	for _, r := range text {
		last = r
	}
	return last
}
