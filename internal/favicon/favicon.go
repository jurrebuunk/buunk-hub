// Package favicon finds and keeps the icon of each catalogue app, so Home
// can show the site's own icon. The server fetches it: the browser can't
// read another site's HTML to find where its icon is.
package favicon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/sync/singleflight"
)

const (
	maxPage  = 512 << 10 // enough to find the <head>
	maxIcon  = 256 << 10
	keepOK   = 24 * time.Hour
	keepMiss = time.Hour // a site without an icon, or not reachable
)

// ErrNone means the site has no usable icon (or couldn't be reached).
var ErrNone = errors.New("no icon")

type Icon struct {
	Type string
	Data []byte
}

type entry struct {
	icon    Icon
	err     error
	expires time.Time
}

// Cache fetches icons on first use and keeps them.
type Cache struct {
	Client *http.Client
	Log    *slog.Logger

	mu      sync.Mutex
	entries map[string]entry
	group   singleflight.Group
	now     func() time.Time
}

func NewCache(log *slog.Logger) *Cache {
	return &Cache{
		Client: &http.Client{
			Timeout: 5 * time.Second,
			// Only https, also after redirects.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if req.URL.Scheme != "https" || len(via) >= 5 {
					return errors.New("redirect refused")
				}
				return nil
			},
		},
		Log:     log,
		entries: map[string]entry{},
		now:     time.Now,
	}
}

// Get returns the icon of the site at siteURL, or ErrNone.
func (c *Cache) Get(ctx context.Context, siteURL string) (Icon, error) {
	c.mu.Lock()
	e, ok := c.entries[siteURL]
	c.mu.Unlock()
	if ok && c.now().Before(e.expires) {
		return e.icon, e.err
	}
	v, _, _ := c.group.Do(siteURL, func() (any, error) {
		// Not tied to one request: others may be waiting for the same icon.
		icon, err := c.find(context.WithoutCancel(ctx), siteURL)
		e := entry{icon: icon, expires: c.now().Add(keepOK)}
		if err != nil {
			c.Log.Info("no favicon", "site", siteURL, "err", err)
			e = entry{err: ErrNone, expires: c.now().Add(keepMiss)}
		}
		c.mu.Lock()
		c.entries[siteURL] = e
		c.mu.Unlock()
		return e, nil
	})
	e = v.(entry)
	return e.icon, e.err
}

// find tries the icons the page declares, best first, then /favicon.ico.
func (c *Cache) find(ctx context.Context, siteURL string) (Icon, error) {
	base, err := url.Parse(siteURL)
	if err != nil || base.Scheme != "https" {
		return Icon{}, fmt.Errorf("not an https URL: %q", siteURL)
	}
	candidates := []string{}
	if page, final, err := c.fetch(ctx, base.String(), maxPage); err == nil {
		candidates = append(candidates, IconLinks(page, final)...)
	}
	candidates = append(candidates, base.ResolveReference(&url.URL{Path: "/favicon.ico"}).String())
	var errs []error
	for _, u := range candidates {
		body, _, err := c.fetch(ctx, u, maxIcon)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if t, ok := imageType(body); ok {
			return Icon{Type: t, Data: body}, nil
		}
		errs = append(errs, fmt.Errorf("%s: not an image", u))
	}
	return Icon{}, errors.Join(errs...)
}

func (c *Cache) fetch(ctx context.Context, u string, limit int64) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "buunk-hub (favicon)")
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("%s: %s", u, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(body)) > limit {
		return nil, nil, fmt.Errorf("%s: too large", u)
	}
	return body, resp.Request.URL, nil
}

// IconLinks returns the icons a page declares, best first: larger sizes
// before smaller, apple-touch-icon (usually 180 px) before plain icons.
func IconLinks(page []byte, base *url.URL) []string {
	type link struct {
		href string
		rank int
	}
	var links []link
	z := html.NewTokenizer(bytes.NewReader(page))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		t := z.Token()
		if t.Data == "body" {
			break
		}
		if t.Data != "link" {
			continue
		}
		var rel, href, sizes string
		for _, a := range t.Attr {
			switch strings.ToLower(a.Key) {
			case "rel":
				rel = strings.ToLower(a.Val)
			case "href":
				href = a.Val
			case "sizes":
				sizes = strings.ToLower(a.Val)
			}
		}
		rels := strings.Fields(rel)
		isIcon, isApple := false, false
		for _, r := range rels {
			isIcon = isIcon || r == "icon"
			isApple = isApple || r == "apple-touch-icon"
		}
		if href == "" || !(isIcon || isApple) {
			continue
		}
		u, err := base.Parse(href)
		if err != nil || u.Scheme != "https" {
			continue
		}
		rank := 16
		if isApple {
			rank = 180
		}
		if sizes == "any" {
			rank = 1000 // scalable (SVG)
		} else if w, _, ok := strings.Cut(sizes, "x"); ok {
			if n, err := strconv.Atoi(w); err == nil {
				rank = n
			}
		}
		links = append(links, link{u.String(), rank})
	}
	// Stable, best first.
	for i := 1; i < len(links); i++ {
		for j := i; j > 0 && links[j].rank > links[j-1].rank; j-- {
			links[j], links[j-1] = links[j-1], links[j]
		}
	}
	out := make([]string, len(links))
	for i, l := range links {
		out[i] = l.href
	}
	return out
}

// imageType sniffs the bytes; what the site's Content-Type says doesn't
// count (Cloudreve answers /favicon.ico with its HTML page).
func imageType(b []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(b, []byte{0, 0, 1, 0}):
		return "image/x-icon", true
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", true
	case bytes.HasPrefix(b, []byte("GIF8")):
		return "image/gif", true
	case len(b) > 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "image/webp", true
	case bytes.HasPrefix(b, []byte{0xff, 0xd8, 0xff}):
		return "image/jpeg", true
	}
	head := bytes.ToLower(bytes.TrimSpace(b[:min(len(b), 512)]))
	if bytes.HasPrefix(head, []byte("<svg")) || (bytes.HasPrefix(head, []byte("<?xml")) && bytes.Contains(head, []byte("<svg"))) {
		return mime.TypeByExtension(".svg"), true
	}
	return "", false
}
