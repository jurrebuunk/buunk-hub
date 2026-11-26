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
	// Catalogue is the path of the apps list (deploy/catalogue.yaml).
	Catalogue string

	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	// KeycloakAdminURL is the realm's admin API, for setting pictures. On
	// the cluster it is Keycloak's internal address: /admin is not public.
	KeycloakAdminURL string

	// S3 bucket for profile pictures.
	S3Endpoint, S3Bucket, S3AccessKey, S3SecretKey string

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
		Catalogue:        get("HUB_CATALOGUE", "/etc/hub/catalogue.yaml"),
		OIDCIssuer:       required("HUB_OIDC_ISSUER"),
		OIDCClientID:     get("HUB_OIDC_CLIENT_ID", "hub"),
		OIDCClientSecret: required("HUB_OIDC_CLIENT_SECRET"),
		S3Endpoint:       required("HUB_S3_ENDPOINT"),
		S3Bucket:         get("HUB_S3_BUCKET", "hub-avatars"),
		S3AccessKey:      required("HUB_S3_ACCESS_KEY"),
		S3SecretKey:      required("HUB_S3_SECRET_KEY"),
	}
	// https://auth.example/realms/x → https://auth.example/admin/realms/x
	c.KeycloakAdminURL = get("HUB_KEYCLOAK_ADMIN_URL", strings.Replace(c.OIDCIssuer, "/realms/", "/admin/realms/", 1))
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
