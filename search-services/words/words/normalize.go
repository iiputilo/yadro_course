package words

import (
	"regexp"
	"strings"

	"github.com/kljensen/snowball/english"
)

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

func Normalize(phrase string) []string {
	if phrase == "" {
		return nil
	}
	lc := strings.ToLower(phrase)
	tokens := wordRe.FindAllString(lc, -1)

	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t == "" || english.IsStopWord(t) {
			continue
		}
		stem := english.Stem(t, true)
		if stem == "" {
			continue
		}
		out = append(out, stem)
	}
	return out
}
