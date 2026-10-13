package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"syscall"
	"time"
)

// bindMessage explains a failed listen: a busy port gets the dedicated advice, any other error keeps the system's words.
func bindMessage(port int, err error) string {
	if addrInUse(err) {
		return busyPortMessage(port)
	}
	return fmt.Sprintf("cannot listen on port %d: %v. Choose another one with --port or in the config file (and update your OBS links).", port, err)
}

// wsaeAddrInUse is the Windows socket error for an address already in use.
const wsaeAddrInUse = 10048

func addrInUse(err error) bool {
	var errno syscall.Errno
	return errors.Is(err, syscall.EADDRINUSE) || (errors.As(err, &errno) && errno == wsaeAddrInUse)
}

// busyPortMessage explains why port cannot be used: another qsc answers there, or another program holds it.
func busyPortMessage(port int) string {
	if answersAsQsc(port) {
		return fmt.Sprintf(`qsc is already running on port %d. Open http://127.0.0.1:%d/ or run "qsc stop".`, port, port)
	}
	return fmt.Sprintf("port %d is used by another program. Choose another one with --port or in the config file (and update your OBS links).", port)
}

func answersAsQsc(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/status", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var body map[string]json.RawMessage
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if json.Unmarshal(data, &body) != nil {
		return false
	}
	for _, k := range []string{"version", "checks", "uptime_s"} {
		if _, ok := body[k]; !ok {
			return false
		}
	}
	return true
}
