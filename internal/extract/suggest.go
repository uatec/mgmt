package extract

import "strings"

// Suggest returns the candidate closest to word by Levenshtein distance,
// compared case-insensitively, when the distance is at most
// max(2, len(word)/3). Ties go to the alphabetically first candidate. It
// returns "" when nothing is close enough.
func Suggest(word string, candidates []string) string {
	w := strings.ToLower(word)
	limit := max(2, len(w)/3)
	best, bestDist := "", limit+1
	for _, c := range candidates {
		d := levenshtein(w, strings.ToLower(c))
		if d < bestDist || (d == bestDist && best != "" && c < best) {
			best, bestDist = c, d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
