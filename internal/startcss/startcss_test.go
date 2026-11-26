package startcss

import (
	"errors"
	"strings"
	"testing"
)

func TestClean(t *testing.T) {
	keep := ".start {\n  --fw-bg: #121a17;\n  --fw-r-lg: 20px;\n}\n.start-clock { letter-spacing: -0.06em; }"
	if got, err := Clean("  " + keep + "\n"); err != nil || got != keep {
		t.Errorf("plain style changed: %q %v", got, err)
	}
	for _, bad := range []string{
		`.start{background:url(https://evil.test/x.png)}`,
		`.start{background:URL( "https://evil.test/x.png" )}`,
		`.start{background:u\72 l(https://evil.test/x.png)}`,
		`.start{background:image-set("https://evil.test/x.png" 1x)}`,
		`.start{background:-webkit-image-set("https://evil.test/x.png" 1x)}`,
		`@import "https://evil.test/a.css"; .start{}`,
		`@IMPORT url(https://evil.test/a.css) screen;`,
		`@font-face{font-family:x;src:url(https://evil.test/f.woff2)} .start{font-family:x}`,
		`.start{} </style><script>alert(1)</script>`,
	} {
		got, err := Clean(bad)
		if err != nil {
			t.Fatal(err)
		}
		low := strings.ToLower(got)
		for _, w := range []string{"url(", "image-set(", "<", "@import", "@font-face", "\\"} {
			if strings.Contains(low, w) {
				t.Errorf("Clean(%q) = %q: still has %q", bad, got, w)
			}
		}
	}
	if _, err := Clean(strings.Repeat("a", MaxSize+1)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("too large: %v", err)
	}
}
