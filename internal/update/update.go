// Package update resolves, downloads, verifies, and installs homeyctl releases
// from GitHub Releases. It mirrors install.sh so that the binary can upgrade
// itself without Homebrew or a copy of the script.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	Repo    = "fishfisher/homeyctl"
	binName = "homeyctl"
)

// Client talks to GitHub. The base URLs are fields so tests can point them at
// an httptest server.
type Client struct {
	HTTP         *http.Client
	APIBase      string // https://api.github.com
	DownloadBase string // https://github.com
}

func NewClient() *Client {
	return &Client{
		HTTP:         &http.Client{Timeout: 60 * time.Second},
		APIBase:      "https://api.github.com",
		DownloadBase: "https://github.com",
	}
}

// LatestTag returns the tag of the latest published release, e.g. "v1.4.1".
func (c *Client) LatestTag(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.APIBase, Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub returned %s for the latest release", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("could not parse the latest release: %w", err)
	}
	if body.TagName == "" {
		return "", errors.New("the latest release has no tag")
	}
	return body.TagName, nil
}

// AssetName is the release asset for the running platform. Releases ship
// macOS builds only, matching install.sh.
func AssetName() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("homeyctl releases ship macOS builds only (running on %s)", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "arm64", "amd64":
		return fmt.Sprintf("%s-darwin-%s", binName, runtime.GOARCH), nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

// Download fetches asset for tag and verifies it against the release's
// checksums.txt. Verification is mandatory: a release without a checksum for
// the asset is refused rather than installed unverified.
func (c *Client) Download(ctx context.Context, tag, asset string) ([]byte, error) {
	base := fmt.Sprintf("%s/%s/releases/download/%s", c.DownloadBase, Repo, tag)

	sums, err := c.get(ctx, base+"/checksums.txt", 1<<20)
	if err != nil {
		return nil, fmt.Errorf("could not fetch checksums.txt for %s: %w", tag, err)
	}
	expected, err := checksumFor(sums, asset)
	if err != nil {
		return nil, err
	}

	data, err := c.get(ctx, base+"/"+asset, 200<<20)
	if err != nil {
		return nil, fmt.Errorf("could not download %s: %w", asset, err)
	}
	sum := sha256.Sum256(data)
	if actual := hex.EncodeToString(sum[:]); actual != expected {
		return nil, fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset, expected, actual)
	}
	return data, nil
}

func (c *Client) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func checksumFor(sums []byte, asset string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(sums)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("%s is not listed in checksums.txt; refusing to install an unverified binary", asset)
}

// Install atomically replaces target with data. The new file is written next
// to the target and renamed over it, so an interrupted upgrade never leaves a
// half-written binary behind.
func Install(target string, data []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".homeyctl-upgrade-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s (%w); reinstall with install.sh or set HOMEYCTL_BIN_DIR", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpName, target)
}

// Executable returns the resolved path of the running binary.
func Executable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

// Newer reports whether candidate is a strictly higher version than current.
// Both may carry a leading "v". A current version that is not a release
// (e.g. "dev" or a commit hash) is never considered outdated.
func Newer(candidate, current string) bool {
	c, ok1 := parse(candidate)
	cur, ok2 := parse(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range c {
		if c[i] != cur[i] {
			return c[i] > cur[i]
		}
	}
	return false
}

// IsRelease reports whether v looks like a released version.
func IsRelease(v string) bool {
	_, ok := parse(v)
	return ok
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	// Pre-release and build suffixes are ignored for ordering; releases here
	// are plain MAJOR.MINOR.PATCH.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// NormalizeTag ensures a leading "v", so both "1.4.1" and "v1.4.1" work.
func NormalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag != "" && !strings.HasPrefix(tag, "v") {
		return "v" + tag
	}
	return tag
}
