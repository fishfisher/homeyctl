package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var zoneRenameIcon string

var zonesRenameCmd = &cobra.Command{
	Use:   "rename <name-or-id> <new-name>",
	Short: "Rename a zone",
	Long: `Rename a zone, optionally changing its icon.

Use 'homeyctl zones icons' to see available icons.

Examples:
  homeyctl zones rename "Office" "Home Office"
  homeyctl zones rename "Office" "Aksels rom" --icon bedroomSingle
  homeyctl zones rename abc123-zone-id "New Name"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]
		newName := args[1]

		zone, err := findZone(nameOrID)
		if err != nil {
			return err
		}

		updates := map[string]interface{}{
			"name": newName,
		}

		if zoneRenameIcon != "" {
			updates["icon"] = zoneRenameIcon
		}

		if err := apiClient.UpdateZone(zone.ID, updates); err != nil {
			return err
		}

		if zoneRenameIcon != "" {
			fmt.Printf("Renamed zone '%s' to '%s' with icon '%s'\n", zone.Name, newName, zoneRenameIcon)
		} else {
			fmt.Printf("Renamed zone '%s' to '%s'\n", zone.Name, newName)
		}
		return nil
	},
}

var zonesSetIconCmd = &cobra.Command{
	Use:   "set-icon <name-or-id> <icon>",
	Short: "Set the icon for a zone",
	Long: `Set the icon for a zone without changing its name.

Use 'homeyctl zones icons' to see available icons.

Examples:
  homeyctl zones set-icon "Aksels rom" bedroomSingle
  homeyctl zones set-icon "Garden" garden`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]
		newIcon := args[1]

		zone, err := findZone(nameOrID)
		if err != nil {
			return err
		}

		updates := map[string]interface{}{
			"icon": newIcon,
		}

		if err := apiClient.UpdateZone(zone.ID, updates); err != nil {
			return err
		}

		fmt.Printf("Changed icon for zone '%s' from '%s' to '%s'\n", zone.Name, zone.Icon, newIcon)
		return nil
	},
}

var zonesDeleteCmd = &cobra.Command{
	Use:   "delete <name-or-id>",
	Short: "Delete a zone",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		zone, err := findZone(args[0])
		if err != nil {
			return err
		}

		if err := apiClient.DeleteZone(zone.ID); err != nil {
			return err
		}

		fmt.Printf("Deleted zone: %s\n", zone.Name)
		return nil
	},
}

func init() {
	zonesCmd.AddCommand(zonesRenameCmd)
	zonesRenameCmd.Flags().StringVar(&zoneRenameIcon, "icon", "", "Change the zone icon (use 'zones icons' to see available icons)")
	zonesCmd.AddCommand(zonesSetIconCmd)
	zonesCmd.AddCommand(zonesDeleteCmd)
}
