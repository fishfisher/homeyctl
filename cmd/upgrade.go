package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fishfisher/homeyctl/internal/update"
)

// newUpdateClient is a variable so tests can point it at a fake GitHub.
var newUpdateClient = update.NewClient

// executablePath is a variable so tests can upgrade a scratch file instead of
// the test binary.
var executablePath = update.Executable

// assetName is a variable so tests run on CI's Linux runners, where the
// real resolver refuses because releases are macOS-only.
var assetName = update.AssetName

// errStale makes `upgrade --check` exit non-zero when behind, without the
// root command printing an error for it.
type errStale struct{}

func (errStale) Error() string { return "a newer version is available" }

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade homeyctl to the latest GitHub release",
	Long: `Upgrade homeyctl in place from GitHub Releases.

The release asset is verified against the release's checksums.txt before it
replaces the running binary; a missing or mismatching checksum aborts the
upgrade. The replacement is atomic, so an interrupted upgrade leaves the old
binary intact.

After upgrading, refresh the bundled AI skill with:
  homeyctl install-skill --force

Examples:
  homeyctl upgrade                   # Install the latest release
  homeyctl upgrade --check           # Report only; exits 1 when behind
  homeyctl upgrade --tag v1.4.0      # Install a specific release`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		check, _ := cmd.Flags().GetBool("check")
		pinned, _ := cmd.Flags().GetString("tag")

		parent := cmd.Context()
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
		defer cancel()
		client := newUpdateClient()

		current := versionInfo.Version
		target := update.NormalizeTag(pinned)
		if target == "" {
			latest, err := client.LatestTag(ctx)
			if err != nil {
				return err
			}
			target = latest
		}
		upToDate := update.IsRelease(current) && !update.Newer(target, current) && pinned == ""

		if check {
			stale := update.Newer(target, current)
			if isJSON() {
				out, _ := json.MarshalIndent(map[string]any{
					"current":         current,
					"latest":          target,
					"updateAvailable": stale,
				}, "", "  ")
				fmt.Println(string(out))
			} else if stale {
				fmt.Printf("homeyctl %s is available (you have %s). Run: homeyctl upgrade\n", target, update.NormalizeTag(current))
			} else {
				fmt.Printf("homeyctl %s is up to date.\n", update.NormalizeTag(current))
			}
			if stale {
				cmd.SilenceErrors = true
				return errStale{}
			}
			return nil
		}

		if upToDate {
			if isJSON() {
				out, _ := json.MarshalIndent(map[string]any{"current": current, "latest": target, "upgraded": false}, "", "  ")
				fmt.Println(string(out))
				return nil
			}
			fmt.Printf("homeyctl %s is already the latest release.\n", update.NormalizeTag(current))
			return nil
		}

		asset, err := assetName()
		if err != nil {
			return err
		}
		exe, err := executablePath()
		if err != nil {
			return fmt.Errorf("cannot locate the running binary: %w", err)
		}

		if !isJSON() {
			fmt.Fprintf(os.Stderr, "Downloading homeyctl %s (%s)...\n", target, asset)
		}
		data, err := client.Download(ctx, target, asset)
		if err != nil {
			return err
		}
		if err := update.Install(exe, data); err != nil {
			return err
		}

		if isJSON() {
			out, _ := json.MarshalIndent(map[string]any{
				"previous": current, "installed": target, "path": exe, "upgraded": true,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}
		color.Green("Upgraded homeyctl %s → %s (%s)\n", update.NormalizeTag(current), target, exe)
		fmt.Println("Checksum verified. Refresh the AI skill with: homeyctl install-skill --force")
		return nil
	},
}

func init() {
	upgradeCmd.Flags().Bool("check", false, "Only report whether a newer release exists (exit 1 when behind)")
	upgradeCmd.Flags().String("tag", "", "Install this release tag instead of the latest, e.g. v1.4.0")
	rootCmd.AddCommand(upgradeCmd)
}
