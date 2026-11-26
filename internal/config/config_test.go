package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func valid() map[string]string {
	return map[string]string{
		"HUB_PUBLIC_URL":         "https://hub.example.test",
		"HUB_DATABASE_URL":       "postgres://hub@db/hub",
		"HUB_OIDC_ISSUER":        "https://auth.example.test/realms/test",
		"HUB_OIDC_CLIENT_SECRET": "secret",
		"HUB_SESSION_KEY":        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"HUB_S3_ENDPOINT":        "https://s3.example.test",
		"HUB_S3_ACCESS_KEY":      "a",
		"HUB_S3_SECRET_KEY":      "s",
	}
}

func TestDefaults(t *testing.T) {
	c, err := load(env(valid()))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || c.OIDCClientID != "hub" {
		t.Errorf("Addr = %q, OIDCClientID = %q", c.Addr, c.OIDCClientID)
	}
	if c.KeycloakAdminURL != "https://auth.example.test/admin/realms/test" || c.S3Bucket != "hub-avatars" {
		t.Errorf("KeycloakAdminURL = %q, S3Bucket = %q", c.KeycloakAdminURL, c.S3Bucket)
	}
	if c.PublicURL.String() != "https://hub.example.test" || len(c.SessionKey) != 32 {
		t.Errorf("PublicURL = %v, key %d bytes", c.PublicURL, len(c.SessionKey))
	}
}

func TestRequired(t *testing.T) {
	_, err := load(env(nil))
	if err == nil {
		t.Fatal("empty environment accepted")
	}
	for _, name := range []string{"HUB_PUBLIC_URL", "HUB_DATABASE_URL", "HUB_OIDC_ISSUER", "HUB_OIDC_CLIENT_SECRET", "HUB_SESSION_KEY", "HUB_S3_ENDPOINT"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error doesn't mention %s: %v", name, err)
		}
	}
}

func TestInvalid(t *testing.T) {
	for name, value := range map[string]string{
		"HUB_ADDR":        "9000",
		"HUB_PUBLIC_URL":  "https://hub.example.test/sub",
		"HUB_SESSION_KEY": "c2hvcnQ=",
	} {
		m := valid()
		m[name] = value
		if _, err := load(env(m)); err == nil {
			t.Errorf("%s=%q accepted", name, value)
		}
	}
	m := valid()
	m["HUB_PUBLIC_URL"] = "ftp://hub.example.test"
	if _, err := load(env(m)); err == nil {
		t.Error("ftp public URL accepted")
	}
}
