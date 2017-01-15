package providers

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	trailingApostrophe = regexp.MustCompile(`'([a-z]{1,2}\s)`)
)

func RemoveTrailingApostrophe(str string) string {
	return trailingApostrophe.ReplaceAllString(str, "$1")
}

func RemoveTrailingApostrophes(str string) string {
	return ""
}

// as per http://en.wikipedia.org/wiki/Hepburn_romanization#Variations
func RomanizeHepburn(str string) string {
	str = strings.Replace(str, "ō", "ou", -1)
	str = strings.Replace(str, "ū", "uu", -1)
	return str
}

func NormalizeTitle(title string) string {
	normalizedTitle := title
	normalizedTitle = strings.ToLower(normalizedTitle)
	normalizedTitle = RomanizeHepburn(title)
	normalizedTitle = strings.ToLower(normalizedTitle)
	normalizedTitle = RemoveTrailingApostrophe(normalizedTitle)
	normalizedTitle, _, _ = transform.String(transform.Chain(
		norm.NFD,
		transform.RemoveFunc(func(r rune) bool {
			return unicode.Is(unicode.Mn, r)
		}),
		norm.NFC), normalizedTitle)
	normalizedTitle = strings.ToLower(normalizedTitle)
	normalizedTitle = regexp.MustCompile(`\(\d+\)`).ReplaceAllString(normalizedTitle, " ")
	normalizedTitle = strings.Map(func(r rune) rune {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' {
			return ' '
		}
		return r
	}, normalizedTitle)
	normalizedTitle = regexp.MustCompile(`\s+`).ReplaceAllString(normalizedTitle, " ")
	normalizedTitle = strings.TrimSpace(normalizedTitle)

	return normalizedTitle
}
