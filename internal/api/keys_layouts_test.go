package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
)

func TestPutAcceptsEveryLayoutAndKeepsItsKeys(t *testing.T) {
	for n := 1; n <= keys.MaxKeys; n++ {
		e := newEnv(t)
		list := make([]string, n)
		for i := range list {
			list[i] = fmt.Sprintf(`"F%d"`, i+1)
		}
		body := fmt.Sprintf(`{"port":5555,"keys":{"enabled":true,"layout":"%dk","k%d":[%s],"scratch":"CTRL_R"}}`, n, n, strings.Join(list, ","))
		if w := e.put(body, nil); w.Code != 200 {
			t.Fatalf("%dk: %d %s", n, w.Code, w.Body.String())
		}
		var cfg struct {
			Config config.Config `json:"config"`
		}
		decode(t, e.get("/api/config"), &cfg)
		k := cfg.Config.Keys
		if k.Layout != keys.LayoutName(n) || len(k.Lanes(n)) != n || k.Lanes(n)[n-1] != keys.Key(fmt.Sprintf("F%d", n)) || k.Scratch != "CTRL_R" {
			t.Fatalf("%dk: keys = %+v", n, k)
		}
	}
}

func TestPutRejectsWrongCountAndDuplicatesForEveryLayout(t *testing.T) {
	for n := 1; n <= keys.MaxKeys; n++ {
		field := fmt.Sprintf("keys.k%d", n)
		cases := map[string]string{
			"count": fmt.Sprintf(`{"port":5555,"keys":{"k%d":["A","A","A","A","A","A","A","A","A","A","A"]}}`, n),
			"empty": fmt.Sprintf(`{"port":5555,"keys":{"k%d":[]}}`, n),
			"name":  fmt.Sprintf(`{"port":5555,"keys":{"k%d":%s}}`, n, `["ENTER"`+strings.Repeat(`,"ENTER"`, n-1)+`]`),
		}
		for name, body := range cases {
			e := newEnv(t)
			w := e.put(body, nil)
			var er errBody
			decode(t, w, &er)
			if w.Code != 400 || er.Field != field {
				t.Errorf("%dk %s: code %d body %+v", n, name, w.Code, er)
			}
			e.noFile(t)
		}
	}
	e := newEnv(t)
	w := e.put(`{"port":5555,"keys":{"scratch":"ENTER"}}`, nil)
	var er errBody
	decode(t, w, &er)
	if w.Code != 400 || er.Field != "keys.scratch" {
		t.Fatalf("scratch: code %d body %+v", w.Code, er)
	}
}

func TestChangingAnyLayoutNeedsARestart(t *testing.T) {
	run := keys.Defaults()
	run.Enabled = true
	e := newEnv(t, withKeys(keys.Status{Enabled: true, Supported: true, Layout: "auto"}, run))
	base := `"quaver_dir":"` + jsonPath(e.quaver) + `","port":5555`
	for _, body := range []string{`{"enabled":true,"k10":["Q","W","E","R","T","Y","U","I","O","P"]}`, `{"enabled":true,"scratch":"CTRL_L"}`, `{"enabled":true,"layout":"10k"}`} {
		w := e.put(`{`+base+`,"keys":`+body+`}`, nil)
		var got struct {
			RestartRequired bool `json:"restart_required"`
		}
		decode(t, w, &got)
		if w.Code != 200 || !got.RestartRequired {
			t.Fatalf("%s: code %d restart %v", body, w.Code, got.RestartRequired)
		}
	}
}

func TestPutV010KeysSectionWithShiftInALaneListIsAccepted(t *testing.T) {
	for _, layout := range []string{"auto", "7k"} {
		e := newEnv(t)
		body := `{"port":5555,"keys":{"enabled":true,"layout":"` + layout + `","k4":["Q","W","E","R"],"k7":["SHIFT_L","S","D","SPACE","J","K","L"]}}`
		if w := e.put(body, nil); w.Code != 200 {
			t.Fatalf("%s: %d %s", layout, w.Code, w.Body.String())
		}
		var cfg struct {
			Config config.Config `json:"config"`
		}
		decode(t, e.get("/api/config"), &cfg)
		if cfg.Config.Keys.Scratch != "" || cfg.Config.Keys.K7[0] != "SHIFT_L" {
			t.Fatalf("%s: keys = %+v", layout, cfg.Config.Keys)
		}
		raw := e.get("/api/config").Body.String()
		if strings.Contains(raw, `"scratch"`) {
			t.Fatalf("%s: the config exposes a scratch key nobody set: %s", layout, raw)
		}
	}
}
