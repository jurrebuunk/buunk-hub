package i18n

import "testing"

func TestPick(t *testing.T) {
	for _, tc := range []struct {
		locale, accept string
		want           Lang
	}{
		{"", "", EN},
		{"nl", "en-US", NL},
		{"en", "nl-NL", EN},
		{"de", "nl-BE,nl;q=0.9", NL},
		{"", "de-DE,nl;q=0.8,en;q=0.5", NL},
		{"", "en-GB,nl;q=0.8", EN},
		{"", "fr-FR", EN},
		{"", "garbage;;;", EN},
	} {
		if got := Pick(tc.locale, tc.accept); got != tc.want {
			t.Errorf("Pick(%q, %q) = %s, want %s", tc.locale, tc.accept, got, tc.want)
		}
	}
}
