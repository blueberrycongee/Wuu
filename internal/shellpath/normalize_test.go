package shellpath

import "testing"

func TestRewriteCmdNullRedirect(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"go test ./... 2>nul", "go test ./... 2>/dev/null"},
		{"go test ./... 2> nul", "go test ./... 2> /dev/null"},
		{"echo hi >nul", "echo hi >/dev/null"},
		{"echo hi >NUL", "echo hi >/dev/null"},
		{"cmd 2>>nul", "cmd 2>>/dev/null"},
		{"cmd 2>nul; echo done", "cmd 2>/dev/null; echo done"},
		{"cmd &>nul", "cmd &>/dev/null"},
		// Real filenames that merely start with nul stay untouched.
		{"cat 2>nul.txt", "cat 2>nul.txt"},
		{"grep nul file", "grep nul file"},
		{"echo 2>/dev/null", "echo 2>/dev/null"},
	}
	for _, tc := range cases {
		if got := rewriteCmdNullRedirect(tc.in); got != tc.want {
			t.Errorf("rewriteCmdNullRedirect(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
