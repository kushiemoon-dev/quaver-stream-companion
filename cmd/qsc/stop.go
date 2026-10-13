package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/config"
)

// stopWait is how long qsc stop waits for the port to stop answering.
var stopWait = 5 * time.Second

func stop(args []string, out, errw io.Writer) int {
	port, code := stopPort(args, errw)
	if code >= 0 {
		return code
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/api/quit", bytes.NewReader([]byte("{}")))
	if err != nil {
		fmt.Fprintln(errw, "error:", err)
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "qsc")
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "dial" {
			fmt.Fprintf(out, "qsc is not running on port %d\n", port)
			return 0
		}
		fmt.Fprintf(errw, "error: cannot ask %s to stop: %v\n", addr, err)
		return 1
	}
	defer resp.Body.Close()
	var body struct {
		Status string `json:"status"`
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusAccepted || json.Unmarshal(data, &body) != nil || body.Status != "stopping" {
		fmt.Fprintf(errw, "error: port %d did not accept the stop request (HTTP %d); is it another program?\n", port, resp.StatusCode)
		return 1
	}

	for deadline := time.Now().Add(stopWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err != nil {
			fmt.Fprintln(out, "qsc stopped")
			return 0
		}
		c.Close()
	}
	fmt.Fprintf(errw, "error: qsc on port %d is still answering after %s\n", port, stopWait)
	return 1
}

// stopPort returns the port to stop and -1, or 0 and the exit code to use.
// The port flag wins; the config file is only read (and must be valid) when it is absent.
func stopPort(args []string, errw io.Writer) (int, int) {
	fs := flag.NewFlagSet("qsc stop", flag.ContinueOnError)
	fs.SetOutput(errw)
	port := fs.Int("port", config.DefaultPort, "local port qsc listens on")
	cfgPath := fs.String("config", "", "path to the config file (default: per-user config folder)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, 0
		}
		return 0, 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errw, "error: unexpected argument %q\n", fs.Arg(0))
		return 0, 2
	}
	set := false
	fs.Visit(func(f *flag.Flag) { set = set || f.Name == "port" })
	if set {
		if *port < 1 || *port > 65535 {
			fmt.Fprintf(errw, "error: invalid port %d (use 1-65535)\n", *port)
			return 0, 2
		}
		return *port, -1
	}
	path := *cfgPath
	if path == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return config.DefaultPort, -1
		}
		path = p
	}
	cfg, err := config.Load(path)
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(errw, "error: cannot read config %s: %v\n", path, pathErr.Err)
		return 0, 1
	}
	if err != nil {
		fmt.Fprintf(errw, "error: invalid config %s: %v\n", path, err)
		return 0, 1
	}
	return cfg.Port, -1
}
