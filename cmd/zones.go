package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

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

		if isTableFormat() {
			var zones map[string]Zone
			if err := json.Unmarshal(data, &zones); err != nil {
				return fmt.Errorf("failed to parse zones: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tICON\tID")
			fmt.Fprintln(w, "----\t----\t--")
			for _, z := range zones {
				fmt.Fprintf(w, "%s\t%s\t%s\n", z.Name, z.Icon, z.ID)
			}
			w.Flush()
			return nil
		}

		outputJSON(data)
		return nil
	},
}

var zonesIconsCmd = &cobra.Command{
	Use:   "icons",
	Short: "List available zone icons",
	Long: `List all known zone icons that can be used with the --icon flag.

Note: This list may not be exhaustive. Homey may support additional icons.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isTableFormat() {
			fmt.Println("Available zone icons:")
			fmt.Println()
			for _, icon := range KnownZoneIcons {
				fmt.Printf("  %s\n", icon)
			}
			fmt.Println()
			fmt.Println("Use: homeyctl zones rename <zone> <new-name> --icon <icon>")
			fmt.Println("Or:  homeyctl zones set-icon <zone> <icon>")
			return nil
		}

		jsonData, _ := json.MarshalIndent(KnownZoneIcons, "", "  ")
		fmt.Println(string(jsonData))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(zonesCmd)
	zonesCmd.AddCommand(zonesListCmd)
	zonesCmd.AddCommand(zonesIconsCmd)
}
