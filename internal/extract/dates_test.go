package extract

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestFileDate(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"notes/2026-W43.md", d(2026, 10, 19), true},
		{"2026-W01.md", d(2025, 12, 29), true},
		{"2026-W53.md", d(2026, 12, 28), true},
		{"2026-10-22-1on1.md", d(2026, 10, 22), true},
		{"a/b/2026-10-22.md", d(2026, 10, 22), true},
		{"misc.md", time.Time{}, false},
		{"2026-W54.md", time.Time{}, false},
		{"2025-W53.md", time.Time{}, false},
		{"2026-02-30.md", time.Time{}, false},
		{"2026-W00.md", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := FileDate(c.in)
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("FileDate(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestHeadingDate(t *testing.T) {
	ref := d(2026, 10, 19)
	cases := []struct {
		in   string
		ref  time.Time
		want time.Time
		ok   bool
	}{
		{"Sam 2026-10-22", ref, d(2026, 10, 22), true},
		{"Sam (Wed 22 Oct)", ref, d(2026, 10, 22), true},
		{"Sam (Fri 1 Jan)", d(2026, 12, 28), d(2027, 1, 1), true},
		{"2026-W43", ref, d(2026, 10, 19), true},
		{"Sam wed 22 oct 2025", ref, d(2025, 10, 22), true},
		{"Sam 22 October", ref, d(2026, 10, 22), true},
		{"Sam (Sat 31 Dec)", d(2027, 1, 3), d(2026, 12, 31), true},
		{"1:1 Sam", ref, time.Time{}, false},
		{"Sam 30 Feb 2026", ref, time.Time{}, false},
		{"1:1 May", ref, time.Time{}, false},
		{"1:1 Jan", ref, time.Time{}, false},
		{"1:1 June", ref, time.Time{}, false},
		{"Sam 1 May", ref, d(2026, 5, 1), true},
		{"1:1 Sam (1 May)", ref, d(2026, 5, 1), true},
	}
	for _, c := range cases {
		got, ok := HeadingDate(c.in, c.ref)
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("HeadingDate(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestDueDate(t *testing.T) {
	mon := d(2026, 10, 19)
	cases := []struct {
		in    string
		entry time.Time
		want  time.Time
		ok    bool
	}{
		{"fri", mon, d(2026, 10, 23), true},
		{"Friday", mon, d(2026, 10, 23), true},
		{"mon", mon, d(2026, 10, 26), true},
		{"28oct", mon, d(2026, 10, 28), true},
		{"28 Oct", mon, d(2026, 10, 28), true},
		{"28 OCT", mon, d(2026, 10, 28), true},
		{"19oct", mon, d(2026, 10, 19), true},
		{"2026-11-02", mon, d(2026, 11, 2), true},
		{"sam", mon, time.Time{}, false},
		{"3jan", d(2026, 12, 29), d(2027, 1, 3), true},
		{"30feb", mon, time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := DueDate(c.in, c.entry)
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("DueDate(%q,%v) = %v,%v want %v,%v", c.in, c.entry, got, ok, c.want, c.ok)
		}
	}
}
