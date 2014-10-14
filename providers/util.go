package providers

import (
	"regexp"
	"strings"
	"unicode"

	"code.google.com/p/go.text/transform"
	"code.google.com/p/go.text/unicode/norm"
)

func NormalizeTitle(title string) string {
	normalizedTitle := title
	normalizedTitle, _, _ = transform.String(transform.Chain(
		norm.NFD,
		transform.RemoveFunc(func(r rune) bool {
			return unicode.Is(unicode.Mn, r)
		}),
		norm.NFC), normalizedTitle)
	normalizedTitle = strings.ToLower(normalizedTitle)
	normalizedTitle = regexp.MustCompile(`\(\d+\)`).ReplaceAllString(normalizedTitle, " ")
	normalizedTitle = strings.Map(func(r rune) rune {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return ' '
		}
		return r
	}, normalizedTitle)
	normalizedTitle = regexp.MustCompile(`\s+`).ReplaceAllString(normalizedTitle, " ")
	normalizedTitle = strings.TrimSpace(normalizedTitle)

	return normalizedTitle
}
