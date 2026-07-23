package devserver

import (
	"testing"
)

func TestSplitRespectingQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"name=World", []string{"name=World"}},
		{"name=World city=Paris", []string{"name=World", "city=Paris"}},
		{`name="hello world"`, []string{"name=hello world"}},
		{`name='hello world'`, []string{"name=hello world"}},
		{`a=1 b="two three" c=4`, []string{"a=1", "b=two three", "c=4"}},
		{"", nil},
		{"single", []string{"single"}},
	}

	for _, tc := range tests {
		got := splitRespectingQuotes(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("splitRespectingQuotes(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitRespectingQuotes(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`hello`, "hello"},
		{`""`, ""},
		{`"don't mix'`, `"don't mix'`},
	}

	for _, tc := range tests {
		got := unquote(tc.input)
		if got != tc.want {
			t.Errorf("unquote(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
