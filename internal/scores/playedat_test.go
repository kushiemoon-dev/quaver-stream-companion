package scores

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("zone %s unavailable: %v", name, err)
	}
	return loc
}

func TestParsePlayedAt(t *testing.T) {
	paris := time.FixedZone("CEST", 2*3600)
	cases := []struct{ in, want string }{
		{"10/14/2026 20:30", "2026-10-14T20:30:00+02:00"},
		{"10/14/2026 20:30:15", "2026-10-14T20:30:15+02:00"},
		{"10/14/2026 8:30 PM", "2026-10-14T20:30:00+02:00"},
		{"10/14/2026 8:30:15 PM", "2026-10-14T20:30:15+02:00"},
		{"1/5/2026 12:05 AM", "2026-01-05T00:05:00+02:00"},
		{"2026-10-12 13:06:06", "2026-10-12T13:06:06+02:00"},
		{"2026-10-12 13:06", "2026-10-12T13:06:00+02:00"},
		{"2026-10-12T13:06:06", "2026-10-12T13:06:06+02:00"},
		{"2026-10-12T13:06:06Z", "2026-10-12T13:06:06Z"},
		{"01/01/0001 00:00", "0001-01-01T00:00:00+02:00"},
		{"12/31/9999 23:59", "9999-12-31T23:59:00+02:00"},
		{"01/01/10000 00:00", "01/01/10000 00:00"},
		{"01/01/0000 00:00", "0000-01-01T00:00:00+02:00"},
		{"  10/14/2026 20:30  ", "2026-10-14T20:30:00+02:00"},
		{"02/29/2024 23:59", "2024-02-29T23:59:00+02:00"},
		{"02/29/2026 10:00", "02/29/2026 10:00"},
		{"13/14/2026 20:30", "13/14/2026 20:30"},
		{"garbage", "garbage"},
		{"", ""},
	}
	for _, c := range cases {
		if got := parsePlayedAt(c.in, paris); got != c.want {
			t.Errorf("parsePlayedAt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParsePlayedAtDaylightSaving(t *testing.T) {
	paris := zone(t, "Europe/Paris")
	// 2026-03-29 02:30 does not exist in Paris, 2026-10-25 02:30 happens twice.
	gap := parsePlayedAt("03/29/2026 02:30", paris)
	if gap != "2026-03-29T03:30:00+02:00" {
		t.Fatalf("gap time = %q", gap)
	}
	overlap := parsePlayedAt("10/25/2026 02:30", paris)
	if overlap != "2026-10-25T02:30:00+02:00" && overlap != "2026-10-25T02:30:00+01:00" {
		t.Fatalf("overlap time = %q", overlap)
	}
	if got := parsePlayedAt("07/01/2026 12:00", paris); got != "2026-07-01T12:00:00+02:00" {
		t.Errorf("summer = %q", got)
	}
	if got := parsePlayedAt("01/01/2026 12:00", paris); got != "2026-01-01T12:00:00+01:00" {
		t.Errorf("winter = %q", got)
	}
}

func TestParsePlayedAtBoundedAndSafe(t *testing.T) {
	utc := time.UTC
	huge := "10/14/2026 20:30" + strings.Repeat(" ", 10*1024)
	if got := parsePlayedAt(huge, utc); got != huge {
		t.Error("a huge value must come back unchanged")
	}
	seeds := []string{"\x00", "10/14/2026 20:30\x00", "99/99/9999 99:99", "0/0/0 0:0", "-1/-1/-1", "10/14/2026 20:30 XM",
		"१०/१४/२०२६", "2026-10-12 13:06:06.123456789", "9999999999/1/1 1:1", "\xff\xfe", strings.Repeat("/", 200)}
	for _, s := range seeds {
		got := parsePlayedAt(s, utc)
		if _, err := time.Parse(time.RFC3339, got); err != nil && got != s {
			t.Errorf("parsePlayedAt(%q) = %q, want the input or a timestamp", s, got)
		}
	}
	for i := 0; i < 2000; i++ {
		b := make([]byte, i%40)
		for j := range b {
			b[j] = "0123456789/: -TAPM\xc3"[(i*31+j*17+i/7)%18]
		}
		parsePlayedAt(string(b), utc)
	}
}

func TestScoreJSONCarriesBothTimes(t *testing.T) {
	b, err := json.Marshal(Score{PlayedAt: "2026-10-14T20:30:00+02:00", PlayedAtRaw: "10/14/2026 20:30"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["played_at"] != "2026-10-14T20:30:00+02:00" || m["played_at_raw"] != "10/14/2026 20:30" {
		t.Errorf("json = %s", b)
	}
}
