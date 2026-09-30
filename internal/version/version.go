// Package version holds the build's version string.
package version

// Version is set at release time with
//
//	-ldflags "-X github.com/KaanBahaSever/pangolin/internal/version.Version=1.2.3"
//
// and is "dev" for a plain "go build".
var Version = "dev"
