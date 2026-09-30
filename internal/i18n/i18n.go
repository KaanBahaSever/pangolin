// Package i18n translates the interface. English source strings are the keys,
// so the code reads naturally and an untranslated string falls back to English.
package i18n

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Lang is a supported interface language.
type Lang string

// Supported languages.
const (
	English Lang = "en"
	Turkish Lang = "tr"
)

// Languages lists the supported languages in display order.
var Languages = []Lang{English, Turkish}

// Name returns the language's own name, for the language selector.
func (l Lang) Name() string {
	if l == Turkish {
		return "Türkçe"
	}
	return "English"
}

var turkish atomic.Bool

// Set selects the interface language. Unknown values select English.
func Set(l Lang) {
	turkish.Store(l == Turkish)
}

// Current returns the selected language.
func Current() Lang {
	if turkish.Load() {
		return Turkish
	}
	return English
}

// FromLocale picks the language for a locale such as "tr-TR" or "en_US".
func FromLocale(locale string) Lang {
	if strings.HasPrefix(strings.ToLower(locale), "tr") {
		return Turkish
	}
	return English
}

// T translates an English source string into the current language.
func T(s string) string {
	if turkish.Load() {
		if t, ok := tr[s]; ok {
			return t
		}
	}
	return s
}

// Tf translates a format string and formats it.
func Tf(format string, args ...any) string {
	return fmt.Sprintf(T(format), args...)
}
