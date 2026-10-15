package server

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// FuzzCrossOrigin: a request is same-origin only without Origin, or with one Origin whose host equals Host.
func FuzzCrossOrigin(f *testing.F) {
	f.Add("http://127.0.0.1:7000", "127.0.0.1:7000", 1)
	f.Add("null", "127.0.0.1:7000", 1)
	f.Add("http://EVIL.com", "evil.com", 1)
	f.Add("", "127.0.0.1:7000", 0)
	f.Add("http://127.0.0.1:7000", "127.0.0.1:7000", 2)
	f.Fuzz(func(t *testing.T, origin, host string, n int) {
		n %= 4
		if n < 0 {
			n = -n
		}
		r := httptest.NewRequest("GET", "/events", nil)
		r.Host = host
		for i := 0; i < n; i++ {
			r.Header.Add("Origin", origin)
		}
		cross := crossOrigin(r)
		switch {
		case n == 0:
			if cross {
				t.Fatal("no Origin is cross-origin")
			}
		case n > 1:
			if !cross {
				t.Fatal("several Origin headers are same-origin")
			}
		default:
			u, err := url.Parse(origin)
			same := err == nil && u.Host != "" && strings.EqualFold(u.Host, host)
			if cross == same {
				t.Fatalf("crossOrigin(%q, host %q) = %v", origin, host, cross)
			}
		}
	})
}
