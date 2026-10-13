package paths

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var vdfPath = regexp.MustCompile(`"path"\s+"([^"]+)"`)
var plainDrive = regexp.MustCompile(`^[A-Za-z]:$`)

var evalSymlinks = filepath.EvalSymlinks

// resolved follows links and junctions in dir; a folder that cannot be resolved is returned as is, so later stats report it.
func resolved(dir string) string {
	if r, err := evalSymlinks(dir); err == nil {
		return r
	}
	return dir
}

// ResolvesToUNC reports whether dir, or the folder a link inside it leads to, is a network path.
func ResolvesToUNC(dir string) bool {
	return IsUNC(dir) || IsUNC(resolved(dir))
}

// IsQuaverDir reports whether dir looks like a Quaver install folder.
// Links are followed and the target is checked for network paths too.
func IsQuaverDir(dir string) bool {
	if ResolvesToUNC(dir) {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "Quaver.exe"))
	return err == nil
}

// DefaultSteamRoots lists the usual Steam install folders for this OS.
func DefaultSteamRoots() []string {
	home, _ := os.UserHomeDir()
	return SteamRootsFor(runtime.GOOS, home)
}

// SteamRootsFor lists the usual Steam install folders for goos and the given home folder.
func SteamRootsFor(goos, home string) []string {
	if goos == "windows" {
		return []string{
			`C:\Program Files (x86)\Steam`,
			`C:\Program Files\Steam`,
		}
	}
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam"),
	}
}

// libraries returns the Steam root plus every absolute local library listed in libraryfolders.vdf.
func libraries(steamRoot string) []string {
	out := []string{steamRoot}
	data, err := readCapped(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"), maxVDFSize)
	if err != nil {
		return out
	}
	for _, m := range vdfPath.FindAllStringSubmatch(string(data), -1) {
		if p := strings.ReplaceAll(m[1], `\\`, `\`); usableLibrary(p) {
			out = append(out, p)
		}
	}
	return out
}

// usableLibrary accepts only absolute local paths without parent references.
func usableLibrary(p string) bool {
	if IsUNC(p) || !filepath.IsAbs(p) || strings.ContainsRune(p, 0) {
		return false
	}
	for _, part := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return false
		}
	}
	return true
}

// uniqueDirs drops folders that resolve to one already listed (symlinked Steam roots).
// A folder that cannot be resolved is kept, so later stats report it.
func uniqueDirs(dirs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range dirs {
		key := d
		if r, err := filepath.EvalSymlinks(d); err == nil {
			key = r
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, d)
		}
	}
	return out
}

// LogPath returns Quaver's runtime.log, trying the usual spelling first and then a
// case-insensitive match for case-sensitive file systems.
func LogPath(dir string) string {
	return resolveName(resolveName(dir, "Logs"), "runtime.log")
}

// DBPath returns quaver.db, with the same case fallback as LogPath.
func DBPath(dir string) string { return resolveName(dir, "quaver.db") }

func resolveName(dir, name string) string {
	exact := filepath.Join(dir, name)
	if _, err := os.Lstat(exact); err == nil {
		return exact
	}
	names, err := dirNames(dir, scanCap)
	if err != nil {
		return exact
	}
	for _, n := range names {
		if equalASCIIFold(n, name) {
			return filepath.Join(dir, n)
		}
	}
	return exact
}

// equalASCIIFold ignores case for ASCII letters only, so Unicode lookalikes never match.
func equalASCIIFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// IsUNC reports whether p is a network path (or a drive-less rooted path on
// Windows), which a stat could send to an SMB host. Both separators count, mixed.
func IsUNC(p string) bool {
	isSep := func(c byte) bool { return c == '/' || c == '\\' }
	if len(p) >= 2 && isSep(p[0]) && isSep(p[1]) {
		return true
	}
	if runtime.GOOS != "windows" {
		return false
	}
	if p != "" && isSep(p[0]) {
		return true
	}
	v := filepath.VolumeName(p)
	return v != "" && !plainDrive.MatchString(v)
}

// Find returns the Quaver install folder. A non-empty override is validated
// and returned as is; otherwise the Steam libraries are searched.
func Find(override string, steamRoots []string) (string, error) {
	if override != "" {
		if ResolvesToUNC(override) {
			return "", fmt.Errorf("%q is a network path; use a folder on a local drive", override)
		}
		if IsQuaverDir(override) {
			return override, nil
		}
		return "", fmt.Errorf("%q is not a Quaver folder (Quaver.exe not found)", override)
	}
	var tried []string
	var roots []string
	for _, root := range steamRoots {
		if !IsUNC(root) {
			roots = append(roots, root)
		}
	}
	for _, root := range uniqueDirs(roots) {
		for _, lib := range uniqueDirs(libraries(root)) {
			dir := filepath.Join(lib, "steamapps", "common", "Quaver")
			tried = append(tried, dir)
			if IsQuaverDir(dir) {
				return dir, nil
			}
		}
	}
	return "", fmt.Errorf("quaver folder not found (tried: %s); pass it with --quaver-dir", strings.Join(tried, ", "))
}

// scanCap bounds how many directory entries a case-insensitive lookup reads.
var scanCap = 4096

const maxVDFSize = 1 << 20

func readCapped(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

func dirNames(dir string, limit int) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.Readdirnames(limit)
	if err == io.EOF {
		err = nil
	}
	return names, err
}
