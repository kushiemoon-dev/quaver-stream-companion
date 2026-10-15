package diagnose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// A bare name of exactly minBareName runes is redacted as a segment; a shorter one is left alone.
func TestRedactBareNameLengthBoundary(t *testing.T) {
	if got := RedactHome(`D:\Zoey\x Zoey`, "", false, "Zoey"); got != `D:\~\x ~` {
		t.Fatalf("4-rune name: got %q", got)
	}
	if got := RedactHome(`D:\Zoe\x`, "", false, "Zoe"); got != `D:\Zoe\x` {
		t.Fatalf("3-rune name must stay: got %q", got)
	}
}

func TestLastSegmentOfARootedName(t *testing.T) {
	for in, want := range map[string]string{`\bob`: "bob", `/bob/`: "bob", `C:\Users\bob`: "bob", "bob": "bob"} {
		if got := lastSegment(in); got != want {
			t.Errorf("lastSegment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCaseInsensitiveHome(t *testing.T) {
	for _, h := range []string{`C:\Users\Bob`, "C:/Users/Bob", `\\srv\Bob`} {
		if !caseInsensitiveHome(h) {
			t.Errorf("caseInsensitiveHome(%q) = false", h)
		}
	}
	if got, want := caseInsensitiveHome("/home/bob"), runtime.GOOS == "windows"; got != want {
		t.Errorf("caseInsensitiveHome(/home/bob) = %v on %s", got, runtime.GOOS)
	}
	if runtime.GOOS != "windows" && caseInsensitiveHome("c") {
		t.Error(`caseInsensitiveHome("c") = true`)
	}
}

func TestUserNamesKeepsTwoLetterNamesAndDropsDots(t *testing.T) {
	if names := userNames(`C:\Users\ab`); !slices.Contains(names, "ab") {
		t.Errorf("two-letter home name missing: %q", names)
	}
	if names := userNames(`C:\Users\.`); slices.Contains(names, ".") {
		t.Errorf("dot kept: %q", names)
	}
	if names := userNames(`C:\Users\z`); slices.Contains(names, "z") {
		t.Errorf("one-letter name kept: %q", names)
	}
}

// cutRunes keeps the longest rune-aligned prefix, not just any prefix.
func TestCutRunesKeepsTheLongestPrefix(t *testing.T) {
	if got := cutRunes("abcdef", 6); got != "abcdef" {
		t.Fatalf("exact length cut: %q", got)
	}
	if got := cutRunes("aé€x", 4); got != "aé [cut]" {
		t.Fatalf("got %q", got)
	}
	if got := cutRunes("abcdef", 3); got != "abc [cut]" {
		t.Fatalf("got %q", got)
	}
}

func TestListNamesEdges(t *testing.T) {
	empty := t.TempDir()
	if got := listNames(empty); got != "(none)" {
		t.Errorf("empty dir: %q", got)
	}
	if got := listNames(filepath.Join(empty, "missing")); got != "(none)" {
		t.Errorf("missing dir: %q", got)
	}
	full := t.TempDir()
	for i := 0; i < reportListMax; i++ {
		if err := os.WriteFile(filepath.Join(full, fmt.Sprintf("t%03d.css", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := listNames(full); strings.HasSuffix(got, "...") || strings.Count(got, ", ")+1 != reportListMax {
		t.Errorf("exactly %d entries must be listed without an ellipsis", reportListMax)
	}
	if err := os.WriteFile(filepath.Join(full, "zzz.css"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := listNames(full); !strings.HasSuffix(got, ", ...") || strings.Contains(got, "zzz.css") {
		t.Errorf("%d entries must end with an ellipsis", reportListMax+1)
	}
}

// Collect uses the Home it is given, not the account's own home folder.
func TestCollectRedactsTheGivenHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Profiles", "zedhome")
	cfg := filepath.Join(home, "cfg", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	report := Collect(context.Background(), Source{Version: "t", ConfigPath: cfg, Home: home, PortOwnedByUs: true})
	want := "Config file: ~" + string(filepath.Separator) + filepath.Join("cfg", "config.json")
	if !strings.Contains(report, want) {
		t.Fatalf("report lacks %q", want)
	}
	if strings.Contains(strings.ToLower(report), "zedhome") {
		t.Fatal("home folder name leaked")
	}
}
