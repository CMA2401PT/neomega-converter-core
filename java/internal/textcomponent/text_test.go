package textcomponent

import "testing"

func TestToSection(t *testing.T) {
	for _, test := range []struct{ name, in, want string }{
		{"plain", "hello", "hello"},
		{"json plain", `{"text":"hello"}`, "hello"},
		{"color", `{"text":"hi","color":"green"}`, "§ahi"},
		{"format", `{"text":"x","bold":true,"underlined":true}`, "§l§nx"},
		{"root extra", `{"text":"","extra":[{"text":"a","color":"green"},{"text":"b"}]}`, "§aa§rb"},
		{"nested extra", `{"text":"a","color":"red","extra":[{"text":"b"}]}`, "§cab"},
		{"fallback", `{"translate":"x","fallback":"fb"}`, "fb"},
		{"unknown color", `{"text":"x","color":"#ff0000"}`, "x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ToSection(test.in); got != test.want {
				t.Fatalf("ToSection(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}
