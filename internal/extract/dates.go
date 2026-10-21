package extract

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	fileWeekRe = regexp.MustCompile(`^(\d{4})-W(\d{2})(?:\D|$)`)
	fileDayRe  = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})`)
	isoDayRe   = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)
	isoWeekRe  = regexp.MustCompile(`\b(\d{4})-W(\d{2})\b`)
	// optional weekday is skipped; only "D Mon [YYYY]" matters.
	headDMYRe = regexp.MustCompile(`(?i)\b(\d{1,2})\s+([a-z]{3,9})(?:\s+(\d{4}))?\b`)
	dueDMRe   = regexp.MustCompile(`^(\d{1,2})\s?([A-Za-z]{3,9})$`)
)

var monthNames = map[string]time.Month{}
var weekdayNames = map[string]time.Weekday{}

func init() {
	for m := time.January; m <= time.December; m++ {
		n := strings.ToLower(m.String())
		monthNames[n] = m
		monthNames[n[:3]] = m
	}
	for w := time.Sunday; w <= time.Saturday; w++ {
		n := strings.ToLower(w.String())
		weekdayNames[n] = w
		weekdayNames[n[:3]] = w
	}
}

// ymd builds a UTC-midnight date, rejecting invalid calendar dates.
func ymd(y, m, day int) (time.Time, bool) {
	t := time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC)
	if t.Year() != y || int(t.Month()) != m || t.Day() != day {
		return time.Time{}, false
	}
	return t, true
}

// isoWeekMonday returns the Monday of ISO week w of year y.
func isoWeekMonday(y, w int) (time.Time, bool) {
	jan4 := time.Date(y, time.January, 4, 0, 0, 0, 0, time.UTC)
	back := (int(jan4.Weekday()) + 6) % 7
	week1 := jan4.AddDate(0, 0, -back)
	t := week1.AddDate(0, 0, (w-1)*7)
	if w < 1 {
		return time.Time{}, false
	}
	if iy, iw := t.ISOWeek(); iy != y || iw != w {
		return time.Time{}, false
	}
	return t, true
}

// FileDate parses a note file's base name: YYYY-Www.md or YYYY-MM-DD*.md.
func FileDate(path string) (time.Time, bool) {
	base := filepath.Base(path)
	if m := fileWeekRe.FindStringSubmatch(base); m != nil {
		y, _ := strconv.Atoi(m[1])
		w, _ := strconv.Atoi(m[2])
		return isoWeekMonday(y, w)
	}
	if m := fileDayRe.FindStringSubmatch(base); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		dd, _ := strconv.Atoi(m[3])
		return ymd(y, mo, dd)
	}
	return time.Time{}, false
}

// HeadingDate finds the first date in heading text. ref supplies the year
// when the heading omits it: the nearest of ref's year-1, year, year+1 wins.
func HeadingDate(text string, ref time.Time) (time.Time, bool) {
	type hit struct {
		pos int
		t   time.Time
	}
	var best *hit
	consider := func(pos int, t time.Time, ok bool) {
		if ok && (best == nil || pos < best.pos) {
			best = &hit{pos, t}
		}
	}
	if m := isoDayRe.FindStringSubmatchIndex(text); m != nil {
		y, _ := strconv.Atoi(text[m[2]:m[3]])
		mo, _ := strconv.Atoi(text[m[4]:m[5]])
		dd, _ := strconv.Atoi(text[m[6]:m[7]])
		t, ok := ymd(y, mo, dd)
		consider(m[0], t, ok)
	}
	if m := isoWeekRe.FindStringSubmatchIndex(text); m != nil {
		y, _ := strconv.Atoi(text[m[2]:m[3]])
		w, _ := strconv.Atoi(text[m[4]:m[5]])
		t, ok := isoWeekMonday(y, w)
		consider(m[0], t, ok)
	}
	for _, m := range headDMYRe.FindAllStringSubmatchIndex(text, -1) {
		mon, ok := monthNames[strings.ToLower(text[m[4]:m[5]])]
		if !ok {
			continue
		}
		dd, _ := strconv.Atoi(text[m[2]:m[3]])
		var t time.Time
		if m[6] >= 0 {
			y, _ := strconv.Atoi(text[m[6]:m[7]])
			t, ok = ymd(y, int(mon), dd)
		} else {
			t, ok = nearestYear(int(mon), dd, ref)
		}
		if ok {
			consider(m[0], t, true)
			break
		}
	}
	if best == nil {
		return time.Time{}, false
	}
	return best.t, true
}

func nearestYear(mon, day int, ref time.Time) (time.Time, bool) {
	var best time.Time
	found := false
	var bestDiff time.Duration
	for _, y := range []int{ref.Year() - 1, ref.Year(), ref.Year() + 1} {
		t, ok := ymd(y, mon, day)
		if !ok {
			continue
		}
		diff := t.Sub(ref)
		if diff < 0 {
			diff = -diff
		}
		if !found || diff < bestDiff {
			best, bestDiff, found = t, diff, true
		}
	}
	return best, found
}

// DueDate resolves an @-token body (without '@') relative to the entry date.
// Weekdays resolve strictly after entry; DDmon resolves to the first
// occurrence on or after entry.
func DueDate(body string, entry time.Time) (time.Time, bool) {
	if m := isoDayRe.FindStringSubmatch(body); m != nil && m[0] == body {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		dd, _ := strconv.Atoi(m[3])
		return ymd(y, mo, dd)
	}
	if w, ok := weekdayNames[strings.ToLower(body)]; ok {
		delta := (int(w) - int(entry.Weekday()) + 7) % 7
		if delta == 0 {
			delta = 7
		}
		return entry.AddDate(0, 0, delta), true
	}
	if m := dueDMRe.FindStringSubmatch(body); m != nil {
		mon, ok := monthNames[strings.ToLower(m[2])]
		if !ok {
			return time.Time{}, false
		}
		dd, _ := strconv.Atoi(m[1])
		for _, y := range []int{entry.Year(), entry.Year() + 1, entry.Year() + 2, entry.Year() + 3, entry.Year() + 4} {
			if t, ok := ymd(y, int(mon), dd); ok && !t.Before(entry) {
				return t, true
			}
		}
	}
	return time.Time{}, false
}
