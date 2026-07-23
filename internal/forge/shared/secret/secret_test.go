package secret

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestString(t *testing.T) {
	s := New("super-secret-key")
	if s.String() != "***" {
		t.Errorf("String() = %q, want %q", s.String(), "***")
	}
}

func TestGoString(t *testing.T) {
	s := New("key")
	if s.GoString() != "secret.Str{***}" {
		t.Errorf("GoString() = %q", s.GoString())
	}
}

func TestValue(t *testing.T) {
	s := New("actual-key-value")
	if s.Value() != "actual-key-value" {
		t.Errorf("Value() = %q", s.Value())
	}
}

func TestFormat(t *testing.T) {
	s := New("key123")

	tests := []struct {
		format string
		want   string
	}{
		{"%s", "***"},
		{"%v", "***"},
		{"%+v", "secret.Str{***}"},
		{"%#v", "secret.Str{***}"},
		{"%q", "***"},
	}

	for _, tt := range tests {
		got := fmt.Sprintf(tt.format, s)
		if got != tt.want {
			t.Errorf("Sprintf(%q) = %q, want %q", tt.format, got, tt.want)
		}
	}
}

func TestMarshalJSON(t *testing.T) {
	s := New("secret-key")
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if string(data) != `"***"` {
		t.Errorf("JSON = %s, want %q", data, "***")
	}
}

func TestMarshalJSON_InStruct(t *testing.T) {
	type config struct {
		Key Str `json:"key"`
	}
	c := config{Key: New("real-api-key")}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(data) != `{"key":"***"}` {
		t.Errorf("JSON = %s", data)
	}
}

func TestIsEmpty(t *testing.T) {
	if !New("").IsEmpty() {
		t.Error("empty string should be empty")
	}
	if New("x").IsEmpty() {
		t.Error("non-empty string should not be empty")
	}
}

func TestNeverLeaksInSprintf(t *testing.T) {
	s := New("sk-ant-" + "super-secret-key-12345") // split literal: synthetic test value, not a real key
	formats := []string{"%s", "%v", "%+v", "%#v", "%q", "%x"}
	for _, f := range formats {
		got := fmt.Sprintf(f, s)
		if got == s.Value() || len(got) > 30 {
			t.Errorf("format %q may leak secret: %q", f, got)
		}
	}
}
