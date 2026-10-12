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

// IsQuaverDir reports whether dir looks like a Quaver install folder.
func IsQuaverDir(dir string) bool {
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
		out = append(out, strings.ReplaceAll(m[1], `\\`, `\`))
	}
	return out
}

// IsUNC reports whether p is a network path, which a stat would try to reach.
func IsUNC(p string) bool {
	return strings.HasPrefix(p, "\\\\") || strings.HasPrefix(p, "//")
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
