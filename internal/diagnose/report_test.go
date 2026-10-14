package diagnose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func TestRedactHomeCoversTheSpellingsOfAPath(t *testing.T) {
	bs := string(rune(92))
	homes := []string{"Alice", "Chloé", "André", "Борис", "李", "Bob (Work)", "Zoe.", "Alice.old", "ALICE~1", "José María"}
	for _, name := range homes {
		home := "C:" + bs + "Users" + bs + name
		cases := map[string]string{
			home + bs + "AppData" + bs + "x":  "~" + bs + "AppData" + bs + "x",
			home:                              "~",
			"C:/Users/" + name + "/AppData/x": "~/AppData/x",
			strings.ReplaceAll(home, bs, bs+bs) + bs + bs + "x": "~" + bs + bs + "x",
			bs + bs + "?" + bs + home + bs + "x":                bs + bs + "?" + bs + "~" + bs + "x",
			"in '" + home + "' and \"" + home + "\"":            "in '~' and \"~\"",
			strings.ToLower(home) + bs + "x and " + home:        "~" + bs + "x and ~",
			home + " " + home: "~ ~",
		}
		for in, want := range cases {
			if got := RedactHome(in, home, true, name); got != want {
				t.Errorf("home %q: RedactHome(%q) = %q, want %q", name, in, got, want)
			}
		}
		if got := RedactHome(home+bs+"x", home, false); strings.Contains(got, name) {
			t.Errorf("home %q leaks: %q", name, got)
		}
	}
	other := "D:" + bs + "Games" + bs + "Quaver"
	if got := RedactHome(other, "C:"+bs+"Users"+bs+"Alice", true, "Alice"); got != other {
		t.Errorf("unrelated path changed: %q", got)
	}
	if got := RedactHome("C:"+bs+"Users"+bs+"Alice2"+bs+"x", "C:"+bs+"Users"+bs+"Alice", true, "Alice"); strings.Contains(got, "Alice2") {
		t.Errorf("a different user must be hidden by the segment rule: %q", got)
	}
	if got := RedactHome("/home/carol/x", "", false); got != "/home/~/x" {
		t.Errorf("segment after home must go even without a home: %q", got)
	}
	if got := RedactHome("login Zoëlle ok", "", false, "Zoëlle"); got != "login ~ ok" {
		t.Errorf("bare name: %q", got)
	}
	if got := RedactHome("Zoëllex stays", "", false, "Zoëlle"); got != "Zoëllex stays" {
		t.Errorf("a name inside a longer word must stay: %q", got)
	}
}

func TestCutRunesKeepsUTF8Valid(t *testing.T) {
	s := strings.Repeat("é", 400)
	got := cutRunes(s, 501)
	if !utf8.ValidString(got) || len(got) > 501+len(" [cut]") {
		t.Fatalf("invalid or too long: %d bytes valid=%v", len(got), utf8.ValidString(got))
	}
}

func TestReportRedactsEveryField(t *testing.T) {
	src, d := newSource(t)
	bs := string(rune(92))
	home := "C:" + bs + "Users" + bs + "Chloé (Work)"
	src.Home = home
	src.ConfigPath = home + bs + "AppData" + bs + "config.json"
	d.AppendLog(t, "opened "+home+bs+"replays"+bs+"a.qr", "path "+strings.ReplaceAll(home, bs, "/")+"/x")
	report := Collect(context.Background(), src)
	if strings.Contains(report, "Chloé") {
		t.Fatalf("home name leaked: %s", report)
	}
}

func newSource(t *testing.T) (Source, *testutil.FakeDir) {
	t.Helper()
	d := testutil.NewQuaverDir(t)
	cfgDir := t.TempDir()
	return Source{
		Version:     "v9.9.9",
		QuaverDir:   d.Path,
		ConfigPath:  filepath.Join(cfgDir, "config.json"),
		ThemesDir:   filepath.Join(cfgDir, "themes"),
		OverlaysDir: filepath.Join(cfgDir, "overlays"),
		Port:        0,
		Home:        cfgDir,
	}, d
}

func TestReportContent(t *testing.T) {
	src, d := newSource(t)
	os.WriteFile(src.ConfigPath, []byte(`{"port": 7123, "language": "fr", "keys": {"enabled": true, "k4": ["Q","W","O","P"]}}`), 0o600)
	os.MkdirAll(filepath.Join(src.ThemesDir), 0o755)
	os.WriteFile(filepath.Join(src.ThemesDir, "mine.css"), []byte("a{}"), 0o600)
	os.MkdirAll(filepath.Join(src.OverlaysDir, "cool-card"), 0o755)
	for i := 1; i <= 80; i++ {
		d.AppendLog(t, fmt.Sprintf("runtime %d", i))
		fmt.Fprintf(appendTo(t, filepath.Join(filepath.Dir(src.ConfigPath), "qsc.log")), "qsc line %d\n", i)
	}
	src.Port = 7123
	report := Collect(context.Background(), src)

	for _, want := range []string{
		"v9.9.9", runtime.GOOS + "/" + runtime.GOARCH, runtime.Version(), "Review this report before you share it",
		`"port": 7123`, `"language": "fr"`, "[ OK ] quaver_dir", "mine.css", "cool-card",
		"qsc line 80", "qsc line 31", "runtime 80", "runtime 31",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	for _, bad := range []string{"qsc line 30\n", "runtime 30\n", `"Q"`, `"W"`, src.Home} {
		if strings.Contains(report, bad) {
			t.Errorf("report must not contain %q", bad)
		}
	}
	if !strings.Contains(report, "~") {
		t.Error("home folder must be replaced by ~")
	}
}

func appendTo(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestReportHandlesABrokenConfigAndMissingFiles(t *testing.T) {
	src, _ := newSource(t)
	os.WriteFile(src.ConfigPath, []byte("{not json"), 0o600)
	report := Collect(context.Background(), src)
	if !strings.Contains(report, "config problem") || !strings.Contains(report, "(no qsc.log)") {
		t.Fatalf("report = %s", report)
	}
}

func TestReportTruncatesLongLines(t *testing.T) {
	src, d := newSource(t)
	d.AppendLog(t, strings.Repeat("z", 5000))
	report := Collect(context.Background(), src)
	if strings.Contains(report, strings.Repeat("z", 600)) {
		t.Fatal("a log line must be cut")
	}
}

func TestHomeRedactorHidesAnAccentedUserFolder(t *testing.T) {
	bs := string(rune(92))
	home := "C:" + bs + "Users" + bs + "Zo" + string(rune(0xEB)) + " M" + string(rune(0xFC)) + "ller"
	redact := HomeRedactor(home)

	in := "dir=" + home + bs + "AppData" + bs + "Roaming" + bs + "qsc err=open " + home + bs + "x"
	got := redact(in)
	if strings.Contains(got, "ller") || strings.Contains(got, "Zo") {
		t.Fatalf("redacted = %q", got)
	}
	if !strings.Contains(got, "~"+bs+"AppData") {
		t.Fatalf("redacted = %q", got)
	}
	if redact("nothing to hide: D:/Games/Quaver") != "nothing to hide: D:/Games/Quaver" {
		t.Fatal("paths outside the home folder must stay")
	}
}

func TestHomeRedactorIsCheapPerValue(t *testing.T) {
	bs := string(rune(92))
	home := "C:" + bs + "Users" + bs + "Alice"
	redact := HomeRedactor(home)
	allocs := testing.AllocsPerRun(50, func() { redact("dir=" + home + bs + "AppData err=none") })
	if allocs > 40 {
		t.Fatalf("%v allocations per value: the patterns must be compiled once", allocs)
	}
}

func TestShortUserNamesAreNotReplacedAsWords(t *testing.T) {
	bs := string(rune(92))
	home := "C:" + bs + "Users" + bs + "Al"
	redact := HomeRedactor(home)

	got := redact("Al wrote Al.txt to " + home + bs + "docs")
	if want := "Al wrote Al.txt to ~" + bs + "docs"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	long := HomeRedactor("C:" + bs + "Users" + bs + "Alice")("hello Alice from C:" + bs + "Users" + bs + "Alice" + bs + "x")
	if strings.Contains(long, "Alice") {
		t.Fatalf("a name of four letters or more is still hidden: %q", long)
	}
}
