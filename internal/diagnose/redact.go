package diagnose

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// RedactHome hides the user's home folder and name in s. The home path (plain, slash, JSON-escaped and
// short-name spellings) becomes "~" when a separator, quote, space or the end follows it, so names with
// accents, dots, parentheses or non-Latin letters are covered. names are bare user names matched as whole
// path segments, and any segment right after Users or home is replaced too.
func RedactHome(s, home string, caseInsensitive bool, names ...string) string {
	bs := string(rune(92))
	sep := "[" + bs + bs + "/\"'`" + `\s]`
	flags := ""
	if caseInsensitive {
		flags = "(?i)"
	}
	if home != "" {
		variants := []string{home, filepath.ToSlash(home), strings.ReplaceAll(home, bs, bs+bs)}
		s = regexp.MustCompile(flags+"(?:"+quoteAll(variants)+")("+sep+"|$)").ReplaceAllString(s, "~${1}")
	}
	if len(names) > 0 {
		re := regexp.MustCompile(flags + "(^|" + sep + ")(?:" + quoteAll(names) + ")(" + sep + "|$)")
		for i := 0; i < 4; i++ {
			next := re.ReplaceAllString(s, "${1}~${2}")
			if next == s {
				break
			}
			s = next
		}
	}
	generic := regexp.MustCompile("(?i)((?:^|" + sep + ")(?:Users|home)(?:" + bs + bs + "{1,2}|/))[^" + bs + bs + "/\"'`" + bs + "s]+")
	return generic.ReplaceAllString(s, "${1}~")
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

// userNames lists the bare names that identify the user: the home folder name, its short form and the login name.
func userNames(home string) []string {
	var out []string
	add := func(n string) {
		if len(n) >= 2 && n != "." && n != string(filepath.Separator) {
			out = append(out, n)
		}
	}
	if home != "" {
		add(filepath.Base(home))
		if short := shortPath(home); short != "" {
			add(filepath.Base(short))
		}
	}
	add(os.Getenv("USERNAME"))
	add(os.Getenv("USER"))
	return out
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
