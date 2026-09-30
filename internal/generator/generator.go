// Package generator creates random passwords from the operating system CSPRNG.
package generator

import (
	"crypto/rand"
	"errors"
	"math/big"
)

// Length limits for generated passwords.
const (
	MinLength     = 8
	MaxLength     = 128
	DefaultLength = 20
)

const (
	lower  = "abcdefghijklmnopqrstuvwxyz"
	upper  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits = "0123456789"
	// Symbols that are accepted by most sites and unambiguous to type.
	symbols = "!@#$%^&*()-_=+[]{};:,.?"
)

// Options selects the length and character classes of a password.
type Options struct {
	Length  int
	Lower   bool
	Upper   bool
	Digits  bool
	Symbols bool
}

// DefaultOptions returns a 20-character password using every class.
func DefaultOptions() Options {
	return Options{Length: DefaultLength, Lower: true, Upper: true, Digits: true, Symbols: true}
}

// Generate returns a random password that contains at least one character
// from every selected class.
func Generate(o Options) (string, error) {
	if o.Length < MinLength || o.Length > MaxLength {
		return "", errors.New("generator: length out of range")
	}
	var classes []string
	for _, c := range []struct {
		on  bool
		set string
	}{{o.Lower, lower}, {o.Upper, upper}, {o.Digits, digits}, {o.Symbols, symbols}} {
		if c.on {
			classes = append(classes, c.set)
		}
	}
	if len(classes) == 0 {
		return "", errors.New("generator: no character class selected")
	}

	all := ""
	out := make([]byte, 0, o.Length)
	for _, set := range classes {
		all += set
		out = append(out, set[randInt(len(set))])
	}
	for len(out) < o.Length {
		out = append(out, all[randInt(len(all))])
	}
	// Fisher–Yates, so the guaranteed characters are not always in front.
	for i := len(out) - 1; i > 0; i-- {
		j := randInt(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return string(out), nil
}

// randInt returns a uniform integer in [0, n). rand.Int uses rejection
// sampling, so there is no modulo bias.
func randInt(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic("generator: system random source failed: " + err.Error())
	}
	return int(v.Int64())
}
