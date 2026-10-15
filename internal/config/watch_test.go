package config

import (
	"os"
	"strings"
	"testing"
)

func TestWatchQuaverProcessDefaultsToOn(t *testing.T) {
	if !Defaults().WatchQuaverProcess {
		t.Fatal("the process check must be on by default")
	}
	for name, content := range map[string]string{
		"missing key":     `{"port": 9000, "language": "fr"}`,
		"empty object":    `{}`,
		"empty file":      ``,
		"older v0.1.0":    `{"version": 1, "quaver_dir": "", "port": 7878, "language": "en", "overlay": {"theme": "dark"}, "keys": {"enabled": false}}`,
		"explicit true":   `{"watch_quaver_process": true}`,
		"unrelated extra": `{"future": {"a": 1}}`,
	} {
		c, err := Load(write(t, content))
		if err != nil || !c.WatchQuaverProcess {
			t.Errorf("%s: got %v, %v", name, c.WatchQuaverProcess, err)
		}
	}
}

func TestWatchQuaverProcessCanBeTurnedOffAndRoundTrips(t *testing.T) {
	p := write(t, `{"watch_quaver_process": false, "port": 9000}`)
	c, err := Load(p)
	if err != nil || c.WatchQuaverProcess {
		t.Fatalf("got %v, %v", c.WatchQuaverProcess, err)
	}
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p)
	if !strings.Contains(string(raw), `"watch_quaver_process": false`) {
		t.Fatalf("file = %s", raw)
	}
	again, err := Load(p)
	if err != nil || again.WatchQuaverProcess {
		t.Fatalf("reloaded %v, %v", again.WatchQuaverProcess, err)
	}
	again.WatchQuaverProcess = true
	if err := Save(p, again); err != nil {
		t.Fatal(err)
	}
	last, _ := Load(p)
	if !last.WatchQuaverProcess {
		t.Fatal("not turned back on")
	}
}

func TestSavingKeepsUnknownKeysNextToTheProcessSetting(t *testing.T) {
	p := write(t, `{"watch_quaver_process": false, "future_setting": {"a": [1, 2]}}`)
	c, _ := Load(p)
	c.Port = 9100
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p)
	s := string(raw)
	if !strings.Contains(s, `"future_setting"`) || !strings.Contains(s, `"watch_quaver_process": false`) {
		t.Fatalf("file = %s", s)
	}
}
