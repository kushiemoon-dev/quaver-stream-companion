package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

func doctorOutput(t *testing.T, keysJSON string) (int, string) {
	t.Helper()
	d := testutil.NewQuaverDir(t)
	cfg := writeConfig(t, fmt.Sprintf(`{"quaver_dir": "%s"%s}`, jsonPath(d.Path), keysJSON))
	var out, errw bytes.Buffer
	code := run(context.Background(), []string{"doctor", "--config", cfg, "--port", fmt.Sprint(freePort(t))}, &out, &errw)
	return code, out.String()
}

func TestDoctorPrintsKeysOffByDefault(t *testing.T) {
	code, out := doctorOutput(t, "")
	if code != 0 || !strings.Contains(out, "Lane keys: off") {
		t.Fatalf("code %d out %q", code, out)
	}
}

func TestDoctorPrintsKeysWhenEnabled(t *testing.T) {
	code, out := doctorOutput(t, `, "keys": {"enabled": true}`)
	if !strings.Contains(out, "Lane keys:") || strings.Contains(out, "Lane keys: off") {
		t.Fatalf("code %d out %q", code, out)
	}
	if keys.Supported() && (code != 0 || !strings.Contains(out, "[ OK ] Lane keys")) {
		t.Fatalf("supported platform: code %d out %q", code, out)
	}
	if !keys.Supported() && (code != 1 || !strings.Contains(out, "[FAIL] Lane keys")) {
		t.Fatalf("unsupported platform: code %d out %q", code, out)
	}
}

func TestDoctorRejectsBadKeysConfig(t *testing.T) {
	cfg := writeConfig(t, `{"keys": {"k4": ["A","S"]}}`)
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"doctor", "--config", cfg}, &out, &errw); code != 1 || !strings.Contains(out.String(), "keys.k4") {
		t.Fatalf("code %d stdout %q", code, out.String())
	}
}

func TestServeReportsKeysStatus(t *testing.T) {
	for _, tc := range []struct {
		name, cfg string
		enabled   bool
	}{{"default", `{}`, false}, {"enabled", `{"keys": {"enabled": true, "layout": "7k"}}`, true}} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := writeConfig(t, tc.cfg)
			s := startServer(t, "--config", cfg)
			code, body := s.req(t, "GET", "/api/status", "", "", nil)
			var st struct {
				Keys keys.Status `json:"keys"`
			}
			if code != 200 || json.Unmarshal([]byte(body), &st) != nil {
				t.Fatalf("%d %q", code, body)
			}
			if st.Keys.Enabled != tc.enabled || (tc.enabled && st.Keys.Layout != "7k") {
				t.Fatalf("keys = %+v", st.Keys)
			}
			if tc.enabled && (testSamplerCreated.Load() == 0 || !st.Keys.Supported) {
				t.Fatalf("fake factory not used: created=%d status=%+v", testSamplerCreated.Load(), st.Keys)
			}
		})
	}
}
