package gamestate

import "testing"

func TestParseLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Line
		ok   bool
	}{
		{
			name: "screen switch",
			in:   "[1:05:29] - RUNTIME - IMPORTANT: Screen has been switched to type: `Gameplay`",
			want: Line{Kind: KindState, State: "gameplay"},
			ok:   true,
		},
		{
			name: "score saved",
			in:   "[1:06:06] - RUNTIME - IMPORTANT: Successfully saved score to database with id: 78",
			want: Line{Kind: KindScoreSaved, ScoreID: 78},
			ok:   true,
		},
		{
			name: "replay saved",
			in:   "[1:06:06] - RUNTIME - IMPORTANT: Successfully saved replay to path: D:\\SteamLibrary\\steamapps\\common\\Quaver/Data/r/78.qr",
			want: Line{Kind: KindReplaySaved, ReplayPath: "D:\\SteamLibrary\\steamapps\\common\\Quaver/Data/r/78.qr"},
			ok:   true,
		},
		{
			name: "game closed",
			in:   "[3:31:14] - RUNTIME - IMPORTANT: Discord RPC has shutdown",
			want: Line{Kind: KindExit},
			ok:   true,
		},
		{name: "scheduled change ignored", in: "[1:05:29] - RUNTIME - IMPORTANT: Scheduled Screen Change"},
		{name: "loaded but not switched ignored", in: "[1:05:29] - RUNTIME - IMPORTANT: Screen `Gameplay` has been loaded proceeding to switch."},
		{name: "empty", in: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseLine(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("ParseLine(%q) = %+v, %v; want %+v, %v", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}
