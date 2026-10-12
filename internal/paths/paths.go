package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var vdfPath = regexp.MustCompile(`"path"\s+"([^"]+)"`)
var plainDrive = regexp.MustCompile(`^[A-Za-z]:$`)

// IsQuaverDir reports whether dir looks like a Quaver install folder.
func IsQuaverDir(dir string) bool {
	if IsUNC(dir) {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "Quaver.exe"))
	return err == nil
}

// DefaultSteamRoots lists the usual Steam install folders for this OS.
func DefaultSteamRoots() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			`C:\Program Files (x86)\Steam`,
			`C:\Program Files\Steam`,
		}
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []string{
			filepath.Join(home, ".steam", "steam"),
			filepath.Join(home, ".local", "share", "Steam"),
			filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		}
	}
}

// libraries returns the Steam root plus every library listed in libraryfolders.vdf.
func libraries(steamRoot string) []string {
	out := []string{steamRoot}
	data, err := os.ReadFile(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return out
	}
	for _, m := range vdfPath.FindAllStringSubmatch(string(data), -1) {
		if p := strings.ReplaceAll(m[1], `\\`, `\`); !IsUNC(p) {
			out = append(out, p)
		}
	}
	return out
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
		if IsUNC(override) {
			return "", fmt.Errorf("%q is a network path; use a folder on a local drive", override)
		}
		if IsQuaverDir(override) {
			return override, nil
		}
		return "", fmt.Errorf("%q is not a Quaver folder (Quaver.exe not found)", override)
	}
	var tried []string
	for _, root := range steamRoots {
		if IsUNC(root) {
			continue
		}
		for _, lib := range libraries(root) {
			dir := filepath.Join(lib, "steamapps", "common", "Quaver")
			tried = append(tried, dir)
			if IsQuaverDir(dir) {
				return dir, nil
			}
		}
	}
	return "", fmt.Errorf("quaver folder not found (tried: %s); pass it with --quaver-dir", strings.Join(tried, ", "))
}
