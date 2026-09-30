package generator

import (
	"strings"
	"testing"
)

func TestGenerateContainsEverySelectedClass(t *testing.T) {
	for i := 0; i < 200; i++ {
		o := Options{Length: MinLength, Lower: true, Upper: true, Digits: true, Symbols: true}
		p, err := Generate(o)
		if err != nil {
			t.Fatal(err)
		}
		if len(p) != o.Length {
			t.Fatalf("length %d", len(p))
		}
		for _, set := range []string{lower, upper, digits, symbols} {
			if !strings.ContainsAny(p, set) {
				t.Fatalf("%q has no character from %q", p, set)
			}
		}
	}
}

func TestGenerateRespectsDisabledClasses(t *testing.T) {
	for i := 0; i < 100; i++ {
		p, err := Generate(Options{Length: 40, Digits: true})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Trim(p, digits) != "" {
			t.Fatalf("%q contains non-digits", p)
		}
	}
}

func TestGenerateRejectsInvalidOptions(t *testing.T) {
	for _, o := range []Options{
		{Length: MinLength - 1, Lower: true},
		{Length: MaxLength + 1, Lower: true},
		{Length: 20},
	} {
		if _, err := Generate(o); err == nil {
			t.Errorf("%+v was accepted", o)
		}
	}
}

func TestGenerateIsNotRepeating(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		p, _ := Generate(DefaultOptions())
		if seen[p] {
			t.Fatalf("duplicate password %q", p)
		}
		seen[p] = true
	}
}

// Every character of the alphabet should turn up somewhere in a large sample.
func TestGenerateCoversAlphabet(t *testing.T) {
	var sample strings.Builder
	for i := 0; i < 300; i++ {
		p, _ := Generate(Options{Length: MaxLength, Lower: true, Upper: true, Digits: true, Symbols: true})
		sample.WriteString(p)
	}
	for _, c := range lower + upper + digits + symbols {
		if !strings.ContainsRune(sample.String(), c) {
			t.Errorf("character %q never generated", c)
		}
	}
}
