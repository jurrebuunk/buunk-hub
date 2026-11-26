// Package config reads the server's settings. They come from environment
// variables only, each documented in README.md.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	// Addr is where the HTTP server listens, host:port.
	Addr string
	// PublicURL is the hub's address as people see it, without a path:
	// the OIDC redirect, cookie security and the Origin check derive from it.
	PublicURL *url.URL
	// DatabaseURL is a Postgres connection string.
	DatabaseURL string

	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string

	// SessionKey (32 bytes) seals the sign-in cookie and the tokens stored
	// in the database.
	SessionKey []byte
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	get := func(name, def string) string {
		if v, ok := lookup(name); ok && v != "" {
			return v
		}
		return def
	}
	var errs []error
	required := func(name string) string {
		v := get(name, "")
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
		return v
	}

	c := Config{
		Addr:             get("HUB_ADDR", ":8080"),
		DatabaseURL:      required("HUB_DATABASE_URL"),
		OIDCIssuer:       required("HUB_OIDC_ISSUER"),
		OIDCClientID:     get("HUB_OIDC_CLIENT_ID", "hub"),
		OIDCClientSecret: required("HUB_OIDC_CLIENT_SECRET"),
	}
	if _, _, err := net.SplitHostPort(c.Addr); err != nil {
		errs = append(errs, fmt.Errorf("HUB_ADDR %q: %w", c.Addr, err))
	}
	if v := required("HUB_PUBLIC_URL"); v != "" {
		u, err := url.Parse(v)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("HUB_PUBLIC_URL: %w", err))
		case u.Scheme != "https" && u.Scheme != "http", u.Host == "",
			strings.TrimSuffix(u.Path, "/") != "", u.RawQuery != "", u.Fragment != "":
			errs = append(errs, fmt.Errorf("HUB_PUBLIC_URL %q: want scheme://host[:port] only", v))
		default:
			u.Path = ""
			c.PublicURL = u
		}
	}
	if v := required("HUB_SESSION_KEY"); v != "" {
		key, err := base64.StdEncoding.DecodeString(v)
		if err != nil || len(key) != 32 {
			errs = append(errs, errors.New("HUB_SESSION_KEY: want 32 bytes, base64 (openssl rand -base64 32)"))
		}
		c.SessionKey = key
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return c, nil
}
