package relay

import (
	"net/url"
	"testing"
)

func TestQueryInt(t *testing.T) {
	q, _ := url.ParseQuery("lines=2000&rows=0&cols=abc&big=999999")

	cases := []struct {
		key  string
		want int
	}{
		{"lines", 2000},
		{"rows", 24},
		{"cols", 80},
		{"big", 100},
		{"missing", 7},
	}
	fallbacks := map[string]int{"lines": 100, "rows": 24, "cols": 80, "big": 100, "missing": 7}

	for _, tc := range cases {
		if got := queryInt(q, tc.key, fallbacks[tc.key], 5000); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.key, got, tc.want)
		}
	}
}
