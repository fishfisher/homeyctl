package cmd

import (
	"strings"
	"testing"
)

func TestCommandSkipsConfigLoading(t *testing.T) {
	// Commands that should skip config loading (don't need API client)
	skipCommands := []struct {
		name       string
		cmdPath    string
		cmdName    string
		shouldSkip bool
	}{
		// Commands that SHOULD skip config loading
		{"config command", "homeyctl config", "config", true},
		{"config set-host", "homeyctl config set-host", "set-host", true},
		{"config show", "homeyctl config show", "show", true},
		{"version command", "homeyctl version", "version", true},
		{"help command", "homeyctl help", "help", true},
		{"completion command", "homeyctl completion", "completion", true},
		{"install-skill command", "homeyctl install-skill", "install-skill", true},
		{"root command", "homeyctl", "homeyctl", true},

		// Auth commands that should skip config loading
		{"auth command", "homeyctl auth", "auth", true},
		{"auth login", "homeyctl auth login", "login", true},
		{"auth api-key", "homeyctl auth api-key", "api-key", true},
		{"auth status", "homeyctl auth status", "status", true},
		{"auth token", "homeyctl auth token", "token", true},
		{"auth token create", "homeyctl auth token create", "create", true},
		{"auth token scopes", "homeyctl auth token scopes", "scopes", true},
		{"auth token list", "homeyctl auth token list", "list", true},
		{"auth token delete", "homeyctl auth token delete", "delete", true},

		// Commands that should NOT skip config loading (need API client)
		// This is the key fix for GitHub issues #4 and #5
		{"flows create command", "homeyctl flows create", "create", false},
		{"flows create --advanced", "homeyctl flows create", "create", false},
		{"flows list command", "homeyctl flows list", "list", false},
		{"flows get command", "homeyctl flows get", "get", false},
		{"flows update command", "homeyctl flows update", "update", false},
		{"flows delete command", "homeyctl flows delete", "delete", false},
		{"flows trigger command", "homeyctl flows trigger", "trigger", false},
		{"flows cards command", "homeyctl flows cards", "cards", false},
		{"devices list command", "homeyctl devices list", "list", false},
		{"devices get command", "homeyctl devices get", "get", false},
		{"devices set command", "homeyctl devices set", "set", false},
		{"zones list command", "homeyctl zones list", "list", false},
	}

	for _, tc := range skipCommands {
		t.Run(tc.name, func(t *testing.T) {
			shouldSkip := shouldSkipConfigLoading(tc.cmdPath, tc.cmdName)
			if shouldSkip != tc.shouldSkip {
				t.Errorf("shouldSkipConfigLoading(%q, %q) = %v, want %v",
					tc.cmdPath, tc.cmdName, shouldSkip, tc.shouldSkip)
			}
		})
	}
}

// TestFlowsCreateRequiresClient specifically tests the fix for GitHub issues #4 and #5
// The bug was that cmd.Name() == "create" matched both "auth token create" and "flows create",
// causing apiClient to be nil for flows create, resulting in a segmentation fault.
func TestFlowsCreateRequiresClient(t *testing.T) {
	// These two commands both have name "create" but only auth token create should skip
	tokenCreate := shouldSkipConfigLoading("homeyctl auth token create", "create")
	flowsCreate := shouldSkipConfigLoading("homeyctl flows create", "create")

	if !tokenCreate {
		t.Error("auth token create should skip config loading (handles its own OAuth)")
	}
	if flowsCreate {
		t.Error("flows create should NOT skip config loading (needs apiClient)")
	}
}

// shouldSkipConfigLoading mirrors the logic in PersistentPreRunE
// This allows us to test the logic without executing actual commands
func shouldSkipConfigLoading(cmdPath, cmdName string) bool {
	if cmdName == "config" || cmdName == "version" || cmdName == "help" ||
		cmdName == "set-host" || cmdName == "show" ||
		cmdName == "completion" || cmdName == "install-skill" ||
		cmdName == "auth" || cmdName == "login" || cmdName == "api-key" ||
		cmdName == "status" || cmdName == "scopes" ||
		strings.HasPrefix(cmdPath, "homeyctl auth") ||
		cmdPath == "homeyctl" {
		return true
	}

	return false
}
