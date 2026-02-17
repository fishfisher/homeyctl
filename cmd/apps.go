package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

type App struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
	Ready   bool   `json:"ready"`
}

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "Manage apps",
	Long:  `List, view, and restart Homey apps.`,
}

// findApp finds an app by name or ID from the list of all apps
func findApp(nameOrID string) (*App, error) {
	data, err := apiClient.GetApps()
	if err != nil {
		return nil, err
	}

	var apps map[string]App
	if err := json.Unmarshal(data, &apps); err != nil {
		return nil, fmt.Errorf("failed to parse apps: %w", err)
	}

	for _, a := range apps {
		if a.ID == nameOrID || strings.EqualFold(a.Name, nameOrID) {
			return &a, nil
		}
	}

	return nil, fmt.Errorf("app not found: %s", nameOrID)
}

var appsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := apiClient.GetApps()
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var apps map[string]App
		if err := json.Unmarshal(data, &apps); err != nil {
			return fmt.Errorf("failed to parse apps: %w", err)
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Name", "Version", "Enabled", "Ready", "ID")
		tbl.WithHeaderFormatter(headerFmt)
		for _, a := range apps {
			enabled := "yes"
			if !a.Enabled {
				enabled = "no"
			}
			ready := "yes"
			if !a.Ready {
				ready = "no"
			}
			tbl.AddRow(a.Name, a.Version, enabled, ready, a.ID)
		}
		tbl.Print()
		return nil
	},
}

var appsGetCmd = &cobra.Command{
	Use:   "get <name-or-id>",
	Short: "Get app details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]

		data, err := apiClient.GetApps()
		if err != nil {
			return err
		}

		var apps map[string]App
		if err := json.Unmarshal(data, &apps); err != nil {
			return fmt.Errorf("failed to parse apps: %w", err)
		}

		// Find app by name or ID
		var appID string
		for _, a := range apps {
			if a.ID == nameOrID || strings.EqualFold(a.Name, nameOrID) {
				appID = a.ID
				break
			}
		}

		if appID == "" {
			return fmt.Errorf("app not found: %s", nameOrID)
		}

		appData, err := apiClient.GetApp(appID)
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(appData)
			return nil
		}

		// App get shows full JSON detail by default (complex nested data)
		outputJSON(appData)
		return nil
	},
}

var appsRestartCmd = &cobra.Command{
	Use:   "restart <name-or-id>",
	Short: "Restart an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]

		data, err := apiClient.GetApps()
		if err != nil {
			return err
		}

		var apps map[string]App
		if err := json.Unmarshal(data, &apps); err != nil {
			return fmt.Errorf("failed to parse apps: %w", err)
		}

		// Find app by name or ID
		var app *App
		for _, a := range apps {
			if a.ID == nameOrID || strings.EqualFold(a.Name, nameOrID) {
				app = &a
				break
			}
		}

		if app == nil {
			return fmt.Errorf("app not found: %s", nameOrID)
		}

		if err := apiClient.RestartApp(app.ID); err != nil {
			return err
		}

		color.Green("Restarted app: %s\n", app.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(appsCmd)
	appsCmd.AddCommand(appsListCmd)
	appsCmd.AddCommand(appsGetCmd)
	appsCmd.AddCommand(appsRestartCmd)
}
