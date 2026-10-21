// Package extract turns notes and resources into facts.
package extract

import "strings"

// RawToken is a sigil token found on a single line. Start and End are byte
// offsets of the whole token (including the sigil) within the line.
type RawToken struct {
	Sigil      byte
	Body       string
	Start, End int
}

var months = map[string]bool{
	"jan": true, "feb": true, "mar": true, "apr": true, "may": true, "jun": true,
	"jul": true, "aug": true, "sep": true, "oct": true, "nov": true, "dec": true,
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isBody(c byte) bool {
	return isLetter(c) || isDigit(c) || c == '.' || c == '_' || c == '-'
}

// ScanLine finds tokens with sigils + - ~ ! @ in a single line. A sigil counts
// only at line start or after whitespace or '(' and when followed by an ASCII
// letter (or a digit, for '@'). It never returns nil.
func ScanLine(line string) []RawToken {
	out := []RawToken{}
	for i := 0; i < len(line); i++ {
		c := line[i]
		if !strings.ContainsRune("+-~!@", rune(c)) {
			continue
		}
		if i > 0 && line[i-1] != ' ' && line[i-1] != '\t' && line[i-1] != '(' {
			continue
		}
		if i+1 >= len(line) {
			continue
		}
		n := line[i+1]
		if !isLetter(n) && !(c == '@' && isDigit(n)) {
			continue
		}
		j := i + 1
		for j < len(line) && isBody(line[j]) {
			j++
		}
		scanEnd := j
		end := j
		for end > i+1 && (line[end-1] == '.' || line[end-1] == '-' || line[end-1] == '_') {
			end--
		}
		// "@28 Oct": 1-2 digit day, one space, 3-letter month.
		if c == '@' && end == scanEnd && end-(i+1) <= 2 && allDigits(line[i+1:end]) &&
			end+1 < len(line) && line[end] == ' ' {
			k := end + 1
			m := k
			for m < len(line) && isLetter(line[m]) {
				m++
			}
			if m-k == 3 && (m == len(line) || !isDigit(line[m])) && months[strings.ToLower(line[k:m])] {
				end = m
				scanEnd = m
			}
		}
		out = append(out, RawToken{Sigil: c, Body: line[i+1 : end], Start: i, End: end})
		i = scanEnd - 1
	}
	return out
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return s != ""
}
