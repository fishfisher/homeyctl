package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var devicesRenameCmd = &cobra.Command{
	Use:   "rename <name-or-id> <new-name>",
	Short: "Rename a device",
	Long: `Rename a device.

Examples:
  homeyctl devices rename "Old Name" "New Name"
  homeyctl devices rename abc123-device-id "New Name"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]
		newName := args[1]

		device, err := findDevice(nameOrID)
		if err != nil {
			return err
		}

		updates := map[string]interface{}{
			"name": newName,
		}

		if err := apiClient.UpdateDevice(device.ID, updates); err != nil {
			return err
		}

		fmt.Printf("Renamed device '%s' to '%s'\n", device.Name, newName)
		return nil
	},
}

var devicesDeleteCmd = &cobra.Command{
	Use:   "delete <name-or-id>",
	Short: "Delete a device",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		device, err := findDevice(args[0])
		if err != nil {
			return err
		}

		if err := apiClient.DeleteDevice(device.ID); err != nil {
			return err
		}

		fmt.Printf("Deleted device: %s\n", device.Name)
		return nil
	},
}

func init() {
	devicesCmd.AddCommand(devicesRenameCmd)
	devicesCmd.AddCommand(devicesDeleteCmd)
}
