// Package assets embeds the static files shipped inside the binary.
package assets

import _ "embed"

// LogoSVG is the application logo.
//
//go:embed logo.svg
var LogoSVG []byte
