// Package config reads, writes and validates the user settings file.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/paths"
)

// DefaultPort is the port used when none is configured.
const DefaultPort = 7878

const defaultTheme = "neutral-light"

var accentRe = regexp.MustCompile(`^([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// Overlay holds the overlay appearance settings.
type Overlay struct {
	Theme  string `json:"theme"`
	Accent string `json:"accent"`
}

// Config is the content of the settings file.
type Config struct {
	QuaverDir string      `json:"quaver_dir"`
	Port      int         `json:"port"`
	Language  string      `json:"language"`
	Overlay   Overlay     `json:"overlay"`
	Keys      keys.Config `json:"keys"`
}

// FieldError describes an invalid setting.
type FieldError struct {
	Field, Msg string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Msg }

// Defaults returns the settings used when no file exists.
func Defaults() Config {
	return Config{Port: DefaultPort, Overlay: Overlay{Theme: defaultTheme}, Keys: keys.Defaults()}
}

// DefaultPath returns the per-user location of the settings file.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "quaver-stream-companion", "config.json"), nil
}

// Load reads the settings file. A missing or empty file gives the defaults,
// and absent or zero fields take their default value.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Defaults(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	c := Defaults()
	if len(bytes.TrimSpace(data)) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return Normalize(c), nil
}

// Normalize gives every zero port, theme or key setting its default value.
func Normalize(c Config) Config {
	d := Defaults()
	if c.Port == 0 {
		c.Port = d.Port
	}
	if c.Overlay.Theme == "" {
		c.Overlay.Theme = d.Overlay.Theme
	}
	c.Keys = c.Keys.Normalized()
	return c
}

// Save writes the settings atomically, creating the folder if needed.
func Save(path string, c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(append(data, '\n'))
	cerr := tmp.Close()
	if err := firstErr(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := renameWithRetry(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// Validate checks every field and returns a *FieldError for the first invalid one.
func (c Config) Validate(themes []string) error {
	if c.Port < 1 || c.Port > 65535 {
		return &FieldError{"port", "must be between 1 and 65535"}
	}
	if paths.IsUNC(c.QuaverDir) {
		return &FieldError{"quaver_dir", "must be a folder on a local drive, not a network path"}
	}
	if c.QuaverDir != "" && !paths.IsQuaverDir(c.QuaverDir) {
		return &FieldError{"quaver_dir", "must be a folder containing Quaver.exe"}
	}
	switch c.Language {
	case "", "en", "fr", "de":
	default:
		return &FieldError{"language", "must be one of en, fr, de"}
	}
	if c.Overlay.Theme != "" && !slices.Contains(themes, c.Overlay.Theme) {
		return &FieldError{"overlay.theme", "unknown theme"}
	}
	if c.Overlay.Accent != "" && !accentRe.MatchString(c.Overlay.Accent) {
		return &FieldError{"overlay.accent", "must be 3, 4, 6 or 8 hexadecimal digits without #"}
	}
	if field, msg := c.Keys.Check(); field != "" {
		return &FieldError{field, msg}
	}
	return nil
}

// renameWithRetry retries briefly: on Windows a rename over a file that another
// reader has open (the Quaver folder watcher, an antivirus scan) fails with a sharing error.
func renameWithRetry(from, to string) error {
	err := os.Rename(from, to)
	for i := 0; err != nil && i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		err = os.Rename(from, to)
	}
	return err
}
