package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fishfisher/homeyctl/internal/client"
	"github.com/fishfisher/homeyctl/internal/config"
)

var (
	cfg       *config.Config
	apiClient *client.Client

	jsonFlag bool

	versionInfo struct {
		Version string
		Commit  string
		Date    string
	}
)

func SetVersionInfo(v, commit, date string) {
	versionInfo.Version = v
	versionInfo.Commit = commit
	versionInfo.Date = date
	if v != "" && v != "dev" {
		rootCmd.Version = v
	}
}

const setupInstructions = `
Welcome to homeyctl! To get started, run:

  homeyctl auth

This will guide you through authentication.

Or use a specific method:
  homeyctl auth login            Log in via browser (OAuth)
  homeyctl auth api-key <key>    Set API key from my.homey.app

After setup, try:
  homeyctl devices list
  homeyctl flows list

For more help: homeyctl --help
`

// version is set via ldflags at build/release time.
var version string

func buildVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			if len(s.Value) > 7 {
				return s.Value[:7]
			}
			return s.Value
		}
	}
	return "dev"
}

var rootCmd = &cobra.Command{
	Use:     "homeyctl",
	Short:   "CLI for Homey smart home",
	Long:    `A command-line interface for controlling Homey devices, flows, and more.`,
	Version: buildVersion(),
	// A runtime failure is not a usage mistake. Printing the whole flag block
	// after every network or validation error buries the message that matters,
	// especially for agents reading this output.
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if configured, show setup instructions if not
		loadedCfg, _ := config.Load()
		if !isConfigured(loadedCfg) {
			// Check for legacy config and show migration instructions
			config.CheckLegacyConfig()
			fmt.Print(setupInstructions)
			return
		}
		// If configured, show normal help
		cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.CommandPath() == "homeyctl flows validate" {
			online, _ := cmd.Flags().GetBool("online")
			if !online {
				return nil
			}
		}
		if cmd.CommandPath() == "homeyctl flows create" {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if dryRun {
				return nil
			}
		}
		if shouldSkipConfigLoading(cmd.CommandPath(), cmd.Name()) {
			return nil
		}

		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if !isConfigured(cfg) {
			return fmt.Errorf("no API token configured. Run: homeyctl auth")
		}
		if err := cfg.ValidateEndpoint(); err != nil {
			return err
		}

		apiClient = client.New(cfg)
		return nil
	},
}

func isConfigured(cfg *config.Config) bool {
	return cfg != nil && cfg.EffectiveToken() != ""
}

func shouldSkipConfigLoading(cmdPath, cmdName string) bool {
	return cmdName == "config" || cmdName == "version" || cmdName == "help" ||
		cmdName == "set-host" || cmdName == "show" ||
		cmdName == "completion" || cmdName == "install-skill" ||
		cmdName == "auth" || cmdName == "login" || cmdName == "api-key" ||
		cmdName == "status" || cmdName == "scopes" ||
		strings.HasPrefix(cmdPath, "homeyctl auth") ||
		strings.HasPrefix(cmdPath, "homeyctl config") ||
		cmdPath == "homeyctl"
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	rootCmd.Flags().BoolP("version", "v", false, "Print version")
}

// outputJSON pretty-prints JSON data
func outputJSON(data []byte) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err == nil {
		pretty, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(pretty))
		return
	}
	fmt.Println(string(data))
}

// isJSON returns true if JSON output is requested via --json flag
func isJSON() bool {
	return jsonFlag
}
