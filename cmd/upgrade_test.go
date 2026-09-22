package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fishfisher/homeyctl/internal/update"
)

// testUpgrade points upgrade at a fake GitHub serving latest=v1.5.0 and a
// scratch "running binary". It returns the binary's path.
func testUpgrade(t *testing.T, current string, checksumsOK bool) string {
	t.Helper()
	const asset = "homeyctl-test-asset"
	newBinary := []byte("homeyctl v1.5.0")
	sum := sha256.Sum256(newBinary)
	if !checksumsOK {
		sum = sha256.Sum256([]byte("something else"))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+update.Repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.5.0"}`)
	})
	mux.HandleFunc("/"+update.Repo+"/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), asset)
		case strings.HasSuffix(r.URL.Path, "/"+asset):
			_, _ = w.Write(newBinary)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	exe := filepath.Join(t.TempDir(), "homeyctl")
	if err := os.WriteFile(exe, []byte("homeyctl old"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldClient, oldExe, oldAsset, oldVersion := newUpdateClient, executablePath, assetName, versionInfo.Version
	newUpdateClient = func() *update.Client {
		return &update.Client{HTTP: srv.Client(), APIBase: srv.URL, DownloadBase: srv.URL}
	}
	executablePath = func() (string, error) { return exe, nil }
	assetName = func() (string, error) { return asset, nil }
	versionInfo.Version = current
	t.Cleanup(func() {
		newUpdateClient, executablePath, assetName, versionInfo.Version = oldClient, oldExe, oldAsset, oldVersion
		for _, f := range []string{"check", "tag"} {
			_ = upgradeCmd.Flags().Set(f, upgradeCmd.Flags().Lookup(f).DefValue)
		}
	})
	return exe
}

func TestUpgradeInstallsVerifiedLatest(t *testing.T) {
	exe := testUpgrade(t, "1.4.1", true)
	if err := upgradeCmd.RunE(upgradeCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "homeyctl v1.5.0" {
		t.Fatalf("binary not replaced: %q", got)
	}
}

func TestUpgradeChecksumMismatchKeepsOldBinary(t *testing.T) {
	exe := testUpgrade(t, "1.4.1", false)
	err := upgradeCmd.RunE(upgradeCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "homeyctl old" {
		t.Fatalf("binary changed despite failed verification: %q", got)
	}
}

func TestUpgradeNoopWhenCurrent(t *testing.T) {
	exe := testUpgrade(t, "1.5.0", true)
	if err := upgradeCmd.RunE(upgradeCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "homeyctl old" {
		t.Fatalf("binary replaced although current: %q", got)
	}
}

func TestUpgradeCheckReportsStaleWithoutInstalling(t *testing.T) {
	exe := testUpgrade(t, "1.4.1", true)
	_ = upgradeCmd.Flags().Set("check", "true")
	err := upgradeCmd.RunE(upgradeCmd, nil)
	if !errors.As(err, &errStale{}) {
		t.Fatalf("--check when stale should return errStale, got %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "homeyctl old" {
		t.Fatalf("--check installed something: %q", got)
	}
}

func TestUpgradeCheckPassesWhenCurrent(t *testing.T) {
	testUpgrade(t, "1.5.0", true)
	_ = upgradeCmd.Flags().Set("check", "true")
	if err := upgradeCmd.RunE(upgradeCmd, nil); err != nil {
		t.Fatalf("--check when current: %v", err)
	}
}

func TestUpgradePinnedTagReinstallsEvenWhenCurrent(t *testing.T) {
	exe := testUpgrade(t, "1.5.0", true)
	_ = upgradeCmd.Flags().Set("tag", "1.5.0") // no leading v on purpose
	if err := upgradeCmd.RunE(upgradeCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "homeyctl v1.5.0" {
		t.Fatalf("pinned tag not installed: %q", got)
	}
}
