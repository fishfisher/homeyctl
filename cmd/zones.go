package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

// Zone represents a Homey zone
type Zone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Parent string `json:"parent"`
	Icon   string `json:"icon"`
}

// KnownZoneIcons contains all known zone icons available in Homey
var KnownZoneIcons = []string{
	"home",
	"livingRoom",
	"kitchen",
	"bedroom",
	"bedroomSingle",
	"bedroomDouble",
	"bedroomKids",
	"bathroom",
	"toilet",
	"office",
	"garage",
	"garden",
	"gardenShed",
	"basement",
	"attic",
	"hallway",
	"laundryRoom",
	"gameRoom",
	"diningRoom",
	"closet",
	"staircase",
	"balcony",
	"terrace",
	"pool",
	"gym",
	"sauna",
	"workshop",
	"storage",
	"groundFloor",
	"firstFloor",
	"secondFloor",
	"thirdFloor",
	"default",
}

var zonesCmd = &cobra.Command{
	Use:   "zones",
	Short: "Manage zones",
	Long:  `List, view, create, and manage Homey zones.`,
}

// findZone finds a zone by name or ID from the list of all zones
func findZone(nameOrID string) (*Zone, error) {
	data, err := apiClient.GetZones()
	if err != nil {
		return nil, err
	}

	var zones map[string]Zone
	if err := json.Unmarshal(data, &zones); err != nil {
		return nil, fmt.Errorf("failed to parse zones: %w", err)
	}

	for _, z := range zones {
		if z.ID == nameOrID || strings.EqualFold(z.Name, nameOrID) {
			return &z, nil
		}
	}

	return nil, fmt.Errorf("zone not found: %s", nameOrID)
}

var zonesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all zones",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := apiClient.GetZones()
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var zones map[string]Zone
		if err := json.Unmarshal(data, &zones); err != nil {
			return fmt.Errorf("failed to parse zones: %w", err)
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Name", "Icon", "ID")
		tbl.WithHeaderFormatter(headerFmt)
		for _, z := range zones {
			tbl.AddRow(z.Name, z.Icon, z.ID)
		}
		tbl.Print()
		return nil
	},
}

var zonesGetCmd = &cobra.Command{
	Use:   "get <zone>",
	Short: "Get zone details",
	Long: `Get detailed information about a specific zone.

Examples:
  homeyctl zones get "Living Room"
  homeyctl zones get abc123-zone-id`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		zone, err := findZone(args[0])
		if err != nil {
			return err
		}

		// Get full zone data from API
		data, err := apiClient.GetZone(zone.ID)
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var z struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Icon   string `json:"icon"`
			Parent string `json:"parent"`
			Active bool   `json:"active"`
		}
		if err := json.Unmarshal(data, &z); err != nil {
			return fmt.Errorf("failed to parse zone: %w", err)
		}

		color.New(color.Bold).Println(z.Name)
		fmt.Printf("  Icon:   %s\n", z.Icon)
		fmt.Printf("  ID:     %s\n", z.ID)
		fmt.Printf("  Parent: %s\n", z.Parent)
		fmt.Printf("  Active: %v\n", z.Active)
		return nil
	},
}

var zonesIconsCmd = &cobra.Command{
	Use:   "icons",
	Short: "List available zone icons",
	Long: `List all known zone icons that can be used with the --icon flag.

Note: This list may not be exhaustive. Homey may support additional icons.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isJSON() {
			jsonData, _ := json.MarshalIndent(KnownZoneIcons, "", "  ")
			fmt.Println(string(jsonData))
			return nil
		}

		color.New(color.Bold).Println("Available zone icons:")
		fmt.Println()
		for _, icon := range KnownZoneIcons {
			fmt.Printf("  %s\n", icon)
		}
		fmt.Println()
		fmt.Println("Use: homeyctl zones rename <zone> <new-name> --icon <icon>")
		fmt.Println("Or:  homeyctl zones set-icon <zone> <icon>")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(zonesCmd)
	zonesCmd.AddCommand(zonesListCmd)
	zonesCmd.AddCommand(zonesGetCmd)
	zonesCmd.AddCommand(zonesIconsCmd)
}
