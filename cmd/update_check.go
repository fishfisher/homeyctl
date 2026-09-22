package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/fishfisher/homeyctl/internal/config"
	"github.com/fishfisher/homeyctl/internal/update"
)

// updateCheckWait bounds how long a finished command waits for a version
// lookup still in flight. Most commands spend longer than this on the Homey
// API, so the lookup is normally done already; when it is not, it is dropped
// and retried by the next command rather than slowing this one down.
const updateCheckWait = 400 * time.Millisecond

var pendingUpdate *update.Pending

func startUpdateCheck(cmd *cobra.Command) {
	pendingUpdate = nil
	if !updateCheckEnabled(cmd) {
		return
	}
	path, err := update.StatePath()
	if err != nil {
		return
	}
	pendingUpdate = update.StartCheck(newUpdateClient(), path, versionInfo.Version, time.Now())
}

func finishUpdateCheck() {
	if pendingUpdate == nil {
		return
	}
	if notice := pendingUpdate.Finish(updateCheckWait); notice != "" {
		// stderr, so it never mixes into --json output on stdout.
		fmt.Fprintln(os.Stderr, notice)
	}
	pendingUpdate = nil
}

func updateCheckEnabled(cmd *cobra.Command) bool {
	if !update.IsRelease(versionInfo.Version) {
		return false // dev builds have nothing to compare against
	}
	if os.Getenv("HOMEYCTL_NO_UPDATE_CHECK") != "" || os.Getenv("CI") != "" {
		return false
	}
	switch cmd.Name() {
	case "upgrade", "version", "completion", "help", "__complete", "__completeNoDesc", "set-update-check":
		return false
	}
	if strings.HasPrefix(cmd.CommandPath(), "homeyctl completion") {
		return false
	}
	loaded, err := config.Load()
	if err == nil && loaded.NoUpdateCheck {
		return false
	}
	return true
}
