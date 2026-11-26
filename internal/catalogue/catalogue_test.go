package catalogue

import (
	"os"
	"strings"
	"testing"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
)

// The real catalogue must always load.
func TestDeployCatalogue(t *testing.T) {
	apps, err := Load("../../deploy/catalogue.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) == 0 {
		t.Fatal("no apps")
	}
}

const good = `
apps:
  - id: files
    name: Files
    url: https://files.example.test
    icon: cloud
    description: { en: Files., nl: Bestanden. }
    roles: [files/user, files/admin]
  - id: lab
    name: Lab
    url: https://lab.example.test
    icon: terminal
    description: { en: Lab., nl: Lab. }
    roles: [warp/warp]
    internal: true
`

func TestVisibleTo(t *testing.T) {
	apps, err := Parse([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		roles auth.Roles
		want  [2]bool
	}{
		{nil, [2]bool{false, false}},
		{auth.Roles{"files": {"admin"}}, [2]bool{true, false}},
		{auth.Roles{"files": {"user"}, "warp": {"warp"}}, [2]bool{true, true}},
		// A role of the same name in another client doesn't count.
		{auth.Roles{"other": {"user", "warp"}}, [2]bool{false, false}},
	} {
		got := [2]bool{apps[0].VisibleTo(tc.roles), apps[1].VisibleTo(tc.roles)}
		if got != tc.want {
			t.Errorf("roles %v: visible %v, want %v", tc.roles, got, tc.want)
		}
	}
}

func TestParseRefuses(t *testing.T) {
	for name, edit := range map[string][2]string{
		"unknown field": {"icon: cloud", "icon: cloud\n    colour: red"},
		"http url":      {"https://files", "http://files"},
		"no dutch":      {"{ en: Files., nl: Bestanden. }", "{ en: Files. }"},
		"no roles":      {"[files/user, files/admin]", "[]"},
		"bad role":      {"[files/user, files/admin]", "[files]"},
		"duplicate id":  {"id: lab", "id: files"},
		"bad icon":      {"icon: terminal", "icon: <b>"},
	} {
		y := strings.Replace(good, edit[0], edit[1], 1)
		if _, err := Parse([]byte(y)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Load(os.DevNull + "-missing"); err == nil {
		t.Error("missing file accepted")
	}
}
