package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func writeVDF(t *testing.T, root string, libs ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("\"libraryfolders\"\n{\n")
	for i, l := range libs {
		fmt.Fprintf(&b, "\t\"%d\"\n\t{\n\t\t\"path\"\t\t\"%s\"\n\t}\n", i, strings.ReplaceAll(l, bs, bs+bs))
	}
	b.WriteString("}\n")
	p := filepath.Join(root, "steamapps", "libraryfolders.vdf")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSteamRootsForLinux(t *testing.T) {
	got := SteamRootsFor("linux", "/home/u")
	want := []string{
		filepath.Join("/home/u", ".steam", "steam"),
		filepath.Join("/home/u", ".local", "share", "Steam"),
		filepath.Join("/home/u", ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join("/home/u", "snap", "steam", "common", ".local", "share", "Steam"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("roots = %v, want %v", got, want)
	}
	if len(SteamRootsFor("linux", "")) != 0 {
		t.Fatal("an empty home must give no roots")
	}
	if got := SteamRootsFor("windows", "windows-home"); len(got) != 2 {
		t.Fatalf("windows roots = %v", got)
	}
}

func TestFindEachLinuxRootKind(t *testing.T) {
	home := t.TempDir()
	for _, root := range SteamRootsFor("linux", home) {
		want := filepath.Join(root, "steamapps", "common", "Quaver")
		touch(t, filepath.Join(want, "Quaver.exe"))
		got, err := Find("", []string{root})
		if err != nil || got != want {
			t.Fatalf("root %s: Find = %q, %v", root, got, err)
		}
		if err := os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFindSecondLibraryAndMissingEntries(t *testing.T) {
	root, lib := t.TempDir(), t.TempDir()
	want := filepath.Join(lib, "steamapps", "common", "Quaver")
	touch(t, filepath.Join(want, "Quaver.exe"))
	writeVDF(t, root, root, filepath.Join(t.TempDir(), "gone"), lib)
	got, err := Find("", []string{root})
	if err != nil || got != want {
		t.Fatalf("Find = %q, %v", got, err)
	}
}

func TestLibrariesIgnoreHostileEntries(t *testing.T) {
	root := t.TempDir()
	abs := t.TempDir()
	writeVDF(t, root,
		"steamapps/rel", "../up", "./here", winSteam, ".."+bs+"x",
		"//server/share", uncPath("server", "share"), abs+string(filepath.Separator)+".."+string(filepath.Separator)+"escape", abs)
	got := libraries(root)
	want := []string{root, abs}
	if runtime.GOOS == "windows" {
		want = []string{root, winSteam, abs}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("libraries = %v, want %v", got, want)
	}
}

func TestFindSymlinkedRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	real := filepath.Join(base, "real")
	want := filepath.Join(real, "steamapps", "common", "Quaver")
	touch(t, filepath.Join(want, "Quaver.exe"))
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	got, err := Find("", []string{link, real})
	if err != nil || got != filepath.Join(link, "steamapps", "common", "Quaver") {
		t.Fatalf("Find = %q, %v", got, err)
	}
	_, err = Find("", []string{link, real, link})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUniqueRootsDedupesSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if got := uniqueDirs([]string{link, real, link}); !reflect.DeepEqual(got, []string{link}) {
		t.Fatalf("uniqueDirs = %v", got)
	}
}

func TestSymlinkLoopDoesNotHang(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	if err := os.Symlink(b, a); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, b); err != nil {
		t.Fatal(err)
	}
	if _, err := Find("", []string{a, b}); err == nil || !strings.Contains(err.Error(), "--quaver-dir") {
		t.Fatalf("err = %v", err)
	}
}

func TestLogAndDBPathCaseVariants(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("case variants need a case-sensitive file system")
	}
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "logs", "Runtime.LOG"))
	touch(t, filepath.Join(dir, "Quaver.DB"))
	if got, want := LogPath(dir), filepath.Join(dir, "logs", "Runtime.LOG"); got != want {
		t.Fatalf("LogPath = %q, want %q", got, want)
	}
	if got, want := DBPath(dir), filepath.Join(dir, "Quaver.DB"); got != want {
		t.Fatalf("DBPath = %q, want %q", got, want)
	}
}

func TestLogAndDBPathPreferExactAndDefault(t *testing.T) {
	dir := t.TempDir()
	if got, want := LogPath(dir), filepath.Join(dir, "Logs", "runtime.log"); got != want {
		t.Fatalf("missing LogPath = %q", got)
	}
	if got, want := DBPath(dir), filepath.Join(dir, "quaver.db"); got != want {
		t.Fatalf("missing DBPath = %q", got)
	}
	touch(t, filepath.Join(dir, "Logs", "runtime.log"))
	touch(t, filepath.Join(dir, "quaver.db"))
	if got, want := LogPath(dir), filepath.Join(dir, "Logs", "runtime.log"); got != want {
		t.Fatalf("exact LogPath = %q", got)
	}
	if got, want := DBPath(dir), filepath.Join(dir, "quaver.db"); got != want {
		t.Fatalf("exact DBPath = %q", got)
	}
}

func TestEqualASCIIFoldRejectsLookalikes(t *testing.T) {
	long := string([]rune{0x17F}) // long s, folds to "s" under Unicode rules
	if equalASCIIFold("Logs", "logs") != true || equalASCIIFold("Lo"+long, "Los") || equalASCIIFold("K", string([]rune{0x212A})) {
		t.Fatal("ASCII fold must match ASCII case only")
	}
}

func TestResolveNameIgnoresUnicodeLookalike(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a case-sensitive file system")
	}
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "Lo"+string([]rune{0x17F})+"s", "x"))
	if got, want := resolveName(dir, "Logs"), filepath.Join(dir, "Logs"); got != want {
		t.Fatalf("resolveName = %q", got)
	}
}

func TestDirNamesHonoursCap(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 10; i++ {
		touch(t, filepath.Join(dir, fmt.Sprintf("f%02d", i)))
	}
	if got, err := dirNames(dir, 3); err != nil || len(got) != 3 {
		t.Fatalf("dirNames cap 3 = %v, %v", got, err)
	}
	if got, err := dirNames(dir, 100); err != nil || len(got) != 10 {
		t.Fatalf("dirNames cap 100 = %v, %v", got, err)
	}
}

func TestLibrariesCapsVDFSize(t *testing.T) {
	root, lib := t.TempDir(), t.TempDir()
	p := filepath.Join(root, "steamapps", "libraryfolders.vdf")
	touch(t, p)
	pad := strings.Repeat(" ", maxVDFSize)
	body := pad + fmt.Sprintf("\"path\"\t\"%s\"\n", strings.ReplaceAll(lib, bs, bs+bs))
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := libraries(root); !reflect.DeepEqual(got, []string{root}) {
		t.Fatalf("libraries = %v", got)
	}
}

func TestWriteVDFDoublesBackslashes(t *testing.T) {
	root := t.TempDir()
	writeVDF(t, root, winSteam)
	data, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if err != nil || !strings.Contains(string(data), "C:"+bs+bs+"Games"+bs+bs+"Steam") {
		t.Fatalf("vdf = %q, %v", data, err)
	}
}

var winSteam = "C:" + bs + "Games" + bs + "Steam"
