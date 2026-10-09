package sqlite

import (
	"strings"
	"unicode"
)

// ftsQuery turns free text typed by a person into a safe FTS5 MATCH expression.
// Each word becomes a quoted prefix phrase and the phrases are ANDed, so
// operators, column filters and punctuation in the input are plain text.
// Words with no letter or digit are dropped; an input with none returns "".
func ftsQuery(text string) string {
	var parts []string
	for _, w := range strings.Fields(text) {
		if !strings.ContainsFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			continue
		}
		parts = append(parts, `"`+strings.ReplaceAll(w, `"`, `""`)+`"*`)
	}
	return strings.Join(parts, " ")
}
