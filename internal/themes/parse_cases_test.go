package themes

import "testing"

func TestParseColorsCases(t *testing.T) {
	for _, tc := range []struct {
		name, css string
		want      Colors
	}{
		{"semicolon in a comment", ":root { /* a; --bg: #111; */ --fg: #222; --accent: #333; }", Colors{FG: "#222", Accent: "#333"}},
		{"duplicate property, last wins", ":root { --bg: #111; --bg: #222; }", Colors{BG: "#222"}},
		{"uppercase hex", ":root { --bg: #ABCDEF; --fg: #FFF; }", Colors{BG: "#ABCDEF", FG: "#FFF"}},
		{"url with a semicolon is not a color", ":root { --bg: url(data:image/png;base64,AA); --fg: #222; }", Colors{FG: "#222"}},
		{"important is not a hex color", ":root { --bg: #111 !important; --fg: #222; }", Colors{FG: "#222"}},
		{"spaces around colon and semicolon", ":root { --bg : #111 ; --fg  :  #222  ;--accent:#333 ; }", Colors{BG: "#111", FG: "#222", Accent: "#333"}},
		{"invalid duplicate clears the value", ":root { --bg: #111; --bg: red; }", Colors{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseColors(tc.css); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
