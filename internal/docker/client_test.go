package docker

import "testing"

func TestTrimContainerName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/mycontainer", "mycontainer"},
		{"mycontainer", "mycontainer"},
		{"/", ""},
		{"", ""},
		{"//double-slash", "/double-slash"}, // only strips one leading slash
	}
	for _, tc := range cases {
		if got := trimContainerName(tc.in); got != tc.want {
			t.Errorf("trimContainerName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
