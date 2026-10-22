package extract

import (
	"reflect"
	"testing"
)

func TestScanLine(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"+ownership led the review", []string{"+ownership"}},
		{"+ownership, -planning.", []string{"+ownership", "-planning"}},
		{"(+planning) and +delivery.ownership:", []string{"+planning", "+delivery.ownership"}},
		{"+1 from me, -5 points, well-known, a - b", []string{}},
		{"x+ownership a-planning", []string{}},
		{"!self-assessment submitted", []string{"!self-assessment"}},
		{"@sam draft @fri", []string{"@sam", "@fri"}},
		{"due @28 Oct ok", []string{"@28 Oct"}},
		{"due @28 oct, ok", []string{"@28 oct"}},
		{"due @28 October", []string{"@28"}},
		{"due @28 and more", []string{"@28"}},
		{"@2026-10-28", []string{"@2026-10-28"}},
		{"@28oct", []string{"@28oct"}},
		{"a@b.com https://x.com/@foo", []string{}},
		{"~crossteam ~planning", []string{"~crossteam", "~planning"}},
		{"+ownership. and +a_b- end_", []string{"+ownership", "+a_b"}},
		{"", []string{}},
		{"++comms went well", []string{"+comms"}},
		{"--comms stalled", []string{"-comms"}},
		{"(++comms) and ++delivery.ownership,", []string{"+comms", "+delivery.ownership"}},
		{"C++ x++ a--b a++b", []string{}},
		{"-- then ++1 and ++ alone", []string{}},
		{"+++comms ---comms", []string{}},
		{"++comms --planning ++a_b-", []string{"+comms", "-planning", "+a_b"}},
	}
	for _, tc := range tests {
		got := []string{}
		toks := ScanLine(tc.line)
		if toks == nil {
			t.Errorf("%q: nil slice", tc.line)
		}
		for _, k := range toks {
			got = append(got, string(k.Sigil)+k.Body)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %v want %v", tc.line, got, tc.want)
		}
	}
}

func TestScanLineOffsets(t *testing.T) {
	line := "a +own b"
	toks := ScanLine(line)
	if len(toks) != 1 || toks[0].Start != 2 || toks[0].End != 6 {
		t.Fatalf("got %+v", toks)
	}
	line = "x +own. @28 Oct."
	toks = ScanLine(line)
	if len(toks) != 2 || line[toks[0].Start:toks[0].End] != "+own" || line[toks[1].Start:toks[1].End] != "@28 Oct" {
		t.Fatalf("got %+v", toks)
	}
}

func TestScanLineStrong(t *testing.T) {
	line := "a ++comms, (--planning) +own"
	toks := ScanLine(line)
	if len(toks) != 3 {
		t.Fatalf("got %+v", toks)
	}
	for i, want := range []struct {
		text   string
		strong bool
	}{{"++comms", true}, {"--planning", true}, {"+own", false}} {
		if line[toks[i].Start:toks[i].End] != want.text || toks[i].Strong != want.strong {
			t.Errorf("token %d: %+v, want %q strong=%v", i, toks[i], want.text, want.strong)
		}
	}
}
