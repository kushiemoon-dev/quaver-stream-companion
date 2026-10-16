package main

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServeExposesTheSessionAndResetAnnouncesIt(t *testing.T) {
	s := startServer(t)
	if code, body := s.req(t, "GET", "/state", "", "", nil); code != 200 || !strings.Contains(body, `"session":{"started_at"`) {
		t.Fatalf("/state: %d %q", code, body)
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/events", s.port))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	seen := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if typ, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				seen <- typ
			}
		}
	}()
	if got := <-seen; got != "snapshot" {
		t.Fatalf("first event = %q", got)
	}
	hdr := map[string]string{"Content-Type": "application/json", "X-Requested-With": "qsc"}
	if code, body := s.req(t, "POST", "/api/session/reset", "", "{}", hdr); code != 200 {
		t.Fatalf("reset: %d %q", code, body)
	}
	select {
	case got := <-seen:
		if got != "session.updated" {
			t.Fatalf("event = %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no session.updated after the reset")
	}
}
