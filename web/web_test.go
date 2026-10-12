package web

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestEmbeddedAssetsExist(t *testing.T) {
	for _, p := range []string{
		"overlays/common.js",
		"overlays/overlay.css",
		"overlays/now-playing/index.html",
		"themes/neutral-light.css",
		"dashboard/index.html",
	} {
		if _, err := fs.Stat(FS, p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

func TestNowPlayingUsesSafeDomUpdates(t *testing.T) {
	data, err := fs.ReadFile(FS, "overlays/now-playing/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "innerHTML") {
		t.Fatal("overlay must use textContent, not innerHTML (titles come from map files)")
	}
	if !strings.Contains(string(data), "qscConnect") {
		t.Fatal("overlay must connect through qscConnect")
	}
}

func dashboardFiles(t *testing.T, ext string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := fs.WalkDir(FS, "dashboard", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ext) {
			return err
		}
		data, rerr := fs.ReadFile(FS, p)
		out[p] = string(data)
		return rerr
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDashboardFilesExist(t *testing.T) {
	for _, p := range []string{
		"dashboard/index.html",
		"dashboard/style.css",
		"dashboard/favicon.svg",
		"dashboard/app.js",
		"dashboard/api.js",
		"dashboard/i18n.js",
		"dashboard/pages/status.js",
		"dashboard/pages/overlay-catalog.js",
		"dashboard/pages/overlay-url.js",
		"dashboard/pages/overlay-controls.js",
		"dashboard/pages/theme-model.js",
		"dashboard/pages/overlays.js",
		"dashboard/pages/history-model.js",
		"dashboard/pages/history.js",
		"dashboard/pages/settings-model.js",
		"dashboard/pages/settings.js",
		"dashboard/i18n/en.json",
	} {
		if _, err := fs.Stat(FS, p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

func TestDashboardIndexLoadsOnlyRelativeAssets(t *testing.T) {
	data, err := fs.ReadFile(FS, "dashboard/index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	if regexp.MustCompile(`(?i)https?://|//[a-z0-9.-]+/`).MatchString(html) {
		t.Fatal("index.html must not reference an external URL")
	}
	refs := regexp.MustCompile(`(?:src|href)="([^"]+)"`).FindAllStringSubmatch(html, -1)
	for _, m := range refs {
		if strings.HasPrefix(m[1], "/") || strings.Contains(m[1], ":") {
			t.Errorf("asset %q must be a relative path", m[1])
			continue
		}
		if _, err := fs.Stat(FS, "dashboard/"+m[1]); err != nil {
			t.Errorf("asset %q referenced by index.html: %v", m[1], err)
		}
	}
}

func TestDashboardScriptsAvoidUnsafeDom(t *testing.T) {
	files := dashboardFiles(t, ".js")
	if len(files) == 0 {
		t.Fatal("no dashboard script found")
	}
	for p, src := range files {
		for _, bad := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s uses %s", p, bad)
			}
		}
	}
}

func loadEnglish(t *testing.T) map[string]string {
	t.Helper()
	data, err := fs.ReadFile(FS, "dashboard/i18n/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var en map[string]string
	if err := json.Unmarshal(data, &en); err != nil {
		t.Fatalf("en.json must be a flat object of strings: %v", err)
	}
	return en
}

func TestEnglishCatalogHasNoEmptyValue(t *testing.T) {
	en := loadEnglish(t)
	if len(en) == 0 {
		t.Fatal("en.json is empty")
	}
	for k, v := range en {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			t.Errorf("empty key or value: %q", k)
		}
	}
}

var (
	dynamicKeySets = map[string][]string{
		"game":     {"initialization", "menu", "select", "loading", "gameplay", "results", "closed", "unknown"},
		"check":    {"quaver_dir", "log", "database", "port"},
		"judgment": {"marv", "perf", "great", "good", "okay", "miss"},
		"nav":      {"status", "overlays", "obs", "history", "settings", "twitch"},
		"status":   {"live", "connecting", "reconnecting"},

		"history.col":    {"map", "difficulty", "result", "accuracy", "combo", "judgments"},
		"history.filter": {"all", "pb", "fc", "failed"},
		"settings.lang":  {"auto", "en", "fr", "de"},
	}
	eventNames = map[string]bool{"game.state": true, "map.started": true, "score.saved": true}
	keyLiteral = regexp.MustCompile(`['"]([a-z_]+(?:\.[a-z_]+)+)['"]`)
)

func TestEveryTranslationKeyUsedExists(t *testing.T) {
	en := loadEnglish(t)
	namespaces := map[string]bool{}
	for k := range en {
		namespaces[strings.SplitN(k, ".", 2)[0]] = true
	}
	used := map[string]bool{}
	for p, src := range dashboardFiles(t, ".js") {
		for _, m := range keyLiteral.FindAllStringSubmatch(src, -1) {
			if !namespaces[strings.SplitN(m[1], ".", 2)[0]] {
				continue
			}
			if eventNames[m[1]] {
				continue
			}
			used[m[1]] = true
			if _, ok := en[m[1]]; !ok {
				t.Errorf("%s uses missing key %q", p, m[1])
			}
		}
	}
	for ns, names := range dynamicKeySets {
		for _, name := range names {
			key := ns + "." + name
			used[key] = true
			if _, ok := en[key]; !ok {
				t.Errorf("dynamic key %q is missing from en.json", key)
			}
		}
	}
	for k := range en {
		if !used[k] {
			t.Logf("key %q is never referenced", k)
		}
	}
}

func TestOverlaysPageIsLoadedAndRegistered(t *testing.T) {
	data, err := fs.ReadFile(FS, "dashboard/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `src="pages/overlays.js"`) {
		t.Error("index.html must load pages/overlays.js")
	}
	page, err := fs.ReadFile(FS, "dashboard/pages/overlays.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "registerPage('overlays'") {
		t.Error("overlays page must register itself")
	}
}

func TestNowPlayingHasDemoModeWithoutEventSource(t *testing.T) {
	data, err := fs.ReadFile(FS, "overlays/now-playing/index.html")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "get('demo') === '1'") {
		t.Fatal("demo must be enabled only by demo=1")
	}
	open := strings.Index(src, "if (isDemo) {")
	split := strings.Index(src, "} else {")
	if open < 0 || split < open {
		t.Fatal("now-playing must branch on isDemo with an else branch")
	}
	demo, live := src[open:split], src[split:]
	if strings.Contains(demo, "qscConnect") || strings.Contains(demo, "EventSource") {
		t.Error("demo branch must not open the event stream")
	}
	if !strings.Contains(live, "qscConnect") {
		t.Error("live branch must connect through qscConnect")
	}
}

func TestCommonJSValidatesParamsAndFetchesSavedDefaults(t *testing.T) {
	data, err := fs.ReadFile(FS, "overlays/common.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, want := range []string{
		"/^[a-z0-9-]+$/",
		"/^[0-9a-fA-F]{3,8}$/",
		"fetch('/api/config'",
		"!isDemo && !(urlTheme && urlAccent)",
		"AbortController",
		"FALLBACK_THEME = 'neutral-light'",
		"visibility = 'hidden'",
		".catch(function () { return {}; })",
		"window.qscConnect",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("common.js is missing %q", want)
		}
	}
}

func TestCommonJSFetchTimeoutBelowRevealTimeout(t *testing.T) {
	data, err := fs.ReadFile(FS, "overlays/common.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	val := func(name string) int {
		m := regexp.MustCompile(name + `\s*=\s*(\d+)`).FindStringSubmatch(src)
		if m == nil {
			t.Fatalf("%s not found in common.js", name)
		}
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if fetch, reveal := val("FETCH_TIMEOUT_MS"), val("REVEAL_TIMEOUT_MS"); fetch >= reveal {
		t.Errorf("fetch timeout %d must be below reveal timeout %d", fetch, reveal)
	}
	if !regexp.MustCompile(`try\s*\{[^}]*fetchDefaults\(\)`).MatchString(src) {
		t.Error("the defaults fetch must be started inside a try block")
	}
}

func TestPreviewFrameIsTransparentToParent(t *testing.T) {
	data, err := fs.ReadFile(FS, "dashboard/style.css")
	if err != nil {
		t.Fatal(err)
	}
	rule := regexp.MustCompile(`\.preview-frame\s*\{[^}]*\}`).FindString(string(data))
	if rule == "" {
		t.Fatal(".preview-frame rule not found")
	}
	if !regexp.MustCompile(`color-scheme\s*:\s*light\b`).MatchString(rule) {
		t.Errorf(".preview-frame must set color-scheme: light, got %q", rule)
	}
}

func TestOverlayURLHelpers(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "check.js")
	if err := os.WriteFile(script, []byte(overlayURLScript), 0o600); err != nil {
		t.Fatal(err)
	}
	lib, err := filepath.Abs("dashboard/pages/overlay-url.js")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, script, lib).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v: %s", err, out)
	}
}

const overlayURLScript = `
const assert = require('assert');
const u = require(process.argv[2]);
const O = 'http://127.0.0.1:7878';
const base = { theme: 'neutral-light', accent: '', defaultTheme: 'neutral-light', defaultAccent: '' };

assert.strictEqual(u.buildOverlayUrl(base, false, O), O + '/overlays/now-playing/');
assert.strictEqual(u.buildOverlayUrl({ ...base, theme: 'dark' }, false, O), O + '/overlays/now-playing/?theme=dark');
assert.strictEqual(u.buildOverlayUrl({ ...base, accent: 'ff00aa' }, false, O), O + '/overlays/now-playing/?accent=ff00aa');
assert.strictEqual(u.buildOverlayUrl({ ...base, accent: '#FF00AA' }, false, O), O + '/overlays/now-playing/?accent=ff00aa');
const same = { theme: 'dark', accent: 'abc', defaultTheme: 'dark', defaultAccent: 'abc' };
assert.strictEqual(u.buildOverlayUrl(same, false, O), O + '/overlays/now-playing/');
assert.strictEqual(u.buildOverlayUrl(same, true, O), '/overlays/now-playing/?theme=dark&accent=abc&demo=1');
assert.strictEqual(u.buildOverlayUrl({ ...base, accent: '' }, true, O), '/overlays/now-playing/?theme=neutral-light&demo=1');

assert.strictEqual(u.normalizeAccent('  #AbC '), 'abc');
for (const ok of ['abc', 'abcd', 'abcdef', 'abcdef12', 'ABC']) assert.ok(u.isValidAccent(u.normalizeAccent(ok)), ok);
for (const bad of ['', 'ab', 'abcde', 'abcdef1', 'abcdef123', 'ggg', 'zzz', '"><script>']) assert.ok(!u.isValidAccent(u.normalizeAccent(bad)), bad);
for (const bad of ['../x', 'zzz!', '"><script>', 'A', '', 'a b']) assert.ok(!u.isValidTheme(bad), bad);

for (const hostile of ['../x', '"><script>', 'a&b=c']) {
  const url = u.buildOverlayUrl({ ...base, theme: hostile, accent: hostile }, false, O);
  assert.strictEqual(url, O + '/overlays/now-playing/', hostile);
}
const dark = u.buildOverlayUrl({ ...base, theme: 'dark-2' }, false, O);
assert.strictEqual(new URL(dark).searchParams.get('theme'), 'dark-2');
`

const maxIdenticalToEnglish = 2

var placeholderRe = regexp.MustCompile(`\{[^{}]*\}`)

func loadCatalog(t *testing.T, lang string) map[string]string {
	t.Helper()
	data, err := fs.ReadFile(FS, "dashboard/i18n/"+lang+".json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s.json must be a flat object of strings: %v", lang, err)
	}
	return m
}

// sharedTerm reports keys whose value is a proper noun, an acronym or a
// game term kept as is in every language.
func sharedTerm(key string) bool {
	switch key {
	case "nav.twitch", "nav.overlays", "nav.obs", "status.version", "settings.version", "history.filter.fc",
		"nav.status", "status.problem", "status.keys", "status.pb", "status.fc", "status.ok",
		"status.live", "status.autoplay", "game.menu", "history.filter.pb", "history.col.map",
		"settings.port", "settings.lang.en", "settings.lang.fr", "settings.lang.de":
		return true
	}
	return strings.HasPrefix(key, "judgment.")
}

func sortedPlaceholders(s string) string {
	found := placeholderRe.FindAllString(s, -1)
	sort.Strings(found)
	return strings.Join(found, " ")
}

func TestCatalogsMatchEnglish(t *testing.T) {
	en := loadEnglish(t)
	for _, lang := range []string{"fr", "de"} {
		cat := loadCatalog(t, lang)
		for k := range en {
			if _, ok := cat[k]; !ok {
				t.Errorf("%s.json is missing key %q", lang, k)
			}
		}
		for k := range cat {
			if _, ok := en[k]; !ok {
				t.Errorf("%s.json has extra key %q", lang, k)
			}
		}
		same, counted := 0, 0
		for k, v := range cat {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s.json: empty value for %q", lang, k)
			}
			if want, got := sortedPlaceholders(en[k]), sortedPlaceholders(v); want != got {
				t.Errorf("%s.json: %q placeholders %q, want %q", lang, k, got, want)
			}
			if sharedTerm(k) {
				continue
			}
			counted++
			if v == en[k] {
				same++
			}
		}
		if same > maxIdenticalToEnglish {
			t.Errorf("%s.json: %d of %d values are identical to English (max %d)", lang, same, counted, maxIdenticalToEnglish)
		}
	}
}

func TestCatalogsHaveNoDashes(t *testing.T) {
	for _, lang := range []string{"en", "fr", "de"} {
		data, err := fs.ReadFile(FS, "dashboard/i18n/"+lang+".json")
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(string(data), "\u2014\u2013") {
			t.Errorf("%s.json contains an em or en dash", lang)
		}
	}
}

func TestCatalogsHaveNoDuplicateKeys(t *testing.T) {
	for _, lang := range []string{"en", "fr", "de"} {
		data, err := fs.ReadFile(FS, "dashboard/i18n/"+lang+".json")
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		if _, err := dec.Token(); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				t.Fatal(err)
			}
			name, _ := key.(string)
			if seen[name] {
				t.Errorf("%s.json: duplicate key %q", lang, name)
			}
			seen[name] = true
			if _, err := dec.Token(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

var fixedDetailRe = regexp.MustCompile(`(?:Detail: |const dirNotFound = |result\([^\n]*, )"([^"]+)"`)

func TestDiagnoseFixedDetailsAreTranslated(t *testing.T) {
	src, err := os.ReadFile("../internal/diagnose/diagnose.go")
	if err != nil {
		t.Fatal(err)
	}
	page, err := fs.ReadFile(FS, "dashboard/pages/status.js")
	if err != nil {
		t.Fatal(err)
	}
	found := fixedDetailRe.FindAllStringSubmatch(string(src), -1)
	if len(found) != 4 {
		t.Fatalf("expected 4 fixed detail strings in diagnose.go, found %d", len(found))
	}
	for _, m := range found {
		entry := regexp.MustCompile(`'` + regexp.QuoteMeta(m[1]) + `': '(check\.detail\.\w+)'`).FindStringSubmatch(string(page))
		if entry == nil {
			t.Errorf("status.js has no translation entry for detail %q", m[1])
			continue
		}
		for _, lang := range []string{"en", "fr", "de"} {
			if _, ok := loadCatalog(t, lang)[entry[1]]; !ok {
				t.Errorf("%s.json is missing %q", lang, entry[1])
			}
		}
	}
}

func TestCommonJSFallsBackOnceWhenThemeFailsToLoad(t *testing.T) {
	data, err := fs.ReadFile(FS, "overlays/common.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, want := range []string{
		"link.onerror = function ()",
		"theme !== FALLBACK_THEME",
		"applyLook(FALLBACK_THEME, accent)",
		"setTimeout(reveal, REVEAL_TIMEOUT_MS)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("common.js is missing %q", want)
		}
	}
	if strings.Contains(src, "link.onerror = reveal") {
		t.Error("a failed theme must fall back before revealing")
	}
}
