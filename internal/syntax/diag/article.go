package diag

import "strings"

// consonantOpenings are the spellings a word opens with that begin with a vowel
// and are said with a consonant: "a use case", "a unit", "a one-of".
var consonantOpenings = []string{"eu", "one", "unif", "union", "uniq", "unit", "univ", "usa", "use", "uti"}

// IndefiniteArticle is the article a phrase reads with in a message, which follows
// how its first word is said: "an analysis case" and "an Activity", but "a use case".
func IndefiniteArticle(phrase string) string {
	first, _, _ := strings.Cut(strings.ToLower(phrase), " ")
	if first == "" || !strings.ContainsRune("aeiou", rune(first[0])) {
		return "a"
	}
	for _, opening := range consonantOpenings {
		if strings.HasPrefix(first, opening) {
			return "a"
		}
	}
	return "an"
}
