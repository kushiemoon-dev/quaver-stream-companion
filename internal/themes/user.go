package themes

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// MaxUserFile is the largest custom theme file that is served or listed.
const MaxUserFile = 64 << 10

// MaxUserThemes is the number of custom themes kept in one listing.
const MaxUserThemes = 100

// maxScanEntries bounds the folder entries read to check a requested name.
const maxScanEntries = 4096

// UserID matches the path of a custom theme as requested on /themes/.
var UserID = regexp.MustCompile(`^[a-z0-9-]+$`)

var userFile = regexp.MustCompile(`^[a-z0-9-]+\.css$`)

// UserDir returns the custom themes folder next to the config file, or "" when there is no config path.
func UserDir(configPath string) string {
	if configPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(configPath), "themes")
}

// ReadUser reads <dir>/<id>.css. Only a regular file (no symlink) of at most MaxUserFile bytes whose
// name on disk is exactly <id>.css is returned, so a case-insensitive file system cannot alias a listed id.
func ReadUser(dir, id string) (data []byte, modTime time.Time, err error) {
	if dir == "" || !UserID.MatchString(id) || !hasExactName(dir, id+".css") {
		return nil, time.Time{}, os.ErrNotExist
	}
	return readUserFile(dir, id)
}

func hasExactName(dir, name string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	names, _ := f.Readdirnames(maxScanEntries)
	return slices.Contains(names, name)
}

func readUserFile(dir, id string) (data []byte, modTime time.Time, err error) {
	p := filepath.Join(dir, id+".css")
	li, err := os.Lstat(p)
	if err != nil {
		return nil, time.Time{}, err
	}
	if !li.Mode().IsRegular() || li.Size() > MaxUserFile {
		return nil, time.Time{}, os.ErrNotExist
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !os.SameFile(li, fi) {
		return nil, time.Time{}, os.ErrNotExist
	}
	data, err = io.ReadAll(io.LimitReader(f, MaxUserFile+1))
	if err != nil {
		return nil, time.Time{}, err
	}
	if len(data) > MaxUserFile {
		return nil, time.Time{}, errors.New("theme file too large")
	}
	return data, li.ModTime(), nil
}

// Merge adds the custom themes of dir to builtin: a custom file with a built-in id replaces it in place,
// the others follow alphabetically. Unreadable, oversized or irregular files are skipped and
// only the first MaxUserThemes valid files (by name) are kept.
func Merge(builtin []Theme, dir string) []Theme {
	out := slices.Clone(builtin)
	if dir == "" {
		return out
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	var extra []Theme
	valid := 0
	for _, e := range entries {
		if !userFile.MatchString(e.Name()) {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".css")
		data, _, err := readUserFile(dir, id)
		if err != nil {
			continue
		}
		if valid == MaxUserThemes {
			break
		}
		valid++
		th := Theme{ID: id, Source: SourceCustom, Colors: ParseColors(string(data))}
		if i := slices.IndexFunc(out, func(b Theme) bool { return b.ID == id }); i >= 0 {
			out[i] = th
		} else {
			extra = append(extra, th)
		}
	}
	slices.SortFunc(extra, func(a, b Theme) int { return strings.Compare(a.ID, b.ID) })
	return append(out, extra...)
}
