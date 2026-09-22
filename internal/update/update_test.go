package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const asset = "homeyctl-darwin-arm64"

// fakeGitHub serves a latest-release endpoint and one release's assets.
type fakeGitHub struct {
	latest    string
	binary    []byte
	checksums string // raw checksums.txt; empty means 404
	apiCalls  atomic.Int32
}

func (f *fakeGitHub) server(t *testing.T) (*httptest.Server, *Client) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+Repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		f.apiCalls.Add(1)
		fmt.Fprintf(w, `{"tag_name":%q}`, f.latest)
	})
	mux.HandleFunc("/"+Repo+"/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			if f.checksums == "" {
				http.NotFound(w, r)
				return
			}
			fmt.Fprint(w, f.checksums)
		case strings.HasSuffix(r.URL.Path, "/"+asset):
			_, _ = w.Write(f.binary)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &Client{HTTP: srv.Client(), APIBase: srv.URL, DownloadBase: srv.URL}
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestLatestTag(t *testing.T) {
	f := &fakeGitHub{latest: "v1.5.0"}
	_, c := f.server(t)
	tag, err := c.LatestTag(context.Background())
	if err != nil || tag != "v1.5.0" {
		t.Fatalf("LatestTag = %q, %v", tag, err)
	}
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	bin := []byte("new binary")
	f := &fakeGitHub{binary: bin, checksums: sha(bin) + "  " + asset + "\nabc  other-asset\n"}
	_, c := f.server(t)
	got, err := c.Download(context.Background(), "v1.5.0", asset)
	if err != nil || string(got) != string(bin) {
		t.Fatalf("Download = %q, %v", got, err)
	}
}

func TestDownloadRefusesMismatch(t *testing.T) {
	f := &fakeGitHub{binary: []byte("tampered"), checksums: sha([]byte("original")) + "  " + asset + "\n"}
	_, c := f.server(t)
	if _, err := c.Download(context.Background(), "v1.5.0", asset); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestDownloadRefusesMissingChecksums(t *testing.T) {
	f := &fakeGitHub{binary: []byte("bin")}
	_, c := f.server(t)
	if _, err := c.Download(context.Background(), "v1.5.0", asset); err == nil {
		t.Fatal("expected an error when checksums.txt is missing")
	}
}

func TestDownloadRefusesUnlistedAsset(t *testing.T) {
	f := &fakeGitHub{binary: []byte("bin"), checksums: "abc  something-else\n"}
	_, c := f.server(t)
	if _, err := c.Download(context.Background(), "v1.5.0", asset); err == nil || !strings.Contains(err.Error(), "not listed") {
		t.Fatalf("expected not-listed error, got %v", err)
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		candidate, current string
		want               bool
	}{
		{"v1.4.1", "1.3.7", true},
		{"v1.10.0", "v1.9.9", true},
		{"v2.0.0", "1.99.99", true},
		{"v1.4.1", "1.4.1", false},
		{"v1.4.0", "1.4.1", false},
		{"v1.4.1", "dev", false},
		{"v1.4.1", "dev-abc1234", false},
		{"", "1.4.1", false},
		{"v1.4.2", "1.4.1-next", true},
	}
	for _, tc := range cases {
		if got := Newer(tc.candidate, tc.current); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.candidate, tc.current, got, tc.want)
		}
	}
}

func TestInstallReplacesTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "homeyctl")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(target, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	info, _ := os.Stat(target)
	if string(got) != "new" || info.Mode().Perm() != 0o755 {
		t.Fatalf("target = %q mode %v", got, info.Mode().Perm())
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".homeyctl-upgrade-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

func TestCheckNotifiesOncePerInterval(t *testing.T) {
	f := &fakeGitHub{latest: "v1.5.0"}
	_, c := f.server(t)
	path := filepath.Join(t.TempDir(), "update-check.json")
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	notice := StartCheck(c, path, "1.4.1", now).Finish(2 * time.Second)
	if !strings.Contains(notice, "v1.5.0") || !strings.Contains(notice, "homeyctl upgrade") {
		t.Fatalf("first notice = %q", notice)
	}

	// An hour later: cached, so no lookup, and already notified today.
	if notice := StartCheck(c, path, "1.4.1", now.Add(time.Hour)).Finish(2 * time.Second); notice != "" {
		t.Fatalf("second notice within interval = %q", notice)
	}
	if n := f.apiCalls.Load(); n != 1 {
		t.Fatalf("GitHub called %d times, want 1 (cache ignored)", n)
	}

	// A day later it looks up again and reminds again.
	if notice := StartCheck(c, path, "1.4.1", now.Add(25*time.Hour)).Finish(2 * time.Second); notice == "" {
		t.Fatal("expected a reminder after the interval")
	}
	if n := f.apiCalls.Load(); n != 2 {
		t.Fatalf("GitHub called %d times, want 2", n)
	}
}

func TestCheckSilentWhenCurrent(t *testing.T) {
	f := &fakeGitHub{latest: "v1.4.1"}
	_, c := f.server(t)
	path := filepath.Join(t.TempDir(), "update-check.json")
	if notice := StartCheck(c, path, "1.4.1", time.Now()).Finish(2 * time.Second); notice != "" {
		t.Fatalf("notice when current = %q", notice)
	}
}

func TestCheckSilentWhenOffline(t *testing.T) {
	c := &Client{HTTP: &http.Client{Timeout: time.Second}, APIBase: "http://127.0.0.1:1", DownloadBase: "http://127.0.0.1:1"}
	path := filepath.Join(t.TempDir(), "update-check.json")
	now := time.Now()
	if notice := StartCheck(c, path, "1.4.1", now).Finish(3 * time.Second); notice != "" {
		t.Fatalf("notice when offline = %q", notice)
	}
	// The failed lookup is recorded, so the next command does not retry.
	if s := loadState(path); !s.CheckedAt.Equal(now) {
		t.Fatalf("failed check not recorded: %+v", s)
	}
}
