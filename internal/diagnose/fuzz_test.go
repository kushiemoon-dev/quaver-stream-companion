package diagnose

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// segmentRe matches name as a whole path segment, the way RedactHome promises to hide it.
func segmentRe(name string, ci bool) *regexp.Regexp {
	flags := ""
	if ci {
		flags = "(?i)"
	}
	return regexp.MustCompile(flags + "(?:^|" + sep + ")" + regexp.QuoteMeta(name) + "(?:" + sep + "|$)")
}

// realisticName is a user name made of letters, digits, dots, dashes, underscores, parentheses or
// accents: no separator, quote or space, and not the "~" placeholder.
func realisticName(n string) bool {
	if utf8.RuneCountInString(n) < minBareName || !utf8.ValidString(n) {
		return false
	}
	return !strings.ContainsAny(n, "\\/\"'` \t\r\n\v\f~")
}

// FuzzRedactHomeName: a user name never survives as a path segment.
func FuzzRedactHomeName(f *testing.F) {
	f.Add(`C:\Users\Kushie\AppData "Kushie" /home/Kushie/x Kushie`, "Kushie", true)
	f.Add("a/Zoé.B/Zoé.B/Zoé.B/Zoé.B/Zoé.B/Zoé.B/Zoé.B/Zoé.B/Zoé.B/b", "Zoé.B", false)
	f.Add("x KUSHIE y", "kushie", true)
	f.Fuzz(func(t *testing.T, s, name string, ci bool) {
		if !realisticName(name) {
			return
		}
		out := RedactHome(s, "", ci, name)
		if segmentRe(name, ci).MatchString(out) {
			t.Fatalf("name %q survives in %q (from %q)", name, out, s)
		}
	})
}

// FuzzRedactHomePath: the home folder, in any of its spellings, never survives when a separator or the end follows.
func FuzzRedactHomePath(f *testing.F) {
	f.Add(`log C:\Users\Kushie\x and C:/Users/Kushie and C:\\Users\\Kushie\\y`, "Kushie")
	f.Add(`"C:\Users\Jean Dupont\AppData"`, "Jean Dupont")
	f.Fuzz(func(t *testing.T, s, user string) {
		if user == "" || strings.ContainsAny(user, "\\/") || !utf8.ValidString(user) {
			return
		}
		home := `C:\Users\` + user
		out := RedactHome(s, home, true, user)
		for _, v := range []string{home, strings.ReplaceAll(home, `\`, "/"), strings.ReplaceAll(home, `\`, `\\`)} {
			re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(v) + "(?:" + sep + "|$)")
			if re.MatchString(out) {
				t.Fatalf("home %q survives in %q (from %q)", v, out, s)
			}
		}
	})
}

// FuzzRedactIdempotent: redacting twice gives the same text as redacting once.
func FuzzRedactIdempotent(f *testing.F) {
	f.Add(`C:\Users\Kushie\a /home/bob/x Kushie`, "Kushie")
	f.Fuzz(func(t *testing.T, s, name string) {
		if !realisticName(name) {
			return
		}
		// A real home folder; a degenerate one such as " " is not idempotent and never occurs.
		r := newHomeRedactor(`C:\Users\`+name, true, []string{name})
		once := r(s)
		if twice := r(once); twice != once {
			t.Fatalf("not idempotent:\n%q\n%q\n%q", s, once, twice)
		}
	})
}

// FuzzCutRunes keeps valid UTF-8 and the byte bound.
func FuzzCutRunes(f *testing.F) {
	f.Add("héllo wörld", 3)
	f.Add("abcdef", 6)
	f.Add("abcdefgh", 6)
	f.Fuzz(func(t *testing.T, s string, max int) {
		if max < 0 || max > 1<<16 || !utf8.ValidString(s) {
			return
		}
		out := cutRunes(s, max)
		if len(s) <= max {
			if out != s {
				t.Fatal("short text changed")
			}
			return
		}
		body := strings.TrimSuffix(out, " [cut]")
		if len(body) > max || len(body) <= max-utf8.UTFMax || !utf8.ValidString(out) || !strings.HasPrefix(s, body) {
			t.Fatalf("cutRunes(%q, %d) = %q", s, max, out)
		}
	})
}
