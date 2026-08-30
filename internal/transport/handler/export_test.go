package handler

import "testing"

func TestSanitizeCSVCell(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"plain text", "plain text"},
		{"123", "123"},
		{"=cmd|'/c calc'!A1", "'=cmd|'/c calc'!A1"}, // formula injection
		{"+1+1", "'+1+1"},               // starts with +
		{"-1+1", "'-1+1"},               // starts with -
		{"@SUM(A1:A2)", "'@SUM(A1:A2)"}, // starts with @
		{"\ttabbed", "'\ttabbed"},       // starts with tab
		{"\rCR", "'\rCR"},               // starts with CR
		{"hello=world", "hello=world"},  // = not at start: safe
		{"=HYPERLINK(\"http://evil\")", "'=HYPERLINK(\"http://evil\")"},
	}
	for _, c := range cases {
		got := sanitizeCSVCell(c.in)
		if got != c.want {
			t.Errorf("sanitizeCSVCell(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
