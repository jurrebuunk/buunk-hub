// Package i18n picks the language for a person: English or Dutch.
package i18n

import (
	"strings"

	"golang.org/x/text/language"
)

type Lang string

const (
	EN Lang = "en"
	NL Lang = "nl"
)

var matcher = language.NewMatcher([]language.Tag{language.English, language.Dutch})

// Pick chooses from the token's locale claim if it names a language we
// have, else from the Accept-Language header, else English.
func Pick(locale, acceptLanguage string) Lang {
	if l, ok := fromLocale(locale); ok {
		return l
	}
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil || len(tags) == 0 {
		return EN
	}
	_, i, conf := matcher.Match(tags...)
	if conf == language.No || i != 1 {
		return EN
	}
	return NL
}

func fromLocale(locale string) (Lang, bool) {
	switch strings.ToLower(strings.SplitN(strings.ReplaceAll(locale, "_", "-"), "-", 2)[0]) {
	case "en":
		return EN, true
	case "nl":
		return NL, true
	}
	return "", false
}
