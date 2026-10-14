package diagnose

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"
)

var bs = string(rune(92))

// sep matches what may follow or precede a path segment: a separator, quote or space.
var sep = "[" + bs + bs + "/\"'`" + `\s]`

// genericUserDir matches the segment right after Users or home.
var genericUserDir = regexp.MustCompile("(?i)((?:^|" + sep + ")(?:Users|home)(?:" + bs + bs + "{1,2}|/))[^" + bs + bs + "/\"'`" + bs + "s]+")

// minBareName is the shortest user name replaced as a word on its own: shorter ones would hit
// ordinary text. Paths under the home folder are always redacted.
const minBareName = 4

// RedactHome hides the user's home folder and name in s. The home path (plain, slash, JSON-escaped and
// short-name spellings) becomes "~" when a separator, quote, space or the end follows it, so names with
// accents, dots, parentheses or non-Latin letters are covered. names are bare user names matched as whole
// path segments, and any segment right after Users or home is replaced too.
func RedactHome(s, home string, caseInsensitive bool, names ...string) string {
	return newHomeRedactor(home, caseInsensitive, names)(s)
}

// newHomeRedactor compiles the patterns once.
func newHomeRedactor(home string, caseInsensitive bool, names []string) func(string) string {
	flags := ""
	if caseInsensitive {
		flags = "(?i)"
	}
	var homeRe, nameRe *regexp.Regexp
	if home != "" {
		variants := []string{home, filepath.ToSlash(home), strings.ReplaceAll(home, bs, "/"), strings.ReplaceAll(home, bs, bs+bs)}
		homeRe = regexp.MustCompile(flags + "(?:" + quoteAll(variants) + ")(" + sep + "|$)")
	}
	var bare []string
	for _, n := range names {
		if utf8.RuneCountInString(n) >= minBareName {
			bare = append(bare, n)
		}
	}
	if len(bare) > 0 {
		nameRe = regexp.MustCompile(flags + "(^|" + sep + ")(?:" + quoteAll(bare) + ")(" + sep + "|$)")
	}
	return func(s string) string {
		if homeRe != nil {
			s = homeRe.ReplaceAllString(s, "~${1}")
		}
		if nameRe != nil {
			for i := 0; i < 4; i++ {
				next := nameRe.ReplaceAllString(s, "${1}~${2}")
				if next == s {
					break
				}
				s = next
			}
		}
		return genericUserDir.ReplaceAllString(s, "${1}~")
	}
}

func quoteAll(list []string) string {
	q := make([]string, 0, len(list))
	for _, v := range list {
		if v != "" {
			q = append(q, regexp.QuoteMeta(v))
		}
	}
	return strings.Join(q, "|")
}

// lastSegment is the last element of a path written with either kind of separator, since a
// Windows path can appear in a report built on any OS.
func lastSegment(p string) string {
	p = strings.TrimRight(p, bs+"/")
	if i := strings.LastIndexAny(p, bs+"/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// caseInsensitiveHome tells whether paths under home compare without case: Windows paths do, wherever the report is made.
func caseInsensitiveHome(home string) bool {
	return runtime.GOOS == "windows" || strings.Contains(home, bs) || (len(home) > 1 && home[1] == ':')
}

// userNames lists the bare names that identify the user: the home folder name, its short form and the login name.
func userNames(home string) []string {
	var out []string
	add := func(n string) {
		if len(n) >= 2 && n != "." && n != string(filepath.Separator) {
			out = append(out, n)
		}
	}
	if home != "" {
		add(lastSegment(home))
		if short := shortPath(home); short != "" {
			add(lastSegment(short))
		}
	}
	add(os.Getenv("USERNAME"))
	add(os.Getenv("USER"))
	return out
}

// HomeRedactor returns a function that applies RedactHome for home, or for the user home folder when
// home is empty, so log lines and the support report hide the same names.
func HomeRedactor(home string) func(string) string {
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return newHomeRedactor(home, caseInsensitiveHome(home), userNames(home))
}

// cutRunes shortens s to at most max bytes on a rune boundary.
func cutRunes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + " [cut]"
}
