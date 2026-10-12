// Package overlays finds and reads the custom overlays kept next to the config file.
package overlays

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// MaxFile is the largest file of a custom overlay that is served.
	MaxFile = 2 << 20
	// MaxFiles is the number of files an overlay may hold before it is refused.
	MaxFiles = 200
	// MaxDepth is the number of path segments below the overlay folder.
	MaxDepth = 3
	// MaxListed is the number of custom overlays kept in one listing.
	MaxListed = 100

	maxScanEntries = 4096
	indexFile      = "index.html"
)

// ID matches the folder name of a custom overlay as requested on /overlays/.
var ID = regexp.MustCompile(`^[a-z0-9-]+$`)

var segment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

var contentTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".htm":   "text/html; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".json":  "application/json",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".ico":   "image/x-icon",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".txt":   "text/plain; charset=utf-8",
}

// File is one served file.
type File struct {
	Data    []byte
	ModTime time.Time
	Type    string
}

// Dir returns the custom overlays folder next to the config file, or "" when there is no config path
// or the config file is on a network share (UNC path).
func Dir(configPath string) string {
	if configPath == "" || strings.HasPrefix(configPath, `\\`) || strings.HasPrefix(configPath, "//") {
		return ""
	}
	return filepath.Join(filepath.Dir(configPath), "overlays")
}

// IsBuiltin tells whether assets embeds a built-in overlay with this id; those always win over custom ones.
func IsBuiltin(assets fs.FS, id string) bool {
	fi, err := fs.Stat(assets, "overlays/"+id)
	return err == nil && fi.IsDir()
}

// Read returns the file rel ("" means index.html, otherwise slash separated segments) of the overlay id in dir.
// Only a regular file (no symlink) with a whitelisted extension and an exact on-disk name is returned,
// and only from an overlay of at most MaxFiles files.
func Read(dir, id, rel string) (File, error) {
	if rel == "" {
		rel = indexFile
	}
	root, err := overlayRoot(dir, id)
	if err != nil {
		return File{}, err
	}
	return readIn(root, rel, MaxFile)
}

func overlayRoot(dir, id string) (string, error) {
	if dir == "" || !ID.MatchString(id) {
		return "", fs.ErrNotExist
	}
	if ok, err := hasEntry(dir, id, fs.ModeDir); err != nil || !ok {
		return "", fs.ErrNotExist
	}
	root := filepath.Join(dir, id)
	if tooMany(root) {
		return "", fs.ErrNotExist
	}
	return root, nil
}

// hasEntry reports whether dir holds an entry with exactly this name and file type (0 for a regular file),
// so a case-insensitive file system cannot alias another name.
func hasEntry(dir, name string, want fs.FileMode) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()
	entries, err := f.ReadDir(maxScanEntries)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	for _, e := range entries {
		if e.Name() == name {
			return e.Type() == want, nil
		}
	}
	return false, nil
}

func tooMany(root string) bool {
	count := 0
	var walk func(dir string, depth int) bool
	walk = func(dir string, depth int) bool {
		f, err := os.Open(dir)
		if err != nil {
			return false
		}
		entries, _ := f.ReadDir(maxScanEntries)
		f.Close()
		for _, e := range entries {
			if count++; count > MaxFiles {
				return true
			}
			if e.Type() == fs.ModeDir && depth < MaxDepth && walk(filepath.Join(dir, e.Name()), depth+1) {
				return true
			}
		}
		return false
	}
	return walk(root, 1)
}

func readIn(root, rel string, limit int64) (File, error) {
	parts := strings.Split(rel, "/")
	if len(parts) > MaxDepth {
		return File{}, fs.ErrNotExist
	}
	for _, p := range parts {
		if !segment.MatchString(p) || strings.HasPrefix(p, ".") {
			return File{}, fs.ErrNotExist
		}
	}
	ctype, ok := contentTypes[strings.ToLower(filepath.Ext(parts[len(parts)-1]))]
	if !ok {
		return File{}, fs.ErrNotExist
	}
	cur := root
	for i, p := range parts {
		want := fs.ModeDir
		if i == len(parts)-1 {
			want = 0
		}
		if ok, err := hasEntry(cur, p, want); err != nil || !ok {
			return File{}, fs.ErrNotExist
		}
		cur = filepath.Join(cur, p)
	}
	return readRegular(root, rel, ctype, limit)
}

// readRegular opens rel through a handle on root, so a folder swapped for a link after the checks cannot lead outside it.
func readRegular(root, rel, ctype string, limit int64) (File, error) {
	rt, err := os.OpenRoot(root)
	if err != nil {
		return File{}, err
	}
	defer rt.Close()
	p := filepath.FromSlash(rel)
	li, err := rt.Lstat(p)
	if err != nil {
		return File{}, err
	}
	if !li.Mode().IsRegular() || li.Size() > limit {
		return File{}, fs.ErrNotExist
	}
	f, err := rt.Open(p)
	if err != nil {
		return File{}, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !os.SameFile(li, fi) {
		return File{}, fs.ErrNotExist
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return File{}, err
	}
	if int64(len(data)) > limit {
		return File{}, errors.New("overlay file too large")
	}
	return File{Data: data, ModTime: li.ModTime(), Type: ctype}, nil
}
