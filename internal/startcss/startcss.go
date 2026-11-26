// Package startcss cleans the style a person writes for their start page.
// The page's Content-Security-Policy is the real guard (it loads nothing
// from other sites); this keeps the saved text honest about that, and makes
// sure it can't end the <style> element it is put in.
package startcss

import (
	"errors"
	"regexp"
	"strings"
)

// MaxSize is the most a person's style may be, in bytes.
const MaxSize = 10 << 10

var ErrTooLarge = errors.New("the style is larger than 10 KB")

var (
	// @import and @namespace statements, to the next ; (or the end).
	atImport = regexp.MustCompile(`(?is)@(import|namespace)\b[^;]*;?`)
	// @font-face blocks: fonts from other sites aren't allowed, and our own
	// fonts aren't served to it.
	fontFace = regexp.MustCompile(`(?is)@font-face\s*\{[^}]*\}`)
	// Anything that loads an address: url(), image-set(), image(),
	// cross-fade(), src() and their -webkit- forms, up to the closing
	// parenthesis (strings in them can't hold one unescaped, and escapes
	// are gone by then).
	loader = regexp.MustCompile(`(?is)(-webkit-)?\b(url|image-set|image|cross-fade|src)\s*\([^)]*\)?`)
)

// Clean returns css with what isn't allowed removed: <, backslash escapes
// (which could spell url( another way), @import, @namespace, @font-face and
// every function that loads an address.
func Clean(css string) (string, error) {
	if len(css) > MaxSize {
		return "", ErrTooLarge
	}
	css = strings.NewReplacer("<", "", "\\", "", "\x00", "").Replace(css)
	css = atImport.ReplaceAllString(css, "")
	css = fontFace.ReplaceAllString(css, "")
	css = loader.ReplaceAllString(css, "none")
	return strings.TrimSpace(css), nil
}
