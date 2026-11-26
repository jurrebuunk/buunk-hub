// Package catalogue is the list of apps on Home: shared, defined by the
// owner in Git (deploy/catalogue.yaml), each shown to whoever has one of
// its client roles.
package catalogue

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/i18n"
)

type App struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	// Icon is a Material Symbols name, like "cloud".
	Icon string `yaml:"icon"`
	// Description in every language: en and nl.
	Description map[i18n.Lang]string `yaml:"description"`
	// Roles as "client/role"; any one of them shows the app.
	Roles []string `yaml:"roles"`
	// Internal apps are only reachable over WARP.
	Internal bool `yaml:"internal"`
}

type file struct {
	Apps []App `yaml:"apps"`
}

var (
	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	iconPattern = regexp.MustCompile(`^[a-z0-9_]+$`)
)

// Load reads and checks the catalogue. A mistake in it stops the hub from
// starting, so it shows up in the rollout instead of as a missing app.
func Load(path string) ([]App, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

func Parse(b []byte) ([]App, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var f file
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("catalogue: %w", err)
	}
	var errs []error
	seen := map[string]bool{}
	for i, a := range f.Apps {
		bad := func(format string, args ...any) {
			errs = append(errs, fmt.Errorf("catalogue: app %d (%s): %s", i+1, a.ID, fmt.Sprintf(format, args...)))
		}
		if !idPattern.MatchString(a.ID) || seen[a.ID] {
			bad("id must be unique, lowercase letters, digits and -")
		}
		seen[a.ID] = true
		if a.Name == "" {
			bad("no name")
		}
		if u, err := url.Parse(a.URL); err != nil || u.Scheme != "https" || u.Host == "" {
			bad("url must be https://…")
		}
		if !iconPattern.MatchString(a.Icon) {
			bad("icon must be a Material Symbols name")
		}
		for _, l := range []i18n.Lang{i18n.EN, i18n.NL} {
			if a.Description[l] == "" {
				bad("no description in %s", l)
			}
		}
		if len(a.Roles) == 0 {
			bad("no roles: nobody would see it")
		}
		for _, r := range a.Roles {
			if client, role, ok := strings.Cut(r, "/"); !ok || client == "" || role == "" {
				bad("role %q must be client/role", r)
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return f.Apps, nil
}

// VisibleTo says whether someone with roles may see the app.
func (a App) VisibleTo(roles auth.Roles) bool {
	for _, r := range a.Roles {
		client, role, _ := strings.Cut(r, "/")
		for _, have := range roles[client] {
			if have == role {
				return true
			}
		}
	}
	return false
}
