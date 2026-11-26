package pages

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jurrebuunk/buunk-hub/internal/safenet"
)

// Framing asks a site, when a page is saved, whether it lets the hub frame
// it. Many sites don't; the page then becomes a link card instead of an
// empty frame.
type Framing struct {
	Client *http.Client
	Hub    *url.URL // the origin that frames: https://hub.buunk.org
	Log    *slog.Logger
}

func NewFraming(hub *url.URL, log *slog.Logger) *Framing {
	// The address is typed by a person: public addresses only.
	return &Framing{Client: safenet.Client(5 * time.Second), Hub: hub, Log: log}
}

// Check returns the header that refuses framing, or "" when the hub may
// frame the site. A site the hub can't reach (only on WARP, say, or down)
// is embedded anyway: the person's browser may well reach it, and the
// page has an "open in a new tab" button either way.
func (f *Framing) Check(ctx context.Context, address string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "buunk-hub (framing check)")
	resp, err := f.Client.Do(req)
	if err != nil {
		f.Log.Info("framing check: no answer", "site", address, "err", err)
		return ""
	}
	// Only the headers count; read a little so the connection can be reused.
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	// After redirects, as the browser's frame would see it.
	return Refusal(resp.Header, f.Hub)
}

// Refusal reads the response headers as a browser would when framing the
// page in hub: CSP frame-ancestors decides when it is there (then
// X-Frame-Options is ignored), otherwise X-Frame-Options. It returns the
// header that refuses, or "".
func Refusal(h http.Header, hub *url.URL) string {
	found := false
	for _, header := range h.Values("Content-Security-Policy") {
		// One header can hold several policies, separated by commas; each
		// is enforced.
		for _, policy := range strings.Split(header, ",") {
			for _, directive := range strings.Split(policy, ";") {
				fields := strings.Fields(directive)
				if len(fields) == 0 || !strings.EqualFold(fields[0], "frame-ancestors") {
					continue
				}
				found = true
				if !ancestorsAllow(fields[1:], hub) {
					return "Content-Security-Policy: " + strings.Join(fields, " ")
				}
			}
		}
	}
	if found {
		return ""
	}
	for _, header := range h.Values("X-Frame-Options") {
		for _, v := range strings.Split(header, ",") {
			switch v = strings.ToLower(strings.TrimSpace(v)); v {
			case "deny", "sameorigin":
				return "X-Frame-Options: " + v
			}
		}
	}
	return ""
}

// ancestorsAllow says whether a frame-ancestors source list lets hub frame
// the page. 'self' is the framed site's own origin, never the hub's.
func ancestorsAllow(sources []string, hub *url.URL) bool {
	for _, s := range sources {
		s = strings.ToLower(s)
		switch {
		case s == "'none'":
			return false
		case s == "*":
			return true
		case strings.HasPrefix(s, "'"):
			continue
		case strings.HasSuffix(s, ":") && !strings.Contains(s, "/"):
			// A scheme: https: (or http:, which also matches https).
			if s == hub.Scheme+":" || s == "http:" && hub.Scheme == "https" {
				return true
			}
		case hostSourceMatches(s, hub):
			return true
		}
	}
	return false
}

// hostSourceMatches matches [scheme://]host[:port][/path] against hub.
// The path doesn't matter for frame-ancestors.
func hostSourceMatches(s string, hub *url.URL) bool {
	scheme := ""
	if i := strings.Index(s, "://"); i >= 0 {
		scheme, s = s[:i], s[i+3:]
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	host, port := s, ""
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		host, port = s[:i], s[i+1:]
	}
	if scheme != "" && scheme != hub.Scheme && !(scheme == "http" && hub.Scheme == "https") {
		return false
	}
	hubHost := strings.ToLower(hub.Hostname())
	switch {
	case strings.HasPrefix(host, "*."):
		if !strings.HasSuffix(hubHost, host[1:]) {
			return false
		}
	case host != hubHost:
		return false
	}
	// No port: the default one, 443 for the hub on https (an http: source
	// is upgraded, so 443 too).
	defaultPort := map[string]string{"https": "443", "http": "80"}[hub.Scheme]
	hubPort := hub.Port()
	if hubPort == "" {
		hubPort = defaultPort
	}
	switch port {
	case "*":
		return true
	case "":
		return hubPort == defaultPort
	default:
		return port == hubPort
	}
}
