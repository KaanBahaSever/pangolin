package assets

import (
	"bytes"
	"os"
	"testing"
)

// The website serves its own copy of the logo, because GitHub Pages only
// publishes docs/. The two must not drift apart.
func TestWebsiteLogoMatches(t *testing.T) {
	site, err := os.ReadFile("../docs/logo.svg")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(site, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(LogoSVG, []byte("\r\n"), []byte("\n"))) {
		t.Fatal("docs/logo.svg differs from assets/logo.svg; copy the asset over")
	}
}
