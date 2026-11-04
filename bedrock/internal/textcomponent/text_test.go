package textcomponent

import "testing"

func TestStyled(t *testing.T) {
	for _, test := range []struct{ name, in, want string }{
		{"plain", "hello", `{"text":"hello"}`},
		{"empty", "", `{"text":""}`},
		{"color", "§a绿色", `{"text":"绿色","color":"green"}`},
		{"color then bold", "§a绿§l粗", `{"text":"","extra":[{"text":"绿","color":"green"},{"text":"粗","color":"green","bold":true}]}`},
		{"reset", "§c红§r普通", `{"text":"","extra":[{"text":"红","color":"red"},{"text":"普通"}]}`},
		{"colour resets formatting", "§l粗§c红", `{"text":"","extra":[{"text":"粗","bold":true},{"text":"红","color":"red"}]}`},
		{"unknown code kept", "§z未知", `{"text":"§z未知"}`},
		{"newline inside run", "§a甲\n乙", `{"text":"甲\n乙","color":"green"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Styled(test.in); got != test.want {
				t.Fatalf("Styled(%q) = %s, want %s", test.in, got, test.want)
			}
		})
	}
}
