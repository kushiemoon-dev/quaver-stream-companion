package textout

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/bus"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/scores"
)

func theMap() *scores.Map {
	return &scores.Map{Artist: "Artist", Title: "Title", DifficultyName: "Insane", Creator: "Mapper"}
}

func theScore() *scores.Result {
	return &scores.Result{Score: scores.Score{Grade: "S", Accuracy: 97.4249}}
}

func TestTextsFormat(t *testing.T) {
	got := Texts(theMap(), theScore())
	want := map[string]string{
		"title.txt": "Title", "artist.txt": "Artist", "difficulty.txt": "Insane", "mapper.txt": "Mapper",
		"nowplaying.txt": "Artist - Title [Insane]", "last_grade.txt": "S", "last_accuracy.txt": "97.42%",
	}
	if len(got) != len(Names) {
		t.Fatalf("got %d texts, want %d", len(got), len(Names))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestTextsEmptyWithoutMap(t *testing.T) {
	for name, v := range Texts(nil, theScore()) {
		if v != "" {
			t.Errorf("%s = %q, want empty", name, v)
		}
	}
}

func TestTextsWithoutScoreOrParts(t *testing.T) {
	got := Texts(&scores.Map{Title: "Only"}, nil)
	if got["nowplaying.txt"] != "Only" || got["last_grade.txt"] != "" || got["last_accuracy.txt"] != "" {
		t.Fatalf("texts = %v", got)
	}
}

func TestTextsStripControlsAndCap(t *testing.T) {
	m := &scores.Map{Title: "A\r\nB\tC\x00" + string([]rune{0x2028}) + "D" + string([]rune{0xFEFF}) + "E", Artist: strings.Repeat(string([]rune{0xE9}), 300)}
	got := Texts(m, nil)
	if got["title.txt"] != "ABCDE" {
		t.Errorf("title = %q", got["title.txt"])
	}
	for name, v := range got {
		if len(v) > maxBytes {
			t.Errorf("%s has %d bytes", name, len(v))
		}
	}
	if a := got["artist.txt"]; len(a) != 256 || strings.ContainsRune(a, 0xFFFD) {
		t.Errorf("artist cut at %d bytes, want 256 whole runes", len(a))
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func newWriter(t *testing.T) (*Writer, string) {
	dir := filepath.Join(t.TempDir(), "text")
	return NewWriter(dir, func() bool { return true }), dir
}

func TestWritesFilesWithoutBOMOrNewline(t *testing.T) {
	w, dir := newWriter(t)
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: *theMap()})
	w.Handle(bus.Event{Type: bus.TypeScoreSaved, Data: *theScore()})
	want := map[string]string{"title.txt": "Title", "nowplaying.txt": "Artist - Title [Insane]", "last_grade.txt": "S", "last_accuracy.txt": "97.42%"}
	for name, v := range want {
		if got := read(t, dir, name); got != v {
			t.Errorf("%s = %q, want %q", name, got, v)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != len(Names) {
		t.Errorf("%d files in the folder, want %d (no temp files left)", len(entries), len(Names))
	}
}

func TestEmptyFilesWhenNoMapIsKnown(t *testing.T) {
	w, dir := newWriter(t)
	w.Refresh()
	for _, name := range Names {
		if got := read(t, dir, name); got != "" {
			t.Errorf("%s = %q, want empty", name, got)
		}
	}
}

func TestUnchangedContentIsNotRewritten(t *testing.T) {
	w, dir := newWriter(t)
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: *theMap()})
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	p := filepath.Join(dir, "title.txt")
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: *theMap()})
	if fi, _ := os.Stat(p); !fi.ModTime().Equal(old) {
		t.Fatal("an unchanged file was written again")
	}
	m := *theMap()
	m.Title = "Other"
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: m})
	if got := read(t, dir, "title.txt"); got != "Other" {
		t.Fatalf("title = %q after the map changed", got)
	}
}

func TestDisabledWritesNothingAndCatchesUpWhenEnabled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "text")
	var on atomic.Bool
	w := NewWriter(dir, on.Load)
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: *theMap()})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the folder exists while the setting is off: %v", err)
	}
	on.Store(true)
	w.Handle(bus.Event{Type: bus.TypeScoreSaved, Data: *theScore()})
	if read(t, dir, "title.txt") != "Title" || read(t, dir, "last_grade.txt") != "S" {
		t.Fatal("files must show the whole state once enabled")
	}
	on.Store(false)
	m := *theMap()
	m.Title = "Later"
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: m})
	if read(t, dir, "title.txt") != "Title" {
		t.Fatal("a disabled writer touched a file")
	}
	on.Store(true)
	w.Handle(bus.Event{Type: bus.TypeScoreSaved, Data: *theScore()})
	if got := read(t, dir, "title.txt"); got != "Later" {
		t.Fatalf("title = %q, want Later after re-enabling", got)
	}
}

func TestSeedKeepsKnownValues(t *testing.T) {
	w, dir := newWriter(t)
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: *theMap()})
	w.Seed(&scores.Map{Title: "Seeded"}, theScore())
	if read(t, dir, "title.txt") != "Title" || read(t, dir, "last_grade.txt") != "S" {
		t.Fatal("seed must only fill what is unknown")
	}
}

func TestOtherEventsIgnored(t *testing.T) {
	w, dir := newWriter(t)
	w.Handle(bus.Event{Type: bus.TypeGameState, Data: bus.GameState{State: "play"}})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("an unrelated event wrote files")
	}
}

func TestWriteFailureIsRetriedOnTheNextEvent(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "text")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewWriter(blocker, func() bool { return true })
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: *theMap()})
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: *theMap()})
	if got := read(t, blocker, "title.txt"); got != "Title" {
		t.Fatalf("title = %q after the folder became usable", got)
	}
}

func TestConcurrentEvents(t *testing.T) {
	w, dir := newWriter(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 20 {
				m := *theMap()
				m.Title = "T" + string(rune('A'+i))
				w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: m})
				w.Handle(bus.Event{Type: bus.TypeScoreSaved, Data: scores.Result{Score: scores.Score{Grade: "A", Accuracy: float64(j)}}})
			}
		}()
	}
	wg.Wait()
	if got := read(t, dir, "title.txt"); len(got) != 2 || got[0] != 'T' {
		t.Fatalf("title = %q", got)
	}
	if read(t, dir, "nowplaying.txt") != "Artist - "+read(t, dir, "title.txt")+" [Insane]" {
		t.Fatal("files do not describe one map")
	}
}

func TestDir(t *testing.T) {
	if Dir("") != "" {
		t.Error("no config path must give no folder")
	}
	if got, want := Dir(filepath.Join("a", "config.json")), filepath.Join("a", "text"); got != want {
		t.Errorf("Dir = %q, want %q", got, want)
	}
}

func TestNoFolderMeansNoFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	w := NewWriter("", func() bool { return true })
	w.Handle(bus.Event{Type: bus.TypeMapStarted, Data: *theMap()})
	if entries, _ := os.ReadDir("."); len(entries) != 0 {
		t.Fatalf("%d files written to the working folder", len(entries))
	}
}
