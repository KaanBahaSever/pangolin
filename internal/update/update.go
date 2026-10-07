// Package update asks GitHub for the latest Pangolin release. It is the only
// code in Pangolin that touches the network, and it runs only when the user
// presses "Check" in the settings.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// LatestAPI is the GitHub endpoint describing the newest release.
	LatestAPI = "https://api.github.com/repos/KaanBahaSever/pangolin/releases/latest"
	// DownloadPage is where the user is sent to get a new version. It is a
	// constant: nothing from the network decides what the browser opens.
	DownloadPage = "https://github.com/KaanBahaSever/pangolin/releases/latest"

	timeout = 15 * time.Second
	// A release description is a few kilobytes; refuse anything much larger.
	maxResponse = 1 << 20
)

// Latest returns the version of the newest release, such as "1.2.3", from
// the GitHub API at url (normally LatestAPI). The request carries nothing
// but the running version in the User-Agent header.
func Latest(ctx context.Context, url, current string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Pangolin/"+current)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update: GitHub answered %s", resp.Status)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponse)).Decode(&release); err != nil {
		return "", fmt.Errorf("update: unreadable answer: %w", err)
	}
	version := strings.TrimPrefix(release.TagName, "v")
	if _, err := parse(version); err != nil {
		return "", err
	}
	return version, nil
}

// Newer reports whether latest is a later version than current. A current
// version that is not a release number (a development build) is never
// considered out of date.
func Newer(current, latest string) bool {
	c, err := parse(current)
	if err != nil {
		return false
	}
	l, err := parse(latest)
	if err != nil {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// IsRelease reports whether v is a release number rather than a
// development build.
func IsRelease(v string) bool {
	_, err := parse(v)
	return err == nil
}

// parse reads "1.2.3" (an optional leading "v" is allowed).
func parse(version string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 {
		return out, errors.New("update: not a version number: " + version)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, errors.New("update: not a version number: " + version)
		}
		out[i] = n
	}
	return out, nil
}
