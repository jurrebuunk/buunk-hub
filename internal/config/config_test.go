package config

import "testing"

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestDefaults(t *testing.T) {
	c, err := load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", c.Addr)
	}
}

func TestAddr(t *testing.T) {
	c, err := load(env(map[string]string{"HUB_ADDR": "127.0.0.1:9000"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != "127.0.0.1:9000" {
		t.Errorf("Addr = %q", c.Addr)
	}
	if _, err := load(env(map[string]string{"HUB_ADDR": "9000"})); err == nil {
		t.Error("HUB_ADDR without a port was accepted")
	}
}
