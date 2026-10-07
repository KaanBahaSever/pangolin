package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		current, latest string
		want            bool
	}{
		{"1.1.1", "1.2.0", true},
		{"1.1.1", "1.1.10", true},
		{"1.9.0", "2.0.0", true},
		{"1.1.1", "1.1.1", false},
		{"1.2.0", "1.1.9", false},
		{"v1.0.0", "v1.0.1", true},
		{"dev", "9.9.9", false},
		{"0.0.0-dev", "1.0.0", false},
		{"1.0.0", "garbage", false},
	} {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.current, c.latest, got)
		}
	}
}

func TestIsRelease(t *testing.T) {
	if !IsRelease("1.2.3") || IsRelease("dev") || IsRelease("0.0.0-dev") {
		t.Fatal("IsRelease misclassifies")
	}
}

func serve(t *testing.T, status int, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.UserAgent(), "Pangolin/") {
			t.Errorf("User-Agent = %q", r.UserAgent())
		}
		if r.Method != http.MethodGet || r.ContentLength > 0 {
			t.Errorf("unexpected %s with %d bytes of body", r.Method, r.ContentLength)
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestLatest(t *testing.T) {
	url := serve(t, http.StatusOK, `{"tag_name":"v1.4.2","html_url":"https://example.com/elsewhere"}`)
	got, err := Latest(context.Background(), url, "1.1.1")
	if err != nil || got != "1.4.2" {
		t.Fatalf("Latest = %q, %v", got, err)
	}
}

func TestLatestRejectsBadAnswers(t *testing.T) {
	for name, url := range map[string]string{
		"not found":   serve(t, http.StatusNotFound, `{"message":"Not Found"}`),
		"not json":    serve(t, http.StatusOK, `<html>`),
		"bad version": serve(t, http.StatusOK, `{"tag_name":"latest"}`),
		"no version":  serve(t, http.StatusOK, `{}`),
	} {
		if v, err := Latest(context.Background(), url, "1.0.0"); err == nil {
			t.Errorf("%s: accepted, version %q", name, v)
		}
	}
}

func TestLatestOffline(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens there any more
	if _, err := Latest(context.Background(), url, "1.0.0"); err == nil {
		t.Fatal("no error without a server")
	}
}
