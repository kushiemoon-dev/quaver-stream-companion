package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func isSepByte(c byte) bool { return c == '/' || c == '\\' }

// FuzzIsUNC: two leading separators are always a network path; on Windows a rooted path without a
// drive and a non-drive volume are too.
func FuzzIsUNC(f *testing.F) {
	for _, s := range []string{`//`, `\\`, `\/`, `/`, `\\srv\share`, `//srv/share`, `/\srv`, `C:\x`, `C:`, `\x`, `\\?\C:\x`, `\\.\pipe\x`, `\??\UNC\srv\s`, ``, `c:x`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		got := IsUNC(p)
		if len(p) >= 2 && isSepByte(p[0]) && isSepByte(p[1]) && !got {
			t.Fatalf("IsUNC(%q) = false for a double-separator path", p)
		}
		if runtime.GOOS == "windows" {
			if p != "" && isSepByte(p[0]) && !got {
				t.Fatalf("IsUNC(%q) = false for a rooted drive-less path", p)
			}
			if v := filepath.VolumeName(p); v != "" && !plainDrive.MatchString(v) && !got {
				t.Fatalf("IsUNC(%q) = false with volume %q", p, v)
			}
		}
	})
}

// FuzzUsableLibrary: an accepted library is absolute, local, NUL-free, has no ".." segment, and stays
// local once the Quaver folder is joined to it.
func FuzzUsableLibrary(f *testing.F) {
	for _, s := range []string{`D:\SteamLibrary`, `/mnt/games/SteamLibrary`, `D:\a\..\b`, `\\srv\lib`, `D:\x` + "\x00", `D:/a/./b`, `C:\..`, `relative\lib`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		if !usableLibrary(p) {
			return
		}
		if IsUNC(p) || !filepath.IsAbs(p) || strings.ContainsRune(p, 0) {
			t.Fatalf("usableLibrary(%q) accepted a bad path", p)
		}
		for _, part := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
			if part == ".." {
				t.Fatalf("usableLibrary(%q) accepted a parent segment", p)
			}
		}
		dir := filepath.Join(p, "steamapps", "common", "Quaver")
		if IsUNC(dir) || !filepath.IsAbs(dir) {
			t.Fatalf("joined Quaver dir %q of %q is not a local absolute path", dir, p)
		}
	})
}

// FuzzLibrariesVDF: every library read from an arbitrary libraryfolders.vdf passes usableLibrary.
func FuzzLibrariesVDF(f *testing.F) {
	f.Add([]byte(`"libraryfolders" { "0" { "path" "C:\\Program Files (x86)\\Steam" } "1" { "path" "D:\\SteamLibrary" } }`))
	f.Add([]byte(`"path" "\\\\srv\\share" "path"	"/home/u/lib" "path" "D:\\a\\..\\b"`))
	f.Fuzz(func(t *testing.T, vdf []byte) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "steamapps"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"), vdf, 0o644); err != nil {
			t.Fatal(err)
		}
		libs := libraries(root)
		if len(libs) == 0 || libs[0] != root {
			t.Fatalf("root missing: %q", libs)
		}
		for _, l := range libs[1:] {
			if !usableLibrary(l) {
				t.Fatalf("unusable library %q returned", l)
			}
		}
	})
}

// FuzzEqualASCIIFold matches a plain ASCII-only case fold.
func FuzzEqualASCIIFold(f *testing.F) {
	f.Add("Logs", "logs")
	f.Add("runtime.log", "RUNTIME.LOG")
	f.Add("K", "\u212a")
	f.Add("@[`{", "@[`{")
	f.Add("@", "`")
	f.Add("[", "{")
	f.Add("AZ", "az")
	f.Fuzz(func(t *testing.T, a, b string) {
		lower := func(s string) string {
			bs := []byte(s)
			for i, c := range bs {
				if 'A' <= c && c <= 'Z' {
					bs[i] = c + 'a' - 'A'
				}
			}
			return string(bs)
		}
		if got, want := equalASCIIFold(a, b), lower(a) == lower(b); got != want {
			t.Fatalf("equalASCIIFold(%q, %q) = %v, want %v", a, b, got, want)
		}
	})
}

func TestUniqueDirsDropsRepeats(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	missing := filepath.Join(a, "missing")
	got := uniqueDirs([]string{a, b, a, missing, missing})
	if len(got) != 3 || got[0] != a || got[1] != b || got[2] != missing {
		t.Fatalf("uniqueDirs = %q", got)
	}
}
