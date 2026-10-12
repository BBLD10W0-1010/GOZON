package cli

import (
	"fmt"
	"hm1/internal/domain"
	"strings"
	"unicode"
)

// Split supports quoted titles and paths, including empty quoted arguments.
// Backslashes in Windows paths remain literal; only an escaped quote is special.
func Split(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote rune
	started := false
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if quote != 0 {
			if r == '\\' && i+1 < len(runes) && runes[i+1] == quote {
				word.WriteRune(quote)
				i++
			} else if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(r)
		started = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("незакрытая кавычка: %w", domain.ErrInvalid)
	}
	if started {
		words = append(words, word.String())
	}
	return words, nil
}
