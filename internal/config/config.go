// Package config reads the server's settings. They come from environment
// variables only, each documented in README.md.
package config

import (
	"fmt"
	"net"
	"os"
)

type Config struct {
	// Addr is where the HTTP server listens, host:port.
	Addr string
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	c := Config{Addr: ":8080"}
	if v, ok := lookup("HUB_ADDR"); ok && v != "" {
		c.Addr = v
	}
	if _, _, err := net.SplitHostPort(c.Addr); err != nil {
		return Config{}, fmt.Errorf("HUB_ADDR %q: %w", c.Addr, err)
	}
	return c, nil
}
